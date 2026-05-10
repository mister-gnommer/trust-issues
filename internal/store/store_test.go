// 🤖 AI-generated
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
	if err := s.UpsertUser(ctx, "u1", "Kris", now); err != nil {
		t.Fatal(err)
	}
	// Update display name
	if err := s.UpsertUser(ctx, "u1", "Krzysztof", now); err != nil {
		t.Fatal(err)
	}
	var name string
	if err := s.db.QueryRow(`SELECT display_name FROM users WHERE id = ?`, "u1").Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "Krzysztof" {
		t.Errorf("display_name = %q, want Krzysztof", name)
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

func TestPlayLifecycle_skipped(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	t0 := time.Now().UTC().Truncate(time.Second)

	// First-ever play: no prev
	first := Play{
		UserID:                "u1",
		TrackID:               "t1",
		PlaylistID:            ptr("p1"),
		PlaylistSnapshotID:    ptr("snap-1"),
		ShuffleState:          true,
		SmartShuffle:          false,
		PlayedAt:              t0,
		ProgressMSAtDetection: 0,
	}
	id1, err := s.CloseAndInsertPlay(ctx, nil, 0, time.Time{}, first)
	if err != nil {
		t.Fatal(err)
	}

	// Open play exists
	open, err := s.LastOpenPlay(ctx, "u1")
	if err != nil || open == nil || open.ID != id1 {
		t.Fatalf("open = %+v err=%v", open, err)
	}

	// Skip after 10s on a 240s track → skipped (10s < max(30s, 60s) = 60s)
	t1 := t0.Add(10 * time.Second)
	second := Play{
		UserID: "u1", TrackID: "t2", PlaylistID: ptr("p1"), PlaylistSnapshotID: ptr("snap-1"),
		ShuffleState: true, PlayedAt: t1,
	}
	id2, err := s.CloseAndInsertPlay(ctx, open, 240_000, t1, second)
	if err != nil {
		t.Fatal(err)
	}
	var skipped *bool
	var endedAt *time.Time
	if err := s.db.QueryRow(`SELECT ended_at, skipped FROM plays WHERE id = ?`, id1).Scan(&endedAt, &skipped); err != nil {
		t.Fatal(err)
	}
	if skipped == nil || !*skipped {
		t.Errorf("first play skipped = %v, want true", skipped)
	}

	// Listen for 90s on a 60s track → not skipped (90s >= max(30s, 15s) = 30s)
	open2, _ := s.LastOpenPlay(ctx, "u1")
	if open2.ID != id2 {
		t.Fatalf("open2.ID = %d want %d", open2.ID, id2)
	}
	t2 := t1.Add(90 * time.Second)
	third := Play{UserID: "u1", TrackID: "t3", ShuffleState: true, PlayedAt: t2}
	if _, err := s.CloseAndInsertPlay(ctx, open2, 60_000, t2, third); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`SELECT skipped FROM plays WHERE id = ?`, id2).Scan(&skipped); err != nil {
		t.Fatal(err)
	}
	if skipped == nil || *skipped {
		t.Errorf("second play skipped = %v, want false", skipped)
	}

	// 30s threshold floor: short track (60s), 25s listened → skipped (25 < 30)
	open3, _ := s.LastOpenPlay(ctx, "u1")
	t3 := t2.Add(25 * time.Second)
	fourth := Play{UserID: "u1", TrackID: "t4", ShuffleState: true, PlayedAt: t3}
	if _, err := s.CloseAndInsertPlay(ctx, open3, 60_000, t3, fourth); err != nil {
		t.Fatal(err)
	}
	var thirdSkipped *bool
	if err := s.db.QueryRow(`SELECT skipped FROM plays WHERE id = ?`, open3.ID).Scan(&thirdSkipped); err != nil {
		t.Fatal(err)
	}
	if thirdSkipped == nil || !*thirdSkipped {
		t.Errorf("third play skipped = %v, want true", thirdSkipped)
	}
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
