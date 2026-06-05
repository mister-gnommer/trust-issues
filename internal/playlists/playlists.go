package playlists

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/mister-gnommer/trust-issues/internal/playback"
	"github.com/mister-gnommer/trust-issues/internal/spotify"
	"github.com/mister-gnommer/trust-issues/internal/store"
)

// Source is the subset of *spotify.Client used by the syncer.
type Source interface {
	ListPlaylists(ctx context.Context) ([]spotify.Playlist, error)
	ListPlaylistItems(ctx context.Context, playlistID string) ([]spotify.Track, error)
	ListLikedTracks(ctx context.Context) ([]spotify.Track, error)
}

// Recorder is the subset of *store.Store used by the syncer.
type Recorder interface {
	UpsertArtists(ctx context.Context, artists []store.Artist) error
	UpsertTracks(ctx context.Context, tracks []store.Track) error
	LatestSnapshotID(ctx context.Context, userID, playlistID string) (string, bool, error)
	InsertSnapshot(ctx context.Context, snap store.Snapshot, tracks []store.SnapshotTrack) error
}

type Account struct {
	UserID      string
	DisplayName string
}

type Config struct {
	Interval time.Duration // default 1h
}

func (c Config) withDefaults() Config {
	if c.Interval == 0 {
		c.Interval = time.Hour
	}
	return c
}

// Run performs an immediate sync, then re-syncs on cfg.Interval until ctx is canceled.
func Run(ctx context.Context, log *slog.Logger, cfg Config, account Account, src Source, rec Recorder) error {
	cfg = cfg.withDefaults()
	log = log.With("user_id", account.UserID, "component", "playlists")

	tick := func() error {
		if err := SyncOnce(ctx, log, account, src, rec); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			log.Warn("sync failed", "err", err)
		}
		return nil
	}

	if err := tick(); err != nil {
		return err
	}
	t := time.NewTicker(cfg.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			if err := tick(); err != nil {
				return err
			}
		}
	}
}

// SyncOnce runs a single full pass: regular playlists + Liked Songs.
func SyncOnce(ctx context.Context, log *slog.Logger, account Account, src Source, rec Recorder) error {
	if err := syncPlaylists(ctx, log, account, src, rec); err != nil {
		return err
	}
	if err := syncLiked(ctx, log, account, src, rec); err != nil {
		return err
	}
	return nil
}

func syncPlaylists(ctx context.Context, log *slog.Logger, account Account, src Source, rec Recorder) error {
	pls, err := src.ListPlaylists(ctx)
	if err != nil {
		return fmt.Errorf("list playlists: %w", err)
	}
	for _, p := range pls {
		stored, ok, err := rec.LatestSnapshotID(ctx, account.UserID, p.ID)
		if err != nil {
			log.Warn("LatestSnapshotID failed", "playlist_id", p.ID, "err", err)
			continue
		}
		if ok && stored == p.SnapshotID {
			continue // unchanged
		}
		tracks, err := src.ListPlaylistItems(ctx, p.ID)
		if err != nil {
			log.Warn("list items failed", "playlist_id", p.ID, "err", err)
			continue
		}
		persisted, err := storeTracksAndSnapshot(ctx, rec, tracks, p.SnapshotID, account.UserID, p.ID, p.Name)
		if err != nil {
			log.Warn("playlist sync failed", "playlist_id", p.ID, "err", err)
		} else {
			log.Info("playlist snapshot stored",
				"playlist_id", p.ID,
				"snapshot_id", p.SnapshotID,
				"tracks_persisted", persisted,
				"tracks_api_total", p.Tracks.Total)
		}
	}
	return nil
}

func syncLiked(ctx context.Context, log *slog.Logger, account Account, src Source, rec Recorder) error {
	tracks, err := src.ListLikedTracks(ctx)
	if err != nil {
		return fmt.Errorf("list liked: %w", err)
	}
	syntheticID := likedSnapshotID(tracks)
	likedPlaylistID := playback.LikedPlaylistID(account.UserID)

	stored, ok, err := rec.LatestSnapshotID(ctx, account.UserID, likedPlaylistID)
	if err != nil {
		return err
	}
	if ok && stored == syntheticID {
		return nil
	}
	n, err := storeTracksAndSnapshot(ctx, rec, tracks, syntheticID, account.UserID, likedPlaylistID, "Liked Songs")
	if err != nil {
		return err
	}
	log.Info("liked songs snapshot stored", "snapshot_id", syntheticID, "tracks", n)
	return nil
}

func storeTracksAndSnapshot(ctx context.Context, rec Recorder, tracks []spotify.Track, snapshotID, userID, playlistID, playlistName string) (int, error) {
	if err := persistTracksAndArtists(ctx, rec, tracks); err != nil {
		return 0, err
	}
	snapTracks := make([]store.SnapshotTrack, 0, len(tracks))
	for i, t := range tracks {
		snapTracks = append(snapTracks, store.SnapshotTrack{TrackID: t.ID, Position: i})
	}
	snap := store.Snapshot{
		ID:           snapshotID,
		UserID:       userID,
		PlaylistID:   playlistID,
		PlaylistName: playlistName,
		CapturedAt:   time.Now().UTC(),
	}
	if err := rec.InsertSnapshot(ctx, snap, snapTracks); err != nil {
		return 0, err
	}
	return len(snapTracks), nil
}

// persistTracksAndArtists upserts every track and artist from the given list.
// This re-updates unchanged rows — deliberate: the snapshot ID check in callers
// already skips playlists that haven't changed, and bulk-upserting in a single
// transaction is faster than per-row existence checks for typical playlist sizes.
func persistTracksAndArtists(ctx context.Context, rec Recorder, tracks []spotify.Track) error {
	artistByID := make(map[string]store.Artist)
	storeTracks := make([]store.Track, 0, len(tracks))
	for _, t := range tracks {
		artistIDs := make([]string, 0, len(t.Artists))
		for _, a := range t.Artists {
			if a.ID == "" {
				continue
			}
			artistByID[a.ID] = store.Artist{ID: a.ID, Name: a.Name}
			artistIDs = append(artistIDs, a.ID)
		}
		storeTracks = append(storeTracks, store.Track{
			ID:         t.ID,
			Name:       t.Name,
			DurationMS: t.DurationMS,
			ArtistIDs:  artistIDs,
		})
	}
	artists := make([]store.Artist, 0, len(artistByID))
	for _, a := range artistByID {
		artists = append(artists, a)
	}
	if err := rec.UpsertArtists(ctx, artists); err != nil {
		return fmt.Errorf("upsert artists: %w", err)
	}
	if err := rec.UpsertTracks(ctx, storeTracks); err != nil {
		return fmt.Errorf("upsert tracks: %w", err)
	}
	return nil
}

// likedSnapshotID is sha256(sorted_track_ids), first 16 hex chars, prefixed.
// Liked Songs has no snapshot_id from the API, so this hash is our change detector.
func likedSnapshotID(tracks []spotify.Track) string {
	ids := make([]string, 0, len(tracks))
	for _, t := range tracks {
		if t.ID == "" {
			continue
		}
		ids = append(ids, t.ID)
	}
	sort.Strings(ids)
	h := sha256.New()
	for _, id := range ids {
		h.Write([]byte(id))
		h.Write([]byte{0})
	}
	return "liked-" + hex.EncodeToString(h.Sum(nil))[:16]
}
