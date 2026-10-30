package youtube

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestSearchVideosParamsAndBuckets(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/search" || q.Get("part") != "snippet" || q.Get("type") != "video" ||
			q.Get("q") != "Maná Oye Mi Amor" || q.Get("maxResults") != "5" {
			http.Error(w, "bad request "+r.URL.String(), http.StatusBadRequest)
			return
		}
		w.Write([]byte(`{"items":[
			{"id":{"kind":"youtube#video","videoId":"v1"},"snippet":{"title":"Oye Mi Amor &quot;en vivo&quot;","channelTitle":"Maná - Topic"}},
			{"id":{"kind":"youtube#channel","channelId":"UCx"},"snippet":{"title":"not a video"}}
		]}`))
	})
	c := NewClient(srv.Client()).WithBaseURL(srv.URL)
	got, err := c.SearchVideos(context.Background(), "Maná Oye Mi Amor", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "v1" || got[0].Title != `Oye Mi Amor "en vivo"` || got[0].ChannelTitle != "Maná - Topic" {
		t.Errorf("unexpected results %+v", got)
	}
	if c.SearchesUsed() != 1 || c.UnitsUsed() != 1 {
		t.Errorf("searches=%d units=%d, want 1 and 1", c.SearchesUsed(), c.UnitsUsed())
	}
}

func TestAddVideoBody(t *testing.T) {
	var body map[string]any
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/playlistItems" || r.URL.Query().Get("part") != "snippet" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		json.NewDecoder(r.Body).Decode(&body)
		w.Write([]byte(`{"id":"item1"}`))
	})
	c := NewClient(srv.Client()).WithBaseURL(srv.URL)
	if err := c.AddVideo(context.Background(), "PL1", "v1"); err != nil {
		t.Fatal(err)
	}
	snippet := body["snippet"].(map[string]any)
	res := snippet["resourceId"].(map[string]any)
	if snippet["playlistId"] != "PL1" || res["kind"] != "youtube#video" || res["videoId"] != "v1" {
		t.Errorf("unexpected body %v", body)
	}
	if _, hasPos := snippet["position"]; hasPos {
		t.Error("no position: videos are appended in Spotify order")
	}
	if c.UnitsUsed() != 50 {
		t.Errorf("units = %d, want 50", c.UnitsUsed())
	}
}

func TestPlaylistVideoIDsPaginates(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("playlistId") != "PL1" || r.URL.Query().Get("part") != "contentDetails" {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		if r.URL.Query().Get("pageToken") == "" {
			w.Write([]byte(`{"items":[{"contentDetails":{"videoId":"a"}}],"nextPageToken":"P2"}`))
			return
		}
		w.Write([]byte(`{"items":[{"contentDetails":{"videoId":"b"}},{"contentDetails":{}}]}`))
	})
	c := NewClient(srv.Client()).WithBaseURL(srv.URL)
	ids, err := c.PlaylistVideoIDs(context.Background(), "PL1")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != "a" || ids[1] != "b" || c.UnitsUsed() != 2 {
		t.Errorf("ids=%v units=%d", ids, c.UnitsUsed())
	}
}
