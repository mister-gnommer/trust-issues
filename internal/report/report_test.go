package report_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mister-gnommer/trust-issues/internal/analysis"
	"github.com/mister-gnommer/trust-issues/internal/report"
)

var update = flag.Bool("update", false, "update golden files")

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func loc(t *testing.T) *time.Location {
	t.Helper()
	l, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	return l
}

// testResults returns a slice of analysis.Result for deterministic golden-file testing.
// Two accounts:
//   - Alice: Favourites playlist, three snapshots (normal, skipped, normal with artist test)
//   - Bob: Discover Weekly, one snapshot (normal with track + artist test)
func testResults(t *testing.T, loc *time.Location) []analysis.Result {
	t.Helper()

	// snap1: k=4, N=8, obs=[5,2,1,0], expected=2.0 each
	// chi2=7.0, df=3, p≈0.0719, V=sqrt(7/(8*3))≈0.5401
	// contrib: [4.5000, 2.0000, 0.5000, 0.0000]
	// residual: [2.12, 0.00, -0.71, -1.41]
	// flagged: none (threshold 3)
	// MinExpected: 2.0 (< 5, so track warning)
	capturedAt := time.Date(2026, 6, 20, 12, 0, 0, 0, loc)

	// snap2: N=3, M=2, skipped (M < minPlays, e.g. minPlays=30)
	// obs=[1,1,0], expected=0.67 each
	capturedAt2 := time.Date(2026, 6, 20, 14, 30, 0, 0, loc)

	return []analysis.Result{
		{
			UserID:        "u1",
			DisplayName:   "Alice",
			SnapshotID:    "snap1",
			PlaylistID:    "p1",
			PlaylistName:  "Favourites",
			CapturedAt:    capturedAt,
			CategoryCount: 4,
			TotalPlays:    8,
			TrackTest: &analysis.ChiSquaredResult{
				Chi2: 7.0, DF: 3, P: 0.0719, Effect: 0.5401, MinExpected: 2.0,
			},
			TrackRows: []analysis.TrackRow{
				{TrackID: "t1", Name: "Track One", Observed: 5, Expected: 2.0, Contribution: 4.5, Residual: 2.1213, Flagged: false, DeviationPct: 150.00},
				{TrackID: "t4", Name: "Track Four", Observed: 0, Expected: 2.0, Contribution: 2.0, Residual: -1.4142, Flagged: false, DeviationPct: -100.00},
				{TrackID: "t3", Name: "Track Three", Observed: 1, Expected: 2.0, Contribution: 0.5, Residual: -0.7071, Flagged: false, DeviationPct: -50.00},
				{TrackID: "t2", Name: "Track Two", Observed: 2, Expected: 2.0, Contribution: 0.0, Residual: 0.0, Flagged: false, DeviationPct: 0.00},
			},
		},
		{
			UserID:        "u1",
			DisplayName:   "Alice",
			SnapshotID:    "snap2",
			PlaylistID:    "p1",
			PlaylistName:  "Favourites",
			CapturedAt:    capturedAt2,
			CategoryCount: 3,
			TotalPlays:    2,
			Skipped:       true,
			SkipReason:    "total plays < min_plays",
			TrackRows: []analysis.TrackRow{
				{TrackID: "t1", Name: "Track One", Observed: 1, Expected: 0.67},
				{TrackID: "t2", Name: "Track Two", Observed: 1, Expected: 0.67},
				{TrackID: "t3", Name: "Track Three", Observed: 0, Expected: 0.67},
			},
		},
		{
			UserID:        "u2",
			DisplayName:   "Bob",
			SnapshotID:    "snap3",
			PlaylistID:    "p2",
			PlaylistName:  "Discover Weekly",
			CapturedAt:    time.Date(2026, 6, 20, 10, 0, 0, 0, loc),
			CategoryCount: 2,
			TotalPlays:    35,
			TrackTest: &analysis.ChiSquaredResult{
				Chi2: 0.1, DF: 1, P: 0.7518, Effect: 0.0534, MinExpected: 17.5,
			},
			TrackRows: []analysis.TrackRow{
				{TrackID: "t1", Name: "Track A", Observed: 18, Expected: 17.5, Contribution: 0.0143, Residual: 0.1195, Flagged: false, DeviationPct: 2.86},
				{TrackID: "t2", Name: "Track B", Observed: 17, Expected: 17.5, Contribution: 0.0143, Residual: -0.1195, Flagged: false, DeviationPct: -2.86},
			},
			ArtistTest: &analysis.ChiSquaredResult{
				Chi2: 0.05, DF: 1, P: 0.8231, Effect: 0.0378, MinExpected: 17.5,
			},
			ArtistRows: []analysis.ArtistRow{
				{ArtistID: "a1", Name: "Artist One", Observed: 18, Expected: 17.5, Contribution: 0.0143, Residual: 0.1195, Flagged: false, DeviationPct: 2.86},
				{ArtistID: "a2", Name: "Artist Two", Observed: 17, Expected: 17.5, Contribution: 0.0143, Residual: -0.1195, Flagged: false, DeviationPct: -2.86},
			},
		},
		{
			UserID:        "u1",
			DisplayName:   "Alice",
			SnapshotID:    "snap4",
			PlaylistID:    "p1",
			PlaylistName:  "Favourites",
			CapturedAt:    capturedAt,
			CategoryCount: 4,
			TotalPlays:    12,
			TrackTest: &analysis.ChiSquaredResult{
				Chi2: 10.0, DF: 3, P: 0.0186, Effect: 0.5270, MinExpected: 2.0,
			},
			ArtistTest: &analysis.ChiSquaredResult{
				Chi2: 5.0, DF: 2, P: 0.0821, Effect: 0.4564, MinExpected: 10.0,
			},
			TrackRows: []analysis.TrackRow{
				{TrackID: "t1", Name: "Track One", Observed: 5, Expected: 2.0, Contribution: 4.5, Residual: 2.1213, Flagged: false, DeviationPct: 150.00},
				{TrackID: "t4", Name: "Track Four", Observed: 0, Expected: 2.0, Contribution: 2.0, Residual: -1.4142, Flagged: false, DeviationPct: -100.00},
				{TrackID: "t3", Name: "Track Three", Observed: 1, Expected: 2.0, Contribution: 0.5, Residual: -0.7071, Flagged: false, DeviationPct: -50.00},
				{TrackID: "t2", Name: "Track Two", Observed: 2, Expected: 2.0, Contribution: 0.0, Residual: 0.0, Flagged: false, DeviationPct: 0.00},
			},
			ArtistRows: []analysis.ArtistRow{
				{ArtistID: "a1", Name: "Artist One", Observed: 7, Expected: 6.0, Contribution: 0.1667, Residual: 0.4082, Flagged: false, DeviationPct: 16.67},
				{ArtistID: "a2", Name: "Artist Two", Observed: 3, Expected: 4.0, Contribution: 0.25, Residual: -0.5, Flagged: false, DeviationPct: -25.00},
				{ArtistID: "a3", Name: "Artist Three", Observed: 2, Expected: 2.0, Contribution: 0.0, Residual: 0.0, Flagged: false, DeviationPct: 0.00},
			},
		},
	}
}

// testResultsFlagged returns results for testing the flagged-tracks section.
func testResultsFlagged(t *testing.T, loc *time.Location) []analysis.Result {
	t.Helper()

	// Global p > 0.01 (not flagged globally), but one track with |residual| > 3.
	// k=4, N=12, obs=[10,1,1,0], expected=3.0 each
	return []analysis.Result{
		{
			UserID:        "u1",
			DisplayName:   "Alice",
			SnapshotID:    "snap_flag",
			PlaylistID:    "p1",
			PlaylistName:  "Favourites",
			CapturedAt:    time.Date(2026, 6, 20, 12, 0, 0, 0, loc),
			CategoryCount: 4,
			TotalPlays:    12,
			TrackTest: &analysis.ChiSquaredResult{
				Chi2: 15.0, DF: 3, P: 0.5, Effect: 0.6455, MinExpected: 3.0,
			},
			TrackRows: []analysis.TrackRow{
				{TrackID: "t1", Name: "Overplayed", Observed: 10, Expected: 3.0, Contribution: 16.3333, Residual: 4.0415, Flagged: true, DeviationPct: 233.33},
				{TrackID: "t2", Name: "Normal", Observed: 1, Expected: 3.0, Contribution: 1.3333, Residual: -1.1547, Flagged: false, DeviationPct: -66.67},
				{TrackID: "t3", Name: "Normal Too", Observed: 1, Expected: 3.0, Contribution: 1.3333, Residual: -1.1547, Flagged: false, DeviationPct: -66.67},
				{TrackID: "t4", Name: "Skipped Track", Observed: 0, Expected: 3.0, Contribution: 3.0, Residual: -1.7321, Flagged: false, DeviationPct: -100.00},
			},
		},
	}
}

func TestRenderSummary_empty(t *testing.T) {
	l := loc(t)
	reportDate := time.Date(2026, 6, 21, 0, 0, 0, 0, l)
	generatedAt := time.Date(2026, 6, 21, 3, 0, 0, 0, l)

	for _, input := range [][]analysis.Result{nil, {}} {
		got := report.RenderSummary(reportDate, generatedAt, l, input, 3)
		if got == "" {
			t.Fatal("empty result for empty input")
		}
		if !strings.Contains(got, "No shuffle plays recorded yet for any account") {
			t.Error("empty result should mention no data")
		}
	}
}

func TestRenderData_empty(t *testing.T) {
	l := loc(t)
	reportDate := time.Date(2026, 6, 21, 0, 0, 0, 0, l)
	generatedAt := time.Date(2026, 6, 21, 3, 0, 0, 0, l)

	for _, input := range [][]analysis.Result{nil, {}} {
		got := report.RenderData(reportDate, generatedAt, l, input, 3)
		if got == "" {
			t.Fatal("empty result for empty input")
		}
		if !strings.Contains(got, "No shuffle plays recorded yet for any account") {
			t.Error("empty data should mention no data")
		}
	}
}

func TestRenderSummary_golden(t *testing.T) {
	l := loc(t)
	reportDate := time.Date(2026, 6, 21, 0, 0, 0, 0, l)
	generatedAt := time.Date(2026, 6, 21, 3, 0, 0, 0, l)
	results := testResults(t, l)

	got := report.RenderSummary(reportDate, generatedAt, l, results, 3)
	golden := filepath.Join("testdata", "summary.golden.md")

	if *update {
		if err := os.WriteFile(golden, []byte(got), 0644); err != nil {
			t.Fatal(err)
		}
		return
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("summary output does not match golden file\nUpdate with: go test -update ./internal/report/")
	}
}

func TestRenderData_golden(t *testing.T) {
	l := loc(t)
	reportDate := time.Date(2026, 6, 21, 0, 0, 0, 0, l)
	generatedAt := time.Date(2026, 6, 21, 3, 0, 0, 0, l)
	results := testResults(t, l)

	got := report.RenderData(reportDate, generatedAt, l, results, 3)
	golden := filepath.Join("testdata", "data.golden.md")

	if *update {
		if err := os.WriteFile(golden, []byte(got), 0644); err != nil {
			t.Fatal(err)
		}
		return
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("data output does not match golden file\nUpdate with: go test -update ./internal/report/")
	}
}

func TestRenderSummary_flaggedTracks(t *testing.T) {
	l := loc(t)
	reportDate := time.Date(2026, 6, 21, 0, 0, 0, 0, l)
	generatedAt := time.Date(2026, 6, 21, 3, 0, 0, 0, l)
	results := testResultsFlagged(t, l)

	got := report.RenderSummary(reportDate, generatedAt, l, results, 3)

	// Global verdict should be "not flagged" (p=0.5 >= 0.01)
	if !strings.Contains(got, "not flagged (p >= 0.01)") {
		t.Error("expected global verdict 'not flagged'")
	}

	// Flagged tracks section should mention the overplayed track
	if !strings.Contains(got, "Flagged tracks") {
		t.Error("expected Flagged tracks section")
	}
	if !strings.Contains(got, "Overplayed") {
		t.Error("expected overplayed track in flagged section")
	}
	if !strings.Contains(got, "over") {
		t.Error("expected 'over' direction for positive residual")
	}
}

func testResultsFlaggedArtists(t *testing.T, loc *time.Location) []analysis.Result {
	t.Helper()

	// Global p > 0.01 (not flagged globally), but one artist with |residual| > 3.
	return []analysis.Result{
		{
			UserID:        "u1",
			DisplayName:   "Alice",
			SnapshotID:    "snap_flag_artist",
			PlaylistID:    "p1",
			PlaylistName:  "Favourites",
			CapturedAt:    time.Date(2026, 6, 20, 12, 0, 0, 0, loc),
			CategoryCount: 4,
			TotalPlays:    24,
			TrackTest: &analysis.ChiSquaredResult{
				Chi2: 3.0, DF: 3, P: 0.392, Effect: 0.204, MinExpected: 6.0,
			},
			TrackRows: []analysis.TrackRow{
				{TrackID: "t1", Name: "Track One", Observed: 6, Expected: 6.0, Contribution: 0.0, Residual: 0.0, Flagged: false, DeviationPct: 0.00},
				{TrackID: "t2", Name: "Track Two", Observed: 6, Expected: 6.0, Contribution: 0.0, Residual: 0.0, Flagged: false, DeviationPct: 0.00},
				{TrackID: "t3", Name: "Track Three", Observed: 6, Expected: 6.0, Contribution: 0.0, Residual: 0.0, Flagged: false, DeviationPct: 0.00},
				{TrackID: "t4", Name: "Track Four", Observed: 6, Expected: 6.0, Contribution: 0.0, Residual: 0.0, Flagged: false, DeviationPct: 0.00},
			},
			ArtistTest: &analysis.ChiSquaredResult{
				Chi2: 20.0, DF: 2, P: 0.5, Effect: 0.645, MinExpected: 8.0,
			},
			ArtistRows: []analysis.ArtistRow{
				{ArtistID: "a1", Name: "Overplayed Artist", Observed: 18, Expected: 8.0, Contribution: 12.5, Residual: 3.536, Flagged: true, DeviationPct: 125.00},
				{ArtistID: "a2", Name: "Normal Artist", Observed: 4, Expected: 8.0, Contribution: 2.0, Residual: -1.414, Flagged: false, DeviationPct: -50.00},
				{ArtistID: "a3", Name: "Underplayed", Observed: 2, Expected: 8.0, Contribution: 4.5, Residual: -2.121, Flagged: false, DeviationPct: -75.00},
			},
		},
	}
}

func TestRenderSummary_flaggedArtists(t *testing.T) {
	l := loc(t)
	reportDate := time.Date(2026, 6, 21, 0, 0, 0, 0, l)
	generatedAt := time.Date(2026, 6, 21, 3, 0, 0, 0, l)
	results := testResultsFlaggedArtists(t, l)

	got := report.RenderSummary(reportDate, generatedAt, l, results, 3)

	if !strings.Contains(got, "Flagged artists") {
		t.Error("expected Flagged artists section")
	}
	if !strings.Contains(got, "Overplayed Artist") {
		t.Error("expected overplayed artist in flagged section")
	}
	if !strings.Contains(got, "over") {
		t.Error("expected 'over' direction for positive residual")
	}
}

func TestRenderSummary_skippedNoExpected(t *testing.T) {
	l := loc(t)
	reportDate := time.Date(2026, 6, 21, 0, 0, 0, 0, l)
	generatedAt := time.Date(2026, 6, 21, 3, 0, 0, 0, l)

	results := []analysis.Result{
		{
			UserID:        "u1",
			DisplayName:   "Alice",
			SnapshotID:    "snap_skip",
			PlaylistID:    "p1",
			PlaylistName:  "Favourites",
			CapturedAt:    time.Date(2026, 6, 20, 12, 0, 0, 0, l),
			CategoryCount: 2,
			TotalPlays:    1,
			Skipped:       true,
			SkipReason:    "insufficient data",
			TrackRows: []analysis.TrackRow{
				{TrackID: "t1", Name: "Track One", Observed: 1},
				{TrackID: "t2", Name: "Track Two", Observed: 0},
			},
		},
	}

	got := report.RenderSummary(reportDate, generatedAt, l, results, 3)

	if !strings.Contains(got, "Track ID") {
		t.Error("expected 'Track ID' column in skipped table")
	}
	if !strings.Contains(got, "Track One") {
		t.Error("expected row in skipped table")
	}
	if strings.Contains(got, "Expected") {
		t.Error("skipped table without expected counts should not have Expected column")
	}
}

func TestWriteAll_refusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	date := time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)

	// Pre-create both files
	summaryPath := filepath.Join(dir, "20260621.md")
	dataPath := filepath.Join(dir, "20260621-data.md")

	if err := os.WriteFile(summaryPath, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dataPath, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}

	err := report.WriteAll(dir, date, "new summary", "new data")
	if err == nil {
		t.Fatal("expected error for overwrite, got nil")
	}
	if !strings.Contains(err.Error(), "refusing") {
		t.Errorf("error should mention 'refusing': %v", err)
	}

	// Verify original files unchanged
	got, _ := os.ReadFile(summaryPath)
	if string(got) != "existing" {
		t.Errorf("summary file modified: got %q", string(got))
	}
	got, _ = os.ReadFile(dataPath)
	if string(got) != "existing" {
		t.Errorf("data file modified: got %q", string(got))
	}
}

func TestWriteAll_createsDir(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "nonexistent", "subdir")
	date := time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)

	if err := report.WriteAll(dir, date, "summary", "data"); err != nil {
		t.Fatalf("WriteAll: %v", err)
	}

	summaryPath := filepath.Join(dir, "20260621.md")
	dataPath := filepath.Join(dir, "20260621-data.md")

	if _, err := os.Stat(summaryPath); err != nil {
		t.Errorf("summary file not created: %v", err)
	}
	if _, err := os.Stat(dataPath); err != nil {
		t.Errorf("data file not created: %v", err)
	}

	got, _ := os.ReadFile(summaryPath)
	if string(got) != "summary" {
		t.Errorf("summary content: got %q, want 'summary'", string(got))
	}
	got, _ = os.ReadFile(dataPath)
	if string(got) != "data" {
		t.Errorf("data content: got %q, want 'data'", string(got))
	}
}
