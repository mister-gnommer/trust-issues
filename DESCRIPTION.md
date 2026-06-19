# trust-issues

A Go service that runs permanently on a VPS, silently watches what you're playing on Spotify in shuffle mode, and tells you whether Spotify's shuffle is actually random — or whether it's secretly playing Metallica more than it should.

**Report reproducibility:** If the analysis finds non-random distribution, the report must include enough raw data for anyone holding only the report to independently verify the finding — no access to the database required. The report consists of two files: `reports/YYYYMMDD.md` (scannable summary with top anomalies) and `reports/YYYYMMDD-data.md` (full per-track and per-artist tables appendix). Concretely this means: per-track and per-artist play counts, expected counts, chi-squared statistic, degrees of freedom, p-value, and which playlist snapshot the calculation was based on (snapshot ID + track count at that time).

---

## Problem statement

Spotify's shuffle feels biased. Some artists appear constantly; others almost never. This tool records every track played in shuffle mode, compares actual play frequencies to the statistically expected ones (based on how many songs each artist/track has in the active playlist), and surfaces deviations that are large enough to be non-random.

**Multi-account:** The app is designed from the start to support multiple Spotify accounts. Other people can grant access to their account, giving us more data for analysis. One app instance handles all accounts — one polling goroutine per account running concurrently. All DB tables are scoped by `user_id`.

---

## How it works — high level

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

Three background components, one set per registered account:

1. **Poller** — polls `GET /me/player` and detects track changes.
2. **Playlist syncer** — periodically re-fetches all playlist contents when `snapshot_id` changes.
3. **Analysis engine** — computes expected vs. actual play frequencies. Runs on a daily schedule and writes a markdown report.

---

## Data collected per play

Every time a new track is detected (regardless of shuffle state):

| Field | Source | Notes |
|---|---|---|
| `user_id` | config | Which account this play belongs to |
| `track_id` | `item.id` | Spotify track ID |
| `track_name` | `item.name` | |
| `track_duration_ms` | `item.duration_ms` | Full track length — used for skip detection |
| `artist_ids` | `item.artists[].id` | Multiple for collabs |
| `artist_names` | `item.artists[].name` | |
| `album_id` | `item.album.id` | |
| `playlist_id` | `context.uri` (split) | Null if no playlist context |
| `playlist_snapshot_id` | from syncer cache | Which version of the playlist was active |
| `shuffle_state` | `shuffle_state` | From `/me/player` |
| `smart_shuffle` | `smart_shuffle` | From `/me/player` — Spotify's AI shuffle variant |
| `played_at` | server timestamp | When the poller first detected this track |
| `ended_at` | server timestamp | When the next track was detected — set retroactively |
| `progress_ms_at_detection` | `progress_ms` | How far in when we first caught it |
| `skipped` | computed | True if `(ended_at - played_at) < max(30s, duration_ms * 25%)` |

**On `track_duration_ms`:** Already stored — together with `ended_at - played_at` it gives the fraction of the song actually heard. A 30-second detection in a 3-minute song (17%) is very different from 30 seconds in a 13-minute song (4%). The `skipped` flag uses `max(30s, 25% of duration)` as its threshold.

---

## Randomness analysis

For a given playlist with `N` total tracks, if shuffle were uniform random, each track has probability `1/N` of being selected each time. Over `M` shuffle plays from that playlist:

- **Expected plays per track**: `M / N`
- **Expected plays per artist with K tracks**: `K * M / N`

The tool computes the actual vs. expected ratio for each track and artist. Significant deviations are flagged using a **chi-squared goodness-of-fit test**: a high chi-squared value (low p-value) means the distribution is statistically non-random.

> **Example:** Playlist has 200 songs, Metallica has 8 of them. Expected share: 4%. If across 500 plays Metallica appears 40 times (8%), that's 2× the expected rate. The chi-squared test tells us whether this gap is noise or signal.

**Historical tracking:** Each daily report is kept permanently. This means you can see that Metallica was within tolerance in April but drifted out in May. The raw data is always there for recalculation; the reports are snapshots of the state at the time they were generated.

---

## Polling & rate limits

Spotify's rate limit is roughly **1–2 requests/second** sustained. The strategy minimizes calls while still catching skips.

**Two-phase polling strategy:**

| Phase | When | Interval |
|---|---|---|
| Idle | Nothing playing (`204` or `is_playing: false`) | Every 30 seconds |
| Active | Playback detected | Every 5 seconds |

```
1. Poll /me/player
2. If 204 or is_playing=false → increment no-playback counter
   - If no-playback for < 1 minute → stay in active phase, poll every 5s (handles brief pauses)
   - If no-playback for ≥ 1 minute → switch to idle phase, poll every 30s
3. New track detected → record it, switch to active phase, reset no-playback counter
4. Poll every 5s in active phase
5. When track changes → update previous play's ended_at, compute skipped, record new play
```

**On skipped songs:** The app records every track Spotify selects, regardless of whether it was skipped. The `skipped` flag is set after the fact when the next track is detected. This is intentional — randomness analysis is about what Spotify *chose* to play, not what you finished listening to. Skipped plays are included in the randomness calculation by default but can be filtered out in analysis.

---

## Playlist syncing

Playlists are dynamic. We need to know what the playlist contained **at the time each play happened** for accurate probability calculations.

**Sync strategy:**
- Check all playlists every **1 hour**: fetch `/me/playlists`, compare `snapshot_id` to stored value
- Only re-fetch tracks for playlists where `snapshot_id` changed
- Store the full track list per snapshot — old snapshots are never deleted

**Edge case — playlist edited between syncs:** If you add a song after the hourly sync, plays happening before the next sync will be associated with the previous snapshot (which doesn't include that song). Maximum inconsistency window: 1 hour. This is acceptable — the snapshot approach handles it gracefully, and once the sync runs, future plays will use the new snapshot.

---

## Database schema (proposed)

Multi-account from day one — every table is scoped by `user_id`.

```sql
-- Accounts
CREATE TABLE users (
    id           TEXT PRIMARY KEY,  -- Spotify user ID
    display_name TEXT NOT NULL,
    added_at     DATETIME NOT NULL
);

-- Spotify entities (cached/synced, shared across users)
CREATE TABLE artists (id TEXT PRIMARY KEY, name TEXT NOT NULL);
CREATE TABLE tracks  (id TEXT PRIMARY KEY, name TEXT NOT NULL, duration_ms INTEGER);
CREATE TABLE track_artists (track_id TEXT, artist_id TEXT, PRIMARY KEY (track_id, artist_id));

-- Playlist snapshots (append-only, per user)
CREATE TABLE playlist_snapshots (
    id            TEXT PRIMARY KEY,  -- Spotify snapshot_id
    user_id       TEXT NOT NULL,
    playlist_id   TEXT NOT NULL,
    playlist_name TEXT NOT NULL,
    captured_at   DATETIME NOT NULL
);
CREATE TABLE playlist_snapshot_tracks (
    snapshot_id TEXT NOT NULL,
    track_id    TEXT NOT NULL,
    position    INTEGER,
    PRIMARY KEY (snapshot_id, track_id)
);

-- Play history
CREATE TABLE plays (
    id                       INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id                  TEXT    NOT NULL,
    track_id                 TEXT    NOT NULL,
    playlist_id              TEXT,            -- NULL if context was null
    playlist_snapshot_id     TEXT,            -- NULL if context was null
    shuffle_state            BOOLEAN NOT NULL,
    smart_shuffle            BOOLEAN NOT NULL,
    played_at                DATETIME NOT NULL,
    ended_at                 DATETIME,        -- set when next track detected
    progress_ms_at_detection INTEGER NOT NULL,
    skipped                  BOOLEAN          -- computed when ended_at is set
);
```

---

## Decisions log

| # | Question | Decision |
|---|---|---|
| A1 | Min play duration | Record everything. `skipped = true` if played < max(30s, 25% of duration) |
| A2 | Null-context plays | Record with `playlist_id = null`, excluded from analysis |
| A3 | Non-shuffle plays | Record all, `shuffle_state` stored, analysis filters as needed |
| A4 | Which playlists to sync | All playlists (~100 playlists / ~2000 songs — trivial for SQLite) |
| A5 | Liked Songs handling | `type: "collection"`, synced via `GET /me/tracks`, one extra branch in context parser |
| A6 | Results interface | Daily markdown report written to `reports/YYYYMMDD.md` (summary) + `reports/YYYYMMDD-data.md` (full tables appendix), old ones kept |
| A7 | OAuth on VPS | Manual token: OAuth flow on laptop, paste refresh token into config |
| A8 | Database | SQLite |
| A9 | Historical snapshot accuracy | Snapshot at time of play — historical accuracy maintained |
| A10 | Analysis trigger | Background, daily report generation |

---

## Adding accounts

Accounts are managed via the TOML config file — no SQL client or interface needed. Each account is one `[[accounts]]` entry:

```toml
[app]
client_id     = "your_spotify_client_id"
client_secret = "your_spotify_client_secret"

[[accounts]]
user_id       = "spotify_user_id"
display_name  = "Kris"
refresh_token = "AQD..."

[[accounts]]
user_id       = "spotify_user_id_2"
display_name  = "Friend"
refresh_token = "AQD..."
```

**To add a new account:**
1. The new user does the OAuth flow on their own laptop (see A7 procedure) using your app's `client_id`/`client_secret`
2. They send you their refresh token (or you run the flow for them)
3. Add a new `[[accounts]]` block to the config on the VPS
4. The app watches the config file and hot-reloads it — no restart needed, the new poller goroutine starts automatically

The `users` table in SQLite is populated automatically on first poll — no manual SQL required.

---

## A5 — Liked Songs: testing procedure

Spotify may return a special context URI like `spotify:user:<id>:collection` when playing from Liked Songs. We need to confirm this before writing the handler.

**Steps:**
1. Go to [developer.spotify.com/dashboard](https://developer.spotify.com/dashboard), open your app, and use the built-in **access token generator** (under Settings or the app overview — look for "Get token" or similar). Check the `user-read-playback-state` scope.
2. Start playing something from **Liked Songs** (heart icon library) in shuffle mode on your phone or desktop.
3. Run this curl (replace `<token>`):

```bash
curl -s -H "Authorization: Bearer <token>" \
  https://api.spotify.com/v1/me/player | python3 -m json.tool | grep -A5 '"context"'
```

4. Report back what `context.type` and `context.uri` look like.

**Result:** Liked Songs returns `type: "collection"` and `uri: "spotify:user:<user_id>:collection"`. The `href` points to `GET /me/tracks` which is the endpoint to sync it.

Context parser handles it as a special case alongside `"playlist"` — one extra branch, synced via `GET /me/tracks` instead of `GET /playlists/{id}/items`.

---

## A7 — OAuth setup & refresh token TTL

**Security requirements (updated Feb 2025):** Spotify deprecated the Implicit Grant flow and requires HTTPS redirect URIs — with one explicit exception: `http://127.0.0.1` (loopback) is still allowed. This is exactly what we use.

**Flow for adding an account:**

Since we have a `client_secret`, we use the standard **Authorization Code flow** (not PKCE — that's for public clients without a secret).

1. Register `http://127.0.0.1:8888/callback` as a redirect URI in your Spotify app's dashboard settings
2. Run [`spotify-auth`](https://github.com/mister-gnommer/spotify-auth) **on your laptop** (not the VPS — needs a browser):
   - Starts a temporary HTTP server on `127.0.0.1:8888`
   - Opens the Spotify authorization URL in your browser
   - Catches the callback, exchanges the code for tokens
   - Prints the refresh token
3. Paste the refresh token into the `[[accounts]]` block in the TOML config

`spotify-auth` lives in its own repo — it's a generic Spotify OAuth helper with no trust-issues-specific logic, reusable for any Spotify project.

**Refresh token TTL:** Spotify refresh tokens have no fixed expiry. They remain valid indefinitely unless:
- The user manually revokes access (via spotify.com/account/apps)
- The user changes their Spotify password
- The app's client secret is rotated

Access tokens expire after 1 hour — the app handles renewal automatically.

---

## Technology choices

| Concern | Choice | Reason |
|---|---|---|
| Language | Go | Single binary, easy VPS deploy, goroutines per account |
| Database | SQLite via `mattn/go-sqlite3` | Zero ops, sufficient for the write rate |
| Spotify client | Manual HTTP + `golang.org/x/oauth2` | No maintained Go Spotify SDK worth depending on |
| Config | TOML file | List of accounts (user_id + refresh_token), app credentials |
| Deployment | `systemd` unit | Restarts on crash, standard on Linux |
| Stats | Chi-squared test (pure Go) | Well-understood, no external dependency |
| Reports | Markdown files in `reports/` | `reports/YYYYMMDD.md` (summary) + `reports/YYYYMMDD-data.md` (full tables), one pair per day, never deleted |

---

## What is NOT in scope (v1)

- Web UI / dashboard
- Modifying playlists or playback
- Historical import of past plays (Spotify doesn't expose this)
