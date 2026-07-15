# Changelog

## [Unreleased] — milestone-2-analytics

### Added

- **Analytics engine** (`internal/analysis/`): chi-squared goodness-of-fit test that evaluates
  whether shuffle plays are uniformly distributed across playlist tracks and artists.
  Cramér's V (effect size), standardized residuals, expected-frequency tables.
  Minimum-play and residual-threshold configuration.
- **Report generation** (`internal/report/`): renders two Markdown files per day — a
  human-readable summary and a data appendix with per-playlist breakdowns.
- **Store layer** (`internal/store/`): SQLite schema for plays, tracks, artists, users,
  playlist snapshots, and snapshot membership. Query methods for analytics window selection.
- **Daily report systemd units** (`deploy/trust-issues-report.service`,
  `deploy/trust-issues-report.timer`): oneshot report generation scheduled at 03:00 CET.
- **Report config** (`config.example.toml`): `[reports]` section — min_plays, residual_threshold,
  output directory. `LoadForReport` skips Spotify credential validation.
- **Command-line tests**: integration tests for `runReportOnce` covering empty store, seeded data,
  partial-write auto-recovery, idempotent no-op, and multi-account reports.

### Changed

- **Binary split**: the single `trust-issues` daemon is now two dedicated binaries:
  `trust-issues-poller` (continuous sync) and `trust-issues-report` (oneshot report).
  Each imports only the packages it needs.
- **Config**: `[accounts]` array with `user_id` + `display_name` replaces the single-account
  `[spotify]` block. Multi-account support throughout poller and report.
- **Systemd units** updated to reference the new poller binary.
- **README** reflects the new structure.

### Upgrade from main

1. Build both binaries:
   ```bash
   go build ./cmd/trust-issues-poller/ ./cmd/trust-issues-report/
   ```

2. Copy to deployment directory:
   ```bash
   sudo cp trust-issues-poller trust-issues-report /opt/trust-issues/
   sudo chown trust-issues:trust-issues /opt/trust-issues/trust-issues-poller /opt/trust-issues/trust-issues-report
   ```

3. Update systemd units — see `deploy/trust-issues.service` and `deploy/trust-issues-report.service`
   for updated `ExecStart` lines.

4. Reload and restart:
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl restart trust-issues
   sudo systemctl enable --now trust-issues-report.timer
   ```

   The report side is timer-driven; `enable --now trust-issues-report.timer` picks up the
   updated unit and schedules the next daily run.

5. Remove old binary:
   ```bash
   sudo rm /opt/trust-issues/trust-issues
   ```
