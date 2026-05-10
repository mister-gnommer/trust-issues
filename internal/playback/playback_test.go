// 🤖 AI-generated
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

func TestStep_idleNoPlaybackStaysIdle(t *testing.T) {
	rec := newRec(t)
	cfg := Config{}.withDefaults()
	state := newRunState(rec, Account{UserID: "u1", DisplayName: "Kris"})
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
	state := newRunState(rec, Account{UserID: "u1", DisplayName: "Kris"})
	if err := rec.UpsertUser(context.Background(), "u1", "Kris", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)

	// First track detected.
	got := state.step(context.Background(), newLogger(), cfg,
		playerState("t1", "Track One", 200_000, "spotify:playlist:p1", true), now)
	if got != cfg.ActiveInterval {
		t.Errorf("interval = %v, want %v", got, cfg.ActiveInterval)
	}
	if state.lastPlay == nil || state.lastPlay.TrackID != "t1" {
		t.Fatalf("lastPlay = %+v", state.lastPlay)
	}

	// Same track again — nothing new recorded.
	prevID := state.lastPlay.ID
	state.step(context.Background(), newLogger(), cfg,
		playerState("t1", "Track One", 200_000, "spotify:playlist:p1", true), now.Add(5*time.Second))
	if state.lastPlay.ID != prevID {
		t.Errorf("lastPlay.ID changed for same track")
	}

	// New track detected → previous closes, skipped flag computed.
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
}

func TestStep_grace_thenIdle(t *testing.T) {
	rec := newRec(t)
	cfg := Config{}.withDefaults()
	state := newRunState(rec, Account{UserID: "u1", DisplayName: "Kris"})
	rec.UpsertUser(context.Background(), "u1", "Kris", time.Now().UTC())

	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	state.step(context.Background(), newLogger(), cfg,
		playerState("t1", "T", 200_000, "", true), now)

	// First no-playback tick at +30s — starts the grace counter, still active.
	got := state.step(context.Background(), newLogger(), cfg, nil, now.Add(30*time.Second))
	if got != cfg.ActiveInterval || state.phase != phaseActive {
		t.Errorf("first idle tick: interval=%v phase=%v", got, state.phase)
	}

	// 30s into the grace window — still active.
	got = state.step(context.Background(), newLogger(), cfg, nil, now.Add(60*time.Second))
	if got != cfg.ActiveInterval || state.phase != phaseActive {
		t.Errorf("mid grace: interval=%v phase=%v", got, state.phase)
	}

	// 60s after first no-playback tick — switched to idle.
	got = state.step(context.Background(), newLogger(), cfg, nil, now.Add(90*time.Second))
	if got != cfg.IdleInterval || state.phase != phaseIdle {
		t.Errorf("after grace: interval=%v phase=%v", got, state.phase)
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
