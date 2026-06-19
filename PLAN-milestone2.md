> 🤖 AI-generated

# Milestone 2 — Implementation Plan

Scope (from `TODO.md` and `DESCRIPTION.md`):

- Chi-squared goodness-of-fit test (per-track and per-artist)
- Daily markdown reports to `reports/YYYYMMDD.md`
- Scheduled cron trigger

---

## Architecture

Two new internal packages, following the existing consumer-owns-interface convention:

```
internal/
  analysis/   — chi-squared math + Analyzer (queries store, computes results)
  report/     — Markdown renderer (pure function: Results → string)
```

Scheduling is delegated to the OS — no in-process scheduler. The binary gains a `--report-once` flag that runs analysis + writes the report + exits. A systemd timer (or cron, as fallback) invokes that mode on a daily schedule.

```
deploy/
  trust-issues.service           — existing: long-running poller + syncer
  trust-issues-report.service    — new: oneshot, runs `trust-issues --report-once`
  trust-issues-report.timer      — new: OnCalendar=daily, triggers the oneshot
```

Plus: new `Reader` interface in `analysis` (the store satisfies it), new store query methods, new `[reports]` config block.

**No new external Go dependencies.** Chi-squared CDF is pure Go (incomplete gamma via Lentz continued fraction — ~50 lines). Matches `DESCRIPTION.md` "Stats: Chi-squared test (pure Go)".

---

## Decisions

All decisions confirmed. v1 behavior is fixed; deferred items are tracked in `TODO.md`.

### D1. Analysis time window
Cumulative-to-date (all plays up to report time) per `(user, playlist, snapshot)`. Matches the DESCRIPTION example ("within tolerance in April, drifted out in May"). Chi-squared needs samples; a single day is usually too few. Rolling-window trend analysis is out of scope for v1.

### D2. Snapshot granularity for expected counts
Group plays by `playlist_snapshot_id` and run a separate chi-squared test per `(playlist, snapshot)`. Statistically correct: each play's expected probability came from the snapshot active *at that play*. The report shows one chi-squared result per snapshot within each playlist. We have the snapshot data — using only the latest snapshot's distribution for all plays would be wrong if the playlist ever changed.

### D3. Collab artist counting
Count once per artist on a played track — a track with artists `a1, a2` contributes +1 to each. Expected for an artist with K tracks in the snapshot = `K·M/N`. Sum of observed = sum of expected = `M·(ΣK_a)/N`. Clean and symmetric. Only-primary-artist and fractional-count alternatives were rejected.

### D4. Skipped plays
Include all plays (skipped + non-skipped) by default; state this prominently in the report. No config knob in v1. A knob to filter skipped plays out of analysis is deferred to `TODO.md`.

### D5. Report trigger mechanism
OS-level scheduling, no in-process scheduler:

1. **systemd timer (primary):** a oneshot `trust-issues-report.service` runs `trust-issues --report-once`, paired with a `trust-issues-report.timer` using `OnCalendar=*-*-* 03:00:00`. Gives journal logging, retries, and `systemctl list-timers` visibility for free.
2. **cron (fallback):** a single crontab line for non-systemd boxes (Alpine, containers, macOS dev): `0 3 * * * /opt/trust-issues/trust-issues -config /opt/trust-issues/config.toml -report-once`.
3. **`--report-once` flag** on the binary: runs analysis + writes reports + exits 0. Also useful for manual backfills, ad-hoc runs, and tests — independent of any scheduler.

Fire time is 03:00 local, configured in the systemd unit / crontab (not in `config.toml`). Decouples scheduling from the app (Unix philosophy), removes a package's worth of Go + tests (next-fire-time math, fake-clock injection, DST edge cases).

### D6. CLI shape
Keep the current `flag` package (no cobra). Add `--report-once` boolean. If set, run report and exit 0 without starting pollers. True subcommands (`trust-issues report`) are out of scope for v1.

### D7. Report content — full tables
DESCRIPTION's reproducibility requirement is explicit: per-track AND per-artist play counts, expected counts, chi-squared stat, df, p-value, snapshot ID + track count must all be in the report. Full tables for every track and every artist, sorted by `|observed − expected|` descending (most anomalous on top). That's the spec.

### D8. Minimum plays per snapshot
Chi-squared is unreliable with tiny samples. Skip the test if `M < 30`, still print the raw observed/expected table with a note "insufficient data for chi-squared". Configurable via `reports.min_plays` (default 30).

### D9. Existing report file behavior
Refuse to overwrite an existing `reports/YYYYMMDD.md` (or `YYYYMMDD-data.md`) — error out. Safe, forces intentionality. Suffix `-1`, `-2`, … for same-day re-runs is deferred to `TODO.md`.

### D10. One file or two (summary + data appendix)
Split into two files. A 2000-track playlist produces thousands of table rows; a single file buries the headline at line ~4000.

- `reports/YYYYMMDD.md` — scannable summary: per-account overview, per-snapshot chi²/df/p/flagged, top 10 most-anomalous tracks + artists inline (sorted by contribution desc), link to the data file.
- `reports/YYYYMMDD-data.md` — full per-track and per-artist tables (the reproducibility appendix).

Both files together satisfy DESCRIPTION's "no DB access required" rule. Flat naming (not a per-day subdirectory) keeps it glob-friendly and matches the existing `reports/YYYYMMDD.md` convention. Same overwrite-rule (D9) applies to both files.

---

## Detailed plan per component

### 1. `internal/analysis/chisq.go` — pure math
- `func Statistic(observed, expected []float64) (chi2 float64, df int)` — Σ (o−e)²/e. df = len−1.
- `func PValue(chi2 float64, df int) (float64, error)` — via regularized upper incomplete gamma `Q(df/2, chi2/2)`.
- `func LowerRegGamma(s, x float64) float64` and `func UpperRegGamma(s, x float64) float64` — Lentz continued fraction (Numerical Recipes §6.2). Pure Go, no deps.
- Tests: textbook values (e.g. chi2=3.84, df=1 → p≈0.05; chi2=11.07, df=5 → p≈0.05). Skip-on-zero-expected guard.

### 2. `internal/analysis/analysis.go` — Analyzer
- `type Reader interface { ... }` — defined here, satisfied by `*store.Store`. Methods:
  - `SnapshotsWithShufflePlays(ctx, userID) ([]SnapshotInfo, error)`
  - `SnapshotTrackCounts(ctx, snapshotID) (trackIDToCount map[string]int, totalN int, err error)` — pulls `playlist_snapshot_tracks`
  - `ArtistTrackCounts(ctx, snapshotID) (artistIDToK map[string]int, err error)` — joins `playlist_snapshot_tracks → track_artists`
  - `PlayCountsByTrack(ctx, userID, snapshotID) (map[string]int, error)`
  - `PlayCountsByArtist(ctx, userID, snapshotID) (map[string]int, error)` — joins `plays → track_artists`
  - `ArtistNames(ctx, artistIDs) (map[string]string, error)`, `TrackNames(...)`
- `type Result struct { ... }` — per (playlist, snapshot): playlist info, N, M, per-track chi2/df/p + per-track rows, per-artist chi2/df/p + per-artist rows.
- `func Analyze(ctx, reader, userID) ([]Result, error)` — for each snapshot with ≥1 shuffle play: gather observed/expected, run both tests, build Result. Skip snapshots with `M < reports.min_plays` (still emit raw counts — see D8).

### 3. `internal/report/report.go` — Markdown renderer
Two render functions, one writer that emits both files (see D10):
- `func RenderSummary(reportDate time.Time, results []analysis.Result) string` — pure. Scannable: header, per-account overview, per-snapshot chi²/df/p/flagged, top 10 most-anomalous tracks + artists inline, link to the data file.
- `func RenderData(reportDate time.Time, results []analysis.Result) string` — pure. Full per-track and per-artist tables for every snapshot (the reproducibility appendix).
- `func WriteAll(dir string, date time.Time, summary, data string) error` — writes `reports/YYYYMMDD.md` and `reports/YYYYMMDD-data.md`, refuses to overwrite either (D9). Creates `reports/` dir if missing. Never deletes.

Summary layout:
- Header: title, generation timestamp (UTC), report date `YYYYMMDD`, link to `YYYYMMDD-data.md`
- Per account: heading with display name + user_id
- Per playlist: name, id, snapshots analyzed, current snapshot
- Per snapshot: snapshot ID, captured_at, N (track count), M (shuffle plays), filters applied (shuffle=true, skipped included)
  - **Per-track chi-squared block**: stat, df, p-value, flagged?
  - **Top 10 tracks table** (by contribution desc): track_id, name, observed, expected, contribution, deviation %
  - **Per-artist chi-squared block** + **Top 10 artists table**: same shape
  - "Full tables: see YYYYMMDD-data.md" pointer
- Footer: note on how to reproduce, formulae used

Data appendix layout:
- Header: title, report date, "data appendix for YYYYMMDD.md"
- Per account → per playlist → per snapshot:
  - **Full per-track table**: track_id, name, observed, expected, (o−e)²/e contribution, deviation %, sorted by contribution desc
  - **Full per-artist table**: same shape

### 4. Store additions (`internal/store/store.go`)
New methods to satisfy `analysis.Reader`. Plain SQL with `?` placeholders, per convention. Each gets a unit test in `store_test.go` using the existing `:memory:` pattern.

### 5. Config additions (`internal/config/config.go`)
```toml
[reports]
dir        = "./reports"   # default
min_plays  = 30            # skip chi-squared if M < this (D8)
```

Note: schedule time (hour/minute) is **not** in `config.toml` — it lives in the systemd unit / crontab, since scheduling is now OS-owned.

### 6. `cmd/trust-issues/main.go` wiring
- Parse `--report-once` flag.
- If set: load config, open store, run `analysis.Analyze` for each account, `report.RenderSummary` + `report.RenderData` + `report.WriteAll`, exit 0. Does **not** start pollers or syncers.
- Else: existing behavior (pollers + syncers in `errgroup`). No scheduler goroutine — scheduling is external.

### 7. `deploy/trust-issues-report.service` + `deploy/trust-issues-report.timer`
- `trust-issues-report.service` — `Type=oneshot`, `ExecStart=/opt/trust-issues/trust-issues -config /opt/trust-issues/config.toml -report-once`. Same `User=`, `WorkingDirectory=`, security hardening as the main unit.
- `trust-issues-report.timer` — `OnCalendar=*-*-* 03:00:00`, `Persistent=true` (catches up if missed while powered off), `Unit=trust-issues-report.service`.
- README notes the cron fallback line for non-systemd boxes.

---

## Test plan
- `chisq_test.go`: textbook p-values (tolerance ±1e-4), edge cases (df=1, very large chi2 → p≈0, very small → p≈1).
- `analysis_test.go`: in-memory store, seed known plays, assert observed/expected/chi2/p-value. Two scenarios: uniform (high p) and Metallica-skewed (low p).
- `report_test.go`: golden file or string assertion on a small fixed Result, for **both** `RenderSummary` and `RenderData`. Verify all reproducibility fields present in the data file; verify top-10 cut and data-file link present in the summary.
- `store_test.go`: new query methods, multi-snapshot scenario.
- After: `go vet ./...` and `go test ./...` clean. Build: `go build ./cmd/trust-issues/`.

---

## Suggested build order
1. `chisq.go` + tests (pure math, no DB) — fastest feedback, foundation.
2. Store query methods + tests.
3. `analysis.go` + tests (wires 1 + 2).
4. `report.go` + tests.
5. Config + `main.go` `--report-once` wiring.
6. `deploy/trust-issues-report.service` + `.timer`.
7. Update `TODO.md`, `README.md` status section.

---

## Decision index

| ID | Decision | Deferred to TODO.md |
|---|---|---|
| D1 | Cumulative-to-date per `(user, playlist, snapshot)` | — |
| D2 | Separate chi-squared test per snapshot | — |
| D3 | Count once per artist on a played track | — |
| D4 | Include all plays (skipped + non-skipped); state in report | Config knob to filter skipped plays |
| D5 | OS-level: systemd timer (primary) + cron fallback; `--report-once` flag on the binary | — |
| D6 | `flag` package, `--report-once` boolean | — |
| D7 | Full per-track and per-artist tables, sorted by contribution desc | — |
| D8 | Skip chi-squared if `M < 30`, still print raw counts; configurable via `reports.min_plays` | — |
| D9 | Refuse to overwrite an existing `reports/YYYYMMDD.md` | Suffix `-1`, `-2`, … for same-day re-runs |
| D10 | Two files: `reports/YYYYMMDD.md` (summary) + `reports/YYYYMMDD-data.md` (full tables appendix) | — |
