package store

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var schemaSQL string

const currentSchemaVersion = 1

type Store struct {
	db *sql.DB
}

// New opens (or creates) a SQLite database at dsn and applies the schema.
// dsn may be a file path (auto-created if missing) or ":memory:" for tests.
func New(ctx context.Context, dsn string) (*Store, error) {
	connStr := dsn
	if dsn != ":memory:" {
		connStr += "?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on"
	}
	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// DB exposes the underlying *sql.DB for ad-hoc queries (tests, debugging).
func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	var current int
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&current)
	if err != nil {
		return fmt.Errorf("read schema_version: %w", err)
	}
	if current >= currentSchemaVersion {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO schema_version(version) VALUES (?)`, currentSchemaVersion); err != nil {
		return fmt.Errorf("write schema_version: %w", err)
	}
	return nil
}

func (s *Store) UpsertUser(ctx context.Context, id, displayName string, addedAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users(id, display_name, added_at) VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET display_name = excluded.display_name
	`, id, displayName, addedAt)
	return err
}

func (s *Store) UpsertArtists(ctx context.Context, artists []Artist) error {
	if len(artists) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO artists(id, name) VALUES (?, ?)
		ON CONFLICT(id) DO UPDATE SET name = excluded.name
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, a := range artists {
		if _, err := stmt.ExecContext(ctx, a.ID, a.Name); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// UpsertTracks writes track rows AND their artist links (track_artists) in one tx.
// Artist rows must exist before calling — enforced by foreign key constraint.
func (s *Store) UpsertTracks(ctx context.Context, tracks []Track) error {
	if len(tracks) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	trackStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO tracks(id, name, duration_ms) VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name = excluded.name, duration_ms = excluded.duration_ms
	`)
	if err != nil {
		return err
	}
	defer trackStmt.Close()
	linkStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO track_artists(track_id, artist_id) VALUES (?, ?)
		ON CONFLICT DO NOTHING
	`)
	if err != nil {
		return err
	}
	defer linkStmt.Close()
	for _, t := range tracks {
		if _, err := trackStmt.ExecContext(ctx, t.ID, t.Name, t.DurationMS); err != nil {
			return err
		}
		for _, aid := range t.ArtistIDs {
			if _, err := linkStmt.ExecContext(ctx, t.ID, aid); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// LatestSnapshotID returns the snapshot_id stored most recently for (userID, playlistID).
// The bool is false if no snapshot exists yet.
func (s *Store) LatestSnapshotID(ctx context.Context, userID, playlistID string) (string, bool, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `
		SELECT id FROM playlist_snapshots
		WHERE user_id = ? AND playlist_id = ?
		ORDER BY captured_at DESC LIMIT 1
	`, userID, playlistID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}

// InsertSnapshot writes a playlist_snapshots row and all its tracks atomically.
// If the snapshot ID already exists this is a no-op (ON CONFLICT DO NOTHING on the parent).
func (s *Store) InsertSnapshot(ctx context.Context, snap Snapshot, tracks []SnapshotTrack) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `
		INSERT INTO playlist_snapshots(id, user_id, playlist_id, playlist_name, captured_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO NOTHING
	`, snap.ID, snap.UserID, snap.PlaylistID, snap.PlaylistName, snap.CapturedAt)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("insert snapshot rows affected: %w", err)
	}
	if rows == 0 {
		return tx.Commit()
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO playlist_snapshot_tracks(snapshot_id, track_id, position) VALUES (?, ?, ?)
		ON CONFLICT DO NOTHING
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, t := range tracks {
		if _, err := stmt.ExecContext(ctx, snap.ID, t.TrackID, t.Position); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// TrackDuration returns the duration_ms stored for a track, or (0, false, nil) if unknown.
func (s *Store) TrackDuration(ctx context.Context, trackID string) (int64, bool, error) {
	var d int64
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(duration_ms, 0) FROM tracks WHERE id = ?`, trackID).Scan(&d)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return d, true, nil
}

// LastOpenPlay returns the most recent play for userID where ended_at IS NULL.
// Used on track-change to close the previous play.
func (s *Store) LastOpenPlay(ctx context.Context, userID string) (*Play, error) {
	var p Play
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, track_id, playlist_id, playlist_snapshot_id,
		       shuffle_state, smart_shuffle, played_at, ended_at,
		       progress_ms_at_detection, skipped
		FROM plays
		WHERE user_id = ? AND ended_at IS NULL
		ORDER BY played_at DESC LIMIT 1
	`, userID).Scan(
		&p.ID, &p.UserID, &p.TrackID, &p.PlaylistID, &p.PlaylistSnapshotID,
		&p.ShuffleState, &p.SmartShuffle, &p.PlayedAt, &p.EndedAt,
		&p.ProgressMSAtDetection, &p.Skipped,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// CloseAndInsertPlay closes the previous play (sets ended_at + skipped) and inserts
// the new play row atomically.
//
// prev may be nil — that's the case on first-ever play or after a clean shutdown
// where the previous play was already closed.
//
// Skip threshold (per DESCRIPTION.md A1): a play is "skipped" if it ended in less
// than max(30s, 25% of duration_ms).
func (s *Store) CloseAndInsertPlay(ctx context.Context, prev *Play, prevDurationMS int64, prevEndedAt time.Time, next Play) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if prev != nil {
		skipped := computeSkipped(prev.PlayedAt, prevEndedAt, prevDurationMS)
		if _, err := tx.ExecContext(ctx, `
			UPDATE plays SET ended_at = ?, skipped = ?
			WHERE id = ?
		`, prevEndedAt, skipped, prev.ID); err != nil {
			return 0, err
		}
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO plays(
			user_id, track_id, playlist_id, playlist_snapshot_id,
			shuffle_state, smart_shuffle, played_at, ended_at,
			progress_ms_at_detection, skipped
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		next.UserID, next.TrackID, next.PlaylistID, next.PlaylistSnapshotID,
		next.ShuffleState, next.SmartShuffle, next.PlayedAt, next.EndedAt,
		next.ProgressMSAtDetection, next.Skipped,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

func computeSkipped(playedAt, endedAt time.Time, durationMS int64) bool {
	listenedMS := endedAt.Sub(playedAt).Milliseconds()
	threshold := durationMS / 4
	const floorMS int64 = 30_000
	if threshold < floorMS {
		threshold = floorMS
	}
	return listenedMS < threshold
}
