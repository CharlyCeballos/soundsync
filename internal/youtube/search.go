package youtube

import (
	"context"
	"html"
	"net/url"
	"strconv"
)

// Video is one search result.
type Video struct {
	ID           string
	Title        string
	ChannelTitle string
}

// URL opens the video on YouTube.
func (v Video) URL() string { return "https://www.youtube.com/watch?v=" + v.ID }

// SearchVideos runs one search.list query restricted to videos. It costs
// 1 unit plus one call from the daily search bucket, whatever maxResults is.
// Results keep YouTube's ranking; titles are HTML-unescaped for display only.
func (c *Client) SearchVideos(ctx context.Context, query string, maxResults int) ([]Video, error) {
	q := url.Values{
		"part":       {"snippet"},
		"type":       {"video"},
		"q":          {query},
		"maxResults": {strconv.Itoa(maxResults)},
	}
	var body struct {
		Items []struct {
			ID struct {
				VideoID string `json:"videoId"`
			} `json:"id"`
			Snippet struct {
				Title        string `json:"title"`
				ChannelTitle string `json:"channelTitle"`
			} `json:"snippet"`
		} `json:"items"`
	}
	// Counted before the call, so a network failure overcounts (conservative).
	c.searches.Add(1)
	if err := c.get(ctx, "/search", q, costSearch, &body); err != nil {
		return nil, err
	}
	out := make([]Video, 0, len(body.Items))
	for _, it := range body.Items {
		if it.ID.VideoID == "" {
			continue
		}
		out = append(out, Video{
			ID:           it.ID.VideoID,
			Title:        html.UnescapeString(it.Snippet.Title),
			ChannelTitle: html.UnescapeString(it.Snippet.ChannelTitle),
		})
	}
	return out, nil
}
