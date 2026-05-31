package playback

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/mister-gnommer/trust-issues/internal/spotify"
	"github.com/mister-gnommer/trust-issues/internal/store"
)

// Source is the subset of *spotify.Client used by the poller. Defined here
// so tests can plug in fakes without spinning up an httptest server.
type Source interface {
	GetPlayerState(ctx context.Context) (*spotify.PlayerState, error)
}

// Recorder is the subset of *store.Store used by the poller.
type Recorder interface {
	UpsertUser(ctx context.Context, id, displayName string, addedAt time.Time) error
	UpsertArtists(ctx context.Context, artists []store.Artist) error
	UpsertTracks(ctx context.Context, tracks []store.Track) error
	LatestSnapshotID(ctx context.Context, userID, playlistID string) (string, bool, error)
	LastOpenPlay(ctx context.Context, userID string) (*store.Play, error)
	TrackDuration(ctx context.Context, trackID string) (int64, bool, error)
	CloseAndInsertPlay(ctx context.Context, prev *store.Play, prevDurationMS int64, prevEndedAt time.Time, next store.Play) (int64, error)
}

// Account identifies which Spotify account this poller belongs to.
type Account struct {
	UserID      string
	DisplayName string
}

// Config holds the polling intervals. Zero values fall back to spec defaults.
type Config struct {
	ActiveInterval time.Duration // default 5s
	IdleInterval   time.Duration // default 30s
	IdleAfter      time.Duration // default 1 min of no playback before switching to idle
}

func (c Config) withDefaults() Config {
	if c.ActiveInterval == 0 {
		c.ActiveInterval = 5 * time.Second
	}
	if c.IdleInterval == 0 {
		c.IdleInterval = 30 * time.Second
	}
	if c.IdleAfter == 0 {
		c.IdleAfter = time.Minute
	}
	return c
}

// Run polls /me/player on the configured cadence and records every track change
// to the store. Returns when ctx is canceled. The error returned is whatever
// caused termination (ctx.Err() on clean shutdown).
func Run(ctx context.Context, log *slog.Logger, cfg Config, account Account, src Source, rec Recorder) error {
	cfg = cfg.withDefaults()
	log = log.With("user_id", account.UserID, "component", "playback")

	if err := rec.UpsertUser(ctx, account.UserID, account.DisplayName, time.Now().UTC()); err != nil {
		return fmt.Errorf("upsert user: %w", err)
	}

	state := newRunState(rec, account)
	if err := state.bootstrap(ctx); err != nil {
		log.Warn("bootstrap failed; continuing", "err", err)
	}

	interval := cfg.IdleInterval
	if state.lastPlay != nil {
		interval = cfg.ActiveInterval
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}

		next, err := src.GetPlayerState(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			log.Warn("GetPlayerState failed", "err", err)
			timer.Reset(interval)
			continue
		}

		interval = state.step(ctx, log, cfg, next, time.Now().UTC())
		timer.Reset(interval)
	}
}

type runState struct {
	rec      Recorder
	account  Account
	phase    phase
	noPlayAt time.Time // when we first saw no playback in the active phase
	// lastPlay tracks the currently-open plays row in the DB.
	lastPlay       *store.Play
	lastDurationMS int64
}

type phase int

const (
	phaseIdle phase = iota
	phaseActive
)

func newRunState(rec Recorder, account Account) *runState {
	return &runState{rec: rec, account: account, phase: phaseIdle}
}

// find last open play in store and put it as lastPlay state
func (s *runState) bootstrap(ctx context.Context) error {
	open, err := s.rec.LastOpenPlay(ctx, s.account.UserID)
	if err != nil {
		return err
	}
	if open == nil {
		return nil
	}
	dur, _, err := s.rec.TrackDuration(ctx, open.TrackID)
	if err != nil {
		// Leave state untouched; caller logs and continues.
		return err
	}
	// Edge case: if the service crashed mid-track and restarted after a long
	// gap, the play is stale. Close it immediately to avoid inflating duration.
	// (Requires: crash, restart, same track playing — yes, we know.)
	// Threshold: 1.5x track duration or 60s floor, whichever is larger.
	// Covers pause + crash scenario.
	now := time.Now().UTC()
	threshold := max(time.Duration(dur)*time.Millisecond*3/2, 60*time.Second)
	if now.Sub(open.PlayedAt) > threshold {
		return nil // stale play — treat as no open play
	}
	s.lastPlay = open
	s.lastDurationMS = dur
	s.phase = phaseActive
	return nil
}

// step processes one observation and returns the next polling interval.
// `now` is injected for testability.
func (s *runState) step(ctx context.Context, log *slog.Logger, cfg Config, ps *spotify.PlayerState, now time.Time) time.Duration {
	playing := ps != nil && ps.IsPlaying && ps.Item != nil

	if !playing {
		if s.phase == phaseActive {
			// no playback detected in active phase -> update noPlayAt or set to idle if time exceeds IdleAfter
			if s.noPlayAt.IsZero() {
				s.noPlayAt = now
			}
			if now.Sub(s.noPlayAt) >= cfg.IdleAfter {
				s.phase = phaseIdle
				s.noPlayAt = time.Time{}
			}
		}
		if s.phase == phaseIdle {
			return cfg.IdleInterval
		}
		return cfg.ActiveInterval
	}

	// Playback detected: clear the no-play counter and ensure we're in active phase.
	s.noPlayAt = time.Time{}
	s.phase = phaseActive

	currentID := ps.Item.ID
	if s.lastPlay != nil && s.lastPlay.TrackID == currentID {
		// Same track still playing — nothing to record.
		return cfg.ActiveInterval
	}

	if err := s.recordTrackChange(ctx, ps, now); err != nil {
		log.Warn("record track change failed", "err", err, "track_id", currentID)
	}
	return cfg.ActiveInterval
}

func (s *runState) recordTrackChange(ctx context.Context, ps *spotify.PlayerState, now time.Time) error {
	item := ps.Item

	// 1. Upsert artists referenced by the new track.
	artists := make([]store.Artist, 0, len(item.Artists))
	artistIDs := make([]string, 0, len(item.Artists))
	for _, a := range item.Artists {
		if a.ID == "" {
			continue
		}
		artists = append(artists, store.Artist{ID: a.ID, Name: a.Name})
		artistIDs = append(artistIDs, a.ID)
	}
	if err := s.rec.UpsertArtists(ctx, artists); err != nil {
		return fmt.Errorf("upsert artists: %w", err)
	}

	// 2. Upsert the track itself + track_artists links.
	if err := s.rec.UpsertTracks(ctx, []store.Track{{
		ID:         item.ID,
		Name:       item.Name,
		DurationMS: item.DurationMS,
		ArtistIDs:  artistIDs,
	}}); err != nil {
		return fmt.Errorf("upsert track: %w", err)
	}

	// 3. Resolve playlist context + current snapshot.
	playlistID, snapshotID, err := s.resolveContext(ctx, ps.Context)
	if err != nil {
		return fmt.Errorf("resolve context: %w", err)
	}

	// 4. Close prev play and insert the new one atomically.
	next := store.Play{
		UserID:                s.account.UserID,
		TrackID:               item.ID,
		PlaylistID:            playlistID,
		PlaylistSnapshotID:    snapshotID,
		ShuffleState:          ps.ShuffleState,
		SmartShuffle:          ps.SmartShuffle,
		PlayedAt:              now,
		ProgressMSAtDetection: ps.ProgressMS,
	}
	id, err := s.rec.CloseAndInsertPlay(ctx, s.lastPlay, s.lastDurationMS, now, next)
	if err != nil {
		return fmt.Errorf("close+insert play: %w", err)
	}
	next.ID = id
	s.lastPlay = &next
	s.lastDurationMS = item.DurationMS
	return nil
}

func (s *runState) resolveContext(ctx context.Context, c *spotify.Context) (playlistID, snapshotID *string, err error) {
	if c == nil {
		// No context — playing from search, queue, or direct URL.
		// Recorded with playlist_id=NULL, excluded from analysis.
		return nil, nil, nil
	}
	kind, id := spotify.ParseContextURI(c.URI)
	switch kind {
	case spotify.ContextPlaylist:
		pid := id
		snap, ok, err := s.rec.LatestSnapshotID(ctx, s.account.UserID, pid)
		if err != nil {
			return nil, nil, err
		}
		if !ok { // will happen if playlist wasn't yet synced
			return &pid, nil, nil
		}
		return &pid, &snap, nil
	case spotify.ContextCollection:
		// Liked Songs are recorded under a synthetic playlist_id keyed by user.
		liked := LikedPlaylistID(s.account.UserID)
		snap, ok, err := s.rec.LatestSnapshotID(ctx, s.account.UserID, liked)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			return &liked, nil, nil
		}
		return &liked, &snap, nil
	default:
		// album / artist / show / unknown — record the play but exclude from analysis.
		return nil, nil, nil
	}
}

// LikedPlaylistID is the synthetic playlist_id used for Liked Songs plays and
// snapshots. Exported because the playlists syncer also writes under this key.
func LikedPlaylistID(userID string) string { return "liked:" + userID }
