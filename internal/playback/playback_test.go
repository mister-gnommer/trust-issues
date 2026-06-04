package playback

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/mister-gnommer/trust-issues/internal/spotify"
	"github.com/mister-gnommer/trust-issues/internal/store"
)

func newRec(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func newLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func playerState(trackID, trackName string, durationMS int64, ctxURI string, shuffle bool) *spotify.PlayerState {
	ps := &spotify.PlayerState{
		IsPlaying:    true,
		ShuffleState: shuffle,
		ProgressMS:   1000,
		Item: &spotify.Item{
			ID:         trackID,
			Name:       trackName,
			DurationMS: durationMS,
			Type:       "track",
			Artists:    []spotify.SimpleArtist{{ID: "a1", Name: "A"}},
		},
	}
	if ctxURI != "" {
		ps.Context = &spotify.Context{URI: ctxURI, Type: kindFromURI(ctxURI)}
	}
	return ps
}

func kindFromURI(uri string) string {
	k, _ := spotify.ParseContextURI(uri)
	switch k {
	case spotify.ContextPlaylist:
		return "playlist"
	case spotify.ContextCollection:
		return "collection"
	}
	return ""
}

// TestStep_idleNoPlaybackStaysIdle: when Spotify returns no active playback (nil),
// the state machine should remain in idle phase and continue polling at the slower idle interval.
func TestStep_idleNoPlaybackStaysIdle(t *testing.T) {
	rec := newRec(t)
	cfg := Config{}.withDefaults()
	state := newRunState(rec, Account{UserID: "u1", DisplayName: "Kenny"})
	now := time.Now().UTC()

	got := state.step(context.Background(), newLogger(), cfg, nil, now)
	if got != cfg.IdleInterval {
		t.Errorf("interval = %v, want %v", got, cfg.IdleInterval)
	}
	if state.phase != phaseIdle {
		t.Errorf("phase = %v, want idle", state.phase)
	}
}

func TestStep_playbackTransitionsActive_recordsPlay(t *testing.T) {
	rec := newRec(t)
	cfg := Config{}.withDefaults()
	state := newRunState(rec, Account{UserID: "u1", DisplayName: "Kenny"})
	if err := rec.UpsertUser(context.Background(), "u1", "Kenny", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)

	var prevID int64

	t.Run("first track detected", func(t *testing.T) {
		got := state.step(context.Background(), newLogger(), cfg,
			playerState("t1", "Track One", 200_000, "spotify:playlist:p1", true), now)
		if got != cfg.ActiveInterval {
			t.Errorf("interval = %v, want %v", got, cfg.ActiveInterval)
		}
		if state.lastPlay == nil || state.lastPlay.TrackID != "t1" {
			t.Fatalf("lastPlay = %+v", state.lastPlay)
		}
	})

	t.Run("same track again — nothing new recorded", func(t *testing.T) {
		prevID = state.lastPlay.ID
		state.step(context.Background(), newLogger(), cfg,
			playerState("t1", "Track One", 200_000, "spotify:playlist:p1", true), now.Add(5*time.Second))
		if state.lastPlay.ID != prevID {
			t.Errorf("lastPlay.ID changed for same track")
		}
		var playCount int
		if err := rec.DB().QueryRow(`SELECT COUNT(*) FROM plays WHERE track_id = ?`, "t1").Scan(&playCount); err != nil {
			t.Fatal(err)
		}
		if playCount != 1 {
			t.Errorf("play count for t1 = %d, want 1", playCount)
		}
	})

	t.Run("new track detected → previous closes, skipped flag computed", func(t *testing.T) {
		state.step(context.Background(), newLogger(), cfg,
			playerState("t2", "Track Two", 200_000, "spotify:playlist:p1", true), now.Add(10*time.Second))
		open, err := rec.LastOpenPlay(context.Background(), "u1")
		if err != nil || open == nil || open.TrackID != "t2" {
			t.Fatalf("open = %+v err=%v", open, err)
		}
		// Verify previous play was closed and marked skipped (10s < 30s threshold).
		var skipped *bool
		if err := rec.DB().QueryRow(`SELECT skipped FROM plays WHERE id = ?`, prevID).Scan(&skipped); err != nil {
			t.Fatal(err)
		}
		if skipped == nil || !*skipped {
			t.Errorf("prev skipped = %v, want true", skipped)
		}
	})

	// Long play (90s > 30s threshold) → previous play NOT skipped.
	t.Run("long play not skipped", func(t *testing.T) {
		prevID2 := state.lastPlay.ID
		state.step(context.Background(), newLogger(), cfg,
			playerState("t3", "Track Three", 200_000, "spotify:playlist:p1", true), now.Add(100*time.Second))
		var skipped *bool
		if err := rec.DB().QueryRow(`SELECT skipped FROM plays WHERE id = ?`, prevID2).Scan(&skipped); err != nil {
			t.Fatal(err)
		}
		if skipped == nil || *skipped {
			t.Errorf("skipped = %v, want false", skipped)
		}
	})
}

func TestStep_grace_thenIdle(t *testing.T) {
	rec := newRec(t)
	cfg := Config{}.withDefaults()
	state := newRunState(rec, Account{UserID: "u1", DisplayName: "Kenny"})
	rec.UpsertUser(context.Background(), "u1", "Kenny", time.Now().UTC())

	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	state.step(context.Background(), newLogger(), cfg,
		playerState("t1", "T", 200_000, "", true), now)

	// First no-playback tick starts the grace counter, still active.
	got := state.step(context.Background(), newLogger(), cfg, nil, now.Add(1*time.Second))
	if got != cfg.ActiveInterval || state.phase != phaseActive {
		t.Errorf("grace start: interval=%v phase=%v", got, state.phase)
	}

	// 59s after grace started (< IdleAfter) — still active.
	got = state.step(context.Background(), newLogger(), cfg, nil, now.Add(60*time.Second))
	if got != cfg.ActiveInterval || state.phase != phaseActive {
		t.Errorf("before threshold: interval=%v phase=%v", got, state.phase)
	}

	// 60s after grace started (>= IdleAfter) — switched to idle.
	got = state.step(context.Background(), newLogger(), cfg, nil, now.Add(61*time.Second))
	if got != cfg.IdleInterval || state.phase != phaseIdle {
		t.Errorf("at threshold: interval=%v phase=%v", got, state.phase)
	}
}

func TestResolveContext_collectionAndPlaylist(t *testing.T) {
	rec := newRec(t)
	state := newRunState(rec, Account{UserID: "u1"})
	ctx := context.Background()

	// Pre-seed a snapshot for playlist p1
	rec.InsertSnapshot(ctx, store.Snapshot{
		ID: "snap-p1", UserID: "u1", PlaylistID: "p1", PlaylistName: "P1", CapturedAt: time.Now().UTC(),
	}, nil)
	// And a Liked Songs snapshot
	likedID := LikedPlaylistID("u1")
	rec.InsertSnapshot(ctx, store.Snapshot{
		ID: "snap-liked", UserID: "u1", PlaylistID: likedID, PlaylistName: "Liked Songs", CapturedAt: time.Now().UTC(),
	}, nil)

	pid, sid, err := state.resolveContext(ctx, &spotify.Context{URI: "spotify:playlist:p1"})
	if err != nil {
		t.Fatal(err)
	}
	if pid == nil || *pid != "p1" || sid == nil || *sid != "snap-p1" {
		t.Errorf("playlist: pid=%v sid=%v", deref(pid), deref(sid))
	}

	pid, sid, err = state.resolveContext(ctx, &spotify.Context{URI: "spotify:user:someone:collection"})
	if err != nil {
		t.Fatal(err)
	}
	if pid == nil || *pid != likedID || sid == nil || *sid != "snap-liked" {
		t.Errorf("collection: pid=%v sid=%v", deref(pid), deref(sid))
	}

	pid, sid, err = state.resolveContext(ctx, &spotify.Context{URI: "spotify:album:al1"})
	if err != nil {
		t.Fatal(err)
	}
	if pid != nil || sid != nil {
		t.Errorf("album: pid=%v sid=%v, want nil/nil", deref(pid), deref(sid))
	}

	pid, sid, err = state.resolveContext(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if pid != nil || sid != nil {
		t.Errorf("nil ctx: pid=%v sid=%v", deref(pid), deref(sid))
	}
}

func deref(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

func TestComputeSkipped_thresholds(t *testing.T) {
	t0 := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name       string
		listenedMS int64
		durationMS int64
		want       bool
	}{
		{"short song, listened 29s — skipped (under 30s floor)", 29_000, 60_000, true},
		{"short song, listened 30s — kept (meets floor)", 30_000, 60_000, false},
		{"long song, listened 60s of 240s — kept (60>=60)", 60_000, 240_000, false},
		{"long song, listened 59s of 240s — skipped (59<60)", 59_000, 240_000, true},
		{"very long song, listened 60s of 13min — skipped (60<195)", 60_000, 780_000, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := computeSkipped(t0, t0.Add(time.Duration(tc.listenedMS)*time.Millisecond), tc.durationMS)
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
