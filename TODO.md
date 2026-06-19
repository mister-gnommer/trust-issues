# TODO — deferred from Milestone 1

Milestone 1 ships data collection only (auth, polling, playlist sync, SQLite storage).
Items below are intentionally deferred; capture is here so they aren't lost.

## Analysis & reporting
- [ ] Chi-squared goodness-of-fit analysis engine (per-track and per-artist) — see `DESCRIPTION.md` "Randomness analysis" section
- [ ] Daily markdown report writer to `reports/YYYYMMDD.md` (summary) + `reports/YYYYMMDD-data.md` (full tables appendix) (permanent, never deleted) — must include enough raw data to reproduce the chi-squared finding without DB access (see DESCRIPTION.md "Report reproducibility")
- [ ] Cron / scheduled trigger for daily report
- [ ] Config knob to filter skipped plays out of analysis (v1 includes all plays — skipped + non-skipped — by default; state this in the report)
- [ ] Bonferroni (or similar) multiple-testing correction for the global "flagged?" column (D11) — v1 uses flat p<0.01 with raw p-values always printed
- [ ] FDR/BH (Benjamini-Hochberg) correction for per-track/per-artist residual flags (D15) — v1 uses flat |r|>3 with false-positive rate documented in report footer
- [ ] Streaming/batch analysis for large historical datasets (v1 map-based approach loads all plays per snapshot into memory — fine for ~2000 tracks / thousands of plays)

## Operations
- [ ] Hot config reload via fsnotify — start/stop per-account goroutines on TOML change without restart
- [ ] `deploy/trust-issues.service` systemd unit (Restart=on-failure, journal logging)

## Future / nice-to-have
- [ ] Web UI / dashboard
- [ ] Backfill of historical plays (Spotify API does not currently expose this)
- [ ] Suffix same-day report re-runs with `-1`, `-2`, … instead of refusing (v1 errors out if `reports/YYYYMMDD.md` or `reports/YYYYMMDD-data.md` already exists)
