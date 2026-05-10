# TODO — deferred from Milestone 1

Milestone 1 ships data collection only (auth, polling, playlist sync, SQLite storage).
Items below are intentionally deferred; capture is here so they aren't lost.

## Analysis & reporting
- [ ] Chi-squared goodness-of-fit analysis engine (per-track and per-artist) — see `DESCRIPTION.md` "Randomness analysis" section
- [ ] Daily markdown report writer to `reports/YYYYMMDD.md` (permanent, never deleted) — must include enough raw data to reproduce the chi-squared finding without DB access (see DESCRIPTION.md "Report reproducibility")
- [ ] Cron / scheduled trigger for daily report

## Operations
- [ ] Hot config reload via fsnotify — start/stop per-account goroutines on TOML change without restart
- [ ] `deploy/trust-issues.service` systemd unit (Restart=on-failure, journal logging)

## Future / nice-to-have
- [ ] Web UI / dashboard
- [ ] Backfill of historical plays (Spotify API does not currently expose this)
