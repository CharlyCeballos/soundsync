package spotify

import "strings"

// FindPlaylists returns the playlists whose name matches query, ignoring
// case and surrounding spaces. Exact matches win; otherwise playlists whose
// name contains query are returned, so "Tristes" finds "Tristes 😢😓".
func FindPlaylists(playlists []Playlist, query string) []Playlist {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	var exact, partial []Playlist
	for _, p := range playlists {
		name := strings.ToLower(strings.TrimSpace(p.Name))
		switch {
		case name == q:
			exact = append(exact, p)
		case strings.Contains(name, q):
			partial = append(partial, p)
		}
	}
	if len(exact) > 0 {
		return exact
	}
	return partial
}
