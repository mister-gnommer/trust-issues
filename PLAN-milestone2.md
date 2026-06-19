> 🤖 AI-generated

# Milestone 2 — Implementation Plan

Scope (from `TODO.md` and `DESCRIPTION.md`):

- Chi-squared goodness-of-fit test (per-track and per-artist)
- Daily markdown reports to `reports/YYYYMMDD.md` + `reports/YYYYMMDD-data.md`
- Scheduled cron trigger

---

## Architecture

Two new internal packages, following the existing consumer-owns-interface convention:

```
internal/
  analysis/
    types.go     — Result, TrackRow, ArtistRow (analysis-specific output)
    chisq.go     — pure math: Statistic, PValue, EffectSize, Residual, gamma functions
    analysis.go  — Reader interface + Analyze function
  report/
    report.go    — Markdown renderer (pure functions: Results → string) + WriteAll
```

`SnapshotInfo` lives in `internal/store/types.go` (alongside `Snapshot`, `Play`) — it's a DB projection, and `analysis` imports `store` one-way, matching the existing `playback.Recorder` pattern (`playback` imports `store`, uses `store.Play`/`store.Artist`).

Scheduling is delegated to the OS — no in-process scheduler. The binary gains a `--report-once` flag that runs analysis + writes the report + exits. A systemd timer (or cron, as fallback) invokes that mode on a daily schedule.

```
deploy/
  trust-issues.service           — existing: long-running poller + syncer
  trust-issues-report.service    — new: oneshot, runs `trust-issues --report-once`
  trust-issues-report.timer      — new: OnCalendar=*-*-* 03:00:00 Europe/Warsaw, triggers the oneshot
```

Plus: new `Reader` interface in `analysis` (the store satisfies it), new store query methods, new `[reports]` config block.

**No new external Go dependencies.** Chi-squared CDF is pure Go.
**No schema changes** — new methods query existing `playlist_snapshot_tracks`, `track_artists`, `plays` tables. `currentSchemaVersion` unchanged.

---

## Conventions (applies to all new/modified files)

- **Attribution:** every new Go file gets `// 🤖 AI-generated` at the top; the two new systemd units get `# 🤖 AI-generated`. Runtime-generated report `.md` files are output, not source — exempt. Autonomous modifications to existing files use `// 🤖 AI-start` / `// 🤖 AI-end` fences around added code.
- **Errors:** wrap via `fmt.Errorf("context: %w", err)`. Never discard errors silently.
- **Logging:** `log/slog` with structured key-value pairs. `analysis`/`report`/`--report-once` path logs with a `"component"` key (e.g. `"component", "analysis"`). Pure render functions (`RenderSummary`/`RenderData`) stay log-free.
- **Dates and timezones:** all times displayed in the report (filename date, dates in markdown body, generation timestamp, snapshot `captured_at`) use `Europe/Warsaw` timezone (CET/CEST with automatic DST). Filenames use Go layout `20060102`; dates printed in markdown use `2006-01-02`; timestamps use RFC3339 (`2006-01-02T15:04:05Z07:00` — the offset will be `+01:00` or `+02:00` depending on DST).

---

## Decisions

All decisions confirmed. v1 behavior is fixed; deferred items are tracked in `TODO.md`.

### D1. Analysis time window
Cumulative-to-date (all plays up to report time) per `(user, playlist, snapshot)`. Matches the DESCRIPTION example ("within tolerance in April, drifted out in May"). Chi-squared needs samples; a single day is usually too few. Rolling-window trend analysis is out of scope for v1.

**Limitation acknowledged:** Chi-squared is consistent — as M grows, even tiny deviations become statistically significant. After months of data, the "flagged?" column will be true for most snapshots. The flag means "statistically significant," not "practically non-random." Cramér's V (D14) addresses this by providing an effect-size measure that stays constant as M grows with a fixed deviation rate. The raw p-value + full observed/expected tables are always printed so the user can judge practical significance manually.

### D2. Snapshot granularity for expected counts
Group plays by `playlist_snapshot_id` and run a separate chi-squared test per `(playlist, snapshot)`. Statistically correct: each play's expected probability came from the snapshot active *at that play*. The report shows one chi-squared result per snapshot within each playlist.

**Tradeoff acknowledged:** Frequently-edited playlists have few plays per snapshot → often hit M<30 (D8) → report full of "insufficient data" entries. This is the cost of per-snapshot correctness. An aggregate per-playlist fallback would contradict D2. Documented in report footer.

### D3. Collab artist counting
Count once per artist on a played track — a track with artists `a1, a2` contributes +1 to each.

**Per-artist expected normalization:** For the per-artist test, `M_artist = Σ observed_artist` (total play-artist pairs). `expected_a = K_a · M_artist / (Σ K_a)`. This ensures `ΣE = M_artist = ΣO` exactly, which chi-squared goodness-of-fit requires. The DESCRIPTION formula `K·M/N` is the expected value under H₀; normalization adjusts for the specific sample's collab distribution. For playlists with no collabs (the common case), `M_artist = M` and `ΣK_a = N`, so `expected_a = K_a · M / N` — matches DESCRIPTION exactly.

Only-primary-artist and fractional-count alternatives were rejected.

### D4. Skipped plays
Include all plays (skipped + non-skipped) by default; state this prominently in the report. No config knob in v1. A knob to filter skipped plays out of analysis is deferred to `TODO.md`.

### D5. Report trigger mechanism
OS-level scheduling, no in-process scheduler:

1. **systemd timer (primary):** a oneshot `trust-issues-report.service` runs `trust-issues --report-once`, paired with a `trust-issues-report.timer` using `OnCalendar=*-*-* 03:00:00` with `Timezone=Europe/Warsaw` (see D12). Gives journal logging, retries, and `systemctl list-timers` visibility for free.
2. **cron (fallback):** a single crontab line for non-systemd boxes (Alpine, containers, macOS dev): `0 3 * * * /opt/trust-issues/trust-issues -config /opt/trust-issues/config.toml -report-once`. Note: cron uses the server's timezone; set it to `Europe/Warsaw` if the server isn't already.
3. **`--report-once` flag** on the binary: runs analysis + writes reports + exits 0. Also useful for manual backfills, ad-hoc runs, and tests — independent of any scheduler.

Fire time is 03:00 `Europe/Warsaw` (CET/CEST), configured in the systemd unit / crontab (not in `config.toml`). **DST note:** in DST transitions, the timer may fire once or shift by an hour; harmless for daily reports (noted in README).

**Persistent=true catch-up:** If the VPS was off for N days, the next run produces ONE report covering all accumulated plays (D1 cumulative-to-date), not N backfill reports. Intentional — documented in plan §8 + README.

### D6. CLI shape
Keep the current `flag` package (no cobra). Add `--report-once` boolean. If set, run report and exit 0 without starting pollers. True subcommands (`trust-issues report`) are out of scope for v1.

### D7. Report content — full tables
DESCRIPTION's reproducibility requirement is explicit: per-track AND per-artist play counts, expected counts, chi-squared stat, df, p-value, snapshot ID + track count must all be in the report. Full tables for every track and every artist, sorted by `|observed − expected|` descending (most anomalous on top). That's the spec.

### D8. Minimum plays per snapshot + statistical validity
Chi-squared is unreliable with tiny samples. Skip the test if `M < 30` (configurable via `reports.min_plays`), still print the raw observed/expected table with a note "insufficient data for chi-squared".

**Expected-count warning:** M≥30 is a bare minimum. The standard chi-squared assumption is expected ≥ 5 per category. With M=30 and N=2000, expected=0.015/track — the test runs but is statistically unreliable. The report prints a warning when any expected count < 5: "chi-squared result may be unreliable (min expected = X)". Strict M≥5N would require ~10000 plays for a 2000-track playlist — likely never reached, so we keep M≥30 as floor + warn rather than skip.

**Degrees of freedom:** per-track df = N − 1 (N = tracks in snapshot); per-artist df = A − 1 (A = distinct artists in snapshot).

**Edge cases:** Skip chi-squared if N < 2 (df < 1, degenerate) — still print raw counts with "N<2, chi-squared not applicable." `PValue` errors for df < 1.

### D9. Existing report file behavior + atomicity
Refuse to overwrite an existing `reports/YYYYMMDD.md` (or `YYYYMMDD-data.md`) — error out. Suffix `-1`, `-2`, … for same-day re-runs is deferred to `TODO.md`.

**Atomic writes:** `WriteAll` writes to `.tmp` files first, then `os.Rename` to final names (atomic on POSIX). Pre-check both final names before writing — if either exists, refuse. Reduces TOCTOU race to milliseconds. Full `flock` safety is overkill for v1 (timer fires once daily; concurrent manual runs are rare) — noted as a known minor gap.

**Exit-code semantics:** If both files already exist (prior successful run), log "report already generated" and exit 0 (idempotent no-op, not "failed"). Reserve non-zero exit for actual write/analysis failures.

### D10. One file pair for all accounts
One file pair per day containing all accounts as sections — NOT one file pair per account. `Analyze` is called per account → `[]analysis.Result` collected across all accounts → `RenderSummary`/`RenderData`/`WriteAll` called **once** with the combined slice. Matches the summary layout's "per account" headings and DESCRIPTION's `reports/YYYYMMDD.md` convention.

Split into two files. A 2000-track playlist produces thousands of table rows; a single file buries the headline at line ~4000.

- `reports/YYYYMMDD.md` — scannable summary: per-account overview, per-snapshot chi²/df/p/flagged, top 10 most-anomalous tracks + artists inline (sorted by contribution desc), link to the data file.
- `reports/YYYYMMDD-data.md` — full per-track and per-artist tables (the reproducibility appendix).

Both files together satisfy DESCRIPTION's "no DB access required" rule. Same overwrite-rule (D9) applies to both files.

### D11. Flagging threshold
"Flagged?" column uses **p < 0.01** as the threshold. The raw uncorrected p-value is always printed alongside the flag for transparency.

**Multiple-testing correction:** NOT applied in v1. With dozens of snapshots × tests, flat p<0.01 will produce some false positives. However, the DESCRIPTION spec describes p-value-based flagging without correction, and the full observed/expected tables are always printed for manual judgment. Bonferroni correction is deferred to `TODO.md`.

### D12. Timezone — all times in Europe/Warsaw (CET/CEST)
All times in the report — filename date, dates in markdown body, generation timestamp, snapshot `captured_at` — are displayed in `Europe/Warsaw` timezone (CET in winter, CEST in summer, automatic DST). This applies regardless of the server's timezone.

- **Filename date:** `Europe/Warsaw` date at report generation time. Go layout: `20060102`.
- **Displayed dates in markdown:** `2006-01-02` (per global `yyyy-MM-dd` rule for data/UI).
- **Generation timestamp:** RFC3339 in `Europe/Warsaw` (`2006-01-02T15:04:05Z07:00` — offset `+01:00` or `+02:00` depending on DST).
- **Render functions are pure:** `RenderSummary`/`RenderData` take `reportDate time.Time`, `generatedAt time.Time`, and `loc *time.Location` as parameters — no `time.Now()` inside (required for golden-file testing). All `time.Time` values in `Result` (e.g. `CapturedAt`) are converted to `loc` for display by the render functions.
- **systemd timer:** uses `Timezone=Europe/Warsaw` (or equivalent — confirm via context7-mcp during implementation). If the systemd version doesn't support it, set the server timezone to `Europe/Warsaw`.

### D13. Error handling in `--report-once` loop
- Per-account `Analyze` errors: logged, that account's results omitted, loop continues.
- `WriteAll` failure: log + exit 1.
- All accounts fail: exit 1.
- ≥1 account succeeded: exit 0.
- Lets systemd distinguish success from failure while degrading gracefully.

### D14. Effect-size measure (Cramér's V)
The p-value alone grows insignificant as M accumulates (chi-squared is consistent), so eventually every snapshot gets flagged. An effect-size measure stays constant as M grows with a fixed deviation rate, letting the user judge *practical* significance alongside statistical significance.

**Formula (GOF variant):** `V = sqrt(chi² / (M · (k−1)))` where k = categories (N for tracks, A for artists). Bounded [0,1]: 0 = perfect fit, 1 = maximum deviation. Computed for both per-track and per-artist tests. Displayed in the summary's per-snapshot block alongside chi²/df/p. Not gated on a threshold in v1 — just displayed. Interpretation thresholds (approximate, adapted from Cohen's w conventions): 0.1 small, 0.3 medium, 0.5 large.

**Why V over Cohen's w:** The user's playlists range from <100 tracks to >600 tracks (favourites). V is bounded [0,1] regardless of k, so a V of 0.15 means the same thing for a 600-track playlist and a 50-track playlist — effect sizes are comparable across playlists of different sizes. Cohen's w = `sqrt(chi²/M)` has max = sqrt(k−1), which grows with k, so the same w value means different things for different playlist sizes (w=0.3 is significant for k=50 but not for k=600). V's normalization makes cross-playlist comparison meaningful.

**Dilution caveat:** V's `(k−1)` normalization dilutes local anomalies in large-k playlists — one track at 3× expected in a 600-track playlist gives V≈0.008 (negligible) even though the deviation is real. This is mitigated by D15's standardized residuals, which surface local anomalies regardless of the global V value. V and residuals are complementary: V for "how biased is this playlist overall, comparable across playlists," residuals for "which specific tracks are anomalous."

### D15. Per-track standardized residuals — second, independent signal
The global chi-squared asks "is the *overall* distribution unusual?" With large k, a single track with a strong local anomaly (e.g. 3× expected) gets diluted into the aggregate and may not move the global p-value at all — one track at 15 vs 5 expected (M=500, k=100) contributes ~99% of chi² yet global p ≈ 1 because the other 99 tracks are uniform. The residuals surface those local anomalies regardless of the global verdict.

**Formula:** `r_i = (O_i − E_i) / sqrt(E_i)` — standardized residual. Under H₀, `r_i ~ N(0,1)` approximately. Computed for every track and every artist in each snapshot.

**Flag threshold:** `|r_i| > 3` (configurable via `reports.residual_threshold`, default 3). ~0.27% two-tailed per category under H₀. A track can be flagged even when the global test passes — surfaced separately, not swallowed by the aggregate.

**Ranking note:** `|r_i| = sqrt(contribution)`, and `TrackRows`/`ArtistRows` are already sorted by contribution desc (D7) — so the ranking is identical. The *new* work is the threshold flag and the separate "flagged tracks" section in the summary, not a new sort.

**Applied to both tracks AND artists** — the per-artist test has the same dilution issue when many artists share the playlist.

**Self-gating at low N:** residuals can't reach 3 without meaningful absolute deviation (at E_i=0.3 you need O_i≥3 to hit |r_i|≈4.9), so they won't fire spuriously at low M. But the normal approximation to Poisson degrades at E_i<1, so residual *p-values* are unreliable at low expected counts — the *flag* is still useful as a "look here" signal since the raw counts are always shown. No separate min-N for residuals; D8's M<30 floor covers the global test, and residuals are mathematically self-limiting below that.

**Multiple-comparisons:** With k×snapshots residuals examined daily, |r_i|>3 produces ~0.27% × k false flags per snapshot by chance (e.g. ~8/day for k=100 × 30 snapshots). v1 ships the simple threshold with the rate documented in the report footer; FDR/BH correction deferred to TODO (consistent with D11's Bonferroni deferral). The residual flag is a "look here" signal, not a definitive verdict — raw observed/expected counts are always shown next to the flag, so false positives are eyeballable.

---

## Detailed plan per component

### 1. `internal/analysis/chisq.go` — pure math

```go
// Statistic computes Σ (o−e)²/e and df = len(observed)−1.
// Errors if len(observed) ≠ len(expected), any expected[i] ≤ 0, or slices empty.
// For len==1 returns (0.0, 0, nil) — df=0; caller handles via PValue error.
func Statistic(observed, expected []float64) (chi2 float64, df int, err error)

// PValue returns the upper-tail p-value via Q(df/2, chi2/2).
// Errors if df < 1 or chi2 < 0. Returns 1.0 for chi2 == 0 (any valid df).
func PValue(chi2 float64, df int) (float64, error)

// LowerRegGamma — series expansion for x < s+1 (small chi2 → p close to 1).
// UpperRegGamma — Lentz continued fraction for x ≥ s+1 (large chi2 → p close to 0).
// PValue dispatches on chi2/2 < df/2+1.
func LowerRegGamma(s, x float64) float64
func UpperRegGamma(s, x float64) float64

// EffectSize returns Cramér's V for GOF: V = sqrt(chi2 / (M * (k-1))).
// k = number of categories (len(observed)). Bounded [0,1].
// Errors if M <= 0 or k < 2 (degenerate). See D14.
func EffectSize(chi2 float64, M int, k int) (float64, error)

// Residual returns the standardized residual r_i = (observed - expected) / sqrt(expected).
// Sign indicates over (+) or under (−) representation. |r_i| > threshold → flag (D15).
// Errors if expected <= 0.
func Residual(observed int, expected float64) (float64, error)
```

**Two-branch algorithm:** Numerical Recipes §6.2 uses series expansion (`gser`) for `x < s+1` and Lentz continued fraction (`gcf`) for `x ≥ s+1`. Lentz alone loses precision for small chi2. `PValue` dispatches on `chi2/2 < df/2+1`. ~80 lines total for both branches.

**Error conditions:** `Statistic` errors on mismatched lengths, empty slices, or any `expected[i] ≤ 0` (zero-expected is a caller bug, not a skip signal). `PValue` errors on `df < 1` or `chi2 < 0`.

### 2. `internal/analysis/types.go` — analysis-specific output types

```go
// Result holds chi-squared results for one (playlist, snapshot).
type Result struct {
    UserID, DisplayName string

    SnapshotID, PlaylistID, PlaylistName string
    CapturedAt time.Time
    N int  // tracks in snapshot
    M int  // shuffle plays (after orphan filtering)

    // Per-track test — nil pointers when skipped (D8)
    TrackChi2     *float64
    TrackDF       *int
    TrackP        *float64
    TrackEffect   *float64  // Cramér's V (D14); nil when skipped
    Skipped       bool
    SkipReason    string    // "M < min_plays", "N < 2", etc.

    // Per-artist test — same nullable pattern
    ArtistChi2    *float64
    ArtistDF      *int
    ArtistP       *float64
    ArtistEffect  *float64  // Cramér's V (D14)

    // Full rows sorted by contribution desc (equivalent to |residual| desc — D15)
    TrackRows     []TrackRow
    ArtistRows    []ArtistRow

    // Warning if any expected < 5 (D8); 0 if no warning
    MinExpected   float64
}

type TrackRow struct {
    TrackID, Name string
    Observed      int
    Expected      float64
    Contribution  float64  // (o−e)²/e
    Residual      float64  // r_i = (o−e)/sqrt(e) — signed (D15)
    Flagged       bool     // |Residual| > threshold (D15)
    DeviationPct  float64
}

type ArtistRow struct {
    ArtistID, Name string
    Observed       int
    Expected       float64
    Contribution   float64
    Residual       float64  // signed (D15)
    Flagged        bool     // |Residual| > threshold (D15)
    DeviationPct   float64
}
```

Nullable p-value/chi2/df use `*float64`/`*int` (nil when test skipped) — follows the project convention of pointers for nullable fields (see `store.Play.EndedAt *time.Time`, `store.Play.Skipped *bool`).

### 3. `internal/analysis/analysis.go` — Analyzer

```go
// Reader is the subset of *store.Store used by the analyzer.
// Defined here (consumer-owns-interface), satisfied implicitly by *store.Store.
type Reader interface {
    // Returns snapshots with ≥1 shuffle play (non-null context) for the user.
    SnapshotsWithShufflePlays(ctx context.Context, userID string) ([]store.SnapshotInfo, error)

    // Returns track IDs in the snapshot + N. Each track appears once (PK).
    SnapshotTrackIDs(ctx context.Context, snapshotID string) ([]string, int, error)

    // Returns K (tracks per artist) in the snapshot.
    ArtistTrackCounts(ctx context.Context, snapshotID string) (map[string]int, error)

    // Observed play counts per track — shuffle + non-null context, INNER JOINed
    // against playlist_snapshot_tracks (orphan plays dropped).
    PlayCountsByTrack(ctx context.Context, userID, snapshotID string) (map[string]int, error)

    // Same filter, joined through track_artists.
    PlayCountsByArtist(ctx context.Context, userID, snapshotID string) (map[string]int, error)

    TrackNames(ctx context.Context, trackIDs []string) (map[string]string, error)
    ArtistNames(ctx context.Context, artistIDs []string) (map[string]string, error)
}

// Analyze runs chi-squared tests for all snapshots of one user.
// minPlays is the D8 threshold (from config.reports.min_plays).
// residualThreshold is the D15 flag threshold (from config.reports.residual_threshold).
func Analyze(ctx context.Context, reader Reader, userID string, minPlays, residualThreshold int) ([]Result, error)
```

**Key logic:**
1. Get snapshots from `SnapshotsWithShufflePlays`.
2. For each snapshot:
   - Get track IDs + N from `SnapshotTrackIDs`. If N < 2 → `Skipped=true`, `SkipReason="N < 2"`, emit raw counts only (no chi²/w/residuals).
   - Get observed counts from `PlayCountsByTrack` (orphan plays already dropped by SQL INNER JOIN).
   - **Iterate over snapshot tracks:** `observed[i] = playCounts[trackID]` defaulting to 0. Never-played tracks contribute `(0−e)²/e = e` — omitting them understates chi-squared and hides non-randomness.
   - M = Σ observed. If M < minPlays → `Skipped=true`, `SkipReason="M < min_plays"`, emit raw counts only (no chi²/w/residuals).
   - expected[i] = M / N. If any expected < 5 → set `MinExpected` for warning (D8).
   - Call `Statistic` + `PValue` + `EffectSize(chi2, M, N)`. Build `TrackRows`: each row gets `Contribution`, `Residual` (signed), `Flagged = |Residual| > residualThreshold`. Sort by contribution desc (≡ |residual| desc).
   - Same for artists: iterate over `ArtistTrackCounts` keys, `M_artist = Σ observed_artist`, `expected_a = K_a · M_artist / (Σ K_a)` (D3 normalization). Call `Statistic` + `PValue` + `EffectSize(chi2, M_artist, A)` where A = distinct artists. Build `ArtistRows` with residual + flag.
3. Return `[]Result` — non-nil, possibly empty (new user / all non-shuffle).

**Residuals computed even when global test is skipped?** No — if M < minPlays or N < 2, residuals are not computed (the test is skipped entirely, raw counts only). Residuals are a post-hoc decomposition of the chi-squared test; running them when the test itself is skipped would be inconsistent. The `Flagged` field is false for all rows in skipped results.

### 4. `internal/report/report.go` — Markdown renderer

Two pure render functions + one writer:

```go
// Pure — no time.Now(), no I/O, no logging.
// loc is used to convert all time.Time values for display (D12).
func RenderSummary(reportDate, generatedAt time.Time, loc *time.Location, results []analysis.Result) string
func RenderData(reportDate, generatedAt time.Time, loc *time.Location, results []analysis.Result) string

// Writes dir/YYYYMMDD.md + dir/YYYYMMDD-data.md atomically (D9).
// Refuses overwrite (D9). Creates dir if missing.
// date is already in the target timezone — filename uses date.Format("20060102").
func WriteAll(dir string, date time.Time, summary, data string) error
```

**`WriteAll` atomicity:** pre-check both final paths; if either exists, return error. Write to `.tmp` files, `os.Rename` to final (atomic on POSIX). Clean up `.tmp` on any failure.

**Summary layout:**
- Header: title, generation timestamp (RFC3339 in `Europe/Warsaw`), report date `2006-01-02`, link to `YYYYMMDD-data.md`
- Per account: heading with display name + user_id
- Per playlist: name, id, snapshots analyzed, current snapshot
- Per snapshot: snapshot ID, captured_at (in `Europe/Warsaw`), N (track count), M (shuffle plays), filters applied (shuffle=true, skipped included)
  - **Per-track global block**: chi², df, p-value, Cramér's V (D14), flagged? (p < 0.01 per D11)
  - **Flagged tracks section (D15)** — tracks where `|residual| > threshold`, listed separately from the global verdict. A playlist can pass the global test and still appear here. Each entry: track_id, name, observed, expected, residual (signed), direction (over/under). Empty section rendered as "no tracks flagged by residual." Only present when the test was NOT skipped (M ≥ min_plays AND N ≥ 2).
  - **Top 10 tracks table** (by contribution desc ≡ |residual| desc): track_id, name, observed, expected, contribution, residual, flagged?, deviation %
  - **Per-artist global block** + **Flagged artists section** + **Top 10 artists table**: same shape
  - If `MinExpected > 0`: warning "chi-squared result may be unreliable (min expected = X)"
  - "Full tables: see YYYYMMDD-data.md" pointer
- **Empty results:** header + "No shuffle plays recorded yet for any account." footer.
- Footer: note on how to reproduce, formulae used (chi², p-value, Cramér's V, standardized residual), D1 cumulative-to-date limitation note, residual false-positive rate note ("at |r|>3, expect ~0.27% × k false flags per snapshot by chance under uniform shuffle")

**Data appendix layout (must be self-contained for verification):**
- Header: title, report date, "data appendix for YYYYMMDD.md"
- Per account → per playlist → per snapshot:
  - **Snapshot header block**: snapshot ID, captured_at (in `Europe/Warsaw`), N, M, per-track chi²/df/p/V, per-artist chi²/df/p/V, formulae used (chi², p-value, Cramér's V, standardized residual), filters applied, residual threshold
  - **Full per-track table**: track_id, name, observed, expected, (o−e)²/e contribution, residual (signed), flagged?, deviation %, sorted by contribution desc
  - **Full per-artist table**: same shape

### 5. Store additions (`internal/store/`)

**`types.go` — new type:**
```go
type SnapshotInfo struct {
    ID, UserID, PlaylistID, PlaylistName string
    CapturedAt time.Time
    N int  // track count in snapshot
}
```

**`store.go` — new methods to satisfy `analysis.Reader`:**
- `SnapshotsWithShufflePlays(ctx, userID) ([]SnapshotInfo, error)` — returns snapshots that have ≥1 shuffle play with non-null context.
- `SnapshotTrackIDs(ctx, snapshotID) ([]string, int, error)` — track IDs + N from `playlist_snapshot_tracks`.
- `ArtistTrackCounts(ctx, snapshotID) (map[string]int, error)` — K per artist, joins `playlist_snapshot_tracks → track_artists`.
- `PlayCountsByTrack(ctx, userID, snapshotID) (map[string]int, error)` — **WHERE** `user_id = ? AND playlist_snapshot_id = ? AND shuffle_state = 1 AND playlist_id IS NOT NULL`, **INNER JOIN** against `playlist_snapshot_tracks` on `track_id` (drops orphan plays — plays whose track isn't in the snapshot, per DESCRIPTION's 1-hour inconsistency window).
- `PlayCountsByArtist(ctx, userID, snapshotID) (map[string]int, error)` — same WHERE + INNER JOIN, then joins `track_artists`.
- `TrackNames(ctx, trackIDs) (map[string]string, error)`, `ArtistNames(ctx, artistIDs) (map[string]string, error)`.

Plain SQL with `?` placeholders. Each gets a unit test in `store_test.go` using the existing `:memory:` pattern.

### 6. Config additions (`internal/config/config.go`)

```go
type Reports struct {
    Dir               string `toml:"dir"`
    MinPlays          int    `toml:"min_plays"`
    ResidualThreshold int    `toml:"residual_threshold"`  // D15; default 3
}

type Config struct {
    App      App       `toml:"app"`
    Storage  Storage   `toml:"storage"`
    Reports  Reports   `toml:"reports"`   // NEW
    Accounts []Account `toml:"accounts"`
}
```

**Defaulting in code:** `Load()` calls `applyDefaults()` before `validate()`:
- If `Reports.Dir == ""` → set `"./reports"`
- If `Reports.MinPlays == 0` → set `30`
- If `Reports.ResidualThreshold == 0` → set `3`

**Mode-aware validation:** split into `validateForCapture()` and `validateForReport()`. `--report-once` uses `LoadForReport()` which requires only `storage.database_path` + `accounts[].user_id`/`display_name` — skips `client_id`/`client_secret`/`refresh_token` checks (report-only runs make no Spotify API calls). Normal `Load()` uses `validateForCapture()` (existing checks). Both modes apply `Reports` defaults.

**Extended `validate()`:** `reports.min_plays >= 1`, `reports.dir` non-empty after defaulting.

Update `config.example.toml` with commented `[reports]` block:
```toml
[reports]
dir                = "./reports"   # default
min_plays          = 30            # skip chi-squared if M < this (D8)
residual_threshold = 3             # flag track/artist if |residual| > this (D15)
```

### 7. `cmd/trust-issues/main.go` wiring

**Extract for testability:**
```go
func runReportOnce(ctx context.Context, logger *slog.Logger, cfg *config.Config, st *store.Store, now time.Time) error
func runPollers(ctx context.Context, logger *slog.Logger, cfg *config.Config, st *store.Store) error
```

`run()` dispatches based on `--report-once` flag:
- **If set:** `config.LoadForReport(*configPath)` → `store.New` → `runReportOnce(ctx, logger, cfg, st, time.Now())` → exit 0/1 per D13. Does NOT start pollers or syncers.
- **Else:** `config.Load(*configPath)` → `store.New` → `runPollers` (existing `errgroup` behavior). No scheduler goroutine.

**`runReportOnce` logic (D10, D12, D13):**
1. `loc, err := time.LoadLocation("Europe/Warsaw")` — if error, log + return error.
2. `now = now.In(loc)` — convert to Europe/Warsaw (reassigns the param, not a new variable).
3. `results := []analysis.Result{}`
4. For each account: `analysis.Analyze(ctx, st, account.UserID, cfg.Reports.MinPlays, cfg.Reports.ResidualThreshold)` — on error, log + continue (D13).
5. `summary := report.RenderSummary(now, now, loc, results)`
6. `data := report.RenderData(now, now, loc, results)`
7. `report.WriteAll(cfg.Reports.Dir, now, summary, data)` — on error, return error → exit 1.

Keep `defer st.Close()`. Sequential loop over accounts is fine for read-only oneshot (no `errgroup` needed). Signal ctx optional but harmless to retain.

### 8. `deploy/trust-issues-report.service` + `deploy/trust-issues-report.timer`

**Before writing the timer unit, invoke context7-mcp** to confirm `OnCalendar=` grammar, `Persistent=` catch-up semantics, `Type=oneshot` pairing with timers, and `Timezone=` directive support for `Europe/Warsaw` (D12).

**`trust-issues-report.service`:**
```ini
# 🤖 AI-generated
# Install: same steps as trust-issues.service, plus:
#   sudo cp deploy/trust-issues-report.service /etc/systemd/system/
#   sudo cp deploy/trust-issues-report.timer /etc/systemd/system/
#   sudo systemctl daemon-reload
#   sudo systemctl enable --now trust-issues-report.timer
#   Check:  sudo systemctl list-timers trust-issues-report
#   Logs:   sudo journalctl -u trust-issues-report.service

[Unit]
Description=trust-issues — daily shuffle randomness report
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
User=trust-issues
Group=trust-issues
WorkingDirectory=/opt/trust-issues
ExecStart=/opt/trust-issues/trust-issues -config /opt/trust-issues/config.toml -report-once

StandardOutput=journal
StandardError=journal

# No Restart= — oneshot re-runs are timer-owned, not service-owned
TimeoutStartSec=600

# Security hardening — copied verbatim (systemd does NOT inherit between units)
NoNewPrivileges=true
ProtectHome=true
ProtectSystem=full
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

Note: `ProtectSystem=full` makes `/usr`, `/boot`, `/etc` read-only — does NOT block `/opt/trust-issues/reports` writes.

**`trust-issues-report.timer`:**
```ini
# 🤖 AI-generated

[Unit]
Description=Daily trust-issues report generation
Requires=trust-issues-report.service

[Timer]
OnCalendar=*-*-* 03:00:00
Timezone=Europe/Warsaw
Persistent=true
Unit=trust-issues-report.service

[Install]
WantedBy=timers.target
```

If the systemd version doesn't support `Timezone=`, fallback: set the server timezone to `Europe/Warsaw` (`timedatectl set-timezone Europe/Warsaw`) and omit the `Timezone=` line. Confirm via context7-mcp during implementation.

`Persistent=true` catch-up: if VPS was off for N days, fires ONCE on boot — produces one cumulative report (D1), not N backfill reports.

README notes the cron fallback line for non-systemd boxes + DST note.

---

## Test plan

### `chisq_test.go`
- Textbook p-values: chi2=3.84, df=1 → p≈0.05; chi2=11.07, df=5 → p≈0.05.
- **Tolerance:** ±1e-4 absolute for 0.01 ≤ p ≤ 0.99. For p→0: assert `p < 1e-6`. For p→1: assert `1−p < 1e-6`.
- **Edge cases:** table-driven `TestPValue_edgeCases`: `(0,1)→1.0`, `(0,100)→1.0`, `(3.84,0)→error`, `(-1,5)→error`, `(1e6,5)→<1e-300`, `(1e-10,5)→1.0`. `TestStatistic_mismatchedLengths→error`, `TestStatistic_empty→(0,-1,error)`, `TestStatistic_zeroExpected→error`.
- Series vs Lentz branch: verify both paths produce correct results (small chi2 exercises series, large exercises Lentz).
- **EffectSize (D14):** `TestEffectSize` — known case: chi2=20, M=500, k=100 → V=sqrt(20/(500·99))≈0.02. Errors: M=0, k=1. Bounded [0,1] check on a max-deviation case (all plays in one category → V=1.0).
- **Residual (D15):** `TestResidual` — `(15, 5) → +sqrt(20)≈+4.47`, `(0, 5) → -sqrt(5)≈-2.24`, sign correct. Errors: expected=0, expected<0.

### `analysis_test.go`
≥5 table-driven scenarios:
1. **Uniform** — high p, low V, no flagged tracks.
2. **Metallica-skewed** — low p, high V, Metallica flagged by residual.
3. **M < min_plays boundary (D8)** — M=29: `TrackChi2==nil`, `TrackEffect==nil`, `Skipped==true`, all `TrackRows[i].Flagged==false`, `TrackRows!=nil`. M=30: `TrackChi2!=nil`, `TrackEffect!=nil`.
4. **Orphan track** — play with track_id not in snapshot → excluded from M and chi-squared.
5. **Never-played track in snapshot** — 100-track snapshot, only 5 played → large chi2 (not zero).
6. **Dilution case (D15)** — 100-track snapshot, 99 uniform + 1 track at 3× expected (M=500): global p ≈ 1 (NOT significant), V is small (diluted by k=100), but the one track has |residual| > 3 and is flagged. Asserts the residual signal catches what the global test + V miss.
7. **Empty** — new user, no plays → `[]Result{}` non-nil, empty.
8. **Multi-snapshot (D2)** — two snapshots, one Result per snapshot.
9. **N < 2** — 1-track snapshot → `Skipped=true`, `SkipReason="N < 2"`, no residuals computed.
10. **Per-artist residual flag** — collab-heavy snapshot where one artist is over-represented → `ArtistRows[i].Flagged==true` for that artist.

### `report_test.go`
- **Golden files:** `internal/report/testdata/summary.golden.md` + `data.golden.md` with fixed injected `reportDate` + `generatedAt` in `Europe/Warsaw`. Use `//go:embed` or `os.ReadFile`. Add `-update` flag helper for regeneration.
- **`TestRenderSummary_empty`** — empty `[]Result{}` → valid "no data" markdown, not empty string.
- **`TestWriteAll_refusesOverwrite`** — pre-create both files, call `WriteAll`, assert error contains "exists"/"refuse" AND file contents unchanged.
- **`TestWriteAll_createsDir`** — `reports/` doesn't exist → created, both files written.
- Verify all reproducibility fields present in data file: snapshot ID, N, M, stat, df, p, V, residual threshold, formulae.
- Verify top-10 cut + data-file link present in summary.
- **Flagged-tracks section (D15):** seed a Result with one `TrackRow.Flagged==true` and global p > 0.01 → assert summary shows the track in the "Flagged tracks" section even though the global verdict is "not flagged." Verify the section is absent when no tracks are flagged (or rendered as "no tracks flagged by residual").

### `store_test.go`
Specify each test by invariant:
1. `SnapshotsWithShufflePlays` returns only snapshots with ≥1 shuffle play (non-null context).
2. `PlayCountsByTrack` filters `shuffle_state=TRUE AND playlist_id IS NOT NULL` — seed mix of shuffle/non-shuffle/null-context, assert only shuffle+context counted.
3. `SnapshotTrackIDs` returns correct N.
4. `ArtistTrackCounts` counts K per artist — collab track (2 artists) → both artists get +1 (D3).
5. Orphan track excluded by INNER JOIN — play with track_id not in snapshot → not in result map.
6. Multi-snapshot scenario — two snapshots, queries scoped correctly per snapshot_id.

### `config_test.go`
- `TestLoad_reportsDefaults` — no `[reports]` block → `Dir=="./reports"`, `MinPlays==30`, `ResidualThreshold==3`.
- `TestLoad_reportsOverride` — explicit values honored.
- `TestLoadForReport_skipsCreds` — missing `client_id`/`client_secret`/`refresh_token` → no error in report mode.
- `TestLoadForCapture_requiresCreds` — missing creds → error in capture mode.

### `main_test.go` / integration
- `TestRunReportOnce_emptyStore` — `:memory:` store, no data → writes "no data" report, exits 0.
- `TestRunReportOnce_seededData` — seed plays, assert both files written with expected content.
- `TestRunReportOnce_timezone` — verify filename date and timestamps use `Europe/Warsaw` timezone.

After: `go vet ./...` and `go test ./...` clean. Build: `go build ./cmd/trust-issues/`.

---

## Rollback

1. Disable the timer: `sudo systemctl disable --now trust-issues-report.timer`.
2. Revert `main.go`, `config.go`, `store.go`, `store_test.go` additions.
3. The two new packages (`analysis/`, `report/`) can remain as dead code or be deleted.
4. Config files with no `[reports]` block remain valid because defaulting is in code.
5. **Revert must be atomic** — reverting `config.go` without the new `Reports` struct field breaks compilation (main.go references `cfg.Reports`).
6. `deploy/trust-issues-report.{service,timer}` can be removed or left in place (harmless without the timer enabled).

---

## Suggested build order

1. `chisq.go` + tests (pure math, no DB) — fastest feedback, foundation.
2. `types.go` (analysis) + `SnapshotInfo` in `store/types.go`.
3. Store query methods + tests (INNER JOIN for orphan plays, WHERE clause for shuffle + non-null context).
4. `analysis.go` + tests (wires 1 + 2 + 3).
5. `report.go` + tests (golden files, WriteAll atomicity, Europe/Warsaw timezone).
6. Config + `config.example.toml` + `main.go` `--report-once` wiring (`runReportOnce` extraction, `Europe/Warsaw` location).
7. Invoke context7-mcp for systemd timer syntax (OnCalendar, Persistent, Timezone=, Type=oneshot), then `deploy/trust-issues-report.service` + `.timer`.
8. Update `TODO.md` (check off chi-squared/daily-report/cron/systemd-unit; add deferred items: Bonferroni for global test, FDR for residual flags, streaming analysis, skipped-filter knob, hot reload, same-day suffixes) + `README.md` (Milestone 2 ✅, add `analysis/`+`report/` to project structure, add `[reports]` config section, DST note, cron fallback, Europe/Warsaw timezone note).

---

## Commit plan

Nine commits. Each compiles and passes `go vet ./...` + `go test ./...` in isolation (atomic revert boundary — see §Rollback). Order follows §Suggested build order, with step 6 split into two commits so the config schema and the binary behavior change are reviewable separately.

Subject lines match the existing repo style: short, imperative, no `feat:`/`fix:` prefix, no body required for small commits.

| # | Subject | Files | Deps on prior # | Why atomic |
|---|---|---|---|---|
| 1 | Add chi-squared math package | `internal/analysis/chisq.go`, `internal/analysis/chisq_test.go` | — | Pure math, zero deps outside stdlib. `analysis` package gains functions but no caller yet — compiles, tests pass. Fastest feedback per build-order step 1. |
| 2 | Add analysis result types and SnapshotInfo | `internal/analysis/types.go`, `internal/store/types.go` | 1 | Type declarations only, no logic, no new tests (covered implicitly by later callers). Both packages compile; existing tests unaffected. Groups the two type-only additions from build-order step 2. |
| 3 | Add store query methods for analysis Reader | `internal/store/store.go`, `internal/store/store_test.go` | 2 | `SnapshotInfo` (commit 2) is the return type. Each of the 6 methods is plain SQL against existing tables — no schema change. `*store.Store` now satisfies `analysis.Reader` but nobody calls it as such yet. |
| 4 | Add Analyzer | `internal/analysis/analysis.go`, `internal/analysis/analysis_test.go` | 1, 2, 3 | `Reader` interface + `Analyze` wire chisq + types + store methods. Tests use a fake `Reader` (no DB). `analysis` package is now complete and self-tested. |
| 5 | Add markdown report renderer | `internal/report/report.go`, `internal/report/report_test.go`, `internal/report/testdata/*.golden.md` | 2 | `report` depends only on `analysis.Result` types (commit 2), not on `Analyze`. Pure render functions + `WriteAll`; golden-file tests cover output. Could precede commit 4, but kept in build order for narrative. |
| 6 | Add reports config block | `internal/config/config.go`, `internal/config/config_test.go`, `config.example.toml` | — | New `Reports` struct + `LoadForReport`/`validateForReport` split + defaults. `LoadForReport` is unused until commit 7 — fine (no dead-code lint in Go by default). Config tests cover both modes. No change to binary behavior. |
| 7 | Wire `--report-once` flag in main | `cmd/trust-issues/main.go`, `cmd/trust-issues/main_test.go` | 4, 5, 6 | Extracts `runReportOnce`/`runPollers`, dispatches on the flag, calls `LoadForReport` (commit 6) + `analysis.Analyze` (commit 4) + `report.WriteAll` (commit 5). Integration tests use `:memory:` store. This is the only commit that changes runtime behavior. |
| 8 | Add systemd timer for daily reports | `deploy/trust-issues-report.service`, `deploy/trust-issues-report.timer` | 7 | Units reference the binary flag added in commit 7. No Go code, no tests. context7-mcp consulted first per build-order step 7. Harmless if merged before the timer is `systemctl enable`d. |
| 9 | Update TODO and README for Milestone 2 | `TODO.md`, `README.md` | 1–8 | Docs only. References the new packages, config block, systemd units, and deferred items. Last so it describes the merged state. |

**Notes on the split of build-order step 6:**

- The plan's step 6 bundles config + `main.go` wiring. Splitting them (commits 6 + 7) keeps each diff small and lets a reviewer judge the config schema independently from the `main.go` control-flow change. Merging them is acceptable if the reviewer prefers fewer commits; both remain atomic because commit 6 alone compiles and passes tests.
- The reverse order (main.go before config) is **not** atomic — `runReportOnce` references `cfg.Reports.MinPlays` and `LoadForReport`, so commit 7 cannot precede commit 6.

**Cross-commit invariant:** `go build ./cmd/trust-issues/` must succeed after every commit. The riskiest boundary is 7→8: commit 7 changes the binary but adds no scheduler, so the report path is only reachable via manual `--report-once`. Commit 8 makes it scheduled. If a bisect lands between 7 and 8, the bug is either in the binary's report path (commit 7) or the unit definitions (commit 8) — cleanly separable.

---

## Decision index

| ID | Decision | Deferred to TODO.md |
|---|---|---|
| D1 | Cumulative-to-date per `(user, playlist, snapshot)` | — |
| D2 | Separate chi-squared test per snapshot | — |
| D3 | Count once per artist; normalize expected so ΣO=ΣE | — |
| D4 | Include all plays (skipped + non-skipped); state in report | Config knob to filter skipped plays |
| D5 | OS-level: systemd timer (primary) + cron fallback; `--report-once` flag | — |
| D6 | `flag` package, `--report-once` boolean | — |
| D7 | Full per-track and per-artist tables, sorted by contribution desc (≡ \|residual\| desc) | — |
| D8 | Skip if M<30 (floor) + warn if expected<5; skip if N<2 | — |
| D9 | Refuse overwrite; atomic `.tmp`+rename; exit 0 if already exists | Suffix `-1`, `-2`, … for same-day re-runs |
| D10 | One file pair per day, all accounts as sections | — |
| D11 | Flag threshold p<0.01; raw p always printed | Bonferroni multiple-testing correction |
| D12 | All times in `Europe/Warsaw` (CET/CEST); render functions take `*time.Location` | — |
| D13 | Per-account fail=log+omit; WriteAll fail=exit 1; ≥1 success=exit 0 | — |
| D14 | Cramér's V = sqrt(chi²/(M·(k−1))); bounded [0,1], comparable across playlist sizes; displayed alongside chi²/df/p; not gated on threshold | — |
| D15 | Per-track + per-artist standardized residuals; `\|r_i\|>3` flag (configurable); surfaced separately from global verdict | FDR/BH correction for residual flags |
