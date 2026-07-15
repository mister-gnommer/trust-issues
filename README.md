# trust-issues

A Go service that watches your Spotify shuffle playback and statistically analyzes whether Spotify's shuffle is truly random — or secretly playing Metallica more than it should.

## General QA disclaimer

This project is AI-generated and used as a learning exercise for a TypeScript developer picking up Go. Entrusting it with any credentials — or even running it — marks you as very brave. Or stupid. Or both. I'll run it myself shortly. :D

## What it does

- **Polls** Spotify's `/me/player` API to detect track changes
- **Records** every play with context (playlist, shuffle state, smart shuffle, skip detection)
- **Syncs** playlist contents hourly to know what songs were available at play time
- **Stores** everything in SQLite for later analysis
- **Supports multiple accounts** — one poller + syncer goroutine per account

## Status

**Milestone 1 ✅** — Data collection complete:
- Authentication (OAuth2 refresh tokens)
- Playback polling (two-phase: idle 30s, active 5s)
- Playlist syncing (including Liked Songs)
- SQLite storage with historical snapshots

**Milestone 2 ✅** — Analysis & reporting:
- Chi-squared goodness-of-fit test (per-track, per-artist, Cramér's V effect size, standardized residuals)
- Daily markdown reports to `reports/YYYYMMDD.md` (summary) + `reports/YYYYMMDD-data.md` (full tables)
- Systemd-timer scheduling (daily at 03:00 Europe/Warsaw, `deploy/trust-issues-report.timer`)

## How it works

```
┌────────────────────────────────────────────────────────────┐
│  VPS (always running)                                      │
│                                                            │
│  per account:                                              │
│  ┌─────────────┐    ┌──────────────┐                      │
│  │   Poller    │───▶│              │                      │
│  │  goroutine  │    │   Storage    │                      │
│  └─────────────┘    │   (SQLite)   │                      │
│  ┌─────────────┐    │              │                      │
│  │  Playlist   │───▶│              │                      │
│  │   Syncer    │    └──────┬───────┘                      │
│  └─────────────┘           │                              │
│                            ▼                              │
│                   ┌──────────────────┐                    │
│                   │  Analysis Engine │                    │
│                   │  (chi-squared,   │                    │
│                   │   per-track,     │                    │
│                   │   per-artist)    │                    │
│                   └──────────────────┘                    │
│                            │                              │
│              systemd timer → reports/YYYYMMDD.md           │
│              03:00 Europe/Warsaw                           │
└────────────────────────────────────────────────────────────┘
```

## Why snapshots?

Spotify assigns a `snapshot_id` to each playlist version — like a Git commit hash. Every time you add or remove tracks, the ID changes. This project stores the full track list per snapshot so the analysis can answer: "What tracks were available when this play happened?"

**Without snapshots:**
> "Metallica was played 8 times out of 250 tracks" — but the first play happened when there were only 200 tracks!

**With snapshots:**
> "Play #1: snapshot A (200 tracks), Metallica had 4% of playlist → expected 4%"
> "Play #2: snapshot B (250 tracks), Metallica had 3.2% of playlist → expected 3.2%"

Spotify thought of us. :)
Docs: https://developer.spotify.com/documentation/web-api/reference/get-playlist

## Tech stack

| Component | Choice |
|-----------|--------|
| Language | Go 1.25.9 |
| Database | SQLite (`mattn/go-sqlite3`) |
| Spotify client | Manual HTTP + `golang.org/x/oauth2` |
| Config | TOML (`BurntSushi/toml`) |
| Logging | `log/slog` (structured JSON) |
| Concurrency | `golang.org/x/sync/errgroup` |

## Quick start

```bash
# Build
go build ./cmd/trust-issues-poller/ ./cmd/trust-issues-report/

# Create config (see config.example.toml)
cp config.example.toml config.toml
# Edit with your Spotify credentials and refresh token

# Run the poller (continuous polling + syncing)
./trust-issues-poller -config config.toml

# Generate a one-shot report and exit (no Spotify API calls)
./trust-issues-report -config config.toml
```

## Configuration

```toml
[app]
client_id     = "your_spotify_client_id"
client_secret = "your_spotify_client_secret"

[storage]
database_path = "./trust.db"

[reports]
dir                = "./reports"   # report output directory
min_plays          = 30            # skip chi-squared if M < this
residual_threshold = 3             # flag track/artist if |residual| > this

[[accounts]]
user_id       = "spotify_user_id"
display_name  = "Kris"
refresh_token = "AQD..."
```

See [`config.example.toml`](config.example.toml) for the full example.

### Report-only mode

The report binary (`trust-issues-report`) runs analysis, writes reports, and exits — no Spotify API calls.
This is the entry point for the daily systemd timer:

```bash
./trust-issues-report -config config.toml
```

Config validation in report-only mode skips `client_id`/`client_secret`/`refresh_token` checks.
All times in the report use `Europe/Warsaw` timezone (CET/CEST with automatic DST).

### Scheduling

Daily report generation at 03:00 `Europe/Warsaw` via systemd timer:

```bash
sudo systemctl enable --now trust-issues-report.timer
```

**Cron fallback** (non-systemd boxes):
```
0 3 * * * /opt/trust-issues/trust-issues-report -config /opt/trust-issues/config.toml
```

**DST note:** during daylight-saving transitions the timer may shift by an hour or fire twice; harmless for daily reports. If the VPS was off for N days, `Persistent=true` fires once on boot producing one cumulative report — not N backfill reports.

## Adding accounts

1. Obtain a refresh token via OAuth flow (see [DESCRIPTION.md](DESCRIPTION.md#A7))
2. Add a new `[[accounts]]` block to `config.toml`
3. Restart the service

## Project structure

```
cmd/trust-issues-poller/  # Poller daemon entry point
cmd/trust-issues-report/  # Report generation entry point
internal/
  analysis/           # Chi-squared math + analysis engine
  config/             # TOML config parsing
  report/             # Markdown report renderer + atomic writer
  spotify/            # Spotify Web API client
  store/              # SQLite layer
  playback/           # Poller goroutine
  playlists/          # Playlist syncer goroutine
deploy/
  trust-issues.service           # Poller + syncer daemon
  trust-issues-report.service    # Oneshot report generation
  trust-issues-report.timer      # Daily 03:00 Europe/Warsaw schedule
```

## License

MIT

## Architectural decisions

### One query per logical read, no mega-joins

The analysis engine issues several small SQL queries per snapshot (track IDs, observed play counts per track, artist track counts, observed play counts per artist) rather than one large join that returns all data at once. This is intentional:

- SQLite is a local file — no network roundtrip, so each query is microseconds. 4 queries × 100 snapshots ≈ a few hundred ms, negligible for a daily cron job at 03:00.
- Each query maps to one clear concept and is unit-tested in isolation. A mega-query joining snapshots + tracks + track_artists + plays would return a denormalized row explosion that would have to be re-aggregated in Go, and would be hard to read and test.
- Name lookups (`TrackNames`/`ArtistNames`) are already batched at 900 IDs per query, so they're 2 queries total — not N+1.

If the DB ever moves to Postgres over a network, or snapshot count grows into the thousands, revisit then.
