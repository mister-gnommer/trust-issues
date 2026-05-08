# Spotify — Get Currently Playing Track

Source: https://developer.spotify.com/documentation/web-api/reference/get-the-users-currently-playing-track

---

## Endpoint

```
GET https://api.spotify.com/v1/me/player/currently-playing
```

### Required scope

`user-read-currently-playing`

### Query parameters

| Param | Type | Required | Notes |
|---|---|---|---|
| `market` | string | No | ISO 3166-1 alpha-2 country code. If omitted and no user country on token, content may be considered unavailable. |
| `additional_types` | string | No | Comma-separated. Valid: `track`, `episode`. Default is `track` only. |

### Example request

```bash
curl --request GET \
  --url https://api.spotify.com/v1/me/player/currently-playing \
  --header 'Authorization: Bearer <access_token>'
```

---

## Responses

| Status | Meaning |
|---|---|
| `200` | Something is currently playing — body contains the Currently Playing Object |
| `204` | Nothing is playing, or no active device found — **empty body** |
| `401` | Bad/missing authorization header |
| `403` | Missing or insufficient scope |
| `429` | Rate limit exceeded |

> **Important:** Always check for 204 before reading the body. 204 means no active playback at all.

---

## Response body — Currently Playing Object

```json
{
  "context": {
    "type": "playlist",
    "href": "https://api.spotify.com/v1/playlists/37i9dQZF1DXcBWIGoYBM5M",
    "external_urls": {
      "spotify": "https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M"
    },
    "uri": "spotify:playlist:37i9dQZF1DXcBWIGoYBM5M"
  },
  "timestamp": 1715100000000,
  "progress_ms": 43210,
  "is_playing": true,
  "item": {
    "id": "4iV5W9uYEdYUVa79Axb7Rh",
    "name": "Track Name",
    "uri": "spotify:track:4iV5W9uYEdYUVa79Axb7Rh",
    "duration_ms": 230000,
    "explicit": false,
    "popularity": 72,
    "track_number": 3,
    "disc_number": 1,
    "is_playable": true,
    "href": "https://api.spotify.com/v1/tracks/4iV5W9uYEdYUVa79Axb7Rh",
    "external_urls": { "spotify": "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh" },
    "external_ids": { "isrc": "USRC12345678" },
    "preview_url": "https://p.scdn.co/mp3-preview/...",
    "type": "track",
    "album": { "...": "..." },
    "artists": [{ "...": "..." }]
  },
  "currently_playing_type": "track"
}
```

### Field reference

| Field | Type | Notes |
|---|---|---|
| `is_playing` | boolean | `true` = actively playing now; `false` = paused |
| `progress_ms` | integer | How far into the track in ms |
| `timestamp` | integer | Unix ms timestamp of when this state was last updated |
| `currently_playing_type` | string | `"track"` or `"episode"` |
| `context` | object \| null | Where the track is being played from. **Can be null** — see below. |
| `item` | object | The full Track or Episode object |

---

## The `context` object — playlist detection

This is the key field for knowing **which playlist the user is playing from**.

```json
{
  "context": {
    "type": "playlist",
    "uri": "spotify:playlist:37i9dQZF1DXcBWIGoYBM5M",
    "href": "https://api.spotify.com/v1/playlists/37i9dQZF1DXcBWIGoYBM5M",
    "external_urls": {
      "spotify": "https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M"
    }
  }
}
```

| Field | Notes |
|---|---|
| `context.type` | `"playlist"`, `"album"`, `"artist"`, `"show"` |
| `context.uri` | The Spotify URI — e.g. `spotify:playlist:<playlist_id>` |
| `context.href` | API URL to fetch the full playlist/album/etc. object |

### Extracting the playlist ID

```
context.uri  →  "spotify:playlist:37i9dQZF1DXcBWIGoYBM5M"
                              ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
                              split(':')[2]  →  playlist ID
```

### Comparing to a known playlist

```ts
function isPlayingFromPlaylist(response: CurrentlyPlaying, playlistId: string): boolean {
  return (
    response.context !== null &&
    response.context.type === "playlist" &&
    response.context.uri === `spotify:playlist:${playlistId}`
  );
}
```

### When `context` is `null`

`context` will be `null` (not missing — explicitly null) when the track is played outside of any browseable context, e.g.:

- Played directly from Search results
- Played from a Recommendation / Radio session
- Played via direct URI play command without a playlist context

In these cases you **cannot** determine which playlist the track came from. The track itself (`item`) is still returned.

---

## Notes on `GET /me/player` vs `GET /me/player/currently-playing`

There is also a broader endpoint:

```
GET https://api.spotify.com/v1/me/player
```

Requires scope: `user-read-playback-state`

It returns the same `context` and `item` fields plus additional state:

```json
{
  "device": { "id": "...", "name": "Kitchen speaker", "type": "computer", "volume_percent": 59 },
  "repeat_state": "off",
  "shuffle_state": false,
  "context": { "...": "same as above" },
  "is_playing": true,
  "item": { "...": "same as above" },
  "progress_ms": 43210,
  "timestamp": 1715100000000,
  "currently_playing_type": "track",
  "actions": { "pausing": true, "skipping_next": true, "...": "..." }
}
```

Use `/me/player` if you also need device info or playback controls context. Use `/me/player/currently-playing` if you only care about what's playing + the playlist context.
