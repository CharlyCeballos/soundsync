package spotify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// pageLimit is the page size for playlist listings (API maximum: 50).
const pageLimit = 50

// Playlist is the metadata Soundsync needs about one of the user's playlists.
type Playlist struct {
	ID            string
	Name          string
	OwnerID       string
	OwnerName     string
	Collaborative bool
	// Total is the number of items, or -1 when Spotify does not report it
	// (playlists the user neither owns nor collaborates on).
	Total int
}

// Access describes whether Soundsync can read a playlist's items.
type Access string

const (
	AccessOwned         Access = "owned"
	AccessCollaborative Access = "collaborative"
	// AccessFollowed playlists belong to someone else. Since February 2026,
	// Development Mode apps cannot read their items: skip and report.
	AccessFollowed Access = "followed"
)

// AccessFor classifies p for the user with userID.
func (p Playlist) AccessFor(userID string) Access {
	switch {
	case p.OwnerID == userID:
		return AccessOwned
	case p.Collaborative:
		return AccessCollaborative
	default:
		return AccessFollowed
	}
}

// URL links back to the playlist on Spotify (Developer Policy II.4.2).
func (p Playlist) URL() string { return "https://open.spotify.com/playlist/" + p.ID }

// Track is one playable item of a playlist, with metadata kept exactly as
// Spotify returned it (Design Guidelines: do not alter metadata).
type Track struct {
	ID         string
	Name       string
	Artists    []string
	Album      string
	DurationMS int
	ISRC       string // may be empty
	URI        string
	IsLocal    bool // a local file added to the playlist; no Spotify ID
}

// URL links back to the track on Spotify, or "" for local files.
func (t Track) URL() string {
	if t.ID == "" {
		return ""
	}
	return "https://open.spotify.com/track/" + t.ID
}

// SkipReason explains why a playlist entry yields no track.
type SkipReason string

const (
	SkipEpisode     SkipReason = "podcast episode"
	SkipUnavailable SkipReason = "unavailable"
)

// Entry is one position in a playlist: a Track, or a reason it was skipped.
type Entry struct {
	Position int // 1-based, in Spotify order
	Track    Track
	Skip     SkipReason // empty when Track is usable
}

// MyPlaylists returns every playlist in the user's library (owned,
// collaborative and followed), following pagination.
func (c *Client) MyPlaylists(ctx context.Context) ([]Playlist, error) {
	var out []Playlist
	next := c.baseURL + "/me/playlists?" + url.Values{"limit": {strconv.Itoa(pageLimit)}}.Encode()
	for next != "" {
		var page struct {
			Items []playlistJSON `json:"items"`
			Next  string         `json:"next"`
		}
		if err := c.getURL(ctx, next, &page); err != nil {
			return nil, err
		}
		for _, p := range page.Items {
			out = append(out, p.toPlaylist())
		}
		var err error
		if next, err = c.checkNext(page.Next); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// PlaylistItems returns every entry of a playlist the user owns or
// collaborates on, in Spotify order, following pagination.
func (c *Client) PlaylistItems(ctx context.Context, playlistID string) ([]Entry, error) {
	var out []Entry
	next := c.baseURL + "/playlists/" + url.PathEscape(playlistID) + "/items?" +
		url.Values{"limit": {strconv.Itoa(pageLimit)}}.Encode()
	for next != "" {
		var page struct {
			Items []entryJSON `json:"items"`
			Next  string      `json:"next"`
		}
		if err := c.getURL(ctx, next, &page); err != nil {
			return nil, err
		}
		for _, e := range page.Items {
			entry := e.toEntry()
			entry.Position = len(out) + 1
			out = append(out, entry)
		}
		var err error
		if next, err = c.checkNext(page.Next); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// checkNext only follows pagination links on the API host: the HTTP client
// attaches the user's bearer token to every request it makes.
func (c *Client) checkNext(next string) (string, error) {
	if next == "" {
		return "", nil
	}
	if !strings.HasPrefix(next, c.baseURL+"/") {
		return "", fmt.Errorf("spotify pagination link points outside %s; refusing to follow it", c.baseURL)
	}
	return next, nil
}

// --- JSON shapes (February 2026 revision, with pre-2026 names as fallback) ---

type countJSON struct {
	Total *int `json:"total"`
}

type playlistJSON struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Collaborative bool   `json:"collaborative"`
	Owner         struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"owner"`
	Items  *countJSON `json:"items"`  // since February 2026
	Tracks *countJSON `json:"tracks"` // pre-2026 name
}

func (p playlistJSON) toPlaylist() Playlist {
	total := -1
	for _, c := range []*countJSON{p.Items, p.Tracks} {
		if c != nil && c.Total != nil {
			total = *c.Total
			break
		}
	}
	return Playlist{
		ID:            p.ID,
		Name:          p.Name,
		OwnerID:       p.Owner.ID,
		OwnerName:     p.Owner.DisplayName,
		Collaborative: p.Collaborative,
		Total:         total,
	}
}

type trackJSON struct {
	Type       string `json:"type"`
	ID         string `json:"id"`
	Name       string `json:"name"`
	URI        string `json:"uri"`
	DurationMS int    `json:"duration_ms"`
	IsLocal    bool   `json:"is_local"`
	Artists    []struct {
		Name string `json:"name"`
	} `json:"artists"`
	Album struct {
		Name string `json:"name"`
	} `json:"album"`
	ExternalIDs struct {
		ISRC string `json:"isrc"`
	} `json:"external_ids"`
}

type entryJSON struct {
	IsLocal bool            `json:"is_local"`
	Item    json.RawMessage `json:"item"`  // since February 2026
	Track   json.RawMessage `json:"track"` // pre-2026 name
}

func (e entryJSON) toEntry() Entry {
	raw := e.Item
	if isNullJSON(raw) {
		raw = e.Track
	}
	if isNullJSON(raw) {
		return Entry{Skip: SkipUnavailable}
	}
	var t trackJSON
	if err := json.Unmarshal(raw, &t); err != nil {
		return Entry{Skip: SkipUnavailable}
	}
	if t.Type == "episode" {
		return Entry{Skip: SkipEpisode}
	}
	if t.Name == "" {
		return Entry{Skip: SkipUnavailable}
	}
	artists := make([]string, 0, len(t.Artists))
	for _, a := range t.Artists {
		if a.Name != "" {
			artists = append(artists, a.Name)
		}
	}
	return Entry{Track: Track{
		ID:         t.ID,
		Name:       t.Name,
		Artists:    artists,
		Album:      t.Album.Name,
		DurationMS: t.DurationMS,
		ISRC:       t.ExternalIDs.ISRC,
		URI:        t.URI,
		IsLocal:    e.IsLocal || t.IsLocal,
	}}
}

func isNullJSON(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return s == "" || s == "null"
}
