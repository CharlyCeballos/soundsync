package fakeapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// YouTube keeps a channel's playlists and their videos in memory and serves
// the subset of the YouTube Data API v3 that Soundsync uses. Seed the
// exported fields before serving; read them back after the run, or through
// the locked accessors while requests may still arrive.
type YouTube struct {
	mu sync.Mutex

	Playlists []map[string]any            // playlists.list items, as YouTube returns them
	Search    map[string][]map[string]any // query -> search.list items
	Items     map[string][]Item           // playlist ID -> its entries
	Lengths   map[string]string           // video ID -> ISO 8601 duration served by videos.list
	// FailItemAt makes playlistItems.insert answer quotaExceeded from this
	// 1-based call on, like YouTube when the daily quota runs out; 0 = never.
	FailItemAt int
	// FailSearch does the same for search.list; 0 = never.
	FailSearch int

	Inserts     int // playlists.insert calls
	Updates     int // playlists.update calls
	Searches    int // search.list calls
	ItemInserts int // playlistItems.insert calls
	Deletes     int // playlistItems.delete calls that removed something
}

// Item is one entry of a playlist.
type Item struct{ ID, VideoID, Title, Channel string }

// Counts is a consistent snapshot of the call counters.
type Counts struct{ Inserts, Updates, Searches, ItemInserts, Deletes int }

// Seed puts videos (id, title pairs) in a playlist before the run.
func (f *YouTube) Seed(playlistID string, idTitle ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Items == nil {
		f.Items = map[string][]Item{}
	}
	for i := 0; i+1 < len(idTitle); i += 2 {
		f.Items[playlistID] = append(f.Items[playlistID], Item{ID: "seed-" + idTitle[i], VideoID: idTitle[i], Title: idTitle[i+1]})
	}
}

// Videos returns a copy of a playlist's video IDs. Read through it after a
// cancelled run: a request cancelled on the client side may still be handled.
func (f *YouTube) Videos(playlistID string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var ids []string
	for _, it := range f.Items[playlistID] {
		ids = append(ids, it.VideoID)
	}
	return ids
}

// Counts returns the call counters under the lock.
func (f *YouTube) Counts() Counts {
	f.mu.Lock()
	defer f.mu.Unlock()
	return Counts{Inserts: f.Inserts, Updates: f.Updates, Searches: f.Searches, ItemInserts: f.ItemInserts, Deletes: f.Deletes}
}

// snippetOf finds a video's title and channel among the search results.
func (f *YouTube) snippetOf(videoID string) (title, channel string) {
	for _, results := range f.Search {
		for _, r := range results {
			if r["id"].(map[string]string)["videoId"] == videoID {
				sn := r["snippet"].(map[string]string)
				return sn["title"], sn["channelTitle"]
			}
		}
	}
	return videoID, ""
}

const quotaExceeded = `{"error":{"code":403,"message":"quota","errors":[{"reason":"quotaExceeded"}]}}`

func (f *YouTube) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/search":
		f.Searches++
		if f.FailSearch > 0 && f.Searches >= f.FailSearch {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(quotaExceeded))
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"items": f.Search[r.URL.Query().Get("q")]})
	case r.Method == http.MethodGet && r.URL.Path == "/playlistItems":
		var items []map[string]any
		for _, it := range f.Items[r.URL.Query().Get("playlistId")] {
			items = append(items, map[string]any{
				"id":             it.ID,
				"snippet":        map[string]string{"title": it.Title, "videoOwnerChannelTitle": it.Channel},
				"contentDetails": map[string]string{"videoId": it.VideoID},
			})
		}
		json.NewEncoder(w).Encode(map[string]any{"items": items})
	case r.Method == http.MethodDelete && r.URL.Path == "/playlistItems":
		f.deleteItem(w, r.URL.Query().Get("id"))
	case r.Method == http.MethodPost && r.URL.Path == "/playlistItems":
		f.insertItem(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/videos":
		var items []map[string]any
		for _, id := range strings.Split(r.URL.Query().Get("id"), ",") {
			if d, ok := f.Lengths[id]; ok {
				items = append(items, map[string]any{"id": id, "contentDetails": map[string]string{"duration": d}})
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"items": items})
	case r.Method == http.MethodPut && r.URL.Path == "/playlists":
		f.updatePlaylist(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/playlists":
		json.NewEncoder(w).Encode(map[string]any{"items": f.Playlists})
	case r.Method == http.MethodPost && r.URL.Path == "/playlists":
		var body map[string]map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		f.Inserts++
		p := map[string]any{
			"id":             fmt.Sprintf("PLfake%d", f.Inserts),
			"snippet":        body["snippet"],
			"status":         body["status"],
			"contentDetails": map[string]int{"itemCount": 0},
		}
		f.Playlists = append(f.Playlists, p)
		json.NewEncoder(w).Encode(p)
	default:
		http.NotFound(w, r)
	}
}

func (f *YouTube) deleteItem(w http.ResponseWriter, id string) {
	for pl, list := range f.Items {
		for i, it := range list {
			if it.ID == id {
				f.Items[pl] = append(list[:i:i], list[i+1:]...)
				f.Deletes++
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
	}
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(`{"error":{"code":404,"message":"not found","errors":[{"reason":"playlistItemNotFound"}]}}`))
}

func (f *YouTube) insertItem(w http.ResponseWriter, r *http.Request) {
	f.ItemInserts++
	if f.FailItemAt > 0 && f.ItemInserts >= f.FailItemAt {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(quotaExceeded))
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
	if f.Items == nil {
		f.Items = map[string][]Item{}
	}
	vid := body.Snippet.ResourceID.VideoID
	title, channel := f.snippetOf(vid)
	pl := body.Snippet.PlaylistID
	f.Items[pl] = append(f.Items[pl], Item{ID: fmt.Sprintf("item%d", f.ItemInserts), VideoID: vid, Title: title, Channel: channel})
	w.Write([]byte(`{"id":"item"}`))
}

func (f *YouTube) updatePlaylist(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID      string            `json:"id"`
		Snippet map[string]string `json:"snippet"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	for _, p := range f.Playlists {
		if p["id"] == body.ID {
			p["snippet"] = body.Snippet
			f.Updates++
			json.NewEncoder(w).Encode(p)
			return
		}
	}
	http.NotFound(w, r)
}
