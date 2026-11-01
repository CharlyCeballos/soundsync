package youtube

import (
	"context"
	"html"
	"net/http"
	"net/url"
	"strconv"
)

// Item is one entry of a playlist.
type Item struct {
	ID      string // playlist item ID: deleting needs it, not the video ID
	VideoID string
	Title   string // as YouTube shows it; "Deleted video" or "Private video" when unavailable
	Channel string // the video owner's channel; empty when unavailable
}

// URL opens the item's video on YouTube.
func (it Item) URL() string { return Video{ID: it.VideoID}.URL() }

// PlaylistItems returns a playlist's entries in playlist order.
// Costs 1 unit per page of 50 (snippet costs nothing extra).
func (c *Client) PlaylistItems(ctx context.Context, playlistID string) ([]Item, error) {
	var items []Item
	pageToken := ""
	for {
		q := url.Values{
			"part":       {"snippet,contentDetails"},
			"playlistId": {playlistID},
			"maxResults": {strconv.Itoa(listPageSize)},
		}
		if pageToken != "" {
			q.Set("pageToken", pageToken)
		}
		var page struct {
			Items []struct {
				ID      string `json:"id"`
				Snippet struct {
					Title                  string `json:"title"`
					VideoOwnerChannelTitle string `json:"videoOwnerChannelTitle"`
				} `json:"snippet"`
				ContentDetails struct {
					VideoID string `json:"videoId"`
				} `json:"contentDetails"`
			} `json:"items"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err := c.get(ctx, "/playlistItems", q, costList, &page); err != nil {
			return nil, err
		}
		for _, it := range page.Items {
			if it.ContentDetails.VideoID != "" {
				items = append(items, Item{
					ID:      it.ID,
					VideoID: it.ContentDetails.VideoID,
					Title:   html.UnescapeString(it.Snippet.Title),
					Channel: html.UnescapeString(it.Snippet.VideoOwnerChannelTitle),
				})
			}
		}
		if page.NextPageToken == "" {
			return items, nil
		}
		pageToken = page.NextPageToken
	}
}

// AddVideo appends a video to the end of a playlist. Costs 50 units; there
// is no batch variant.
func (c *Client) AddVideo(ctx context.Context, playlistID, videoID string) error {
	body := map[string]any{
		"snippet": map[string]any{
			"playlistId": playlistID,
			"resourceId": map[string]string{"kind": "youtube#video", "videoId": videoID},
		},
	}
	var created struct {
		ID string `json:"id"`
	}
	return c.post(ctx, "/playlistItems", url.Values{"part": {"snippet"}}, body, costItemInsert, &created)
}

// DeleteItem removes one entry (by playlist item ID) from its playlist.
// Costs 50 units.
func (c *Client) DeleteItem(ctx context.Context, itemID string) error {
	return c.do(ctx, http.MethodDelete, "/playlistItems", url.Values{"id": {itemID}}, nil, costItemDelete, nil)
}
