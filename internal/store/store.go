package store

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"strings"
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
func (s *Store) CloseAndInsertPlay(ctx context.Context, prev *Play, prevEndedAt time.Time, skipped bool, next Play) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if prev != nil {
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

// SnapshotsWithShufflePlays returns snapshots that have at least one shuffle play
// with non-null context for the given user, ordered by captured_at desc.
func (s *Store) SnapshotsWithShufflePlays(ctx context.Context, userID string) ([]Snapshot, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ps.id, ps.user_id, ps.playlist_id, ps.playlist_name, ps.captured_at
		FROM playlist_snapshots ps
		WHERE ps.user_id = ?
		  AND EXISTS (
		      SELECT 1 FROM plays p
		      WHERE p.playlist_snapshot_id = ps.id
		        AND p.shuffle_state = 1
		        AND p.playlist_id IS NOT NULL
		  )
		ORDER BY ps.captured_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("snapshots with shuffle plays: %w", err)
	}
	defer rows.Close()

	var snaps []Snapshot
	for rows.Next() {
		var snap Snapshot
		if err := rows.Scan(&snap.ID, &snap.UserID, &snap.PlaylistID, &snap.PlaylistName, &snap.CapturedAt); err != nil {
			return nil, fmt.Errorf("scan snapshot: %w", err)
		}
		snaps = append(snaps, snap)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return snaps, nil
}

// SnapshotTrackIDs returns the track IDs in a snapshot, ordered by position.
// The second return value is the count (len(ids)).
func (s *Store) SnapshotTrackIDs(ctx context.Context, snapshotID string) ([]string, int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT track_id
		FROM playlist_snapshot_tracks
		WHERE snapshot_id = ?
		ORDER BY position
	`, snapshotID)
	if err != nil {
		return nil, 0, fmt.Errorf("snapshot track IDs: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, 0, fmt.Errorf("scan track ID: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration: %w", err)
	}
	return ids, len(ids), nil
}

// ArtistTrackCounts returns K (number of tracks per artist) for all artists
// in a snapshot. Collaborative tracks are counted once per artist.
func (s *Store) ArtistTrackCounts(ctx context.Context, snapshotID string) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ta.artist_id, COUNT(*) AS k
		FROM playlist_snapshot_tracks pst
		INNER JOIN track_artists ta ON ta.track_id = pst.track_id
		WHERE pst.snapshot_id = ?
		GROUP BY ta.artist_id
	`, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("artist track counts: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var artistID string
		var k int
		if err := rows.Scan(&artistID, &k); err != nil {
			return nil, fmt.Errorf("scan artist count: %w", err)
		}
		counts[artistID] = k
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return counts, nil
}

// PlayCountsByTrack returns observed shuffle play counts per track for a given
// user+snapshot. Orphan plays (track_id not in the snapshot) are dropped by
// the INNER JOIN against playlist_snapshot_tracks.
func (s *Store) PlayCountsByTrack(ctx context.Context, userID, snapshotID string) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.track_id, COUNT(*) AS cnt
		FROM plays p
		INNER JOIN playlist_snapshot_tracks pst
		    ON pst.track_id = p.track_id AND pst.snapshot_id = p.playlist_snapshot_id
		WHERE p.user_id = ?
		  AND p.playlist_snapshot_id = ?
		  AND p.shuffle_state = 1
		  AND p.playlist_id IS NOT NULL
		GROUP BY p.track_id
	`, userID, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("play counts by track: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var trackID string
		var n int
		if err := rows.Scan(&trackID, &n); err != nil {
			return nil, fmt.Errorf("scan play count: %w", err)
		}
		counts[trackID] = n
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return counts, nil
}

// PlayCountsByArtist returns observed shuffle play counts per artist for a given
// user+snapshot. Collaborative tracks contribute +1 to each artist.
// Orphan plays are dropped by the INNER JOIN against playlist_snapshot_tracks.
func (s *Store) PlayCountsByArtist(ctx context.Context, userID, snapshotID string) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ta.artist_id, COUNT(*) AS cnt
		FROM plays p
		INNER JOIN playlist_snapshot_tracks pst
		    ON pst.track_id = p.track_id AND pst.snapshot_id = p.playlist_snapshot_id
		INNER JOIN track_artists ta ON ta.track_id = pst.track_id
		WHERE p.user_id = ?
		  AND p.playlist_snapshot_id = ?
		  AND p.shuffle_state = 1
		  AND p.playlist_id IS NOT NULL
		GROUP BY ta.artist_id
	`, userID, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("play counts by artist: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var artistID string
		var n int
		if err := rows.Scan(&artistID, &n); err != nil {
			return nil, fmt.Errorf("scan artist count: %w", err)
		}
		counts[artistID] = n
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return counts, nil
}

const batchSize = 900

// TrackNames resolves a batch of track IDs to their display names.
// Batches queries in groups of batchSize to avoid SQLite's
// SQLITE_MAX_VARIABLE_NUMBER limit.
func (s *Store) TrackNames(ctx context.Context, trackIDs []string) (map[string]string, error) {
	if len(trackIDs) == 0 {
		return map[string]string{}, nil
	}
	names := make(map[string]string, len(trackIDs))
	for i := 0; i < len(trackIDs); i += batchSize {
		end := min(i+batchSize, len(trackIDs))
		batch := trackIDs[i:end]

		// convert signatures from string to any for the query:
		args := make([]any, len(batch))
		for j, id := range batch {
			args[j] = id
		}

		// generate query string with "?" placeholders for each argument in args/batch:
		query := "SELECT id, name FROM tracks WHERE id IN (" + strings.Repeat("?,", len(batch))[:len(batch)*2-1] + ")"

		if err := func() error {
			rows, err := s.db.QueryContext(ctx, query, args...)
			if err != nil {
				return fmt.Errorf("track names batch %d-%d: %w", i, end, err)
			}
			defer rows.Close()

			for rows.Next() {
				var id, name string
				if err := rows.Scan(&id, &name); err != nil {
					return fmt.Errorf("track names batch %d-%d scan: %w", i, end, err)
				}
				names[id] = name
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("track names batch %d-%d rows: %w", i, end, err)
			}
			return nil
		}(); err != nil {
			return nil, err
		}
	}
	return names, nil
}

// ArtistNames resolves a batch of artist IDs to their display names.
// Batches queries in groups of batchSize to avoid SQLite's
// SQLITE_MAX_VARIABLE_NUMBER limit.
func (s *Store) ArtistNames(ctx context.Context, artistIDs []string) (map[string]string, error) {
	if len(artistIDs) == 0 {
		return map[string]string{}, nil
	}
	names := make(map[string]string, len(artistIDs))
	for i := 0; i < len(artistIDs); i += batchSize {
		end := min(i+batchSize, len(artistIDs))
		batch := artistIDs[i:end]

		args := make([]any, len(batch))
		for j, id := range batch {
			args[j] = id
		}

		query := "SELECT id, name FROM artists WHERE id IN (" + strings.Repeat("?,", len(batch))[:len(batch)*2-1] + ")"

		if err := func() error {
			rows, err := s.db.QueryContext(ctx, query, args...)
			if err != nil {
				return fmt.Errorf("artist names batch %d-%d: %w", i, end, err)
			}
			defer rows.Close()

			for rows.Next() {
				var id, name string
				if err := rows.Scan(&id, &name); err != nil {
					return fmt.Errorf("artist names batch %d-%d scan: %w", i, end, err)
				}
				names[id] = name
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("artist names batch %d-%d rows: %w", i, end, err)
			}
			return nil
		}(); err != nil {
			return nil, err
		}
	}
	return names, nil
}
