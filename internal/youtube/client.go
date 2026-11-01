package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
)

// DefaultBaseURL is the YouTube Data API v3 root.
const DefaultBaseURL = "https://www.googleapis.com/youtube/v3"

// Quota costs in units (Data API overview, updated 2026-10-15). Since then
// search.list costs 1 unit but also draws from its own bucket of
// SearchCallsPerDay calls; everything else shares UnitsPerDay.
const (
	costList           = 1 // channels.list, playlists.list, playlistItems.list (per page)
	costSearch         = 1 // search.list, plus one call from the search bucket
	costPlaylistInsert = WriteUnits
	costPlaylistUpdate = WriteUnits
	costItemInsert     = WriteUnits
	costItemDelete     = WriteUnits
)

// WriteUnits is the cost of every write Soundsync makes: playlists.insert
// and .update, playlistItems.insert and .delete.
const WriteUnits = 50

// Default daily allocations of a Google Cloud project.
const (
	UnitsPerDay       = 10000
	SearchCallsPerDay = 100
)

// MaxPlaylistItems is the most videos a YouTube playlist holds.
const MaxPlaylistItems = 5000

// Client calls the YouTube Data API with an already-authorized HTTP client
// and counts the quota it spends.
type Client struct {
	http     *http.Client
	baseURL  string
	units    atomic.Int64
	searches atomic.Int64
}

// UnitsUsed is the quota spent by this client so far. Failed calls are
// counted at full cost: YouTube charges invalid requests at least 1 unit,
// so this errs on the side of overcounting a daily budget.
func (c *Client) UnitsUsed() int64 { return c.units.Load() }

// SearchesUsed is the number of search.list calls made by this client,
// which draw from the separate daily search bucket.
func (c *Client) SearchesUsed() int64 { return c.searches.Load() }

// NewClient wraps hc, which must add the user's bearer token (see oauth.Flow.Client).
func NewClient(hc *http.Client) *Client {
	return &Client{http: hc, baseURL: DefaultBaseURL}
}

// WithBaseURL points the client at another server (tests).
func (c *Client) WithBaseURL(u string) *Client {
	c.baseURL = strings.TrimRight(u, "/")
	return c
}

// Channel is the YouTube channel that will own the playlists.
type Channel struct {
	ID    string
	Title string
}

// ErrNoChannel means the Google account has no YouTube channel yet.
var ErrNoChannel = errors.New("this Google account has no YouTube channel")

// APIError is a non-2xx response from the Data API.
type APIError struct {
	Status  int
	Reason  string // first errors[].reason, e.g. accessNotConfigured, quotaExceeded
	Message string
}

func (e *APIError) Error() string {
	msg := fmt.Sprintf("youtube API %d", e.Status)
	if e.Reason != "" {
		msg += " " + e.Reason
	}
	if e.Message != "" {
		msg += ": " + e.Message
	}
	return msg
}

// MyChannel returns the authorized user's channel. Costs 1 quota unit.
func (c *Client) MyChannel(ctx context.Context) (Channel, error) {
	var body struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title string `json:"title"`
			} `json:"snippet"`
		} `json:"items"`
	}
	q := url.Values{"part": {"snippet"}, "mine": {"true"}, "maxResults": {"1"}}
	if err := c.get(ctx, "/channels", q, costList, &body); err != nil {
		return Channel{}, err
	}
	if len(body.Items) == 0 {
		return Channel{}, ErrNoChannel
	}
	it := body.Items[0]
	return Channel{ID: it.ID, Title: it.Snippet.Title}, nil
}

func (c *Client) get(ctx context.Context, path string, q url.Values, cost int64, out any) error {
	return c.do(ctx, http.MethodGet, path, q, nil, cost, out)
}

func (c *Client) post(ctx context.Context, path string, q url.Values, body any, cost int64, out any) error {
	return c.do(ctx, http.MethodPost, path, q, body, cost, out)
}

func (c *Client) do(ctx context.Context, method, path string, q url.Values, body any, cost int64, out any) error {
	var reqBody io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path+"?"+q.Encode(), reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("youtube %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	// Counted once the request reached YouTube, whatever the status
	// (conservative: see UnitsUsed).
	c.units.Add(cost)

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return parseAPIError(resp)
	}
	if out == nil { // e.g. DELETE answers 204 with no body
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode youtube %s %s: %w", method, path, err)
	}
	return nil
}

func parseAPIError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	var envelope struct {
		Error struct {
			Message string `json:"message"`
			Errors  []struct {
				Reason string `json:"reason"`
			} `json:"errors"`
		} `json:"error"`
	}
	apiErr := &APIError{Status: resp.StatusCode}
	if json.Unmarshal(body, &envelope) == nil {
		apiErr.Message = envelope.Error.Message
		if len(envelope.Error.Errors) > 0 {
			apiErr.Reason = envelope.Error.Errors[0].Reason
		}
	}
	return apiErr
}
