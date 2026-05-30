package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

const (
	apiBase   = "https://api.spotify.com/v1"
	tokenURL  = "https://accounts.spotify.com/api/token"
	userAgent = "trust-issues/0.1"
)

var spotifyEndpoint = oauth2.Endpoint{TokenURL: tokenURL}

// Client is a Spotify Web API client scoped to one user account. Access tokens
// are refreshed automatically by the underlying oauth2 transport.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient builds a Client backed by an oauth2 token source. The refresh
// token must come from a prior Authorization Code flow. Access tokens are
// fetched on demand and cached until they expire.
func NewClient(ctx context.Context, clientID, clientSecret, refreshToken string) *Client {
	cfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     spotifyEndpoint,
	}
	src := cfg.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})
	httpClient := oauth2.NewClient(ctx, src)
	httpClient.Timeout = 30 * time.Second
	return &Client{
		http:    httpClient,
		baseURL: apiBase,
	}
}

// GetPlayerState calls GET /me/player. Returns (nil, nil) when nothing is playing
// (HTTP 204). Honors Retry-After on 429 with a single retry.
// Docs: https://developer.spotify.com/documentation/web-api/reference/get-information-about-the-users-current-playback
func (c *Client) GetPlayerState(ctx context.Context) (*PlayerState, error) {
	body, status, err := c.get(ctx, "/me/player")
	if err != nil {
		return nil, err
	}
	if status == http.StatusNoContent || len(body) == 0 {
		return nil, nil
	}
	var ps PlayerState
	if err := json.Unmarshal(body, &ps); err != nil {
		return nil, fmt.Errorf("decode player state: %w", err)
	}
	return &ps, nil
}

// ListPlaylists fetches all playlists for the current user, paginating /me/playlists.
// Docs: https://developer.spotify.com/documentation/web-api/reference/get-a-list-of-current-users-playlists
func (c *Client) ListPlaylists(ctx context.Context) ([]Playlist, error) {
	var out []Playlist
	next := "/me/playlists?limit=50"
	for next != "" {
		body, _, err := c.get(ctx, next)
		if err != nil {
			return nil, err
		}
		var page struct {
			Items []Playlist `json:"items"`
			Next  string     `json:"next"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("decode playlists page: %w", err)
		}
		out = append(out, page.Items...)
		next = relativeFromNext(page.Next)
	}
	return out, nil
}

// ListPlaylistItems fetches all tracks for a playlist. Items where Track is nil
// or IsLocal is true are skipped (they have no usable Spotify ID).
// Docs: https://developer.spotify.com/documentation/web-api/reference/get-playlists-tracks
func (c *Client) ListPlaylistItems(ctx context.Context, playlistID string) ([]Track, error) {
	var out []Track
	fields := "next,items(is_local,track(id,name,uri,duration_ms,is_local,artists(id,name),album(id,name)))"
	next := fmt.Sprintf("/playlists/%s/items?limit=50&fields=%s",
		url.PathEscape(playlistID), url.QueryEscape(fields))
	for next != "" {
		body, _, err := c.get(ctx, next)
		if err != nil {
			return nil, err
		}
		var page struct {
			Items []PlaylistItem `json:"items"`
			Next  string         `json:"next"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("decode playlist items page: %w", err)
		}
		for _, it := range page.Items {
			if it.Track == nil || it.IsLocal || it.Track.IsLocal || it.Track.ID == "" {
				continue
			}
			out = append(out, *it.Track)
		}
		next = relativeFromNext(page.Next)
	}
	return out, nil
}

// ListLikedTracks fetches the user's Liked Songs (Saved Tracks) collection.
// Docs: https://developer.spotify.com/documentation/web-api/reference/get-users-saved-tracks
func (c *Client) ListLikedTracks(ctx context.Context) ([]Track, error) {
	var out []Track
	next := "/me/tracks?limit=50"
	for next != "" {
		body, _, err := c.get(ctx, next)
		if err != nil {
			return nil, err
		}
		var page struct {
			Items []SavedTrack `json:"items"`
			Next  string       `json:"next"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("decode saved tracks page: %w", err)
		}
		for _, it := range page.Items {
			if it.Track == nil || it.Track.IsLocal || it.Track.ID == "" {
				continue
			}
			out = append(out, *it.Track)
		}
		next = relativeFromNext(page.Next)
	}
	return out, nil
}

// get performs a GET request against pathOrURL (relative path starting with "/"
// or absolute URL). On 429 it sleeps Retry-After (capped at 60s) and retries once.
func (c *Client) get(ctx context.Context, pathOrURL string) ([]byte, int, error) {
	requestURL := pathOrURL
	if strings.HasPrefix(pathOrURL, "/") {
		requestURL = c.baseURL + pathOrURL
	}

	const maxAttempts = 2
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, 0, err
		}
		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, resp.StatusCode, readErr
		}

		if resp.StatusCode == http.StatusTooManyRequests && attempt < maxAttempts {
			wait := parseRetryAfter(resp.Header.Get("Retry-After"))
			select {
			case <-ctx.Done():
				return nil, resp.StatusCode, ctx.Err()
			case <-time.After(wait):
			}
			continue
		}

		if resp.StatusCode >= 400 {
			// Body intentionally omitted from the error: it can contain
			// account- or query-specific data that doesn't belong in logs.
			return respBody, resp.StatusCode, fmt.Errorf("spotify GET %s: %d", pathOrURL, resp.StatusCode)
		}
		return respBody, resp.StatusCode, nil
	}
	return nil, 0, errors.New("unreachable")
}

func parseRetryAfter(h string) time.Duration {
	const fallback = 5 * time.Second
	const cap = 60 * time.Second
	if h == "" {
		return fallback
	}
	// Spotify returns integer seconds, so Atoi is sufficient.
	// If it ever switches to HTTP-date format, this falls back to 5s.
	secs, err := strconv.Atoi(strings.TrimSpace(h))
	if err != nil || secs < 0 {
		return fallback
	}
	d := time.Duration(secs) * time.Second
	if d > cap {
		return cap
	}
	return d
}

// relativeFromNext converts an absolute "next" URL from a paginated response
// into a path the client can re-issue against c.baseURL. Returns "" for empty.
func relativeFromNext(next string) string {
	if next == "" {
		return ""
	}
	path, _ := strings.CutPrefix(next, apiBase)
	return path
}

// ContextKind classifies a Spotify context URI as parsed by ParseContextURI.
type ContextKind int

const (
	ContextUnknown ContextKind = iota
	ContextPlaylist
	ContextCollection // Liked Songs: spotify:user:<id>:collection
	ContextAlbum
	ContextArtist
	ContextShow
)

// ParseContextURI splits a Spotify context URI like "spotify:playlist:<id>" or
// "spotify:user:<userID>:collection" into kind + id. id is the playlist ID for
// playlists, the user ID for collection (Liked Songs), or the entity ID for
// album/artist/show. Empty input returns (ContextUnknown, "").
// Docs: https://developer.spotify.com/documentation/web-api/reference/get-information-about-the-users-current-playback#context-object
func ParseContextURI(uri string) (ContextKind, string) {
	if uri == "" {
		return ContextUnknown, ""
	}
	parts := strings.Split(uri, ":")
	if len(parts) < 3 || parts[0] != "spotify" {
		return ContextUnknown, ""
	}
	switch parts[1] {
	case "playlist":
		return ContextPlaylist, parts[2]
	case "album":
		return ContextAlbum, parts[2]
	case "artist":
		return ContextArtist, parts[2]
	case "show":
		return ContextShow, parts[2]
	case "user":
		// spotify:user:<id>:collection
		if len(parts) == 4 && parts[3] == "collection" {
			return ContextCollection, parts[2]
		}
	}
	return ContextUnknown, ""
}
