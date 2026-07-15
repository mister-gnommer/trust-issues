package analysis_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/mister-gnommer/trust-issues/v2/internal/analysis"
	"github.com/mister-gnommer/trust-issues/v2/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// setupAnalysisData inserts test data for chi-squared analysis tests.
// Returns the base reference time used for inserted timestamps.
//
// DB state after this returns:
//
//	users:    u1 (Alice), u2 (Bob)
//	artists:  a1, a2
//	tracks:   t1 (a1), t2 (a1), t3 (a2), t4 (a1+a2 collab)
//
//	snapshots (user, playlist, track count):
//	  snap1  u1 p1 "Daily Mix"        4 tracks: t1,t2,t3,t4
//	  snap2  u1 p2 "Discover Weekly"  2 tracks: t1,t2
//
//	plays (all shuffle, u1):
//	  t1×3, t2×1, t4×2  → snap1: 6 total shuffle plays
//	  t1×1               → snap2: 1 total shuffle play
//
// Effective shuffle play counts per track (snap1, u1): t1=3, t2=1, t3=0, t4=2.
func setupAnalysisData(t *testing.T, s *store.Store, ctx context.Context) time.Time {
	t.Helper()
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)

	mustExec(t, s, ctx, `INSERT INTO users(id, display_name, added_at) VALUES ('u1', 'Alice', ?)`, now)
	mustExec(t, s, ctx, `INSERT INTO users(id, display_name, added_at) VALUES ('u2', 'Bob', ?)`, now)

	mustExec(t, s, ctx, `INSERT INTO artists(id, name) VALUES ('a1', 'Artist One')`)
	mustExec(t, s, ctx, `INSERT INTO artists(id, name) VALUES ('a2', 'Artist Two')`)

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

	mustExec(t, s, ctx, `INSERT INTO playlist_snapshot_tracks(snapshot_id, track_id, position) VALUES ('snap1', 't1', 0)`)
	mustExec(t, s, ctx, `INSERT INTO playlist_snapshot_tracks(snapshot_id, track_id, position) VALUES ('snap1', 't2', 1)`)
	mustExec(t, s, ctx, `INSERT INTO playlist_snapshot_tracks(snapshot_id, track_id, position) VALUES ('snap1', 't3', 2)`)
	mustExec(t, s, ctx, `INSERT INTO playlist_snapshot_tracks(snapshot_id, track_id, position) VALUES ('snap1', 't4', 3)`)
	mustExec(t, s, ctx, `INSERT INTO playlist_snapshot_tracks(snapshot_id, track_id, position) VALUES ('snap2', 't1', 0)`)
	mustExec(t, s, ctx, `INSERT INTO playlist_snapshot_tracks(snapshot_id, track_id, position) VALUES ('snap2', 't2', 1)`)

	playTime := now.Add(1 * time.Minute)
	insertPlay := func(userID, trackID, playlistID, snapshotID string, shuffle bool) {
		t.Helper()
		mustExec(t, s, ctx,
			`INSERT INTO plays(user_id, track_id, playlist_id, playlist_snapshot_id, shuffle_state, smart_shuffle, played_at, ended_at, progress_ms_at_detection, skipped) VALUES (?, ?, ?, ?, ?, 0, ?, ?, 0, 0)`,
			userID, trackID, playlistID, snapshotID, shuffle, playTime, playTime.Add(30*time.Second))
		playTime = playTime.Add(1 * time.Minute)
	}

	// snap1: 4 tracks, 6 shuffle plays (t1×3, t2×1, t4×2)
	for range 3 {
		insertPlay("u1", "t1", "p1", "snap1", true)
	}
	insertPlay("u1", "t2", "p1", "snap1", true)
	insertPlay("u1", "t4", "p1", "snap1", true)
	insertPlay("u1", "t4", "p1", "snap1", true)

	// snap2: 2 tracks, 1 shuffle play (t1×1)
	insertPlay("u1", "t1", "p2", "snap2", true)

	return now
}

func mustExec(t *testing.T, s *store.Store, ctx context.Context, query string, args ...any) {
	t.Helper()
	if _, err := s.DB().ExecContext(ctx, query, args...); err != nil {
		t.Fatalf("exec: %v", err)
	}
}

func resultBySnapshot(results []analysis.Result, id string) *analysis.Result {
	for i := range results {
		if results[i].SnapshotID == id {
			return &results[i]
		}
	}
	return nil
}

func TestAnalyze_HappyPath(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	setupAnalysisData(t, s, ctx)

	results, err := analysis.Analyze(ctx, s, "u1", "Alice", 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	snap1 := resultBySnapshot(results, "snap1")
	if snap1 == nil {
		t.Fatal("snap1 not found in results")
	}
	snap2 := resultBySnapshot(results, "snap2")
	if snap2 == nil {
		t.Fatal("snap2 not found in results")
	}

	t.Run("snap1/metadata", func(t *testing.T) {
		if snap1.Skipped {
			t.Error("snap1 should not be skipped")
		}
		if snap1.CategoryCount != 4 {
			t.Errorf("CategoryCount=%d, want 4", snap1.CategoryCount)
		}
		if snap1.TotalPlays != 6 {
			t.Errorf("TotalPlays=%d, want 6", snap1.TotalPlays)
		}
		if snap1.DisplayName != "Alice" {
			t.Errorf("DisplayName=%q, want 'Alice'", snap1.DisplayName)
		}
	})

	t.Run("snap1/track_test", func(t *testing.T) {
		if snap1.TrackTest == nil {
			t.Fatal("TrackTest is nil")
		}
		if math.Abs(snap1.TrackTest.Chi2-3.3333) > 0.01 {
			t.Errorf("TrackTest.Chi2=%g, want ~3.3333", snap1.TrackTest.Chi2)
		}
		if snap1.TrackTest.DF != 3 {
			t.Errorf("TrackTest.DF=%d, want 3", snap1.TrackTest.DF)
		}
		if snap1.TrackTest.P > 0.35 || snap1.TrackTest.P < 0.25 {
			t.Errorf("TrackTest.P=%v, want ~0.31", snap1.TrackTest.P)
		}
		if math.Abs(snap1.TrackTest.Effect-0.4303) > 0.01 {
			t.Errorf("TrackTest.Effect=%v, want ~0.4303", snap1.TrackTest.Effect)
		}
		if math.Abs(snap1.TrackTest.MinExpected-1.5) > 0.01 {
			t.Errorf("TrackTest.MinExpected should be ~1.5 (expected=1.5 < 5), got %g", snap1.TrackTest.MinExpected)
		}
	})

	t.Run("snap1/track_rows", func(t *testing.T) {
		if len(snap1.TrackRows) != 4 {
			t.Fatalf("TrackRows len=%d, want 4", len(snap1.TrackRows))
		}
		if snap1.TrackRows[0].Contribution < snap1.TrackRows[1].Contribution {
			t.Error("TrackRows not sorted by contribution desc")
		}

		had := map[string]bool{}
		for _, row := range snap1.TrackRows {
			had[row.TrackID] = true
			switch row.TrackID {
			case "t1":
				if row.Observed != 3 {
					t.Errorf("t1 observed=%d, want 3", row.Observed)
				}
				if row.Name != "Track One" {
					t.Errorf("t1 name=%q, want 'Track One'", row.Name)
				}
			case "t2":
				if row.Observed != 1 {
					t.Errorf("t2 observed=%d, want 1", row.Observed)
				}
			case "t3":
				if row.Observed != 0 {
					t.Errorf("t3 observed=%d, want 0", row.Observed)
				}
			case "t4":
				if row.Observed != 2 {
					t.Errorf("t4 observed=%d, want 2", row.Observed)
				}
			}
		}
		if !had["t1"] || !had["t2"] || !had["t3"] || !had["t4"] {
			t.Errorf("missing track rows: %v", had)
		}

		for _, row := range snap1.TrackRows {
			if row.Flagged {
				t.Errorf("track %s flagged with threshold=2, residual=%g", row.TrackID, row.Residual)
			}
		}
	})

	t.Run("snap1/artist_test", func(t *testing.T) {
		if snap1.ArtistTest == nil {
			t.Fatal("ArtistTest is nil")
		}
		if math.Abs(snap1.ArtistTest.Chi2-0.75) > 0.01 {
			t.Errorf("ArtistTest.Chi2=%g, want ~0.75", snap1.ArtistTest.Chi2)
		}
		if snap1.ArtistTest.DF != 1 {
			t.Errorf("ArtistTest.DF=%d, want 1", snap1.ArtistTest.DF)
		}
		if math.Abs(snap1.ArtistTest.Effect-0.3062) > 0.01 {
			t.Errorf("ArtistTest.Effect=%v, want ~0.3062", snap1.ArtistTest.Effect)
		}
	})

	t.Run("snap1/artist_rows", func(t *testing.T) {
		if len(snap1.ArtistRows) != 2 {
			t.Fatalf("ArtistRows len=%d, want 2", len(snap1.ArtistRows))
		}
		if snap1.ArtistRows[0].Contribution < snap1.ArtistRows[1].Contribution {
			t.Error("ArtistRows not sorted by contribution desc")
		}
	})

	t.Run("snap2/metadata", func(t *testing.T) {
		if snap2.Skipped {
			t.Error("snap2 should not be skipped with minPlays=1")
		}
		if snap2.CategoryCount != 2 {
			t.Errorf("snap2 CategoryCount=%d, want 2", snap2.CategoryCount)
		}
		if snap2.TotalPlays != 1 {
			t.Errorf("snap2 TotalPlays=%d, want 1", snap2.TotalPlays)
		}
	})

	t.Run("snap2/track_test", func(t *testing.T) {
		if snap2.TrackTest == nil {
			t.Fatal("snap2 TrackTest is nil")
		}
		if math.Abs(snap2.TrackTest.Chi2-1.0) > 0.01 {
			t.Errorf("snap2 TrackTest.Chi2=%v, want 1.0", snap2.TrackTest.Chi2)
		}
		if snap2.TrackTest.DF != 1 {
			t.Errorf("snap2 TrackTest.DF=%v, want 1", snap2.TrackTest.DF)
		}
	})

	t.Run("snap2/no_artist_test", func(t *testing.T) {
		if snap2.ArtistTest != nil {
			t.Error("snap2 ArtistTest should be nil (only 1 artist)")
		}
		if len(snap2.ArtistRows) != 0 {
			t.Errorf("snap2 ArtistRows len=%d, want 0", len(snap2.ArtistRows))
		}
	})
}

func TestAnalyze_SkippedMinPlays(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	setupAnalysisData(t, s, ctx)

	results, err := analysis.Analyze(ctx, s, "u1", "Alice", 5, 2)
	if err != nil {
		t.Fatal(err)
	}

	snap1 := resultBySnapshot(results, "snap1")
	if snap1 == nil {
		t.Fatal("snap1 not found")
	}
	snap2 := resultBySnapshot(results, "snap2")
	if snap2 == nil {
		t.Fatal("snap2 not found")
	}

	// snap1: totalPlays=6 < 5 is false → not skipped
	if snap1.Skipped {
		t.Error("snap1 should not be skipped with minPlays=5, totalPlays=6")
	}

	// snap2: totalPlays=1 < 5 → skipped
	if !snap2.Skipped {
		t.Fatal("snap2 should be skipped with minPlays=5, totalPlays=1")
	}
	if snap2.SkipReason != "total plays < min_plays" {
		t.Errorf("SkipReason=%q, want 'total plays < min_plays'", snap2.SkipReason)
	}
	if snap2.TrackTest != nil {
		t.Error("TrackTest should be nil for skipped result")
	}
	if snap2.ArtistTest != nil {
		t.Error("ArtistTest should be nil for skipped result")
	}

	// TrackRows should still have observed + expected, but no residuals
	if len(snap2.TrackRows) != 2 {
		t.Fatalf("TrackRows len=%d, want 2", len(snap2.TrackRows))
	}
	for _, row := range snap2.TrackRows {
		if row.Residual != 0 {
			t.Errorf("row %s: Residual=%g, want 0 for skipped", row.TrackID, row.Residual)
		}
		if row.Contribution != 0 {
			t.Errorf("row %s: Contribution=%g, want 0 for skipped", row.TrackID, row.Contribution)
		}
	}

	// TrackRows should have names filled
	if snap2.TrackRows[0].Name == "" {
		t.Error("track names should be filled even in skipped results")
	}
}

func TestAnalyze_NoData(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	setupAnalysisData(t, s, ctx)

	results, err := analysis.Analyze(ctx, s, "nonexistent", "Nobody", 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0 for nonexistent user", len(results))
	}

	// Non-nil empty slice
	if results == nil {
		t.Error("results should be non-nil empty slice")
	}
}

func TestAnalyze_FlaggedResiduals(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	setupAnalysisData(t, s, ctx)

	// Use threshold 1 so |residual| > 1 flags t1 and t3 (residual ≈ ±1.225)
	results, err := analysis.Analyze(ctx, s, "u1", "Alice", 1, 1)
	if err != nil {
		t.Fatal(err)
	}

	snap1 := resultBySnapshot(results, "snap1")
	if snap1 == nil {
		t.Fatal("snap1 not found")
	}

	for _, row := range snap1.TrackRows {
		switch row.TrackID {
		case "t1", "t3":
			if !row.Flagged {
				t.Errorf("%s: expected flagged (|residual|=%g > 1)", row.TrackID, math.Abs(row.Residual))
			}
		case "t2", "t4":
			if row.Flagged {
				t.Errorf("%s: unexpected flagged (|residual|=%g <= 1)", row.TrackID, math.Abs(row.Residual))
			}
		}
	}
}

func TestAnalyze_NLessThan2(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)

	// User with one playlist containing a single track — N=1 category (< 2) → should skip.
	mustExec(t, s, ctx, `INSERT INTO users(id, display_name, added_at) VALUES ('u1', 'Alice', ?)`, now)
	mustExec(t, s, ctx, `INSERT INTO artists(id, name) VALUES ('a1', 'Artist One')`)
	mustExec(t, s, ctx, `INSERT INTO tracks(id, name, duration_ms) VALUES ('t1', 'Only Track', 200000)`)
	mustExec(t, s, ctx, `INSERT INTO track_artists(track_id, artist_id) VALUES ('t1', 'a1')`)
	mustExec(t, s, ctx, `INSERT INTO playlist_snapshots(id, user_id, playlist_id, playlist_name, captured_at) VALUES ('snap1', 'u1', 'p1', 'Single Track', ?)`, now)
	mustExec(t, s, ctx, `INSERT INTO playlist_snapshot_tracks(snapshot_id, track_id, position) VALUES ('snap1', 't1', 0)`)
	// One shuffle play — totalPlays=1 >= minPlays=1, but CategoryCount=1 < 2 triggers skip.
	mustExec(t, s, ctx,
		`INSERT INTO plays(user_id, track_id, playlist_id, playlist_snapshot_id, shuffle_state, smart_shuffle, played_at, ended_at, progress_ms_at_detection, skipped) VALUES ('u1', 't1', 'p1', 'snap1', 1, 0, ?, ?, 0, 0)`,
		now.Add(time.Minute), now.Add(time.Minute+30*time.Second))

	results, err := analysis.Analyze(ctx, s, "u1", "Alice", 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}

	r := results[0]
	if !r.Skipped {
		t.Error("expected skipped for N<2")
	}
	if r.SkipReason != "category count < 2" {
		t.Errorf("SkipReason=%q, want 'category count < 2'", r.SkipReason)
	}
	if r.TrackTest != nil {
		t.Error("TrackTest should be nil for skipped result")
	}
	if r.ArtistTest != nil {
		t.Error("ArtistTest should be nil for skipped result")
	}
	if len(r.TrackRows) != 1 {
		t.Fatalf("TrackRows len=%d, want 1", len(r.TrackRows))
	}
	if r.TrackRows[0].Expected != 0 {
		t.Errorf("Expected=%g, want 0 for N<2", r.TrackRows[0].Expected)
	}
}

func TestAnalyze_MinExpectedAtLeast5(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)

	// 2-track playlist, single artist, with 20 shuffle plays evenly split (10 each).
	// Expected per track = 20/2 = 10 >= 5 → chi-squared validity check passes, TrackTest produced.
	mustExec(t, s, ctx, `INSERT INTO users(id, display_name, added_at) VALUES ('u1', 'Alice', ?)`, now)
	mustExec(t, s, ctx, `INSERT INTO artists(id, name) VALUES ('a1', 'Artist One')`)
	mustExec(t, s, ctx, `INSERT INTO tracks(id, name, duration_ms) VALUES ('t1', 'Track One', 200000)`)
	mustExec(t, s, ctx, `INSERT INTO tracks(id, name, duration_ms) VALUES ('t2', 'Track Two', 200000)`)
	mustExec(t, s, ctx, `INSERT INTO track_artists(track_id, artist_id) VALUES ('t1', 'a1')`)
	mustExec(t, s, ctx, `INSERT INTO track_artists(track_id, artist_id) VALUES ('t2', 'a1')`)
	mustExec(t, s, ctx, `INSERT INTO playlist_snapshots(id, user_id, playlist_id, playlist_name, captured_at) VALUES ('snap1', 'u1', 'p1', 'High Plays', ?)`, now)
	mustExec(t, s, ctx, `INSERT INTO playlist_snapshot_tracks(snapshot_id, track_id, position) VALUES ('snap1', 't1', 0)`)
	mustExec(t, s, ctx, `INSERT INTO playlist_snapshot_tracks(snapshot_id, track_id, position) VALUES ('snap1', 't2', 1)`)

	playTime := now.Add(1 * time.Minute)
	for range 10 {
		mustExec(t, s, ctx,
			`INSERT INTO plays(user_id, track_id, playlist_id, playlist_snapshot_id, shuffle_state, smart_shuffle, played_at, ended_at, progress_ms_at_detection, skipped) VALUES ('u1', 't1', 'p1', 'snap1', 1, 0, ?, ?, 0, 0)`,
			playTime, playTime.Add(30*time.Second))
		playTime = playTime.Add(1 * time.Minute)
		mustExec(t, s, ctx,
			`INSERT INTO plays(user_id, track_id, playlist_id, playlist_snapshot_id, shuffle_state, smart_shuffle, played_at, ended_at, progress_ms_at_detection, skipped) VALUES ('u1', 't2', 'p1', 'snap1', 1, 0, ?, ?, 0, 0)`,
			playTime, playTime.Add(30*time.Second))
		playTime = playTime.Add(1 * time.Minute)
	}

	results, err := analysis.Analyze(ctx, s, "u1", "Alice", 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}

	r := results[0]
	if r.TrackTest != nil && r.TrackTest.MinExpected < 5.0 {
		t.Errorf("TrackTest.MinExpected=%g, want >= 5.0 (expected=10 >= 5)", r.TrackTest.MinExpected)
	}
	if r.ArtistTest != nil {
		t.Error("ArtistTest should be nil (only 1 artist)")
	}
}
