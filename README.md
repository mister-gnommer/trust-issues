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

**Milestone 2 🚧** — Analysis (deferred):
- Chi-squared goodness-of-fit test (per-track and per-artist)
- Daily markdown reports to `reports/YYYYMMDD.md`
- Scheduled cron trigger

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
│                   daily cron → reports/YYYYMMDD.md        │
└────────────────────────────────────────────────────────────┘
```

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
go build ./cmd/trust-issues/

# Create config (see config.example.toml)
cp config.example.toml config.toml
# Edit with your Spotify credentials and refresh token

# Run
./trust-issues -config config.toml
```

## Configuration

```toml
[app]
client_id     = "your_spotify_client_id"
client_secret = "your_spotify_client_secret"

[storage]
database_path = "./trust.db"

[[accounts]]
user_id       = "spotify_user_id"
display_name  = "Kris"
refresh_token = "AQD..."
```

See [`config.example.toml`](config.example.toml) for the full example.

## Adding accounts

1. Obtain a refresh token via OAuth flow (see [DESCRIPTION.md](DESCRIPTION.md#A7))
2. Add a new `[[accounts]]` block to `config.toml`
3. Restart the service (hot reload planned for Milestone 2)

## Project structure

```
cmd/trust-issues/     # Entry point
internal/
  config/             # TOML config parsing
  spotify/            # Spotify Web API client
  store/              # SQLite layer
  playback/           # Poller goroutine
  playlists/          # Playlist syncer goroutine
```

## License

MIT
