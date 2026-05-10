-- 🤖 AI-generated
-- trust-issues schema (multi-account from day one)

CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS users (
    id           TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    added_at     DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS artists (
    id   TEXT PRIMARY KEY,
    name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tracks (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    duration_ms INTEGER
);

CREATE TABLE IF NOT EXISTS track_artists (
    track_id  TEXT NOT NULL,
    artist_id TEXT NOT NULL,
    PRIMARY KEY (track_id, artist_id)
);

CREATE TABLE IF NOT EXISTS playlist_snapshots (
    id            TEXT PRIMARY KEY,
    user_id       TEXT NOT NULL,
    playlist_id   TEXT NOT NULL,
    playlist_name TEXT NOT NULL,
    captured_at   DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_playlist_snapshots_user_playlist
    ON playlist_snapshots (user_id, playlist_id, captured_at DESC);

CREATE TABLE IF NOT EXISTS playlist_snapshot_tracks (
    snapshot_id TEXT NOT NULL,
    track_id    TEXT NOT NULL,
    position    INTEGER,
    PRIMARY KEY (snapshot_id, track_id)
);

CREATE TABLE IF NOT EXISTS plays (
    id                       INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id                  TEXT    NOT NULL,
    track_id                 TEXT    NOT NULL,
    playlist_id              TEXT,
    playlist_snapshot_id     TEXT,
    shuffle_state            BOOLEAN NOT NULL,
    smart_shuffle            BOOLEAN NOT NULL,
    played_at                DATETIME NOT NULL,
    ended_at                 DATETIME,
    progress_ms_at_detection INTEGER NOT NULL,
    skipped                  BOOLEAN
);

CREATE INDEX IF NOT EXISTS idx_plays_user_open
    ON plays (user_id, played_at DESC) WHERE ended_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_plays_user_played_at
    ON plays (user_id, played_at);
