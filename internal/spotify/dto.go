// 🤖 AI-generated
package spotify

// PlayerState is the response of GET /v1/me/player.
// 204 (no active device) is converted to (nil, nil) by the client.
type PlayerState struct {
	IsPlaying    bool     `json:"is_playing"`
	ProgressMS   int64    `json:"progress_ms"`
	ShuffleState bool     `json:"shuffle_state"`
	SmartShuffle bool     `json:"smart_shuffle"` // undocumented but observed; defaults to false when absent
	Context      *Context `json:"context"`
	Item         *Item    `json:"item"`
}

type Context struct {
	Type string `json:"type"` // "playlist", "album", "artist", "show", "collection"
	URI  string `json:"uri"`
	Href string `json:"href"`
}

type Item struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	URI        string         `json:"uri"`
	DurationMS int64          `json:"duration_ms"`
	Type       string         `json:"type"` // "track" or "episode"
	Artists    []SimpleArtist `json:"artists"`
	Album      *SimpleAlbum   `json:"album"`
}

type SimpleArtist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type SimpleAlbum struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Playlist is a simplified playlist entry from /v1/me/playlists.
type Playlist struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	URI        string `json:"uri"`
	SnapshotID string `json:"snapshot_id"`
	Tracks     struct {
		Total int `json:"total"`
	} `json:"tracks"`
}

// PlaylistItem is one item from /v1/playlists/{id}/items. Track may be nil
// when a track has been removed from Spotify.
type PlaylistItem struct {
	IsLocal bool   `json:"is_local"`
	Track   *Track `json:"track"`
}

// Track is the full track object embedded in playlist items and /me/tracks responses.
type Track struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	URI        string         `json:"uri"`
	DurationMS int64          `json:"duration_ms"`
	IsLocal    bool           `json:"is_local"`
	Artists    []SimpleArtist `json:"artists"`
	Album      *SimpleAlbum   `json:"album"`
}

// SavedTrack is one item from /v1/me/tracks (Liked Songs).
type SavedTrack struct {
	AddedAt string `json:"added_at"`
	Track   *Track `json:"track"`
}
