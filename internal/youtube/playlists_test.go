package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
)

func TestMyPlaylistsPaginates(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/playlists" || q.Get("mine") != "true" || q.Get("maxResults") != "50" ||
			q.Get("part") != "snippet,status,contentDetails" {
			http.Error(w, "bad request "+r.URL.String(), http.StatusBadRequest)
			return
		}
		switch q.Get("pageToken") {
		case "":
			w.Write([]byte(`{"items":[{"id":"PL1","snippet":{"title":"Uno","description":"x"},"status":{"privacyStatus":"private"},"contentDetails":{"itemCount":3}}],"nextPageToken":"T2"}`))
		case "T2":
			w.Write([]byte(`{"items":[{"id":"PL2","snippet":{"title":"Dos 🎶"},"status":{"privacyStatus":"public"}}]}`))
		default:
			http.Error(w, "unexpected token", http.StatusBadRequest)
		}
	})
	c := NewClient(srv.Client()).WithBaseURL(srv.URL)
	got, err := c.MyPlaylists(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "PL1" || got[0].ItemCount != 3 || got[1].Title != "Dos 🎶" || got[1].Privacy != "public" {
		t.Errorf("unexpected playlists: %+v", got)
	}
	if c.UnitsUsed() != 2 {
		t.Errorf("units = %d, want 2 (one per page)", c.UnitsUsed())
	}
	if got[0].MusicURL() != "https://music.youtube.com/playlist?list=PL1" {
		t.Errorf("bad music link %q", got[0].MusicURL())
	}
}

func TestCreatePlaylistSendsExactBody(t *testing.T) {
	var body map[string]map[string]string
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/playlists" || r.URL.Query().Get("part") != "snippet,status" {
			http.Error(w, "bad request "+r.Method+" "+r.URL.String(), http.StatusBadRequest)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "content type", http.StatusUnsupportedMediaType)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Write([]byte(`{"id":"PLnew","snippet":{"title":"` + body["snippet"]["title"] + `","description":"d"},"status":{"privacyStatus":"private"}}`))
	})
	c := NewClient(srv.Client()).WithBaseURL(srv.URL)
	title := "The Way to My Favorite Part 💕"
	p, err := c.CreatePlaylist(context.Background(), title, "line1\nsoundsync:spotify:abc", PrivacyPrivate)
	if err != nil {
		t.Fatal(err)
	}
	if body["snippet"]["title"] != title || body["snippet"]["description"] != "line1\nsoundsync:spotify:abc" ||
		body["status"]["privacyStatus"] != "private" {
		t.Errorf("unexpected request body: %v", body)
	}
	if len(body) != 2 {
		t.Errorf("request must only carry snippet and status, got %v", body)
	}
	if p.ID != "PLnew" || p.URL() != "https://www.youtube.com/playlist?list=PLnew" {
		t.Errorf("unexpected result %+v", p)
	}
	if c.UnitsUsed() != 50 {
		t.Errorf("units = %d, want 50", c.UnitsUsed())
	}
}

func TestSetDescriptionKeepsTitleLanguageAndPrivacy(t *testing.T) {
	var body map[string]any
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			w.Write([]byte(`{"items":[{"id":"PL1","snippet":{"title":"Covers 🎸","description":"mine","defaultLanguage":"es"},"status":{"privacyStatus":"public"}}]}`))
		case r.Method == http.MethodPut && r.URL.Path == "/playlists" && r.URL.Query().Get("part") == "snippet":
			json.NewDecoder(r.Body).Decode(&body)
			w.Write([]byte(`{"id":"PL1"}`))
		default:
			http.Error(w, "bad request "+r.Method+" "+r.URL.String(), http.StatusBadRequest)
		}
	})
	c := NewClient(srv.Client()).WithBaseURL(srv.URL)
	ps, err := c.MyPlaylists(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := c.SetDescription(context.Background(), ps[0], "mine\nsoundsync:spotify:abc"); err != nil {
		t.Fatal(err)
	}
	snippet, _ := body["snippet"].(map[string]any)
	if body["id"] != "PL1" || snippet["title"] != "Covers 🎸" || snippet["description"] != "mine\nsoundsync:spotify:abc" || snippet["defaultLanguage"] != "es" {
		t.Errorf("unexpected body %v", body)
	}
	if _, ok := body["status"]; ok {
		t.Error("privacy must never be sent on update (Policies III.E.3.f)")
	}
	if c.UnitsUsed() != 51 {
		t.Errorf("units = %d, want 1 + 50", c.UnitsUsed())
	}
}

func TestCreatePlaylistRejectsBadPrivacyWithoutCalling(t *testing.T) {
	var calls atomic.Int32
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1) })
	c := NewClient(srv.Client()).WithBaseURL(srv.URL)
	if _, err := c.CreatePlaylist(context.Background(), "x", "", "secret"); err == nil {
		t.Fatal("want error for invalid privacy")
	}
	if calls.Load() != 0 || c.UnitsUsed() != 0 {
		t.Errorf("no request should be made: calls=%d units=%d", calls.Load(), c.UnitsUsed())
	}
}

// Failed calls are counted at full cost on purpose (conservative budget).
func TestFailedCallsCountQuotaConservatively(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":{"code":403,"message":"The playlist cannot be created.","errors":[{"reason":"maxPlaylistExceeded"}]}}`))
	})
	c := NewClient(srv.Client()).WithBaseURL(srv.URL)
	_, err := c.CreatePlaylist(context.Background(), "x", "", PrivacyPrivate)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Reason != "maxPlaylistExceeded" {
		t.Fatalf("want maxPlaylistExceeded, got %v", err)
	}
	if c.UnitsUsed() != 50 {
		t.Errorf("units = %d, want 50 (conservative count) on failure", c.UnitsUsed())
	}
}
