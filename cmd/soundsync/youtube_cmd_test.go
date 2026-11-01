package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/CharlyCeballos/soundsync/internal/spotify"
	"github.com/CharlyCeballos/soundsync/internal/transfer"
	"github.com/CharlyCeballos/soundsync/internal/youtube"

	"golang.org/x/oauth2"
)

// fakeYouTube keeps the channel's playlists and their videos in memory.
type fakeYouTube struct {
	mu        sync.Mutex
	playlists []map[string]any
	inserts   int

	search      map[string][]map[string]any // query -> search items
	searches    int
	items       map[string][]string // playlist ID -> video IDs
	itemInserts int
	failItemAt  int // from this 1-based playlistItems.insert on, quota is exhausted (like YouTube); 0 = never
}

// videos returns a copy of a playlist's videos. Tests must read through it:
// a request cancelled on the client side may still be handled afterwards.
func (f *fakeYouTube) videos(playlistID string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.items[playlistID]...)
}

// counts returns the number of searches and playlist item inserts so far.
func (f *fakeYouTube) counts() (searches, itemInserts int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.searches, f.itemInserts
}

func (f *fakeYouTube) handler(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/search":
		f.searches++
		json.NewEncoder(w).Encode(map[string]any{"items": f.search[r.URL.Query().Get("q")]})
	case r.Method == http.MethodGet && r.URL.Path == "/playlistItems":
		var items []map[string]any
		for _, id := range f.items[r.URL.Query().Get("playlistId")] {
			items = append(items, map[string]any{"contentDetails": map[string]string{"videoId": id}})
		}
		json.NewEncoder(w).Encode(map[string]any{"items": items})
	case r.Method == http.MethodPost && r.URL.Path == "/playlistItems":
		f.itemInserts++
		if f.failItemAt > 0 && f.itemInserts >= f.failItemAt {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error":{"code":403,"message":"quota","errors":[{"reason":"quotaExceeded"}]}}`))
			return
		}
		var body struct {
			Snippet struct {
				PlaylistID string `json:"playlistId"`
				ResourceID struct {
					VideoID string `json:"videoId"`
				} `json:"resourceId"`
			} `json:"snippet"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if f.items == nil {
			f.items = map[string][]string{}
		}
		f.items[body.Snippet.PlaylistID] = append(f.items[body.Snippet.PlaylistID], body.Snippet.ResourceID.VideoID)
		w.Write([]byte(`{"id":"item"}`))
	case r.Method == http.MethodGet && r.URL.Path == "/playlists":
		json.NewEncoder(w).Encode(map[string]any{"items": f.playlists})
	case r.Method == http.MethodPost && r.URL.Path == "/playlists":
		var body map[string]map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		f.inserts++
		p := map[string]any{
			"id":             fmt.Sprintf("PLfake%d", f.inserts),
			"snippet":        body["snippet"],
			"status":         body["status"],
			"contentDetails": map[string]int{"itemCount": 0},
		}
		f.playlists = append(f.playlists, p)
		json.NewEncoder(w).Encode(p)
	default:
		http.NotFound(w, r)
	}
}

// fakeLibraryAPI serves a Spotify library with two owned playlists and one followed.
func fakeLibraryAPI(t *testing.T) *spotify.Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/playlists/5ZwLvrI8d6VnEQXxB0bMms/items" {
			w.Write([]byte(`{"items":[
				{"item":{"type":"track","id":"1Onx","name":"The Way (feat. Mac Miller)","artists":[{"name":"Ariana Grande"},{"name":"Mac Miller"}]}},
				{"item":{"type":"episode","id":"ep","name":"A podcast"}},
				{"item":{"type":"track","id":"66wk","name":"My Favorite Part","artists":[{"name":"Mac Miller"},{"name":"Ariana Grande"}]}}
			],"next":null}`))
			return
		}
		if r.URL.Path == "/playlists/3yQi6hff0Ic45VJfJoaXIn/items" {
			w.Write([]byte(`{"items":[
				{"item":{"type":"track","id":"c1","name":"Hallelujah","artists":[{"name":"Jeff Buckley"}]}},
				{"item":{"type":"track","id":"c2","name":"Hurt","artists":[{"name":"Johnny Cash"}]}}
			],"next":null}`))
			return
		}
		if r.URL.Path != "/me/playlists" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"items":[
			{"id":"5ZwLvrI8d6VnEQXxB0bMms","name":"The Way to My Favorite Part","owner":{"id":"me"},"items":{"total":2}},
			{"id":"3yQi6hff0Ic45VJfJoaXIn","name":"Covers","owner":{"id":"me"},"items":{"total":2}},
			{"id":"3REKkLasecJIJnNwk8Kzeg","name":"Pentatonix Evolution of Music","owner":{"id":"someone"}}
		],"next":null}`))
	}))
	t.Cleanup(srv.Close)
	return spotify.NewClient(srv.Client()).WithBaseURL(srv.URL)
}

func newTestApp(t *testing.T, yt *fakeYouTube) *app {
	ytSrv := httptest.NewServer(http.HandlerFunc(yt.handler))
	t.Cleanup(ytSrv.Close)
	a := newApp()
	a.newSpotify = func(context.Context, io.Writer, *app) (*spotifySession, error) {
		return &spotifySession{client: fakeLibraryAPI(t), me: spotify.User{ID: "me"}}, nil
	}
	a.newYouTube = func(context.Context, io.Writer, *app) (*youtubeSession, error) {
		return &youtubeSession{
			client:  youtube.NewClient(ytSrv.Client()).WithBaseURL(ytSrv.URL),
			channel: youtube.Channel{ID: "UC1", Title: "Carlos Ceballos"},
			tok:     &oauth2.Token{AccessToken: "x"},
		}, nil
	}
	return a
}

func TestYouTubeCreateFlow(t *testing.T) {
	yt := &fakeYouTube{}
	a := newTestApp(t, yt)
	in := strings.NewReader(strings.Join([]string{
		"youtube create Way", // asks...
		"n",                  // ...declined
		"youtube create Way", // asks...
		"y",                  // ...accepted
		"youtube create Way", // already exists: must not ask
		"youtube playlists",  // so this line runs as a command
		"youtube create Pentatonix",
		"exit",
	}, "\n"))
	var out, errOut bytes.Buffer
	if err := runShell(a, in, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	got := out.String()

	if yt.inserts != 1 {
		t.Fatalf("inserts = %d, want exactly 1\n%s", yt.inserts, got)
	}
	snippet := yt.playlists[0]["snippet"].(map[string]string)
	status := yt.playlists[0]["status"].(map[string]string)
	if snippet["title"] != "The Way to My Favorite Part" {
		t.Errorf("title altered: %q", snippet["title"])
	}
	if ids := transfer.MarkerIDs(snippet["description"]); len(ids) != 1 || ids[0] != "5ZwLvrI8d6VnEQXxB0bMms" {
		t.Errorf("marker missing from description %q", snippet["description"])
	}
	if status["privacyStatus"] != "private" {
		t.Errorf("privacy = %q, want private by default", status["privacyStatus"])
	}

	for _, want := range []string{
		"Cancelled. Nothing was created.",
		`Created "The Way to My Favorite Part" (private).`,
		"https://music.youtube.com/playlist?list=PLfake1",
		"Already on YouTube as",
		"Nothing to create.",
		"spotify:5ZwLvrI8d6VnEQXxB0bMms", // youtube playlists shows the marker
		youtubeAttribution,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("stdout missing %q:\n%s", want, got)
		}
	}
	if n := strings.Count(got, "Create it?"); n != 2 {
		t.Errorf("confirmation asked %d times, want 2:\n%s", n, got)
	}
	if !strings.Contains(errOut.String(), "belongs to another user") {
		t.Errorf("followed playlist not rejected:\n%s", errOut.String())
	}
}

func TestYouTubeCreateYesSkipsPromptAndHonoursPrivacy(t *testing.T) {
	yt := &fakeYouTube{}
	a := newTestApp(t, yt)
	in := strings.NewReader("youtube create Way --yes --privacy unlisted\nexit\n")
	var out, errOut bytes.Buffer
	if err := runShell(a, in, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if yt.inserts != 1 || strings.Contains(out.String(), "Create it?") {
		t.Fatalf("want one insert without prompt:\n%s\n%s", out.String(), errOut.String())
	}
	if p := yt.playlists[0]["status"].(map[string]string)["privacyStatus"]; p != "unlisted" {
		t.Errorf("privacy = %q, want unlisted", p)
	}
}

func TestYouTubeCreateBadPrivacyNeedsNoLogin(t *testing.T) {
	a := newApp()
	a.newSpotify = func(context.Context, io.Writer, *app) (*spotifySession, error) {
		t.Error("must not log in to Spotify")
		return nil, fmt.Errorf("no")
	}
	a.newYouTube = func(context.Context, io.Writer, *app) (*youtubeSession, error) {
		t.Error("must not log in to YouTube")
		return nil, fmt.Errorf("no")
	}
	in := strings.NewReader("youtube create Way --privacy secret\nexit\n")
	var out, errOut bytes.Buffer
	runShell(a, in, &out, &errOut)
	if !strings.Contains(errOut.String(), `invalid --privacy "secret"`) {
		t.Errorf("want privacy error:\n%s", errOut.String())
	}
}

func TestConfirmAnswers(t *testing.T) {
	for answer, want := range map[string]bool{
		"y": true, "YES": true, " sí ": true, "s": true,
		"": false, "n": false, "no": false, "yep": false,
	} {
		a := newApp()
		a.readLine = func() (string, error) { return answer, nil }
		var out bytes.Buffer
		if got := a.confirm(&out, "Go?"); got != want {
			t.Errorf("confirm(%q) = %v, want %v", answer, got, want)
		}
	}
	a := newApp()
	a.readLine = func() (string, error) { return "", io.EOF }
	var out bytes.Buffer
	if a.confirm(&out, "Go?") {
		t.Error("EOF must mean no")
	}
}
