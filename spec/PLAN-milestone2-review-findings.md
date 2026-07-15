# Plan Review — PLAN-milestone2.md

**Panel:** 3 reviewers (pragmatic-code-reviewer + 2× general), different focus angles
**Verdict:** **Revise** — 6 blockers + many important findings must be resolved before execution

**Note on dispositions:** Per user instruction, this file is a handoff document for the build agent. The plan itself was NOT revised inline. Disposition `Apply` = build agent should update the plan/code per the recommendation. `Pushback` = do not apply (reasoning given). `Duplicate` = merged into the lead row cited.

---

## Finding Ledger

| ID | Reviewer | Severity | Summary | Disposition |
|----|----------|----------|---------|-------------|
| R1-F1 | R1 | 🟡 | D8 `M≥30` insufficient; need expected≥5 guard | Apply |
| R1-F2 | R1 | 🟡 | Plays whose track not in snapshot unhandled | Duplicate → R3-F8 |
| R1-F3 | R1 | 🟡 | Shuffle-state filter not explicit in Reader | Duplicate → R3-F12 |
| R1-F4 | R1 | 🟡 | Flagging threshold + multiple-testing correction unspecified | Apply |
| R1-F5 | R1 | 🟡 | Data appendix may omit snapshot-level stats (reproducibility gap) | Apply |
| R1-F6 | R1 | 🟡 | `--report-once` fails config validation (needs Spotify creds) | Apply |
| R1-F7 | R1 | 🟡 | Report-date filename timezone ambiguity | Apply |
| R1-F8 | R1 | 🟡 | Cumulative-to-date + p-only flag → eventually flags everything | Apply (partial; defer Cramér's V to TODO) |
| R1-F9 | R1 | 🟢 | D2+D8 interaction: frequently-edited playlists always skip | Apply |
| R1-F10 | R1 | 🟡 | `SnapshotInfo`, `Result`, `TrackNames` sig, `SnapshotTrackCounts` return undefined | Apply (merged with R2-F6, R2-F9) |
| R1-F11 | R1 | 🟢 | Edge cases: zero plays, 1-track (df=0), 0-track, empty DB | Duplicate → R3-F10 |
| R1-F12 | R1 | 🟢 | `WriteAll` non-atomic; multi-account concurrency unspecified | Duplicate → R3-F14 (atomicity), R3-F13 (multi-acct) |
| R2-F1 | R2 | 🟡 | New files missing `// 🤖 AI-generated` header instruction | Apply |
| R2-F2 | R2 | 🟡 | No slog logging convention stated for new code | Apply |
| R2-F3 | R2 | 🟢 | `fmt.Errorf("...: %w", err)` wrapping not restated | Apply |
| R2-F4 | R2 | 🟡 | Config `[reports]` validate() pattern unspecified | Duplicate → R3-F16 |
| R2-F5 | R2 | 🟡 | Displayed date `YYYYMMDD` violates yyyy-MM-dd rule | Apply |
| R2-F6 | R2 | 🟡 | Nullable p-value representation in `Result` undefined | Duplicate → R1-F10 |
| R2-F7 | R2 | 🟡 | Should invoke context7-mcp for systemd syntax (NOT for Lentz) | Apply |
| R2-F8 | R2 | 🟡 | `OnCalendar=daily` (line 29) vs `*-*-* 03:00:00` (lines 57,146) inconsistency | Apply |
| R2-F9 | R2 | 🟢 | Analysis types should live in `types.go` (mirror store) | Duplicate → R1-F10 |
| R2-F10 | R2 | 🟢 | `--report-once` execution model (errgroup vs seq, defer Close, signal ctx) | Apply |
| R2-F11 | R2 | 🟢 | `config.example.toml` missing from build order | Apply |
| R2-F12 | R2 | 🟢 | No "no schema changes" note | Apply |
| R2-F13 | R2 | 🟢 | Golden file vs string assertion undecided | Duplicate → R3-F5 |
| R3-F1 | R3 | 🟡 | `chisq_test.go` missing edge cases (df=0, mismatched lens, chi2<0, empty) | Apply |
| R3-F2 | R3 | 🟢 | ±1e-4 tolerance too loose for extreme p-values | Apply |
| R3-F3 | R3 | 🔴 | No test for `WriteAll` refuse-overwrite (D9) — safety-critical | Apply |
| R3-F4 | R3 | 🟡 | No test for `--report-once`; extract `runReportOnce` for testability | Apply |
| R3-F5 | R3 | 🟢 | Commit to golden files under `testdata/` with `-update` flag | Apply (merged with R2-F13) |
| R3-F6 | R3 | 🟡 | `analysis_test.go` only 2 scenarios; need M<30, orphan-track, empty, multi-snapshot | Apply |
| R3-F7 | R3 | 🔴 | Zero-expected-count handling undefined ("skip" ambiguous) | Apply |
| R3-F8 | R3 | 🔴 | Tracks in plays but not in snapshot — data-flow undefined | Apply (merged with R1-F2) |
| R3-F9 | R3 | 🟡 | Tracks in snapshot but never played must be in chi-squared sum | Apply |
| R3-F10 | R3 | 🟡 | df=0 / len=1 / empty-results handling undefined | Apply (merged with R1-F11, R3-F18) |
| R3-F11 | R3 | 🟡 | Lentz alone loses precision for small chi2; need series branch too | Apply |
| R3-F12 | R3 | 🔴 | Null-context + non-shuffle filtering not in SQL WHERE | Apply (merged with R1-F3) |
| R3-F13 | R3 | 🔴 | Multi-account: one file or per-account is contradictory | Apply |
| R3-F14 | R3 | 🟡 | Concurrent `--report-once` runs race on file write; non-atomic | Apply (merged with R1-F12) |
| R3-F15 | R3 | 🟡 | systemd default `TimeoutStartSec=90s` may kill large analyses | Apply |
| R3-F16 | R3 | 🔴 | `[reports]` config defaulting not in code — zero-value breaks D8 | Apply (merged with R2-F4) |
| R3-F17 | R3 | 🟡 | Error handling in `--report-once` multi-account loop unspecified | Apply |
| R3-F18 | R3 | 🟡 | Empty results rendering unspecified | Duplicate → R3-F10 |
| R3-F19 | R3 | 🟢 | `Persistent=true` backfill behavior (1 report, not N) not documented | Apply |
| R3-F20 | R3 | 🟡 | Report service must repeat hardening directives verbatim (not inherited) | Apply |
| R3-F21 | R3 | 🟢 | Memory usage: map-based approach fine for v1, flag for TODO | Apply |
| R3-F22 | R3 | 🟢 | DST fall-back/spring-forward of 03:00 not documented | Apply |
| R3-F23 | R3 | 🟢 | `reportDate` vs "generation timestamp" purity for golden files | Apply |
| R3-F24 | R3 | 🟢 | Floating point int→float64 precision | Pushback (no change) |
| R3-F25 | R3 | 🟢 | No rollback plan documented | Apply |
| R3-F26 | R3 | 🟡 | `store_test.go` multi-snapshot scenario too vague; specify invariants | Apply |

**Totals:** 51 findings → 42 lead rows + 9 duplicates. Dispositions: 41 Apply, 1 Pushback, 9 Duplicate. 6 Blockers (all Apply).

---

## Blockers (must resolve before execution)

### 🔴 R3-F3 — No test for `WriteAll` refuse-overwrite (D9)
D9 is a safety-critical operational guarantee ("refuse to overwrite"). The test plan doesn't test it. An untested refuse-overwrite is a silent-data-loss bug waiting to happen.
**Fix:** Add `TestWriteAll_refusesOverwrite` — pre-create `YYYYMMDD.md` + `YYYYMMDD-data.md`, call `WriteAll`, assert error contains "exists"/"refuse" AND file contents unchanged. Also test `WriteAll` creates `reports/` dir if missing.

### 🔴 R3-F7 — Zero-expected-count handling undefined
Plan §1 says "skip-on-zero-expected guard" with no detail. "Skip" is ambiguous: skip-the-term + df=len−1 → wrong p; skip-the-term + df=used−1 → must recompute df; error → caller cleans. This is triggered by real data (R3-F8), not theory.
**Fix:** `Statistic` errors if any `expected[i] <= 0` (clean contract). Caller (`analysis.go`) drops observed counts for tracks not in the snapshot AND reduces M accordingly, builds expected over snapshot tracks only. df = (categories used) − 1. Document in plan.

### 🔴 R3-F8 — Tracks in plays but not in snapshot (merged R1-F2)
DESCRIPTION's "Edge case — playlist edited between syncs" explicitly creates this: plays associated with an old snapshot that didn't contain the played track (1-hour inconsistency window). `PlayCountsByTrack` returns a track not in `SnapshotTrackCounts` → expected=0 → R3-F7.
**Fix:** `PlayCountsByTrack`/`PlayCountsByArtist` SQL must `INNER JOIN plays ON track_id` against `playlist_snapshot_tracks WHERE snapshot_id = ?` — drops orphan plays at SQL level. M (per snapshot) = `SUM(observed)` after join. expected = M/N uses snapshot's N. Document that orphan plays are excluded (consistent with DESCRIPTION's "graceful handling").

### 🔴 R3-F12 — Null-context + non-shuffle filtering not in SQL (merged R1-F3)
DESCRIPTION A2 (null-context excluded) + A3 (non-shuffle filtered) are hard requirements. Reader methods take only `(userID, snapshotID)` so the filter MUST be in SQL. Plan doesn't show the WHERE clause. A bug here silently corrupts M.
**Fix:** WHERE clause for `PlayCountsByTrack`/`PlayCountsByArtist`/`SnapshotsWithShufflePlays`: `WHERE user_id = ? AND playlist_snapshot_id = ? AND shuffle_state = 1 AND playlist_id IS NOT NULL`. `analysis_test.go` must seed a mix of shuffle/non-shuffle/null-context plays and assert only shuffle+context plays are counted.

### 🔴 R3-F13 — Multi-account: one file or per-account is contradictory
Plan §6 reads "for each account: Analyze + Render + WriteAll" (per-account write) but D10/filename/summary-layout imply one file pair with per-account sections. Literal reading → second account's `WriteAll` hits D9 refuse-overwrite and fails.
**Fix:** Clarify: `Analyze` called per account → collect `[]analysis.Result` across all accounts → `RenderSummary`/`RenderData`/`WriteAll` called **once** with combined slice. Update §6 to show the loop collects results, then renders/writes once. Summary file contains all accounts as sections.

### 🔴 R3-F16 — `[reports]` config defaulting not in code (merged R2-F4)
Existing `config.toml` files have no `[reports]` block. BurntSushi/toml leaves zero values → `Dir==""`, `MinPlays==0`. D8's "skip if M<30" becomes "skip if M<0" → never skips → chi-squared runs on M=1 → bogus p-values. The "# default" TOML comment is not code.
**Fix:** In `config.go`, add `Reports struct { Dir string; MinPlays int }` with `toml:"reports"` tag. In `Load()` (or a `applyDefaults()` before `validate()`): if `Dir==""` set `"./reports"`; if `MinPlays==0` set `30`. Extend `validate()`: `min_plays >= 1`, `dir` non-empty after defaulting. Add `TestLoad_reportsDefaults` (no `[reports]` block → defaults applied) + `TestLoad_reportsOverride` (explicit values honored). Follow existing `errors.New("config: ...")` style.

---

## Important findings (should fix)

### 🟡 R1-F1 — D8 `M≥30` statistically insufficient
Chi-squared validity rule: all expected counts ≥ 5, i.e. `M ≥ 5·N`. With M≥30 floor, a 2000-track playlist has expected=0.015/track — test runs, returns a p-value, looks reproducible but is statistically meaningless.
**Fix:** Keep `M≥30` as a hard floor (don't make the tool useless for large playlists — strict `M≥5N` needs 10000 plays for 2000 tracks). Add a soft warning when any expected < 5: print "chi-squared result may be unreliable (min expected = X)" in the report. Also state explicitly: per-artist df = (distinct artists in snapshot) − 1; per-track df = N − 1.

### 🟡 R1-F4 — Flagging threshold + multiple-testing correction unspecified
Summary layout says "flagged?" but no threshold defined, no multiple-testing correction. With dozens of snapshots × tests × accounts, flat p<0.05 flags many by chance.
**Fix:** Specify (a) flag threshold (e.g. p < 0.01), (b) Bonferroni correction per account (threshold / number of tests run for that account) — simplest defensible choice, (c) raw uncorrected p-value always printed alongside the corrected flag for transparency.

### 🟡 R1-F5 — Data appendix may omit snapshot-level stats
Reproducibility requirement: "anyone holding only the report" verifies the finding. Data appendix layout lists only per-row tables, not per-snapshot header (stat/df/p/N/M). Someone sent only the data file can't verify df or p without N.
**Fix:** Data appendix's per-snapshot section must include a header block: snapshot ID, captured_at, N, M, per-track chi²/df/p, per-artist chi²/df/p, formulas used. Data file must be self-contained for verification.

### 🟡 R1-F6 — `--report-once` fails config validation
Existing `validate()` requires `client_id`, `client_secret`, `accounts[].refresh_token` — all unnecessary for report generation (only reads DB). Blocks the "manual backfills, ad-hoc runs, tests" use case D5 promises.
**Fix:** Make `validate()` mode-aware: pass a flag (or split into `Load()` / `LoadForReport()`). Report-only mode requires only `storage.database_path` and `accounts[].user_id` + `display_name`; skips credential/refresh-token checks. Document which fields are required in each mode.

### 🟡 R1-F7 — Report-date filename timezone ambiguity
Summary header says "generation timestamp (UTC)" but filename date + 03:00-local fire time imply local time. A 03:00 local run in UTC-5 = Jan 14 22:00 UTC → filename could be `YYYYMM14` or `YYYYMM15` depending on what drives `time.Now()`. Matters for reproducibility + D9 overwrite check.
**Fix:** State explicitly: filename date = local date at `time.Now()` (matches 03:00 local fire + user's "today's report" intuition). Header generation timestamp = UTC (RFC3339). Document in plan + report footer.

### 🟡 R1-F8 — Cumulative-to-date + p-only flag → eventually flags everything (partial apply)
Chi-squared is consistent — as M grows, even 0.5% deviation becomes significant. After months of data, "flagged?" is true for everything, making it useless (contradicts DESCRIPTION's "within tolerance in April, drifted out in May" which implies practical significance).
**Fix:** Apply (partial): acknowledge the limitation in plan D1 + D8. Frame the flag as "statistically significant" (not "non-random"). **Defer Cramér's V / effect-size measure to `TODO.md`** (avoid v1 scope creep beyond the DESCRIPTION's chi-squared+p-value spec). Add TODO entry: "Add effect-size measure (Cramér's V) so the flag reflects practical, not just statistical, significance as M grows."

### 🟡 R1-F10 — Types/signatures undefined (merged R2-F6, R2-F9)
`SnapshotInfo` fields, `Result` struct fields (esp. nullable p-value/chi2/df when test skipped per D8), `TrackNames` signature, `SnapshotTrackCounts` return shape all undefined. R2-F6: project convention for nullable = pointers (`*float64`, `*int`). R2-F9: types should live in `types.go` mirroring `internal/store/types.go`.
**Fix:**
- `SnapshotInfo struct { ID, UserID, PlaylistID, PlaylistName string; CapturedAt time.Time; N int }` in `internal/analysis/types.go`.
- `Result struct { ... Chi2 *float64; DF *int; P *float64; Skipped bool; SkipReason string; TrackRows []TrackRow; ArtistRows []ArtistRow ... }` — nil pointers when M < min_plays (D8) or N < 2 (R3-F10).
- `TrackNames(ctx, trackIDs []string) (map[string]string, error)`, `ArtistNames(ctx, artistIDs []string) (map[string]string, error)`.
- Change `SnapshotTrackCounts` to `([]string, int, error)` (track IDs + N) — per-snapshot per-track count is always 1 by PK `(snapshot_id, track_id)`.
- Put all analysis types in `internal/analysis/types.go`.

### 🟡 R2-F1 — New files missing `// 🤖 AI-generated` header
6+ new source files + 2 systemd units. Project AGENTS.md §3 requires the header.
**Fix:** Add a "Conventions" note to the plan: every new Go file gets `// 🤖 AI-generated` at top; systemd units get `# 🤖 AI-generated`. Runtime-generated report `.md` files are output, not source — exempt.

### 🟡 R2-F2 — No slog logging convention stated
`--report-once` path + `analysis.Analyze` need logging ("generating report", "skipped snapshot M<N", "wrote report"). Existing convention: `log = log.With("user_id", …, "component", "playlists")` + `log.Info("…", "key", val)`.
**Fix:** State that `analysis`/`report`/`--report-once` path log via slog with a `"component"` key (e.g. `"component", "analysis"`). Pure render functions (`RenderSummary`/`RenderData`) stay log-free (they're pure).

### 🟡 R2-F5 — Displayed date `YYYYMMDD` violates yyyy-MM-dd rule
Global AGENTS.md: "Use `yyyy-MM-dd` for dates in UI, code, and data. For file and directory names use `yyyyMMdd` (no separators)." Filenames `YYYYMMDD.md` are compliant; the date **printed in the markdown header/body** is not.
**Fix:** Distinguish explicitly: filenames use Go layout `20060102`; dates printed in markdown use `2006-01-02`; generation timestamp uses RFC3339 `2006-01-02T15:04:05Z07:00`. State the Go layout strings in the plan to remove all `YYYY`/`yyyy` ambiguity.

### 🟡 R2-F7 — Invoke context7-mcp for systemd (NOT for Lentz)
systemd timer directives (`OnCalendar=`, `Persistent=`, `Type=oneshot`) have subtle semantics — in context7's scope (CLI tool usage). The Lentz algorithm is a "general programming concept" — context7's own rule excludes it.
**Fix:** Add to plan §7: "Before writing the timer unit, invoke context7-mcp to confirm `OnCalendar=` grammar, `Persistent=` catch-up semantics, `Type=oneshot` pairing." Do NOT add a context7 step for the chisq math.

### 🟡 R2-F8 — `OnCalendar` inconsistency
Line 29 sketch says `OnCalendar=daily` (= midnight in systemd); lines 57 + 146 say `OnCalendar=*-*-* 03:00:00`. Different schedules.
**Fix:** Fix line 29 to `OnCalendar=*-*-* 03:00:00` so sketch + decisions agree.

### 🟡 R3-F1 — `chisq_test.go` missing edge cases
Only lists df=1, very-large/small chi2. Missing: df=0, `len(observed)≠len(expected)`, chi2=0 for any df → p=1 exactly, chi2<0 → error, df large (200), empty slices, all-zero expected, `PValue(0,0)`.
**Fix:** Add table-driven `TestPValue_edgeCases`: `(0,1)→1.0`, `(0,100)→1.0`, `(3.84,0)→error`, `(-1,5)→error`, `(1e6,5)→<1e-300`, `(1e-10,5)→1.0`. Add `TestStatistic_mismatchedLengths→error`, `TestStatistic_empty→(0, -1, error)`.

### 🟡 R3-F4 — No test for `--report-once`; extract `runReportOnce`
`main.go`'s `run()` is a monolith that always starts pollers — no testable seam. Without extracting a function, the flag path needs subprocess tests.
**Fix:** Refactor `run()` to extract `runReportOnce(ctx, logger, cfg, st, now time.Time) error` and `runPollers(ctx, ...) error`; `--report-once` dispatches between them. Test `runReportOnce` directly with an in-memory store. Even a smoke test (empty store → writes "no data" report, exits 0) is valuable.

### 🟡 R3-F6 — `analysis_test.go` scenarios insufficient
Only 2 happy-path scenarios (uniform, skewed). Missing: M<min_plays boundary (D8 — assert `Chi2==nil` but `TrackRows!=nil`), orphan-track (R3-F8), empty (new user), multi-snapshot (one Result per snapshot, D2), zero-shuffle-plays snapshot (filtered by `SnapshotsWithShufflePlays`).
**Fix:** Expand to ≥5 table-driven scenarios. The M<30 and orphan-track cases are most important — they exercise the under-specified paths.

### 🟡 R3-F9 — Tracks in snapshot but never played must be in chi-squared sum
For chi-squared validity, every category must be in the sum, including observed=0 (contribution = `(0−e)²/e = e`). If `Analyze` iterates only over `PlayCountsByTrack` keys, it misses never-played tracks → understates chi2 → hides non-randomness. Most common chi-squared implementation bug.
**Fix:** Specify: iteration set = keys of `SnapshotTrackCounts` (snapshot's tracks); `observed[i] = PlayCountsByTrack[trackID]` defaulting to 0. Same for artists via `ArtistTrackCounts`. Add a test: 100-track snapshot, only 5 played → large chi2 (not zero).

### 🟡 R3-F10 — df=0 / len=1 / empty-results handling (merged R1-F11, R3-F18)
1-track snapshot → df=0 → chi-squared degenerate (point mass at 0), p undefined. 0-track snapshot → N=0 → div-by-zero in expected. Empty results (new user / all non-shuffle) → `RenderSummary` must produce valid "no data yet" markdown, not empty string.
**Fix:** `Statistic([x],[e])` returns `(0.0, 0)` for len==1. `PValue(_, 0)` returns error `"chi-squared: df must be ≥ 1"`. `Analyze` skips chi-squared for N<2 snapshots (analogous to D8) but still emits raw counts with "N<2, chi-squared not applicable" note. `Analyze` returns non-nil empty `[]Result{}` on no data. `RenderSummary` emits header + "No shuffle plays recorded yet for any account." footer. Add `TestRenderSummary_empty`.

### 🟡 R3-F11 — Lentz precision for small chi2; need series branch
Numerical Recipes §6.2 uses TWO branches: `gser` (series) for `x < s+1`, `gcf` (Lentz) for `x ≥ s+1`. Lentz alone loses precision for small x (small chi2 → p close to 1). Plan names both `LowerRegGamma` + `UpperRegGamma` but doesn't say they use different algorithms.
**Fix:** Specify: `LowerRegGamma` uses series expansion (small x); `UpperRegGamma` uses Lentz continued fraction (large x); `PValue` dispatches on `chi2/2 < df/2 + 1`. ~30 extra lines. Make explicit they're different algorithms, not both Lentz.

### 🟡 R3-F14 — Concurrent runs race; non-atomic writes (merged R1-F12)
Timer fires while manual run in progress → both write, last wins, silent corruption. Process crashes mid-write → partial/corrupt report + D9 blocks all future same-day reruns.
**Fix:** `WriteAll` writes to `YYYYMMDD.md.tmp` (or `.tmp.<pid>`), then `os.Rename` to final (atomic on POSIX). Combined with D9's pre-check (stat final name before writing). TOCTOU race reduced to milliseconds; full `flock` safety is overkill for v1 — mention as a known minor gap.

### 🟡 R3-F15 — systemd `TimeoutStartSec` default may kill large analyses
systemd default `TimeoutStartSec=90s`. For 2000-track playlists + thousands of plays on a small VPS, analysis could exceed 90s → killed mid-run → no report + journal error.
**Fix:** Plan §7: set `TimeoutStartSec=600` (10 min) on `trust-issues-report.service`. Justify: "chi-squared is O(k) per snapshot; loading plays is the dominant cost; 600s covers ≥10k plays on a 1-vCPU VPS."

### 🟡 R3-F17 — Error handling in `--report-once` multi-account loop
If `Analyze` fails for account 2 of 3 — abort all? continue (partial)? If `WriteAll` fails — exit code? Plan says "exit 0" unconditionally → systemd won't know it failed.
**Fix:** Per-account `Analyze` errors: logged, that account's results omitted (or replaced with "analysis failed" stub section), loop continues. `WriteAll` failure → log + exit 1. All accounts fail → exit 1. ≥1 account succeeded → exit 0. Lets systemd distinguish success from failure while degrading gracefully.

### 🟡 R3-F20 — Report service must repeat hardening directives verbatim
systemd does NOT inherit `[Service]` directives between units. "Same as" is prose that's easy to under-implement. Also: `Restart=on-failure` (on main unit) is questionable for oneshot (could loop); `KillSignal`/`TimeoutStopSec` irrelevant for oneshot.
**Fix:** Plan §7 lists exact directives to copy: `User=`, `Group=`, `WorkingDirectory=`, `NoNewPrivileges=true`, `ProtectHome=true`, `ProtectSystem=full`, `PrivateTmp=true`, `StandardOutput=journal`, `StandardError=journal`. Explicitly EXCLUDE `Restart=on-failure` (or use `Restart=on-failure RestartSec=300` with a note that oneshot restart re-runs whole analysis). Add `TimeoutStartSec=600` (R3-F15). State that `ProtectSystem=full` does not block `/opt` writes (verified — only `/usr`/`/boot`/`/etc` read-only).

### 🟡 R3-F26 — `store_test.go` multi-snapshot scenario too vague
"Multi-snapshot scenario" doesn't specify invariants. Critical store-level invariants untested.
**Fix:** Specify each store test by invariant: (a) `SnapshotsWithShufflePlays` returns only snapshots with ≥1 shuffle play; (b) `PlayCountsByTrack` filters `shuffle_state=TRUE AND playlist_id IS NOT NULL` (R3-F12); (c) `SnapshotTrackCounts` returns correct N; (d) `ArtistTrackCounts` counts K per artist (collab: track with 2 artists → both); (e) orphan track excluded by INNER JOIN (R3-F8). Collab counting (D3) + shuffle/context filter (A2/A3) are most important — they're SQL-enforced.

---

## Minor findings (nice to fix, low risk)

### 🟢 R1-F9 — D2+D8 interaction: frequently-edited playlists always skip
Frequently-changed playlists have few plays per snapshot → almost always hit M<30 → report full of "insufficient data" entries.
**Fix:** Acknowledge the tradeoff in plan D2/D8. Document that frequently-edited playlists need more cumulative plays before per-snapshot results appear. (Aggregate per-playlist fallback deferred — would contradict D2's per-snapshot correctness.)

### 🟢 R2-F3 — `fmt.Errorf("...: %w", err)` wrapping not restated
**Fix:** Add "errors wrapped via `fmt.Errorf("context: %w", err)`" to store-additions + analysis sections. Low risk (pervasive in codebase) but the user flagged it as a check item.

### 🟢 R2-F10 — `--report-once` execution model underspecified
**Fix:** State: keep `defer st.Close()`; sequential loop over accounts is fine for read-only oneshot (or `errgroup` — pick one); signal ctx optional but harmless to retain. Match existing cleanup.

### 🟢 R2-F11 — `config.example.toml` missing from build order
**Fix:** Add "update `config.example.toml` with `[reports]` block" to build-order step 5.

### 🟢 R2-F12 — No "no schema changes" note
**Fix:** Add one-line note: "No schema changes; new methods query existing `playlist_snapshot_tracks`, `track_artists`, `plays` tables. `currentSchemaVersion` unchanged."

### 🟢 R3-F2 — ±1e-4 tolerance too loose for extreme p
±1e-4 absolute passes for any tiny number including buggy `0.0` for large chi2.
**Fix:** Use ±1e-4 absolute only for moderate p (0.01 ≤ p ≤ 0.99). For p→0: assert `p < threshold` (e.g. `< 1e-6`). For p→1: assert `1−p < 1e-6`. Or relative tolerance `|got−want|/want < 1e-6` when `want > 1e-8`.

### 🟢 R3-F5 — Commit to golden files (merged R2-F13)
Leaving "golden file or string assertion" open invites a poor choice. For reproducibility-critical output, golden files document the format + catch drift. String assertions are unreadable for multi-line markdown.
**Fix:** Commit to golden files under `internal/report/testdata/` (`summary.golden.md`, `data.golden.md`) with a fixed injected `reportDate`. Use `//go:embed` or `os.ReadFile` in tests. Add a `-update` flag helper for regeneration. Pros: living spec. Normalize any non-injected time (see R3-F23).

### 🟢 R3-F19 — `Persistent=true` backfill behavior not documented
`OnCalendar=daily + Persistent=true` fires once on boot if missed — not N times for N missed days. Matches D1 cumulative-to-date, but "catches up" wording could be misread as "generates all missed reports."
**Fix:** Add one-line note in plan §7 + README: "If VPS was off for N days, next run produces ONE report covering all accumulated plays (D1 cumulative-to-date), not N backfill reports. Intentional."

### 🟢 R3-F21 — Memory usage
**Fix:** Add to `TODO.md`: "Stream/batch analysis for large historical datasets (current map-based approach loads all plays per snapshot into memory)." Fine for v1 (~2000 tracks, thousands of plays).

### 🟢 R3-F22 — DST behavior of 03:00
**Fix:** One line in README: "Fire time is 03:00 local. In DST transitions, the timer may fire once or shift by an hour; harmless for daily reports."

### 🟢 R3-F23 — `reportDate` vs "generation timestamp" purity
Signature takes one `time.Time` but layout mentions two time fields. If `RenderSummary` calls `time.Now()` internally → impure → breaks golden files.
**Fix:** `RenderSummary`/`RenderData` take `reportDate time.Time` only — use it for both "report date" and "generated at" fields. OR take a second `generatedAt time.Time` parameter (cleaner for backfill). Function must be pure for golden-file testing (R3-F5).

### 🟢 R3-F25 — No rollback plan
**Fix:** Add brief "Rollback" section: "Disable `trust-issues-report.timer`; revert `main.go`, `config.go`, `store.go` additions; the two new packages can remain (dead code) or be deleted. Config files with no `[reports]` block remain valid because defaulting is in code (R3-F16)." Note: revert must be atomic — reverting `config.go` without the new `Reports` struct field breaks compilation.

---

## Pushbacks

### Pushback: R3-F24 — Floating point int→float64 precision

**Reviewer said:** "Play counts (int) convert to float64 exactly up to 2^53 ≈ 9e15 — many orders of magnitude above any realistic count. No precision concern. `[]float64` is the right choice."

**Why not applied:** Reviewer explicitly concluded "No change. Noted for completeness." `[]float64` is correct; int→float64 is exact up to 2^53; no realistic play count approaches this. No plan change needed.

**Risk accepted:** None — this is a confirmation, not a defect.

---

## Consolidated changes for the build agent

Apply these to `PLAN-milestone2.md` (and carry through to execution):

**Statistical correctness:**
1. R1-F1: Add expected<5 warning alongside M≥30 floor; state per-artist df = artists−1, per-track df = N−1.
2. R1-F4: Define flag threshold (p<0.01) + Bonferroni per account; print raw p alongside corrected flag.
3. R1-F8: Acknowledge cumulative-to-date limitation in D1/D8; frame flag as "statistically significant"; add TODO for Cramér's V.
4. R3-F7: `Statistic` errors on `expected[i]<=0`; caller filters + recomputes M + df.
5. R3-F9: Iterate over snapshot tracks (observed defaults to 0); same for artists.
6. R3-F11: Specify series branch (small x) + Lentz branch (large x); `PValue` dispatches on `chi2/2 < df/2+1`.

**Data flow / SQL:**
7. R3-F8: `PlayCountsByTrack`/`PlayCountsByArtist` use INNER JOIN against `playlist_snapshot_tracks` to drop orphan plays; M = SUM(observed) after join.
8. R3-F12: WHERE clause includes `shuffle_state = 1 AND playlist_id IS NOT NULL` on all play-count queries + `SnapshotsWithShufflePlays`.

**Types / signatures:**
9. R1-F10: Define `SnapshotInfo`, `Result` (with `*float64`/`*int` nullable chi2/df/p + `Skipped bool` + `SkipReason string`), `TrackNames`/`ArtistNames` sigs, change `SnapshotTrackCounts` → `([]string, int, error)`. Put types in `internal/analysis/types.go`.
10. R3-F10: Handle df=0 (error), len=1 (skip), N<2 (skip + note), empty results (non-nil `[]Result{}`, `RenderSummary` "no data" footer).

**Config:**
11. R3-F16: Add `Reports` struct + TOML tag; default `Dir="./reports"`, `MinPlays=30` in code; extend `validate()`; add config tests.
12. R1-F6: Make `validate()` mode-aware (report-only skips credential/refresh-token checks).

**Rendering / files:**
13. R3-F13: Clarify one file pair — loop collects `[]Result` across accounts, render+write once.
14. R3-F14: `WriteAll` writes `.tmp` + `os.Rename` (atomic); pre-check before write.
15. R1-F5: Data appendix per-snapshot header includes stat/df/p/N/M + formulas.
16. R2-F5: Filenames use `20060102`; displayed dates use `2006-01-02`; timestamps use RFC3339.
17. R1-F7: Filename date = local; header timestamp = UTC.
18. R3-F23: `RenderSummary`/`RenderData` pure — inject `reportDate` (and `generatedAt` if needed), no `time.Now()` inside.

**CLI / main.go:**
19. R3-F4: Extract `runReportOnce(ctx, logger, cfg, st, now) error`; test it.
20. R3-F17: Specify error semantics — per-account fail = log + omit; `WriteAll` fail = exit 1; all fail = exit 1; ≥1 success = exit 0.
21. R2-F10: Keep `defer st.Close()`; pick sequential or errgroup; document.

**Deploy:**
22. R2-F8: Fix line 29 `OnCalendar=daily` → `OnCalendar=*-*-* 03:00:00`.
23. R3-F15: Add `TimeoutStartSec=600` to report.service.
24. R3-F20: List exact hardening directives to copy; exclude `Restart=on-failure` (or `RestartSec=300` + note); note `ProtectSystem=full` doesn't block `/opt`.
25. R3-F19: Document Persistent=true → 1 catch-up report, not N.
26. R2-F7: Add plan step — invoke context7-mcp for systemd timer syntax before writing units.

**Tests:**
27. R3-F3: `TestWriteAll_refusesOverwrite` + dir-creation test.
28. R3-F1: Expand `chisq_test.go` edge cases (df=0, mismatched lens, chi2<0, empty, chi2=0→p=1).
29. R3-F2: Tolerance strategy — absolute for moderate p, threshold for extreme.
30. R3-F5: Golden files under `internal/report/testdata/` + `-update` flag.
31. R3-F6: Expand `analysis_test.go` to ≥5 scenarios (M<30, orphan-track, empty, multi-snapshot, zero-shuffle).
32. R3-F26: Specify `store_test.go` invariants (shuffle filter, collab counting, orphan exclusion, N, K).

**Conventions / docs:**
33. R2-F1: New files get `// 🤖 AI-generated` (Go) / `# 🤖 AI-generated` (systemd).
34. R2-F2: slog with `"component"` key for analysis/report/CLI path; render functions stay pure/log-free.
35. R2-F3: State `fmt.Errorf("...: %w", err)` convention for new code.
36. R2-F11: Add `config.example.toml` update to build order.
37. R2-F12: Note "no schema changes."
38. R3-F22: README DST note.
39. R3-F25: Add rollback section.
40. R3-F21: TODO entry for streaming analysis.
41. R1-F9: Acknowledge D2+D8 tradeoff in plan.

---

## Open questions for the user

1. **R1-F8 (effect size):** OK to defer Cramér's V to TODO (v1 ships p-value-only flag with documented limitation), or do you want it in v1 now? Reviewer recommends defer; I agree (avoid scope creep beyond DESCRIPTION's chi-squared+p spec).
2. **R1-F1 (M≥5N strict vs hybrid):** Hybrid (M≥30 floor + expected<5 warning) keeps the tool useful for large playlists. Strict M≥5N would require ~10000 plays for a 2000-track playlist — likely never reached. Hybrid recommended — confirm?
3. **R3-F4 (refactor main.go):** Extracting `runReportOnce` is a small refactor of the existing monolith for testability. OK to refactor `run()`, or keep the oneshot path inline and untested?
4. **R3-F13 (report file structure):** Confirm: one file pair per day containing all accounts as sections (not one file pair per account)? This matches the summary layout's "per account" headings.
