# TODO — deferred from Milestone 1 & Milestone 2

Milestone 1 ships data collection only (auth, polling, playlist sync, SQLite storage).
Milestone 2 ships chi-squared analysis, daily markdown reports, and systemd-timer scheduling.

Items below are deferred; capture is here so they aren't lost.

## Analysis & reporting
- [x] Chi-squared goodness-of-fit analysis engine (per-track and per-artist) — see `DESCRIPTION.md` "Randomness analysis" section
- [x] Daily markdown report writer to `reports/YYYYMMDD.md` (summary) + `reports/YYYYMMDD-data.md` (full tables appendix) — must include enough raw data to reproduce the chi-squared finding without DB access (see DESCRIPTION.md "Report reproducibility")
- [x] Cron / scheduled trigger for daily report — systemd timer (`deploy/trust-issues-report.timer`), fires daily at 03:00 Europe/Warsaw
- [ ] Config knob to filter skipped plays out of analysis (v1 includes all plays — skipped + non-skipped — by default; state this in the report)
- [ ] Bonferroni (or similar) multiple-testing correction for the global "flagged?" column — v1 uses flat p<0.01 with raw p-values always printed
- [ ] FDR/BH (Benjamini-Hochberg) correction for per-track/per-artist residual flags — v1 uses flat |r|>3 with false-positive rate documented in report footer
- [ ] Streaming/batch analysis for large historical datasets (v1 map-based approach loads all plays per snapshot into memory — fine for ~2000 tracks / thousands of plays)

## Operations
- [x] `deploy/trust-issues.service` systemd unit (Restart=on-failure, journal logging)
- [x] `deploy/trust-issues-report.service` + `deploy/trust-issues-report.timer` systemd units (oneshot report generation, daily at 03:00 Europe/Warsaw)
- [ ] Hot config reload via fsnotify — start/stop per-account goroutines on TOML change without restart

## Future / nice-to-have
- [ ] Web UI / dashboard
- [ ] Backfill of historical plays (Spotify API does not currently expose this)
- [ ] Suffix same-day report re-runs with `-1`, `-2`, … instead of refusing (v1 errors out if `reports/YYYYMMDD.md` or `reports/YYYYMMDD-data.md` already exists)
