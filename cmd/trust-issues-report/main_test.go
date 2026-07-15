package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mister-gnommer/trust-issues/v2/internal/config"
	"github.com/mister-gnommer/trust-issues/v2/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func mustExec(t *testing.T, s *store.Store, ctx context.Context, query string, args ...any) {
	t.Helper()
	if _, err := s.DB().ExecContext(ctx, query, args...); err != nil {
		t.Fatalf("exec: %v", err)
	}
}

func seedAnalysisData(t *testing.T, s *store.Store, ctx context.Context) {
	t.Helper()
	// Alice (u1), one snapshot (snap1) of playlist "Daily Mix" (p1).
	// Snapshot has 2 tracks (t1, t2) by 1 artist (a1).
	// 4 plays total: t1=3, t2=1, all in shuffle context.
	//
	// Track chi-squared: k=2, N=4, obs=[3,1], expected=2.0 each.
	//   chi2=1.0, df=1, p≈0.317, V=0.5
	//   residual: [0.707, -0.707], flagged: none (threshold 3)
	//   MinExpected: 2.0 (< 5, so low-sample-size warning).
	//
	// Artist chi-squared: k=1 (only a1), df=0 — not computable.
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)

	mustExec(t, s, ctx, `INSERT INTO users(id, display_name, added_at) VALUES ('u1', 'Alice', ?)`, now)
	mustExec(t, s, ctx, `INSERT INTO artists(id, name) VALUES ('a1', 'Artist One')`)
	mustExec(t, s, ctx, `INSERT INTO tracks(id, name, duration_ms) VALUES ('t1', 'Track One', 200000)`)
	mustExec(t, s, ctx, `INSERT INTO tracks(id, name, duration_ms) VALUES ('t2', 'Track Two', 200000)`)
	mustExec(t, s, ctx, `INSERT INTO track_artists(track_id, artist_id) VALUES ('t1', 'a1')`)
	mustExec(t, s, ctx, `INSERT INTO track_artists(track_id, artist_id) VALUES ('t2', 'a1')`)
	mustExec(t, s, ctx, `INSERT INTO playlist_snapshots(id, user_id, playlist_id, playlist_name, captured_at) VALUES ('snap1', 'u1', 'p1', 'Daily Mix', ?)`, now)
	mustExec(t, s, ctx, `INSERT INTO playlist_snapshot_tracks(snapshot_id, track_id, position) VALUES ('snap1', 't1', 0)`)
	mustExec(t, s, ctx, `INSERT INTO playlist_snapshot_tracks(snapshot_id, track_id, position) VALUES ('snap1', 't2', 1)`)

	playTime := now.Add(1 * time.Minute)
	for range 3 {
		mustExec(t, s, ctx,
			`INSERT INTO plays(user_id, track_id, playlist_id, playlist_snapshot_id, shuffle_state, smart_shuffle, played_at, ended_at, progress_ms_at_detection, skipped) VALUES (?, ?, ?, ?, 1, 0, ?, ?, 0, 0)`,
			"u1", "t1", "p1", "snap1", playTime, playTime.Add(30*time.Second))
		playTime = playTime.Add(1 * time.Minute)
	}
	mustExec(t, s, ctx,
		`INSERT INTO plays(user_id, track_id, playlist_id, playlist_snapshot_id, shuffle_state, smart_shuffle, played_at, ended_at, progress_ms_at_detection, skipped) VALUES (?, ?, ?, ?, 1, 0, ?, ?, 0, 0)`,
		"u1", "t2", "p1", "snap1", playTime, playTime.Add(30*time.Second))
}

func testConfig(t *testing.T, reportsDir string) *config.Config {
	t.Helper()
	return &config.Config{
		// Storage.DatabasePath is unused by runReportOnce — the store
		// is opened via newTestStore() in each test. Must be non-empty
		// to pass config validation.
		Storage: config.Storage{DatabasePath: ":memory:"},
		Reports: config.Reports{
			Dir:               reportsDir,
			MinPlays:          1,
			ResidualThreshold: 3,
		},
		Accounts: []config.Account{
			{UserID: "u1", DisplayName: "Alice"},
		},
	}
}

func TestRunReportOnce_emptyStore(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	dir := t.TempDir()
	cfg := testConfig(t, dir)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	err := runReportOnce(ctx, logger, cfg, st, time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("runReportOnce: %v", err)
	}

	summaryPath := filepath.Join(dir, "20260621.md")
	dataPath := filepath.Join(dir, "20260621-data.md")

	if _, err := os.Stat(summaryPath); err != nil {
		t.Errorf("summary file not created: %v", err)
	}
	if _, err := os.Stat(dataPath); err != nil {
		t.Errorf("data file not created: %v", err)
	}

	summary, _ := os.ReadFile(summaryPath)
	if !strings.Contains(string(summary), "No shuffle plays recorded yet for any account") {
		t.Error("empty report should mention no data")
	}
}

func TestRunReportOnce_seededData(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	seedAnalysisData(t, st, ctx)
	dir := t.TempDir()
	cfg := testConfig(t, dir)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	err := runReportOnce(ctx, logger, cfg, st, time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("runReportOnce: %v", err)
	}

	summaryPath := filepath.Join(dir, "20260621.md")
	dataPath := filepath.Join(dir, "20260621-data.md")

	summary, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}
	if !strings.Contains(string(summary), "Alice") {
		t.Error("summary should contain display name")
	}
	if !strings.Contains(string(summary), "snap1") {
		t.Error("summary should contain snapshot ID")
	}
	if !strings.Contains(string(summary), "Daily Mix") {
		t.Error("summary should contain playlist name")
	}

	data, err := os.ReadFile(dataPath)
	if err != nil {
		t.Fatalf("read data: %v", err)
	}
	if !strings.Contains(string(data), "Track One") {
		t.Error("data should contain track names")
	}
}

func TestRunReportOnce_autoRecoversPartialFile(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	dir := t.TempDir()
	cfg := testConfig(t, dir)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Only the summary file exists — partial state from a prior crashed run.
	if err := os.WriteFile(filepath.Join(dir, "20260621.md"), []byte("orphan"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	err := runReportOnce(ctx, logger, cfg, st, time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected auto-recovery to succeed, got: %v", err)
	}

	// Both files should now exist (regenerated).
	if _, err := os.Stat(filepath.Join(dir, "20260621.md")); err != nil {
		t.Errorf("summary file should exist after auto-recovery: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "20260621-data.md")); err != nil {
		t.Errorf("data file should exist after auto-recovery: %v", err)
	}
}

func TestRunReportOnce_alreadyGenerated(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	dir := t.TempDir()
	cfg := testConfig(t, dir)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Pre-create both files — successful prior run.
	if err := os.WriteFile(filepath.Join(dir, "20260621.md"), []byte("existing summary"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "20260621-data.md"), []byte("existing data"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	err := runReportOnce(ctx, logger, cfg, st, time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected nil (idempotent no-op), got: %v", err)
	}

	summary, _ := os.ReadFile(filepath.Join(dir, "20260621.md"))
	if string(summary) != "existing summary" {
		t.Error("summary file was modified by no-op run")
	}
	data, _ := os.ReadFile(filepath.Join(dir, "20260621-data.md"))
	if string(data) != "existing data" {
		t.Error("data file was modified by no-op run")
	}
}

func TestRunReportOnce_multiAccount(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	seedAnalysisData(t, st, ctx)

	// Bob (u2), one snapshot (snap2) of playlist "Discover" (p2).
	// Reuses track t1 and artist a1 from Alice's seed above.
	// Snapshot has 1 track, 1 play — k=1 means no chi-squared test.
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	mustExec(t, st, ctx, `INSERT INTO users(id, display_name, added_at) VALUES ('u2', 'Bob', ?)`, now)
	mustExec(t, st, ctx, `INSERT INTO playlist_snapshots(id, user_id, playlist_id, playlist_name, captured_at) VALUES ('snap2', 'u2', 'p2', 'Discover', ?)`, now)
	mustExec(t, st, ctx, `INSERT INTO playlist_snapshot_tracks(snapshot_id, track_id, position) VALUES ('snap2', 't1', 0)`)
	mustExec(t, st, ctx, `INSERT INTO plays(user_id, track_id, playlist_id, playlist_snapshot_id, shuffle_state, smart_shuffle, played_at, ended_at, progress_ms_at_detection, skipped) VALUES ('u2', 't1', 'p2', 'snap2', 1, 0, ?, ?, 0, 0)`, now, now.Add(30*time.Second))

	dir := t.TempDir()
	cfg := testConfig(t, dir)
	cfg.Accounts = append(cfg.Accounts, config.Account{UserID: "u2", DisplayName: "Bob"})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	err := runReportOnce(ctx, logger, cfg, st, time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("runReportOnce: %v", err)
	}

	summary, err := os.ReadFile(filepath.Join(dir, "20260621.md"))
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}
	if !strings.Contains(string(summary), "Alice") {
		t.Error("summary should contain Alice")
	}
	if !strings.Contains(string(summary), "Bob") {
		t.Error("summary should contain Bob")
	}
}
