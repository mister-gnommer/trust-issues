package store

import (
	"context"
	"fmt"
	"sort"
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

// setupAnalysisData inserts test data for analysis-related store method tests.
// Returns the base reference time used for inserted timestamps.
//
// DB state after this returns:
//
//	users:    u1 (Alice), u2 (Bob)
//	artists:  a1, a2, a3
//	tracks:   t1 (a1), t2 (a1), t3 (a2), t4 (a1+a2 collab)
//
//	snapshots (user, playlist, track count):
//	  snap1  u1 p1 "Daily Mix"        4 tracks: t1,t2,t3,t4
//	  snap2  u1 p2 "Discover Weekly"  2 tracks: t1,t2
//	  snap3  u2 p1 "Daily Mix"        3 tracks: t1,t2,t3
//	  snap4  u1 p3 "No Plays"         1 track:  t1   (no shuffle plays → excluded)
//
//	plays (all shuffle+context unless noted; snap1/u1 unless noted):
//	  t1×3, t2×1, t4×2           → 6 shuffle plays with context
//	  t1×1 non-shuffle           → excluded (shuffle_state=0)
//	  t1×1 playlist_id=NULL      → excluded (no context)
//	  t5×1 (not in snap1)        → excluded (orphan, dropped by INNER JOIN)
//	  t1×1 snap2                 → excluded from snap1 counts (different snapshot)
//	  t1×1 u2/snap3              → excluded from u1 counts (different user)
//
//	Effective shuffle play counts per track (snap1, u1): t1=3, t2=1, t3=0, t4=2.
func setupAnalysisData(t *testing.T, s *Store, ctx context.Context) time.Time {
	t.Helper()
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)

	mustExec(t, s, ctx, `INSERT INTO users(id, display_name, added_at) VALUES ('u1', 'Alice', ?)`, now)
	mustExec(t, s, ctx, `INSERT INTO users(id, display_name, added_at) VALUES ('u2', 'Bob', ?)`, now)

	mustExec(t, s, ctx, `INSERT INTO artists(id, name) VALUES ('a1', 'Artist One')`)
	mustExec(t, s, ctx, `INSERT INTO artists(id, name) VALUES ('a2', 'Artist Two')`)
	mustExec(t, s, ctx, `INSERT INTO artists(id, name) VALUES ('a3', 'Artist Three')`)

	mustExec(t, s, ctx, `INSERT INTO tracks(id, name, duration_ms) VALUES ('t1', 'Track One', 200000)`)
	mustExec(t, s, ctx, `INSERT INTO tracks(id, name, duration_ms) VALUES ('t2', 'Track Two', 200000)`)
	mustExec(t, s, ctx, `INSERT INTO tracks(id, name, duration_ms) VALUES ('t3', 'Track Three', 200000)`)
	mustExec(t, s, ctx, `INSERT INTO tracks(id, name, duration_ms) VALUES ('t4', 'Collab Track', 200000)`)

	mustExec(t, s, ctx, `INSERT INTO track_artists(track_id, artist_id) VALUES ('t1', 'a1')`)
	mustExec(t, s, ctx, `INSERT INTO track_artists(track_id, artist_id) VALUES ('t2', 'a1')`)
	mustExec(t, s, ctx, `INSERT INTO track_artists(track_id, artist_id) VALUES ('t3', 'a2')`)
	mustExec(t, s, ctx, `INSERT INTO track_artists(track_id, artist_id) VALUES ('t4', 'a1')`)
	mustExec(t, s, ctx, `INSERT INTO track_artists(track_id, artist_id) VALUES ('t4', 'a2')`)

	mustExec(t, s, ctx, `INSERT INTO playlist_snapshots(id, user_id, playlist_id, playlist_name, captured_at) VALUES ('snap1', 'u1', 'p1', 'Daily Mix', ?)`, now)
	mustExec(t, s, ctx, `INSERT INTO playlist_snapshots(id, user_id, playlist_id, playlist_name, captured_at) VALUES ('snap2', 'u1', 'p2', 'Discover Weekly', ?)`, now.Add(1*time.Hour))
	mustExec(t, s, ctx, `INSERT INTO playlist_snapshots(id, user_id, playlist_id, playlist_name, captured_at) VALUES ('snap3', 'u2', 'p1', 'Daily Mix', ?)`, now.Add(2*time.Hour))
	mustExec(t, s, ctx, `INSERT INTO playlist_snapshots(id, user_id, playlist_id, playlist_name, captured_at) VALUES ('snap4', 'u1', 'p3', 'No Plays', ?)`, now.Add(3*time.Hour))

	mustExec(t, s, ctx, `INSERT INTO playlist_snapshot_tracks(snapshot_id, track_id, position) VALUES ('snap1', 't1', 0)`)
	mustExec(t, s, ctx, `INSERT INTO playlist_snapshot_tracks(snapshot_id, track_id, position) VALUES ('snap1', 't2', 1)`)
	mustExec(t, s, ctx, `INSERT INTO playlist_snapshot_tracks(snapshot_id, track_id, position) VALUES ('snap1', 't3', 2)`)
	mustExec(t, s, ctx, `INSERT INTO playlist_snapshot_tracks(snapshot_id, track_id, position) VALUES ('snap1', 't4', 3)`)
	mustExec(t, s, ctx, `INSERT INTO playlist_snapshot_tracks(snapshot_id, track_id, position) VALUES ('snap2', 't1', 0)`)
	mustExec(t, s, ctx, `INSERT INTO playlist_snapshot_tracks(snapshot_id, track_id, position) VALUES ('snap2', 't2', 1)`)
	mustExec(t, s, ctx, `INSERT INTO playlist_snapshot_tracks(snapshot_id, track_id, position) VALUES ('snap3', 't1', 0)`)
	mustExec(t, s, ctx, `INSERT INTO playlist_snapshot_tracks(snapshot_id, track_id, position) VALUES ('snap3', 't2', 1)`)
	mustExec(t, s, ctx, `INSERT INTO playlist_snapshot_tracks(snapshot_id, track_id, position) VALUES ('snap3', 't3', 2)`)
	mustExec(t, s, ctx, `INSERT INTO playlist_snapshot_tracks(snapshot_id, track_id, position) VALUES ('snap4', 't1', 0)`)

	playTime := now.Add(1 * time.Minute)
	insertPlay := func(userID, trackID, playlistID, snapshotID string, shuffle bool) {
		t.Helper()
		mustExec(t, s, ctx,
			`INSERT INTO plays(user_id, track_id, playlist_id, playlist_snapshot_id, shuffle_state, smart_shuffle, played_at, ended_at, progress_ms_at_detection, skipped) VALUES (?, ?, ?, ?, ?, 0, ?, ?, 0, 0)`,
			userID, trackID, playlistID, snapshotID, shuffle, playTime, playTime.Add(30*time.Second))
		playTime = playTime.Add(1 * time.Minute)
	}

	// u1, snap1: t1 played 3 times (shuffle, with context)
	for range 3 {
		insertPlay("u1", "t1", "p1", "snap1", true)
	}
	insertPlay("u1", "t2", "p1", "snap1", true) // t2 played 1 time
	insertPlay("u1", "t4", "p1", "snap1", true) // t4 played
	insertPlay("u1", "t4", "p1", "snap1", true) // t4 played again (2 total)

	// Non-shuffle play — excluded
	insertPlay("u1", "t1", "p1", "snap1", false)

	// NULL playlist_id (no context) — excluded
	mustExec(t, s, ctx,
		`INSERT INTO plays(user_id, track_id, playlist_id, playlist_snapshot_id, shuffle_state, smart_shuffle, played_at, ended_at, progress_ms_at_detection, skipped) VALUES ('u1', 't1', NULL, 'snap1', 1, 0, ?, ?, 0, 0)`,
		playTime, playTime.Add(30*time.Second))
	playTime = playTime.Add(1 * time.Minute)

	// Orphan play (t5 not in snap1 tracks) — excluded by INNER JOIN
	mustExec(t, s, ctx,
		`INSERT INTO plays(user_id, track_id, playlist_id, playlist_snapshot_id, shuffle_state, smart_shuffle, played_at, ended_at, progress_ms_at_detection, skipped) VALUES ('u1', 't5_not_in_snap', 'p1', 'snap1', 1, 0, ?, ?, 0, 0)`,
		playTime, playTime.Add(30*time.Second))
	playTime = playTime.Add(1 * time.Minute)

	// Play for snap2 (different snapshot) — excluded from snap1 counts
	insertPlay("u1", "t1", "p2", "snap2", true)

	// Play for u2, snap3 (different user) — excluded from u1 counts
	insertPlay("u2", "t1", "p1", "snap3", true)

	return now
}

func mustExec(t *testing.T, s *Store, ctx context.Context, query string, args ...any) {
	t.Helper()
	if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
		t.Fatalf("exec: %v", err)
	}
}

func TestSnapshotsWithShufflePlays(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	setupAnalysisData(t, s, ctx)

	t.Run("u1 returns two matching snapshots", func(t *testing.T) {
		infos, err := s.SnapshotsWithShufflePlays(ctx, "u1")
		if err != nil {
			t.Fatal(err)
		}
		if len(infos) != 2 {
			t.Fatalf("got %d snapshots, want 2", len(infos))
		}
		ids := make([]string, len(infos))
		for i, info := range infos {
			ids[i] = info.ID
		}
		sort.Strings(ids)
		if ids[0] != "snap1" || ids[1] != "snap2" {
			t.Errorf("got ids %v, want [snap1 snap2]", ids)
		}
		for _, info := range infos {
			if info.UserID != "u1" {
				t.Errorf("info.UserID = %q, want u1", info.UserID)
			}
		}
	})

	t.Run("u2 returns snap3", func(t *testing.T) {
		infos, err := s.SnapshotsWithShufflePlays(ctx, "u2")
		if err != nil {
			t.Fatal(err)
		}
		if len(infos) != 1 || infos[0].ID != "snap3" {
			t.Errorf("got %v, want [snap3]", infos)
		}
	})

	t.Run("snap4 (no plays) is not returned", func(t *testing.T) {
		infos, err := s.SnapshotsWithShufflePlays(ctx, "u1")
		if err != nil {
			t.Fatal(err)
		}
		for _, info := range infos {
			if info.ID == "snap4" {
				t.Error("snap4 should not be returned (no shuffle plays)")
			}
		}
	})

	t.Run("unknown user returns empty slice", func(t *testing.T) {
		infos, err := s.SnapshotsWithShufflePlays(ctx, "nonexistent")
		if err != nil {
			t.Fatal(err)
		}
		if len(infos) != 0 {
			t.Errorf("got %d snapshots, want 0", len(infos))
		}
	})
}

func TestSnapshotTrackIDs(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	setupAnalysisData(t, s, ctx)

	t.Run("snap1 returns 4 tracks in position order", func(t *testing.T) {
		ids, n, err := s.SnapshotTrackIDs(ctx, "snap1")
		if err != nil {
			t.Fatal(err)
		}
		if n != 4 {
			t.Errorf("n = %d, want 4", n)
		}
		want := []string{"t1", "t2", "t3", "t4"}
		if len(ids) != len(want) {
			t.Fatalf("got %d ids, want %d", len(ids), len(want))
		}
		for i := range ids {
			if ids[i] != want[i] {
				t.Errorf("ids[%d] = %q, want %q", i, ids[i], want[i])
			}
		}
	})

	t.Run("snap2 returns 2 tracks", func(t *testing.T) {
		ids, n, err := s.SnapshotTrackIDs(ctx, "snap2")
		if err != nil {
			t.Fatal(err)
		}
		if n != 2 {
			t.Errorf("n = %d, want 2", n)
		}
		if len(ids) != 2 || ids[0] != "t1" || ids[1] != "t2" {
			t.Errorf("got %v, want [t1 t2]", ids)
		}
	})

	t.Run("unknown snapshot returns empty", func(t *testing.T) {
		ids, n, err := s.SnapshotTrackIDs(ctx, "nonexistent")
		if err != nil {
			t.Fatal(err)
		}
		if n != 0 || len(ids) != 0 {
			t.Errorf("got n=%d ids=%v, want 0 []", n, ids)
		}
	})
}

func TestArtistTrackCounts(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	setupAnalysisData(t, s, ctx)

	t.Run("snap1: a1 has 3 tracks, a2 has 2 tracks", func(t *testing.T) {
		counts, err := s.ArtistTrackCounts(ctx, "snap1")
		if err != nil {
			t.Fatal(err)
		}
		if counts["a1"] != 3 {
			t.Errorf("a1 count = %d, want 3", counts["a1"])
		}
		if counts["a2"] != 2 {
			t.Errorf("a2 count = %d, want 2", counts["a2"])
		}
	})

	t.Run("snap2: a1 has 2 tracks, a2 has 0", func(t *testing.T) {
		counts, err := s.ArtistTrackCounts(ctx, "snap2")
		if err != nil {
			t.Fatal(err)
		}
		if counts["a1"] != 2 {
			t.Errorf("a1 count = %d, want 2", counts["a1"])
		}
		if _, ok := counts["a2"]; ok {
			t.Error("a2 should not be present in snap2")
		}
	})

	t.Run("unknown snapshot returns empty map", func(t *testing.T) {
		counts, err := s.ArtistTrackCounts(ctx, "nonexistent")
		if err != nil {
			t.Fatal(err)
		}
		if len(counts) != 0 {
			t.Errorf("got %d artists, want 0", len(counts))
		}
	})
}

func TestPlayCountsByTrack(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	setupAnalysisData(t, s, ctx)

	t.Run("snap1 u1 has correct counts", func(t *testing.T) {
		counts, err := s.PlayCountsByTrack(ctx, "u1", "snap1")
		if err != nil {
			t.Fatal(err)
		}
		if counts["t1"] != 3 {
			t.Errorf("t1 count = %d, want 3", counts["t1"])
		}
		if counts["t2"] != 1 {
			t.Errorf("t2 count = %d, want 1", counts["t2"])
		}
		if counts["t4"] != 2 {
			t.Errorf("t4 count = %d, want 2", counts["t4"])
		}
	})

	t.Run("t3 has 0 plays in snap1 (not in map)", func(t *testing.T) {
		counts, err := s.PlayCountsByTrack(ctx, "u1", "snap1")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := counts["t3"]; ok {
			t.Error("t3 should not be in result (0 plays)")
		}
	})

	t.Run("non-shuffle play excluded", func(t *testing.T) {
		counts, err := s.PlayCountsByTrack(ctx, "u1", "snap1")
		if err != nil {
			t.Fatal(err)
		}
		// t1 should still be 3 — the non-shuffle play is excluded
		if counts["t1"] != 3 {
			t.Errorf("t1 count = %d, want 3 (non-shuffle excluded)", counts["t1"])
		}
	})

	t.Run("NULL playlist_id excluded", func(t *testing.T) {
		counts, err := s.PlayCountsByTrack(ctx, "u1", "snap1")
		if err != nil {
			t.Fatal(err)
		}
		if counts["t1"] != 3 {
			t.Errorf("t1 count = %d, want 3 (NULL playlist_id excluded)", counts["t1"])
		}
	})

	t.Run("orphan play excluded", func(t *testing.T) {
		counts, err := s.PlayCountsByTrack(ctx, "u1", "snap1")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := counts["t5_not_in_snap"]; ok {
			t.Error("orphan play should be excluded")
		}
	})

	t.Run("different snapshot excluded", func(t *testing.T) {
		counts, err := s.PlayCountsByTrack(ctx, "u1", "snap1")
		if err != nil {
			t.Fatal(err)
		}
		// The t1 play for snap2 should not appear in snap1 counts
		if counts["t1"] != 3 {
			t.Errorf("t1 count = %d, want 3 (snap2 play excluded)", counts["t1"])
		}
	})

	t.Run("different user excluded", func(t *testing.T) {
		counts, err := s.PlayCountsByTrack(ctx, "u2", "snap3")
		if err != nil {
			t.Fatal(err)
		}
		if counts["t1"] != 1 {
			t.Errorf("u2 t1 count for snap3 = %d, want 1", counts["t1"])
		}
	})

	t.Run("snap2 u1 has correct counts", func(t *testing.T) {
		counts, err := s.PlayCountsByTrack(ctx, "u1", "snap2")
		if err != nil {
			t.Fatal(err)
		}
		if counts["t1"] != 1 {
			t.Errorf("snap2 t1 count = %d, want 1", counts["t1"])
		}
	})

	t.Run("no matching data returns empty map", func(t *testing.T) {
		counts, err := s.PlayCountsByTrack(ctx, "u1", "nonexistent")
		if err != nil {
			t.Fatal(err)
		}
		if len(counts) != 0 {
			t.Errorf("got %d tracks, want 0", len(counts))
		}
	})
}

func TestPlayCountsByArtist(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	setupAnalysisData(t, s, ctx)

	t.Run("snap1 u1: a1 has 6 plays, a2 has 2 plays", func(t *testing.T) {
		counts, err := s.PlayCountsByArtist(ctx, "u1", "snap1")
		if err != nil {
			t.Fatal(err)
		}
		// a1: t1(3) + t2(1) + t4(2) = 6
		if counts["a1"] != 6 {
			t.Errorf("a1 count = %d, want 6", counts["a1"])
		}
		// a2: t3(0, not in map) + t4(2) = 2
		if counts["a2"] != 2 {
			t.Errorf("a2 count = %d, want 2", counts["a2"])
		}
	})

	t.Run("a3 not in snap1", func(t *testing.T) {
		counts, err := s.PlayCountsByArtist(ctx, "u1", "snap1")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := counts["a3"]; ok {
			t.Error("a3 should not be present (no tracks in snap1)")
		}
	})

	t.Run("snap2 u1: a1 has 1 play", func(t *testing.T) {
		counts, err := s.PlayCountsByArtist(ctx, "u1", "snap2")
		if err != nil {
			t.Fatal(err)
		}
		// snap2 has t1(a1), t2(a1). Only t1 was played (once).
		if counts["a1"] != 1 {
			t.Errorf("a1 count = %d, want 1", counts["a1"])
		}
	})
}

func TestTrackNames(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	setupAnalysisData(t, s, ctx)

	t.Run("known IDs return correct names", func(t *testing.T) {
		names, err := s.TrackNames(ctx, []string{"t1", "t2", "t4"})
		if err != nil {
			t.Fatal(err)
		}
		if names["t1"] != "Track One" {
			t.Errorf("t1 = %q, want 'Track One'", names["t1"])
		}
		if names["t2"] != "Track Two" {
			t.Errorf("t2 = %q, want 'Track Two'", names["t2"])
		}
		if names["t4"] != "Collab Track" {
			t.Errorf("t4 = %q, want 'Collab Track'", names["t4"])
		}
	})

	t.Run("unknown IDs are omitted", func(t *testing.T) {
		names, err := s.TrackNames(ctx, []string{"t1", "nonexistent"})
		if err != nil {
			t.Fatal(err)
		}
		if names["t1"] != "Track One" {
			t.Errorf("t1 = %q, want 'Track One'", names["t1"])
		}
		if _, ok := names["nonexistent"]; ok {
			t.Error("nonexistent track should not be in result")
		}
	})

	t.Run("empty input returns empty map", func(t *testing.T) {
		names, err := s.TrackNames(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(names) != 0 {
			t.Errorf("got %d names, want 0", len(names))
		}
		names, err = s.TrackNames(ctx, []string{})
		if err != nil {
			t.Fatal(err)
		}
		if len(names) != 0 {
			t.Errorf("got %d names, want 0", len(names))
		}
	})

	t.Run("handles >900 track IDs (multi-batch)", func(t *testing.T) {
		n := 1000
		for i := range n {
			mustExec(t, s, ctx, `INSERT INTO tracks(id, name, duration_ms) VALUES (?, ?, 200000)`,
				fmt.Sprintf("big_t%d", i), fmt.Sprintf("Big Track %d", i))
		}
		ids := make([]string, n)
		for i := range n {
			ids[i] = fmt.Sprintf("big_t%d", i)
		}
		names, err := s.TrackNames(ctx, ids)
		if err != nil {
			t.Fatal(err)
		}
		if len(names) != n {
			t.Errorf("got %d names, want %d", len(names), n)
		}
		for i := range n {
			id := fmt.Sprintf("big_t%d", i)
			want := fmt.Sprintf("Big Track %d", i)
			if names[id] != want {
				t.Errorf("names[%q] = %q, want %q", id, names[id], want)
			}
		}
	})
}

func TestArtistNames(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	setupAnalysisData(t, s, ctx)

	t.Run("known IDs return correct names", func(t *testing.T) {
		names, err := s.ArtistNames(ctx, []string{"a1", "a2"})
		if err != nil {
			t.Fatal(err)
		}
		if names["a1"] != "Artist One" {
			t.Errorf("a1 = %q, want 'Artist One'", names["a1"])
		}
		if names["a2"] != "Artist Two" {
			t.Errorf("a2 = %q, want 'Artist Two'", names["a2"])
		}
	})

	t.Run("unknown IDs are omitted", func(t *testing.T) {
		names, err := s.ArtistNames(ctx, []string{"a1", "nonexistent"})
		if err != nil {
			t.Fatal(err)
		}
		if names["a1"] != "Artist One" {
			t.Errorf("a1 = %q, want 'Artist One'", names["a1"])
		}
		if _, ok := names["nonexistent"]; ok {
			t.Error("nonexistent artist should not be in result")
		}
	})

	t.Run("empty input returns empty map", func(t *testing.T) {
		names, err := s.ArtistNames(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(names) != 0 {
			t.Errorf("got %d names, want 0", len(names))
		}
	})
}
