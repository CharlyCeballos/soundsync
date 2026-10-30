package youtube

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// Privacy values accepted by playlists.insert (status.privacyStatus).
const (
	PrivacyPrivate  = "private"
	PrivacyUnlisted = "unlisted"
	PrivacyPublic   = "public"
)

// ValidPrivacy reports whether p is a privacy status YouTube accepts.
func ValidPrivacy(p string) bool {
	return p == PrivacyPrivate || p == PrivacyUnlisted || p == PrivacyPublic
}

// Playlist is one playlist on the user's channel.
type Playlist struct {
	ID          string
	Title       string
	Description string
	Privacy     string
	ItemCount   int
}

// URL opens the playlist on YouTube (Developer Policies III.F.2: make
// YouTube's role as the source clear).
func (p Playlist) URL() string { return "https://www.youtube.com/playlist?list=" + p.ID }

// MusicURL opens the playlist in YouTube Music.
func (p Playlist) MusicURL() string { return "https://music.youtube.com/playlist?list=" + p.ID }

type playlistJSON struct {
	ID      string `json:"id"`
	Snippet struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	} `json:"snippet"`
	Status struct {
		PrivacyStatus string `json:"privacyStatus"`
	} `json:"status"`
	ContentDetails struct {
		ItemCount int `json:"itemCount"`
	} `json:"contentDetails"`
}

func (p playlistJSON) toPlaylist() Playlist {
	return Playlist{
		ID:          p.ID,
		Title:       p.Snippet.Title,
		Description: p.Snippet.Description,
		Privacy:     p.Status.PrivacyStatus,
		ItemCount:   p.ContentDetails.ItemCount,
	}
}

// listPageSize is the playlists.list maximum.
const listPageSize = 50

// MyPlaylists returns every playlist on the authorized user's channel.
// Costs 1 quota unit per page of 50.
func (c *Client) MyPlaylists(ctx context.Context) ([]Playlist, error) {
	var out []Playlist
	pageToken := ""
	for {
		q := url.Values{
			"part":       {"snippet,status,contentDetails"},
			"mine":       {"true"},
			"maxResults": {strconv.Itoa(listPageSize)},
		}
		if pageToken != "" {
			q.Set("pageToken", pageToken)
		}
		var page struct {
			Items         []playlistJSON `json:"items"`
			NextPageToken string         `json:"nextPageToken"`
		}
		if err := c.get(ctx, "/playlists", q, costList, &page); err != nil {
			return nil, err
		}
		for _, p := range page.Items {
			out = append(out, p.toPlaylist())
		}
		if page.NextPageToken == "" {
			return out, nil
		}
		pageToken = page.NextPageToken
	}
}

// CreatePlaylist creates a playlist on the authorized user's channel.
// The title is sent exactly as given. Costs 50 quota units.
func (c *Client) CreatePlaylist(ctx context.Context, title, description, privacy string) (Playlist, error) {
	if !ValidPrivacy(privacy) {
		return Playlist{}, fmt.Errorf("invalid privacy %q (want private, unlisted or public)", privacy)
	}
	// Only writable properties: id and contentDetails are read-only.
	body := map[string]any{
		"snippet": map[string]string{"title": title, "description": description},
		"status":  map[string]string{"privacyStatus": privacy},
	}
	q := url.Values{"part": {"snippet,status"}}
	var created playlistJSON
	if err := c.post(ctx, "/playlists", q, body, costPlaylistInsert, &created); err != nil {
		return Playlist{}, err
	}
	return created.toPlaylist(), nil
}
