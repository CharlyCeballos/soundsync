package spotify

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeAPI serves canned JSON by path+query; "{{base}}" expands to the server URL.
func fakeAPI(t *testing.T, routes map[string]string) (*Client, *[]string) {
	t.Helper()
	var hits []string
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path
		if r.URL.RawQuery != "" {
			key += "?" + r.URL.RawQuery
		}
		hits = append(hits, key)
		body, ok := routes[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"error":{"status":404,"message":"no route %s"}}`, key)
			return
		}
		if strings.HasPrefix(body, "403:") {
			w.WriteHeader(http.StatusForbidden)
			body = strings.TrimPrefix(body, "403:")
		}
		w.Write([]byte(strings.ReplaceAll(body, "{{base}}", srv.URL)))
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.Client()).WithBaseURL(srv.URL), &hits
}

func TestMyPlaylistsPaginatesAndClassifies(t *testing.T) {
	c, hits := fakeAPI(t, map[string]string{
		"/me/playlists?limit=50": `{"items":[
			{"id":"p1","name":"Mine","collaborative":false,"owner":{"id":"me","display_name":"Charly"},"items":{"total":12}},
			{"id":"p2","name":"Shared","collaborative":true,"owner":{"id":"friend","display_name":"Ana"},"items":{"total":3}}
		],"next":"{{base}}/me/playlists?limit=50&offset=50"}`,
		"/me/playlists?limit=50&offset=50": `{"items":[
			{"id":"p3","name":"Top 50 México","collaborative":false,"owner":{"id":"spotify","display_name":"Spotify"}},
			{"id":"p4","name":"Old shape","collaborative":false,"owner":{"id":"me"},"tracks":{"total":7}}
		],"next":null}`,
	})

	got, err := c.MyPlaylists(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(*hits) != 2 {
		t.Errorf("want 2 page requests, got %v", *hits)
	}
	want := []struct {
		id     string
		total  int
		access Access
	}{
		{"p1", 12, AccessOwned},
		{"p2", 3, AccessCollaborative},
		{"p3", -1, AccessFollowed},
		{"p4", 7, AccessOwned},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d playlists, want %d", len(got), len(want))
	}
	for i, w := range want {
		p := got[i]
		if p.ID != w.id || p.Total != w.total || p.AccessFor("me") != w.access {
			t.Errorf("playlist %d = %+v (access %s), want %+v", i, p, p.AccessFor("me"), w)
		}
	}
	if got[2].Name != "Top 50 México" {
		t.Errorf("metadata altered: %q", got[2].Name)
	}
	if got[0].URL() != "https://open.spotify.com/playlist/p1" {
		t.Errorf("bad link %q", got[0].URL())
	}
}

func TestPaginationRefusesForeignHost(t *testing.T) {
	c, hits := fakeAPI(t, map[string]string{
		"/me/playlists?limit=50": `{"items":[],"next":"https://evil.example.com/steal?limit=50"}`,
	})
	_, err := c.MyPlaylists(context.Background())
	if err == nil || !strings.Contains(err.Error(), "refusing") {
		t.Fatalf("want refusal, got %v", err)
	}
	if len(*hits) != 1 {
		t.Errorf("must not request anything after a foreign next link: %v", *hits)
	}
}

func TestPlaylistItemsShapes(t *testing.T) {
	c, _ := fakeAPI(t, map[string]string{
		"/playlists/p1/items?limit=50": `{"items":[
			{"is_local":false,"item":{"type":"track","id":"t1","name":"Oye Mi Amor","uri":"spotify:track:t1","duration_ms":263000,
				"artists":[{"name":"Maná"}],"album":{"name":"¿Dónde Jugarán Los Niños?"},"external_ids":{"isrc":"USWB19200001"}}},
			{"is_local":false,"item":{"type":"episode","id":"e1","name":"Some podcast"}},
			{"is_local":false,"item":null}
		],"next":"{{base}}/playlists/p1/items?limit=50&offset=50"}`,
		"/playlists/p1/items?limit=50&offset=50": `{"items":[
			{"is_local":true,"item":{"type":"track","id":null,"name":"Demo casera","uri":"spotify:local:::Demo+casera:180",
				"artists":[{"name":"Charly"}],"album":{"name":""}}},
			{"is_local":false,"track":{"type":"track","id":"t2","name":"Legacy Shape","artists":[{"name":"A"},{"name":"B"}],"album":{"name":"X"}}}
		],"next":null}`,
	})

	got, err := c.PlaylistItems(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 {
		t.Fatalf("got %d entries, want 5: %+v", len(got), got)
	}
	for i, e := range got {
		if e.Position != i+1 {
			t.Errorf("entry %d has position %d", i, e.Position)
		}
	}

	t1 := got[0].Track
	if got[0].Skip != "" || t1.Name != "Oye Mi Amor" || t1.ISRC != "USWB19200001" ||
		len(t1.Artists) != 1 || t1.Artists[0] != "Maná" || t1.Album != "¿Dónde Jugarán Los Niños?" {
		t.Errorf("track 1 decoded wrong: %+v", got[0])
	}
	if t1.URL() != "https://open.spotify.com/track/t1" {
		t.Errorf("bad link %q", t1.URL())
	}
	if got[1].Skip != SkipEpisode {
		t.Errorf("episode not skipped: %+v", got[1])
	}
	if got[2].Skip != SkipUnavailable {
		t.Errorf("null item not skipped: %+v", got[2])
	}
	if got[3].Skip != "" || !got[3].Track.IsLocal || got[3].Track.URL() != "" {
		t.Errorf("local file wrong: %+v", got[3])
	}
	if got[4].Skip != "" || got[4].Track.Name != "Legacy Shape" || len(got[4].Track.Artists) != 2 {
		t.Errorf("legacy track shape not decoded: %+v", got[4])
	}
}

func TestPlaylistItemsForbidden(t *testing.T) {
	c, _ := fakeAPI(t, map[string]string{
		"/playlists/p3/items?limit=50": `403:{"error":{"status":403,"message":"Forbidden"}}`,
	})
	_, err := c.PlaylistItems(context.Background(), "p3")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 403 {
		t.Fatalf("want 403 APIError, got %v", err)
	}
}
