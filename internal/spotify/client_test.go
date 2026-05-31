package spotify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestClient builds a Client whose baseURL points at the test server
// and whose HTTP client is the default (no oauth2 wrapping).
func newTestClient(srv *httptest.Server) *Client {
	return &Client{http: srv.Client(), baseURL: srv.URL}
}

func TestParseContextURI(t *testing.T) {
	cases := []struct {
		uri      string
		wantKind ContextKind
		wantID   string
	}{
		{"spotify:playlist:abc123", ContextPlaylist, "abc123"},
		{"spotify:user:cartman:collection", ContextCollection, "cartman"},
		{"spotify:album:xyz", ContextAlbum, "xyz"},
		{"spotify:artist:art1", ContextArtist, "art1"},
		{"spotify:show:s1", ContextShow, "s1"},
		{"", ContextUnknown, ""},
		{"not-a-uri", ContextUnknown, ""},
		{"spotify:user:cartman", ContextUnknown, ""}, // user without :collection
	}
	for _, tc := range cases {
		k, id := ParseContextURI(tc.uri)
		if k != tc.wantKind || id != tc.wantID {
			t.Errorf("ParseContextURI(%q) = (%v,%q), want (%v,%q)", tc.uri, k, id, tc.wantKind, tc.wantID)
		}
	}
}

func TestGetPlayerState_204(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/me/player" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()
	c := newTestClient(srv)
	got, err := c.GetPlayerState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("got %+v, want nil", got)
	}
}

func TestGetPlayerState_200(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"is_playing": true,
			"progress_ms": 12345,
			"shuffle_state": true,
			"smart_shuffle": false,
			"context": {
				"type": "playlist",
				"uri": "spotify:playlist:p1",
				"href": "x"
			},
			"item": {
				"id": "t1",
				"name": "T",
				"uri": "spotify:track:t1",
				"duration_ms": 200000,
				"type": "track",
				"artists": [{"id": "a1", "name": "A"}],
				"album": {"id": "al1", "name": "AL"}
			}
		}`))
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()
	c := newTestClient(srv)
	ps, err := c.GetPlayerState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ps == nil || !ps.IsPlaying || !ps.ShuffleState {
		t.Fatalf("ps = %+v", ps)
	}
	if ps.Item.ID != "t1" || len(ps.Item.Artists) != 1 || ps.Item.Artists[0].ID != "a1" {
		t.Errorf("item = %+v", ps.Item)
	}
	if ps.Context == nil || ps.Context.Type != "playlist" {
		t.Errorf("context = %+v", ps.Context)
	}
}

func TestGetPlayerState_429RetryAfter(t *testing.T) {
	var calls int
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()
	c := newTestClient(srv)
	start := time.Now()
	_, err := c.GetPlayerState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("retry took too long: %v", elapsed)
	}
}

func TestListPlaylists_pagination(t *testing.T) {
	page1 := mustJSON(map[string]any{
		"items": []any{
			map[string]any{"id": "p1", "name": "One", "uri": "spotify:playlist:p1", "snapshot_id": "s1"},
		},
		"next": "/me/playlists?limit=50&offset=50", // server sees relative
	})
	page2 := mustJSON(map[string]any{
		"items": []any{
			map[string]any{"id": "p2", "name": "Two", "uri": "spotify:playlist:p2", "snapshot_id": "s2"},
		},
		"next": "",
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.RawQuery, "offset=50") {
			_, _ = w.Write(page2)
			return
		}
		_, _ = w.Write(page1)
	}))
	defer srv.Close()
	c := newTestClient(srv)
	// Override apiBase prefix-match by also storing baseURL into next handler:
	// the test "next" returned by page1 starts with "/" so relativeFromNext keeps it.
	got, err := c.ListPlaylists(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "p1" || got[1].ID != "p2" {
		t.Errorf("got %+v", got)
	}
}

func TestListPlaylistItems_filtersLocalAndNull(t *testing.T) {
	body := mustJSON(map[string]any{
		"items": []any{
			map[string]any{"is_local": false, "track": map[string]any{"id": "t1", "name": "Keep", "duration_ms": 1000, "is_local": false}},
			map[string]any{"is_local": true, "track": map[string]any{"id": "t2", "name": "Local"}},
			map[string]any{"is_local": false, "track": nil},
			map[string]any{"is_local": false, "track": map[string]any{"id": "", "name": "EmptyID"}},
		},
		"next": "",
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	c := newTestClient(srv)
	got, err := c.ListPlaylistItems(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "t1" {
		t.Errorf("got %+v, want [t1]", got)
	}
}

func TestRelativeFromNext(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"https://api.spotify.com/v1/me/playlists?offset=50", "/me/playlists?offset=50"},
		{"/me/playlists?offset=50", "/me/playlists?offset=50"},
		{"https://example.com/other", "https://example.com/other"}, // unrecognized → pass through
	}
	for _, tc := range cases {
		if got := relativeFromNext(tc.in); got != tc.want {
			t.Errorf("relativeFromNext(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
