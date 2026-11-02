package fakeapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
)

// Spotify serves a user's library: /me, /me/playlists and the items of each
// playlist, every list in a single page.
type Spotify struct {
	mu sync.Mutex

	Me        map[string]any              // /me; nil answers 404
	Playlists []map[string]any            // /me/playlists items, as Spotify returns them
	Entries   map[string][]map[string]any // playlist ID -> /playlists/{id}/items entries
}

func (s *Spotify) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case r.URL.Path == "/me" && s.Me != nil:
		json.NewEncoder(w).Encode(s.Me)
	case r.URL.Path == "/me/playlists":
		json.NewEncoder(w).Encode(map[string]any{"items": s.Playlists, "next": nil})
	case strings.HasPrefix(r.URL.Path, "/playlists/") && strings.HasSuffix(r.URL.Path, "/items"):
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/playlists/"), "/items")
		entries, ok := s.Entries[id]
		if !ok {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"items": entries, "next": nil})
	default:
		http.NotFound(w, r)
	}
}

// Track is one playlist entry holding a track, as Spotify returns it.
func Track(id, name string, durationMS int, artists ...string) map[string]any {
	as := make([]map[string]string, len(artists))
	for i, a := range artists {
		as[i] = map[string]string{"name": a}
	}
	return map[string]any{"item": map[string]any{"type": "track", "id": id, "name": name, "duration_ms": durationMS, "artists": as}}
}

// Episode is a podcast entry, which Soundsync skips.
func Episode(id, name string) map[string]any {
	return map[string]any{"item": map[string]any{"type": "episode", "id": id, "name": name}}
}

// SpotifyPlaylist is a library entry. total < 0 leaves the count out, as
// Spotify does for playlists of other users.
func SpotifyPlaylist(id, name, ownerID string, total int) map[string]any {
	p := map[string]any{"id": id, "name": name, "owner": map[string]string{"id": ownerID}}
	if total >= 0 {
		p["items"] = map[string]int{"total": total}
	}
	return p
}
