// Package report generates Markdown reports from shuffle randomness analysis results.
// It covers both summary and full-data appendix output, organized by user and playlist.
package report

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mister-gnommer/trust-issues/internal/analysis"
)

type accountGroup struct {
	UserID      string
	DisplayName string
	playlists   []playlistGroup
}

type playlistGroup struct {
	PlaylistID   string
	PlaylistName string
	snapshots    []analysis.Result
}

// groupResults groups raw results by (user, playlist) preserving insertion order.
func groupResults(results []analysis.Result) []accountGroup {
	type userKey struct{ uid, name string }
	byUser := make(map[userKey][]analysis.Result)
	userOrder := make([]userKey, 0)
	for _, r := range results {
		key := userKey{r.UserID, r.DisplayName}
		if _, ok := byUser[key]; !ok {
			userOrder = append(userOrder, key)
		}
		byUser[key] = append(byUser[key], r)
	}

	groups := make([]accountGroup, 0, len(userOrder))
	for _, key := range userOrder {
		ag := accountGroup{UserID: key.uid, DisplayName: key.name}
		// Nest playlists within each user using the same ordered-map pattern.
		type plKey struct{ pid, pname string }
		byPlaylist := make(map[plKey][]analysis.Result)
		plOrder := make([]plKey, 0)
		for _, r := range byUser[key] {
			pk := plKey{r.PlaylistID, r.PlaylistName}
			if _, ok := byPlaylist[pk]; !ok {
				plOrder = append(plOrder, pk)
			}
			byPlaylist[pk] = append(byPlaylist[pk], r)
		}
		for _, pk := range plOrder {
			ag.playlists = append(ag.playlists, playlistGroup{
				PlaylistID:   pk.pid,
				PlaylistName: pk.pname,
				snapshots:    byPlaylist[pk],
			})
		}
		groups = append(groups, ag)
	}
	return groups
}

func fmtP(p float64) string {
	if p < 0.0001 {
		return fmt.Sprintf("%.2e", p)
	}
	return fmt.Sprintf("%.4f", p)
}

func maybeP(fp *float64) string {
	if fp == nil {
		return "N/A"
	}
	return fmtP(*fp)
}

func maybeChi2(fp *float64) string {
	if fp == nil {
		return "N/A"
	}
	return fmt.Sprintf("%.2f", *fp)
}

func maybeDF(ip *int) string {
	if ip == nil {
		return "N/A"
	}
	return fmt.Sprintf("%d", *ip)
}

func maybeEffect(fp *float64) string {
	if fp == nil {
		return "N/A"
	}
	return fmt.Sprintf("%.4f", *fp)
}

func formatResidual(r float64) string {
	return fmt.Sprintf("%+.2f", r)
}

func formatContribution(c float64) string {
	return fmt.Sprintf("%.4f", c)
}

func formatExpected(e float64) string {
	return fmt.Sprintf("%.2f", e)
}

func formatDeviation(d float64) string {
	return fmt.Sprintf("%+.2f%%", d)
}

func flaggedLabel(flagged bool) string {
	if flagged {
		return "Yes"
	}
	return "No"
}

func testVerdict(t *analysis.ChiSquaredResult) string {
	if t == nil {
		return "skipped"
	}
	if t.P < 0.01 {
		return "flagged (p < 0.01)"
	}
	return "not flagged (p >= 0.01)"
}

func reportHeader(title, linkFmt string, reportDate, generatedAt time.Time, loc *time.Location) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", title)
	fmt.Fprintf(&b, "Generated: %s\n", generatedAt.In(loc).Format(time.RFC3339))
	fmt.Fprintf(&b, "Report date: %s\n", reportDate.In(loc).Format("2006-01-02"))
	filename := reportDate.Format("20060102")
	fmt.Fprintf(&b, linkFmt, filename, filename)
	return b.String()
}

func snapshotBlockSummary(r analysis.Result, loc *time.Location, reportFilename string, threshold int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#### Snapshot: %s\n", r.SnapshotID)
	fmt.Fprintf(&b, "- **Captured at**: %s\n", r.CapturedAt.In(loc).Format(time.RFC3339))
	fmt.Fprintf(&b, "- **Categories (k)**: %d\n", r.CategoryCount)
	fmt.Fprintf(&b, "- **Total plays (N)**: %d\n", r.TotalPlays)
	fmt.Fprintf(&b, "- **Filters**: shuffle only, skipped plays included\n")

	if r.Skipped {
		fmt.Fprintf(&b, "- **Skipped**: %s\n", r.SkipReason)
		b.WriteString("\n**Raw counts:**\n\n")
		b.WriteString(rowTableSkipped(r.TrackRows))
		if len(r.ArtistRows) > 0 {
			b.WriteString("\n**Artist raw counts:**\n\n")
			b.WriteString(rowTableSkipped(r.ArtistRows))
		}
		b.WriteString("\n")
		return b.String()
	}

	b.WriteString("\n")
	b.WriteString(trackBlockSummary(r, threshold))
	if len(r.ArtistRows) > 0 && r.ArtistTest != nil {
		b.WriteString(artistBlockSummary(r, threshold))
	}

	fmt.Fprintf(&b, "Full tables: [%s-data.md](%s-data.md)\n\n", reportFilename, reportFilename)
	return b.String()
}

func trackBlockSummary(r analysis.Result, threshold int) string {
	if r.TrackTest == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("##### Track test\n")
	fmt.Fprintf(&b, "- χ² = %s, df = %s, p = %s, V = %s\n",
		maybeChi2(&r.TrackTest.Chi2), maybeDF(&r.TrackTest.DF), maybeP(&r.TrackTest.P), maybeEffect(&r.TrackTest.Effect))
	fmt.Fprintf(&b, "- **Result**: %s\n\n", testVerdict(r.TrackTest))

	if r.TrackTest.MinExpected < 5 {
		fmt.Fprintf(&b, "> ⚠️ **Track test warning**: minimum expected count = %.2f (< 5), chi-squared result may be unreliable.\n\n", r.TrackTest.MinExpected)
	}

	b.WriteString(flaggedSection(r.TrackRows, threshold))

	b.WriteString("###### Top 10 tracks (by χ² contribution)\n\n")
	b.WriteString(rowTable(r.TrackRows, 10))
	b.WriteString("\n")
	return b.String()
}

func artistBlockSummary(r analysis.Result, threshold int) string {
	if r.ArtistTest == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("##### Artist test\n")
	fmt.Fprintf(&b, "- χ² = %s, df = %s, p = %s, V = %s\n",
		maybeChi2(&r.ArtistTest.Chi2), maybeDF(&r.ArtistTest.DF), maybeP(&r.ArtistTest.P), maybeEffect(&r.ArtistTest.Effect))
	fmt.Fprintf(&b, "- **Result**: %s\n\n", testVerdict(r.ArtistTest))

	if r.ArtistTest.MinExpected < 5 {
		fmt.Fprintf(&b, "> ⚠️ **Artist test warning**: minimum expected count = %.2f (< 5), chi-squared result may be unreliable.\n\n", r.ArtistTest.MinExpected)
	}

	b.WriteString(flaggedSection(r.ArtistRows, threshold))

	b.WriteString("###### Top 10 artists (by χ² contribution)\n\n")
	b.WriteString(rowTable(r.ArtistRows, 10))
	b.WriteString("\n")
	return b.String()
}

// snapshotBlockData renders the data-appendix version of a snapshot (full tables, no summaries).
func snapshotBlockData(r analysis.Result, loc *time.Location, threshold int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#### Snapshot: %s\n", r.SnapshotID)
	fmt.Fprintf(&b, "- **Captured at**: %s\n", r.CapturedAt.In(loc).Format(time.RFC3339))
	fmt.Fprintf(&b, "- **Categories (k)**: %d\n", r.CategoryCount)
	fmt.Fprintf(&b, "- **Total plays (N)**: %d\n", r.TotalPlays)
	fmt.Fprintf(&b, "- **Filters**: shuffle only, skipped plays included\n")

	if !r.Skipped {
		if r.TrackTest != nil {
			fmt.Fprintf(&b, "- **Track test**: χ² = %s, df = %s, p = %s, V = %s\n",
				maybeChi2(&r.TrackTest.Chi2), maybeDF(&r.TrackTest.DF), maybeP(&r.TrackTest.P), maybeEffect(&r.TrackTest.Effect))
		}
		if r.ArtistTest != nil {
			fmt.Fprintf(&b, "- **Artist test**: χ² = %s, df = %s, p = %s, V = %s\n",
				maybeChi2(&r.ArtistTest.Chi2), maybeDF(&r.ArtistTest.DF), maybeP(&r.ArtistTest.P), maybeEffect(&r.ArtistTest.Effect))
		}
		fmt.Fprintf(&b, "- **Residual threshold**: |residual| > %d\n", threshold)
	}

	if r.Skipped {
		fmt.Fprintf(&b, "- **Skipped**: %s\n\n", r.SkipReason)
	} else {
		b.WriteString("\n")
	}

	b.WriteString("**Full per-track table**\n\n")
	if r.Skipped {
		b.WriteString(rowTableSkipped(r.TrackRows))
	} else {
		b.WriteString(rowTableFull(r.TrackRows))
	}

	if len(r.ArtistRows) > 0 {
		b.WriteString("\n**Full per-artist table**\n\n")
		if r.Skipped {
			b.WriteString(rowTableSkipped(r.ArtistRows))
		} else {
			b.WriteString(rowTableFull(r.ArtistRows))
		}
	}

	b.WriteString("\n")
	return b.String()
}

type rowData interface {
	analysis.TrackRow | analysis.ArtistRow

	RowID() string
	RowLabel() string
	RowKind() string
	GetName() string
	GetObserved() int
	GetExpected() float64
	GetContribution() float64
	GetResidual() float64
	GetFlagged() bool
	GetDeviationPct() float64
}

// flaggedSection filters rows to those flagged by residual and renders a table.
func flaggedSection[R rowData](rows []R, threshold int) string {
	var b strings.Builder
	flagged := make([]R, 0)
	for _, row := range rows {
		if row.GetFlagged() {
			flagged = append(flagged, row)
		}
	}
	kind := rows[0].RowKind()
	fmt.Fprintf(&b, "###### Flagged %ss (|residual| > %d)\n\n", kind, threshold)
	if len(flagged) == 0 {
		fmt.Fprintf(&b, "No %ss flagged by residual.\n\n", kind)
		return b.String()
	}
	fmt.Fprintf(&b, "| %s | Name | Observed | Expected | Residual | Direction |\n", rows[0].RowLabel())
	b.WriteString("|---|---|---|---|---|---|\n")
	for _, row := range flagged {
		dir := "over"
		if row.GetResidual() < 0 {
			dir = "under"
		}
		fmt.Fprintf(&b, "| %s | %s | %d | %s | %s | %s |\n",
			row.RowID(), row.GetName(), row.GetObserved(), formatExpected(row.GetExpected()),
			formatResidual(row.GetResidual()), dir)
	}
	b.WriteString("\n")
	return b.String()
}

// rowTable renders a per-item markdown table. limit > 0 truncates rows; 0 means no limit.
func rowTable[R rowData](rows []R, limit int) string {
	if len(rows) == 0 {
		return ""
	}
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	var b strings.Builder
	label := rows[0].RowLabel()
	fmt.Fprintf(&b, "| %s | Name | O | E | χ² contrib | Residual | Flagged | Deviation |\n", label)
	b.WriteString("|---|---|---|---|---|---|---|---|\n")
	for _, row := range rows {
		fmt.Fprintf(&b, "| %s | %s | %d | %s | %s | %s | %s | %s |\n",
			row.RowID(), row.GetName(), row.GetObserved(), formatExpected(row.GetExpected()),
			formatContribution(row.GetContribution()), formatResidual(row.GetResidual()),
			flaggedLabel(row.GetFlagged()), formatDeviation(row.GetDeviationPct()))
	}
	return b.String()
}

func rowTableFull[R rowData](rows []R) string {
	return rowTable(rows, 0)
}

// rowTableSkipped renders a minimal table for skipped snapshots.
// Columns depend on whether expected counts are available (they may be zero if skipped early).
func rowTableSkipped[R rowData](rows []R) string {
	if len(rows) == 0 {
		return ""
	}
	hasExpected := rows[0].GetExpected() != 0
	var b strings.Builder
	label := rows[0].RowLabel()
	if hasExpected {
		fmt.Fprintf(&b, "| %s | Name | Observed | Expected |\n", label)
		b.WriteString("|---|---|---|---|\n")
		for _, row := range rows {
			fmt.Fprintf(&b, "| %s | %s | %d | %s |\n",
				row.RowID(), row.GetName(), row.GetObserved(), formatExpected(row.GetExpected()))
		}
	} else {
		fmt.Fprintf(&b, "| %s | Name | Observed |\n", label)
		b.WriteString("|---|---|---|\n")
		for _, row := range rows {
			fmt.Fprintf(&b, "| %s | %s | %d |\n", row.RowID(), row.GetName(), row.GetObserved())
		}
	}
	return b.String()
}

// reportFooter returns the methodology notes appended to all reports.
func reportFooter() string {
	return strings.TrimSpace(`
**Notes**

- **Reproducibility:** All raw counts, expected counts, and test statistics are shown above. No database access is required to verify the results.
- **Chi-squared GOF:** χ² = Σ (O−E)²/E, df = k−1, p-value from regularized upper incomplete gamma function Q(df/2, χ²/2).
- **Cramér's V (GOF variant):** V = sqrt(χ² / (M · (k−1))), bounded [0,1]. Interpretation: 0.1 small, 0.3 medium, 0.5 large.
- **Standardized residual:** rᵢ = (Oᵢ−Eᵢ) / √Eᵢ ~ N(0,1) under H₀. Flagged if |rᵢ| > threshold.
- **Cumulative-to-date:** Analysis includes all plays up to the report time per (user, playlist, snapshot). Does not use a rolling window.
- **Per-snapshot granularity:** Each chi-squared test runs per individual (playlist, snapshot) pair for statistical correctness. Frequently-edited playlists with few plays per snapshot may produce many "insufficient data" (M < threshold) entries. No aggregate per-playlist fallback is applied.
- **Residual false-positive rate:** At |r| > 3, expect ~0.27% × k false flags per snapshot by chance under uniform shuffle (no multiple-comparison correction applied).
`)
}

// RenderSummary builds the high-level summary report markdown, grouped by user and playlist.
func RenderSummary(reportDate, generatedAt time.Time, loc *time.Location, results []analysis.Result, residualThreshold int) string {
	var b strings.Builder
	b.WriteString(reportHeader("# Shuffle Randomness Report", "Full data: [%s-data.md](%s-data.md)\n", reportDate, generatedAt, loc))
	if len(results) == 0 {
		b.WriteString("\n---\n\nNo shuffle plays recorded yet for any account.\n\n")
		return b.String()
	}
	b.WriteString("\n")

	reportFilename := reportDate.Format("20060102")

	groups := groupResults(results)
	for _, ag := range groups {
		fmt.Fprintf(&b, "## %s (%s)\n\n", ag.DisplayName, ag.UserID)
		for _, pg := range ag.playlists {
			fmt.Fprintf(&b, "### Playlist: %s (%s)\n\n", pg.PlaylistName, pg.PlaylistID)
			fmt.Fprintf(&b, "Snapshots analyzed: %d\n\n", len(pg.snapshots))
			for _, snap := range pg.snapshots {
				b.WriteString(snapshotBlockSummary(snap, loc, reportFilename, residualThreshold))
			}
		}
	}

	b.WriteString("---\n\n")
	b.WriteString(reportFooter())
	b.WriteString("\n")
	return b.String()
}

// RenderData builds the full data-appendix markdown with complete per-snapshot tables.
func RenderData(reportDate, generatedAt time.Time, loc *time.Location, results []analysis.Result, residualThreshold int) string {
	var b strings.Builder
	b.WriteString(reportHeader("# Shuffle Randomness Report — Data Appendix", "Data appendix for [%s.md](%s.md)\n", reportDate, generatedAt, loc))
	if len(results) == 0 {
		b.WriteString("\n---\n\nNo shuffle plays recorded yet for any account.\n\n")
		return b.String()
	}
	b.WriteString("\n")

	groups := groupResults(results)
	for _, ag := range groups {
		fmt.Fprintf(&b, "## %s (%s)\n\n", ag.DisplayName, ag.UserID)
		for _, pg := range ag.playlists {
			fmt.Fprintf(&b, "### Playlist: %s (%s)\n\n", pg.PlaylistName, pg.PlaylistID)
			for _, snap := range pg.snapshots {
				b.WriteString(snapshotBlockData(snap, loc, residualThreshold))
				b.WriteString("---\n\n")
			}
		}
	}

	b.WriteString(reportFooter())
	b.WriteString("\n")
	return b.String()
}

// WriteAll atomically writes summary and data markdown files (prevents partial writes).
func WriteAll(dir string, date time.Time, summary, data string) error {
	filename := date.Format("20060102")
	summaryPath := filepath.Join(dir, filename+".md")
	dataPath := filepath.Join(dir, filename+"-data.md")

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	for _, p := range []string{summaryPath, dataPath} {
		if _, err := os.Stat(p); err == nil {
			return fmt.Errorf("refusing to overwrite existing file: %s", p)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("stat %s: %w", p, err)
		}
	}

	summaryTmp := summaryPath + ".tmp"
	dataTmp := dataPath + ".tmp"

	cleanup := func() {
		os.Remove(summaryTmp)
		os.Remove(dataTmp)
	}

	if err := os.WriteFile(summaryTmp, []byte(summary), 0644); err != nil {
		cleanup()
		return fmt.Errorf("write temporary file %s: %w", summaryTmp, err)
	}
	if err := os.WriteFile(dataTmp, []byte(data), 0644); err != nil {
		cleanup()
		return fmt.Errorf("write temporary file %s: %w", dataTmp, err)
	}
	// Rename data first so that if it fails, no final files exist.
	// If summary rename fails after data succeeded, roll back the data file.
	if err := os.Rename(dataTmp, dataPath); err != nil {
		cleanup()
		return fmt.Errorf("rename %s → %s: %w", dataTmp, dataPath, err)
	}
	if err := os.Rename(summaryTmp, summaryPath); err != nil {
		os.Remove(dataPath)
		cleanup()
		return fmt.Errorf("rename %s → %s: %w", summaryTmp, summaryPath, err)
	}

	return nil
}
