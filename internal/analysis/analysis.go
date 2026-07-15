package analysis

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/mister-gnommer/trust-issues/internal/store"
)

// Reader is the subset of *store.Store used by the analyzer.
type Reader interface {
	SnapshotsWithShufflePlays(ctx context.Context, userID string) ([]store.Snapshot, error)
	SnapshotTrackIDs(ctx context.Context, snapshotID string) ([]string, int, error)
	ArtistTrackCounts(ctx context.Context, snapshotID string) (map[string]int, error)
	PlayCountsByTrack(ctx context.Context, userID, snapshotID string) (map[string]int, error)
	PlayCountsByArtist(ctx context.Context, userID, snapshotID string) (map[string]int, error)
	TrackNames(ctx context.Context, trackIDs []string) (map[string]string, error)
	ArtistNames(ctx context.Context, artistIDs []string) (map[string]string, error)
}

// Analyze runs chi-squared tests for all snapshots of one user.
func Analyze(ctx context.Context, reader Reader, userID, displayName string, minPlays, residualThreshold int) ([]Result, error) {
	snapshots, err := reader.SnapshotsWithShufflePlays(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("snapshots: %w", err)
	}

	results := make([]Result, 0, len(snapshots))
	for _, snap := range snapshots {
		result, err := analyzeSnapshot(ctx, reader, userID, displayName, snap, minPlays, residualThreshold)
		if err != nil {
			return nil, fmt.Errorf("snapshot %s: %w", snap.ID, err)
		}
		results = append(results, result)
	}
	return results, nil
}

func analyzeSnapshot(ctx context.Context, reader Reader, userID, displayName string, snap store.Snapshot, minPlays, residualThreshold int) (Result, error) {
	// Two queries, not one: SnapshotTrackIDs gives the full track universe
	// (chi-squared needs zero-play tracks as deviations from uniform), while
	// PlayCountsByTrack returns only played tracks. Merging them via a LEFT
	// JOIN would save one local-SQL roundtrip but hurt readability and
	// reusability — see README "Architectural decisions".
	trackIDs, _, err := reader.SnapshotTrackIDs(ctx, snap.ID)
	if err != nil {
		return Result{}, fmt.Errorf("track IDs: %w", err)
	}

	playCountsByTrack, err := reader.PlayCountsByTrack(ctx, userID, snap.ID)
	if err != nil {
		return Result{}, fmt.Errorf("play counts by track: %w", err)
	}

	observed := make([]int, len(trackIDs))
	totalPlays := 0
	for i, tid := range trackIDs {
		cnt := playCountsByTrack[tid]
		observed[i] = cnt
		totalPlays += cnt
	}

	result := Result{
		UserID:        snap.UserID,
		DisplayName:   displayName,
		SnapshotID:    snap.ID,
		PlaylistID:    snap.PlaylistID,
		PlaylistName:  snap.PlaylistName,
		CapturedAt:    snap.CapturedAt,
		CategoryCount: len(trackIDs),
		TotalPlays:    totalPlays,
	}

	// --- Track analysis ---
	switch {
	case result.CategoryCount < 2:
		result.Skipped = true
		result.SkipReason = "category count < 2"
		result.TrackRows = makeSkippedTrackRows(trackIDs, observed, 0)

	case totalPlays < minPlays:
		exp := float64(totalPlays) / float64(result.CategoryCount)
		result.Skipped = true
		result.SkipReason = "total plays < min_plays"
		result.TrackRows = makeSkippedTrackRows(trackIDs, observed, exp)

	default:
		exp := float64(totalPlays) / float64(result.CategoryCount)
		expSlice := make([]float64, result.CategoryCount)
		for i := range expSlice {
			expSlice[i] = exp
		}

		tr, err := RunChiSquared(observed, expSlice)
		if err != nil {
			return Result{}, fmt.Errorf("track test: %w", err)
		}
		result.TrackTest = &tr

		result.TrackRows = makeTrackRows(trackIDs, observed, exp, residualThreshold)
	}

	// --- Artist analysis ---
	artistTrackCounts, err := reader.ArtistTrackCounts(ctx, snap.ID)
	if err != nil {
		return Result{}, fmt.Errorf("artist track counts: %w", err)
	}

	if len(artistTrackCounts) >= 2 {
		artistPlayCounts, err := reader.PlayCountsByArtist(ctx, userID, snap.ID)
		if err != nil {
			return Result{}, fmt.Errorf("play counts by artist: %w", err)
		}

		artistIDs := sortedKeys(artistTrackCounts)
		totalK := 0
		for _, k := range artistTrackCounts {
			totalK += k
		}

		aObserved := make([]int, len(artistIDs))
		aM := 0
		for i, aid := range artistIDs {
			cnt := artistPlayCounts[aid]
			aObserved[i] = cnt
			aM += cnt
		}

		aExp := make([]float64, len(artistIDs))
		for i, aid := range artistIDs {
			aExp[i] = float64(artistTrackCounts[aid]) * float64(aM) / float64(totalK)
		}

		if result.Skipped {
			result.ArtistRows = makeSkippedArtistRows(artistIDs, aObserved, aExp)
		} else {
			ar, err := RunChiSquared(aObserved, aExp)
			if err != nil {
				return Result{}, fmt.Errorf("artist test: %w", err)
			}
			result.ArtistTest = &ar

			result.ArtistRows = makeArtistRows(artistIDs, aObserved, aExp, residualThreshold)
		}
	}

	// --- Fetch names ---
	if len(result.TrackRows) > 0 {
		ids := make([]string, len(result.TrackRows))
		for i, row := range result.TrackRows {
			ids[i] = row.TrackID
		}
		names, err := reader.TrackNames(ctx, ids)
		if err != nil {
			return Result{}, fmt.Errorf("track names: %w", err)
		}
		for i := range result.TrackRows {
			result.TrackRows[i].Name = names[result.TrackRows[i].TrackID]
		}
	}

	if len(result.ArtistRows) > 0 {
		ids := make([]string, len(result.ArtistRows))
		for i, row := range result.ArtistRows {
			ids[i] = row.ArtistID
		}
		artistNames, err := reader.ArtistNames(ctx, ids)
		if err != nil {
			return Result{}, fmt.Errorf("artist names: %w", err)
		}
		for i := range result.ArtistRows {
			result.ArtistRows[i].Name = artistNames[result.ArtistRows[i].ArtistID]
		}
	}

	return result, nil
}

// makeSkippedTrackRows creates track rows when chi-squared is skipped.
// exp is 0 when no expected is computable (N < 2), or totalPlays/CategoryCount (M < minPlays).
func makeSkippedTrackRows(trackIDs []string, observed []int, exp float64) []TrackRow {
	rows := make([]TrackRow, len(trackIDs))
	for i, tid := range trackIDs {
		rows[i] = TrackRow{
			TrackID:  tid,
			Observed: observed[i],
			Expected: exp,
		}
	}
	return rows
}

// makeSkippedArtistRows creates artist rows when chi-squared is skipped.
func makeSkippedArtistRows(artistIDs []string, observed []int, expected []float64) []ArtistRow {
	rows := make([]ArtistRow, len(artistIDs))
	for i, aid := range artistIDs {
		rows[i] = ArtistRow{ArtistID: aid, Observed: observed[i], Expected: expected[i]}
	}
	return rows
}

// makeTrackRows creates full track rows with chi-squared residuals.
func makeTrackRows(trackIDs []string, observed []int, exp float64, threshold int) []TrackRow {
	rows := make([]TrackRow, len(trackIDs))
	for i, tid := range trackIDs {
		resid := mustResidual(observed[i], exp)
		rows[i] = TrackRow{
			TrackID:      tid,
			Observed:     observed[i],
			Expected:     exp,
			Contribution: resid * resid,
			Residual:     resid,
			Flagged:      math.Abs(resid) > float64(threshold),
			DeviationPct: (float64(observed[i]) - exp) / exp * 100,
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Contribution > rows[j].Contribution
	})
	return rows
}

// makeArtistRows creates full artist rows with chi-squared residuals.
func makeArtistRows(artistIDs []string, observed []int, exp []float64, threshold int) []ArtistRow {
	rows := make([]ArtistRow, len(artistIDs))
	for i, aid := range artistIDs {
		resid := mustResidual(observed[i], exp[i])
		rows[i] = ArtistRow{
			ArtistID:     aid,
			Observed:     observed[i],
			Expected:     exp[i],
			Contribution: resid * resid,
			Residual:     resid,
			Flagged:      math.Abs(resid) > float64(threshold),
			DeviationPct: (float64(observed[i]) - exp[i]) / exp[i] * 100,
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Contribution > rows[j].Contribution
	})
	return rows
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func mustResidual(observed int, expected float64) float64 {
	r, err := Residual(observed, expected)
	if err != nil {
		panic(fmt.Sprintf("unexpected Residual error: %v", err))
	}
	return r
}
