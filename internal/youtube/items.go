package youtube

import (
	"context"
	"net/url"
	"strconv"
)

// PlaylistVideoIDs returns the video IDs in a playlist, in playlist order.
// Costs 1 unit per page of 50.
func (c *Client) PlaylistVideoIDs(ctx context.Context, playlistID string) ([]string, error) {
	var ids []string
	pageToken := ""
	for {
		q := url.Values{
			"part":       {"contentDetails"},
			"playlistId": {playlistID},
			"maxResults": {strconv.Itoa(listPageSize)},
		}
		if pageToken != "" {
			q.Set("pageToken", pageToken)
		}
		var page struct {
			Items []struct {
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
				ids = append(ids, it.ContentDetails.VideoID)
			}
		}
		if page.NextPageToken == "" {
			return ids, nil
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
