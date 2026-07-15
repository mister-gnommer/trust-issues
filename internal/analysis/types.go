package analysis

import "time"

// ChiSquaredResult holds the complete chi-squared test output for one
// dimension (tracks or artists). MinExpected is always populated; the
// reporting layer checks whether it is < 5 to emit a reliability warning.
type ChiSquaredResult struct {
	Chi2        float64
	DF          int
	P           float64
	Effect      float64 // Cramér's V
	MinExpected float64 // always populated; check < 5 for warning
}

// Result is the output of a single chi-squared analysis run for one
// snapshot/playlist pair. It carries the identifying context (which user,
// playlist, snapshot), the test statistics for the track and artist
// distributions (nil when the test was skipped), and the per-row
// breakdown used to render the report.
//
// Descriptive names are used for readability. Literature mapping:
//
//	CategoryCount → k  (number of categories: tracks or artists)
//	TotalPlays    → N  (total observations: shuffle plays)

type Result struct {
	UserID      string
	DisplayName string

	SnapshotID    string
	PlaylistID    string
	PlaylistName  string
	CapturedAt    time.Time
	CategoryCount int // k in chi-squared literature: number of categories (tracks or artists)
	TotalPlays    int // N in chi-squared literature: total observations (shuffle plays)

	TrackTest  *ChiSquaredResult // chi-squared result for the track distribution; nil when skipped
	ArtistTest *ChiSquaredResult // chi-squared result for the artist distribution; nil when skipped or no artist data
	Skipped    bool
	SkipReason string

	TrackRows  []TrackRow
	ArtistRows []ArtistRow
}

type TrackRow struct {
	TrackID      string
	Name         string
	Observed     int
	Expected     float64
	Contribution float64
	Residual     float64
	Flagged      bool
	DeviationPct float64
}

func (r TrackRow) RowID() string            { return r.TrackID }
func (r TrackRow) RowLabel() string         { return "Track ID" }
func (r TrackRow) RowKind() string          { return "track" }
func (r TrackRow) GetName() string          { return r.Name }
func (r TrackRow) GetObserved() int         { return r.Observed }
func (r TrackRow) GetExpected() float64     { return r.Expected }
func (r TrackRow) GetContribution() float64 { return r.Contribution }
func (r TrackRow) GetResidual() float64     { return r.Residual }
func (r TrackRow) GetFlagged() bool         { return r.Flagged }
func (r TrackRow) GetDeviationPct() float64 { return r.DeviationPct }

type ArtistRow struct {
	ArtistID     string
	Name         string
	Observed     int
	Expected     float64
	Contribution float64
	Residual     float64
	Flagged      bool
	DeviationPct float64
}

func (r ArtistRow) RowID() string            { return r.ArtistID }
func (r ArtistRow) RowLabel() string         { return "Artist ID" }
func (r ArtistRow) RowKind() string          { return "artist" }
func (r ArtistRow) GetName() string          { return r.Name }
func (r ArtistRow) GetObserved() int         { return r.Observed }
func (r ArtistRow) GetExpected() float64     { return r.Expected }
func (r ArtistRow) GetContribution() float64 { return r.Contribution }
func (r ArtistRow) GetResidual() float64     { return r.Residual }
func (r ArtistRow) GetFlagged() bool         { return r.Flagged }
func (r ArtistRow) GetDeviationPct() float64 { return r.DeviationPct }
