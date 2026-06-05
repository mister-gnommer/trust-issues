package playlists

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/mister-gnommer/trust-issues/internal/playback"
	"github.com/mister-gnommer/trust-issues/internal/spotify"
	"github.com/mister-gnommer/trust-issues/internal/store"
)

type fakeSource struct {
	playlists []spotify.Playlist
	items     map[string][]spotify.Track
	liked     []spotify.Track

	itemsCalls int
	likedCalls int
}

func (f *fakeSource) ListPlaylists(ctx context.Context) ([]spotify.Playlist, error) {
	return f.playlists, nil
}
func (f *fakeSource) ListPlaylistItems(ctx context.Context, id string) ([]spotify.Track, error) {
	f.itemsCalls++
	return f.items[id], nil
}
func (f *fakeSource) ListLikedTracks(ctx context.Context) ([]spotify.Track, error) {
	f.likedCalls++
	return f.liked, nil
}

func newRec(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func newLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestSyncOnce_storesAndSkipsUnchanged(t *testing.T) {
	rec := newRec(t)
	ctx := context.Background()
	log := newLogger()
	account := Account{UserID: "u1", DisplayName: "Kris"}

	src := &fakeSource{
		playlists: []spotify.Playlist{
			{ID: "p1", Name: "Mix", URI: "spotify:playlist:p1", SnapshotID: "snap-A"},
			{ID: "p2", Name: "Rock", URI: "spotify:playlist:p2", SnapshotID: "snap-B"},
		},
		items: map[string][]spotify.Track{
			"p1": {{ID: "t1", Name: "T1", DurationMS: 200_000, Artists: []spotify.SimpleArtist{{ID: "a1", Name: "A1"}}}},
			"p2": {{ID: "t2", Name: "T2", DurationMS: 300_000, Artists: []spotify.SimpleArtist{{ID: "a2", Name: "A2"}}}},
		},
		liked: []spotify.Track{{ID: "t3", Name: "Liked", DurationMS: 250_000, Artists: []spotify.SimpleArtist{{ID: "a3", Name: "A3"}}}},
	}

	t.Run("first pass stores all snapshots", func(t *testing.T) {
		if err := SyncOnce(ctx, log, account, src, rec); err != nil {
			t.Fatal(err)
		}
		if src.itemsCalls != 2 {
			t.Errorf("itemsCalls = %d, want 2", src.itemsCalls)
		}
		id, ok, _ := rec.LatestSnapshotID(ctx, "u1", "p1")
		if !ok || id != "snap-A" {
			t.Errorf("snap p1 = %q ok=%v", id, ok)
		}
		likedID, ok, _ := rec.LatestSnapshotID(ctx, "u1", playback.LikedPlaylistID("u1"))
		if !ok || likedID == "" {
			t.Errorf("liked snap missing: %q ok=%v", likedID, ok)
		}
	})

	t.Run("second pass skips unchanged playlists", func(t *testing.T) {
		src.itemsCalls = 0
		src.likedCalls = 0
		if err := SyncOnce(ctx, log, account, src, rec); err != nil {
			t.Fatal(err)
		}
		if src.itemsCalls != 0 {
			t.Errorf("itemsCalls = %d, want 0", src.itemsCalls)
		}
		if src.likedCalls != 1 {
			t.Errorf("likedCalls = %d, want 1", src.likedCalls)
		}
	})

	t.Run("changed playlist refetched", func(t *testing.T) {
		src.playlists[0].SnapshotID = "snap-A2"
		src.items["p1"] = append(src.items["p1"], spotify.Track{ID: "t1b", Name: "Added", DurationMS: 100_000})
		src.itemsCalls = 0
		if err := SyncOnce(ctx, log, account, src, rec); err != nil {
			t.Fatal(err)
		}
		if src.itemsCalls != 1 {
			t.Errorf("itemsCalls = %d, want 1", src.itemsCalls)
		}
		id, _, _ := rec.LatestSnapshotID(ctx, "u1", "p1")
		if id != "snap-A2" {
			t.Errorf("p1 snapshot = %q, want snap-A2", id)
		}
	})
}

func TestLikedSnapshotID_orderInvariant(t *testing.T) {
	a := []spotify.Track{{ID: "x"}, {ID: "y"}, {ID: "z"}}
	b := []spotify.Track{{ID: "z"}, {ID: "x"}, {ID: "y"}}
	if likedSnapshotID(a) != likedSnapshotID(b) {
		t.Error("hash should be stable across track order")
	}
	c := []spotify.Track{{ID: "x"}, {ID: "y"}}
	if likedSnapshotID(a) == likedSnapshotID(c) {
		t.Error("hash should change when track set changes")
	}
}
