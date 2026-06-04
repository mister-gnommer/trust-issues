package store

import (
	"context"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func ptr[T any](v T) *T { return &v }

func TestUpsertUser(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := s.UpsertUser(ctx, "u1", "Kenny", now); err != nil {
		t.Fatal(err)
	}
	// Update display name
	if err := s.UpsertUser(ctx, "u1", "RIP", now); err != nil {
		t.Fatal(err)
	}
	var name string
	if err := s.db.QueryRow(`SELECT display_name FROM users WHERE id = ?`, "u1").Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "RIP" {
		t.Errorf("display_name = %q, want RIP", name)
	}
}

func TestUpsertArtistsAndTracks(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.UpsertArtists(ctx, []Artist{{ID: "a1", Name: "Metallica"}, {ID: "a2", Name: "Iron Maiden"}}); err != nil {
		t.Fatal(err)
	}
	tracks := []Track{
		{ID: "t1", Name: "Master of Puppets", DurationMS: 515_000, ArtistIDs: []string{"a1"}},
		{ID: "t2", Name: "Trooper", DurationMS: 240_000, ArtistIDs: []string{"a2"}},
		{ID: "t3", Name: "Collab", DurationMS: 200_000, ArtistIDs: []string{"a1", "a2"}},
	}
	if err := s.UpsertTracks(ctx, tracks); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM track_artists WHERE track_id = 't3'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("collab links: got %d, want 2", n)
	}
	// Re-upsert is idempotent
	if err := s.UpsertTracks(ctx, tracks); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM track_artists WHERE track_id = 't3'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("after re-upsert: got %d, want 2", n)
	}
}

func TestSnapshots(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	// no snapshot should be found
	if id, ok, err := s.LatestSnapshotID(ctx, "u1", "p1"); err != nil || ok {
		t.Fatalf("expected no snapshot, got id=%q ok=%v err=%v", id, ok, err)
	}

	snap := Snapshot{ID: "snap-1", UserID: "u1", PlaylistID: "p1", PlaylistName: "Mix", CapturedAt: now}
	if err := s.InsertSnapshot(ctx, snap, []SnapshotTrack{
		{TrackID: "t1", Position: 0},
		{TrackID: "t2", Position: 1},
	}); err != nil {
		t.Fatal(err)
	}

	id, ok, err := s.LatestSnapshotID(ctx, "u1", "p1")
	if err != nil || !ok || id != "snap-1" {
		t.Fatalf("got id=%q ok=%v err=%v", id, ok, err)
	}

	// Newer snapshot wins
	snap2 := Snapshot{ID: "snap-2", UserID: "u1", PlaylistID: "p1", PlaylistName: "Mix", CapturedAt: now.Add(time.Hour)}
	if err := s.InsertSnapshot(ctx, snap2, []SnapshotTrack{{TrackID: "t1", Position: 0}}); err != nil {
		t.Fatal(err)
	}
	id, _, _ = s.LatestSnapshotID(ctx, "u1", "p1")
	if id != "snap-2" {
		t.Errorf("latest = %q, want snap-2", id)
	}

	// Re-inserting an existing snapshot is a no-op (no error, no track duplication)
	if err := s.InsertSnapshot(ctx, snap2, []SnapshotTrack{{TrackID: "t9", Position: 5}}); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM playlist_snapshot_tracks WHERE snapshot_id = 'snap-2'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("snapshot-2 tracks: got %d, want 1 (re-insert should be no-op)", n)
	}
}

func TestCloseAndInsertPlay(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	// First-ever play: no prev to close.
	newPlay := func(trackID string, playedAt time.Time) Play {
		return Play{UserID: "u1", TrackID: trackID, ShuffleState: true, PlayedAt: playedAt}
	}
	id1, err := s.CloseAndInsertPlay(ctx, nil, time.Time{}, false, newPlay("t1", now))
	if err != nil {
		t.Fatal(err)
	}

	// Close with skipped=true, insert next.
	open, _ := s.LastOpenPlay(ctx, "u1")
	id2, err := s.CloseAndInsertPlay(ctx, open, now.Add(10*time.Second), true, newPlay("t2", now.Add(10*time.Second)))
	if err != nil {
		t.Fatal(err)
	}
	var skipped *bool
	if err := s.db.QueryRow(`SELECT skipped FROM plays WHERE id = ?`, id1).Scan(&skipped); err != nil {
		t.Fatal(err)
	}
	if skipped == nil || !*skipped {
		t.Errorf("skipped = %v, want true", skipped)
	}
	if open, _ := s.LastOpenPlay(ctx, "u1"); open == nil || open.ID != id2 {
		t.Errorf("open = %+v, want id=%d", open, id2)
	}

	// Close with skipped=false, insert next.
	open, _ = s.LastOpenPlay(ctx, "u1")
	if _, err := s.CloseAndInsertPlay(ctx, open, now.Add(100*time.Second), false, newPlay("t3", now.Add(100*time.Second))); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`SELECT skipped FROM plays WHERE id = ?`, id2).Scan(&skipped); err != nil {
		t.Fatal(err)
	}
	if skipped == nil || *skipped {
		t.Errorf("skipped = %v, want false", skipped)
	}
}
