package youtube

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// videosPageSize is the most IDs videos.list takes per call.
const videosPageSize = 50

// VideoLengths returns the length of each video, keyed by ID. Videos YouTube
// no longer returns (deleted, private) are absent. Costs 1 unit per 50 IDs.
func (c *Client) VideoLengths(ctx context.Context, ids []string) (map[string]time.Duration, error) {
	out := make(map[string]time.Duration, len(ids))
	for start := 0; start < len(ids); start += videosPageSize {
		end := min(start+videosPageSize, len(ids))
		q := url.Values{
			"part":       {"contentDetails"},
			"id":         {strings.Join(ids[start:end], ",")},
			"maxResults": {strconv.Itoa(videosPageSize)},
		}
		var page struct {
			Items []struct {
				ID             string `json:"id"`
				ContentDetails struct {
					Duration string `json:"duration"`
				} `json:"contentDetails"`
			} `json:"items"`
		}
		if err := c.get(ctx, "/videos", q, costList, &page); err != nil {
			return nil, err
		}
		for _, it := range page.Items {
			if d, err := parseISODuration(it.ContentDetails.Duration); err == nil {
				out[it.ID] = d
			}
		}
	}
	return out, nil
}

var isoDurationRE = regexp.MustCompile(`^P(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?$`)

// parseISODuration reads the ISO 8601 durations YouTube uses, e.g. PT3M45S,
// PT1H2M, P1DT2S. "P0D" (live streams) is zero.
func parseISODuration(s string) (time.Duration, error) {
	m := isoDurationRE.FindStringSubmatch(s)
	if m == nil || s == "P" || s == "PT" {
		return 0, fmt.Errorf("bad ISO 8601 duration %q", s)
	}
	units := []time.Duration{24 * time.Hour, time.Hour, time.Minute, time.Second}
	var d time.Duration
	for i, unit := range units {
		if m[i+1] == "" {
			continue
		}
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return 0, err
		}
		d += time.Duration(n) * unit
	}
	return d, nil
}
