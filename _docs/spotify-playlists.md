# Spotify — User Playlists & Playlist Tracks

---

## 1. Get Current User's Playlists

Source: https://developer.spotify.com/documentation/web-api/reference/get-a-list-of-current-users-playlists

```
GET https://api.spotify.com/v1/me/playlists
```

### Required scopes

- `playlist-read-private` — to include private playlists
- `playlist-read-collaborative` — to include collaborative playlists owned by others

### Query parameters

| Param | Type | Required | Default | Max | Notes |
|---|---|---|---|---|---|
| `limit` | integer | No | 20 | 50 | Items per page |
| `offset` | integer | No | 0 | — | Pagination offset |

### Example request

```bash
curl --request GET \
  --url 'https://api.spotify.com/v1/me/playlists?limit=50' \
  --header 'Authorization: Bearer <access_token>'
```

### Response body

Paginated list of **Simplified Playlist Objects**.

```json
{
  "href": "https://api.spotify.com/v1/me/playlists?offset=0&limit=50",
  "limit": 50,
  "next": "https://api.spotify.com/v1/me/playlists?offset=50&limit=50",
  "offset": 0,
  "previous": null,
  "total": 73,
  "items": [
    {
      "id": "3cEYpjA9oz9GiPac4AsH4n",
      "name": "My Playlist",
      "uri": "spotify:playlist:3cEYpjA9oz9GiPac4AsH4n",
      "href": "https://api.spotify.com/v1/playlists/3cEYpjA9oz9GiPac4AsH4n",
      "description": "Some description",
      "public": true,
      "collaborative": false,
      "snapshot_id": "abc123",
      "tracks": {
        "href": "https://api.spotify.com/v1/playlists/3cEYpjA9oz9GiPac4AsH4n/tracks",
        "total": 42
      },
      "owner": {
        "id": "username",
        "display_name": "Username",
        "href": "https://api.spotify.com/v1/users/username",
        "uri": "spotify:user:username",
        "type": "user",
        "external_urls": { "spotify": "https://open.spotify.com/user/username" }
      },
      "images": [
        { "url": "https://i.scdn.co/image/...", "height": 300, "width": 300 }
      ],
      "type": "playlist",
      "external_urls": { "spotify": "https://open.spotify.com/playlist/3cEYpjA9oz9GiPac4AsH4n" }
    }
  ]
}
```

### Key fields per playlist item

| Field | Notes |
|---|---|
| `id` | Playlist ID — use this in the tracks endpoint |
| `name` | Display name |
| `uri` | `spotify:playlist:<id>` — same format as `context.uri` from playback state |
| `tracks.total` | Track count without fetching tracks |
| `public` | `false` = private playlist |
| `collaborative` | `true` = others can add tracks |
| `owner.id` | Useful to filter playlists you own vs. ones you follow |

### Pagination

Max 50 per request. If `next` is not `null`, follow that URL to get the next page.

```ts
// Fetch all playlists
async function getAllPlaylists(token: string) {
  const playlists = [];
  let url: string | null = "https://api.spotify.com/v1/me/playlists?limit=50";
  while (url) {
    const res = await fetch(url, { headers: { Authorization: `Bearer ${token}` } });
    const data = await res.json();
    playlists.push(...data.items);
    url = data.next;
  }
  return playlists;
}
```

---

## 2. Get Playlist Tracks

Source: https://developer.spotify.com/documentation/web-api/reference/get-playlists-items

> **Note:** `/playlists/{id}/tracks` is deprecated. Use `/playlists/{id}/items` instead — same response shape, same parameters.

```
GET https://api.spotify.com/v1/playlists/{playlist_id}/items
```

### Path parameters

| Param | Type | Required | Notes |
|---|---|---|---|
| `playlist_id` | string | Yes | Spotify playlist ID (from `id` field above) |

### Query parameters

| Param | Type | Required | Default | Max | Notes |
|---|---|---|---|---|---|
| `limit` | integer | No | 20 | 50 | Items per page |
| `offset` | integer | No | 0 | — | Pagination offset |
| `market` | string | No | — | — | ISO 3166-1 alpha-2 |
| `fields` | string | No | — | — | Comma-separated field filter — see below |
| `additional_types` | string | No | `track` | — | Also `episode` |

### The `fields` parameter (bandwidth saver)

If you only need track names and IDs, you can drastically reduce response size:

```
fields=items(track(id,name,uri,artists(name)))
```

### Example request

```bash
curl --request GET \
  --url 'https://api.spotify.com/v1/playlists/3cEYpjA9oz9GiPac4AsH4n/items?limit=50' \
  --header 'Authorization: Bearer <access_token>'
```

### Response body

```json
{
  "href": "https://api.spotify.com/v1/playlists/3cEYpjA9oz9GiPac4AsH4n/items?offset=0&limit=50",
  "limit": 50,
  "next": null,
  "offset": 0,
  "previous": null,
  "total": 42,
  "items": [
    {
      "added_at": "2024-01-15T10:30:00Z",
      "added_by": {
        "id": "username",
        "href": "https://api.spotify.com/v1/users/username",
        "type": "user",
        "uri": "spotify:user:username",
        "external_urls": { "spotify": "https://open.spotify.com/user/username" }
      },
      "is_local": false,
      "track": {
        "id": "4iV5W9uYEdYUVa79Axb7Rh",
        "name": "Track Name",
        "uri": "spotify:track:4iV5W9uYEdYUVa79Axb7Rh",
        "href": "https://api.spotify.com/v1/tracks/4iV5W9uYEdYUVa79Axb7Rh",
        "duration_ms": 230000,
        "explicit": false,
        "popularity": 72,
        "track_number": 3,
        "disc_number": 1,
        "preview_url": "https://p.scdn.co/mp3-preview/...",
        "type": "track",
        "is_local": false,
        "external_ids": { "isrc": "USRC12345678" },
        "external_urls": { "spotify": "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh" },
        "artists": [
          {
            "id": "0TnOYIS5sYzu59v9T7w0sR",
            "name": "Artist Name",
            "href": "https://api.spotify.com/v1/artists/0TnOYIS5sYzu59v9T7w0sR",
            "uri": "spotify:artist:0TnOYIS5sYzu59v9T7w0sR",
            "type": "artist",
            "external_urls": { "spotify": "https://open.spotify.com/artist/0TnOYIS5sYzu59v9T7w0sR" }
          }
        ],
        "album": {
          "id": "2up3OPMp9Tb4dAKM2erWXQ",
          "name": "Album Name",
          "album_type": "album",
          "total_tracks": 12,
          "release_date": "2020-03-15",
          "release_date_precision": "day",
          "uri": "spotify:album:2up3OPMp9Tb4dAKM2erWXQ",
          "images": [{ "url": "https://i.scdn.co/image/...", "height": 640, "width": 640 }],
          "artists": [{ "name": "Artist Name", "id": "0TnOYIS5sYzu59v9T7w0sR" }]
        }
      }
    }
  ]
}
```

### Key fields per item

| Field | Notes |
|---|---|
| `track.id` | Spotify track ID |
| `track.name` | Track title |
| `track.uri` | `spotify:track:<id>` |
| `track.artists[].name` | Artist name(s) |
| `track.duration_ms` | Length in milliseconds |
| `is_local` | `true` = local file, `id` will be `null` |
| `added_at` | ISO 8601 datetime |

### Edge cases

- Items where `track` is `null` can appear — happens when a track was removed from Spotify. Always null-check before reading `track.id` etc.
- `is_local: true` tracks have `id: null` — they can't be matched by Spotify ID.
- Playlists can contain episodes (`type: "episode"`) if `additional_types=episode` is passed — otherwise those slots are skipped.

### Pagination — fetch all tracks

```ts
async function getAllPlaylistTracks(token: string, playlistId: string) {
  const tracks = [];
  let url: string | null = `https://api.spotify.com/v1/playlists/${playlistId}/items?limit=50`;
  while (url) {
    const res = await fetch(url, { headers: { Authorization: `Bearer ${token}` } });
    const data = await res.json();
    for (const item of data.items) {
      if (item.track && !item.is_local) {
        tracks.push(item.track);
      }
    }
    url = data.next;
  }
  return tracks;
}
```
