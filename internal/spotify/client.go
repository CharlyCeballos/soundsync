package spotify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DefaultBaseURL is the Spotify Web API root.
const DefaultBaseURL = "https://api.spotify.com/v1"

// Client calls the Spotify Web API with an already-authorized HTTP client.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient wraps hc, which must add the user's bearer token (see oauth.Flow.Client).
func NewClient(hc *http.Client) *Client {
	return &Client{http: hc, baseURL: DefaultBaseURL}
}

// WithBaseURL points the client at another server (tests).
func (c *Client) WithBaseURL(u string) *Client {
	c.baseURL = strings.TrimRight(u, "/")
	return c
}

// User is the subset of the current user's profile Soundsync needs.
type User struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// APIError is a non-2xx response from the Web API.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("spotify API %d: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("spotify API %d", e.Status)
}

// Me returns the profile of the logged-in user.
func (c *Client) Me(ctx context.Context) (User, error) {
	var u User
	err := c.get(ctx, "/me", &u)
	return u, err
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	return c.getURL(ctx, c.baseURL+path, out)
}

// getURL fetches an absolute URL; callers must keep it on c.baseURL.
func (c *Client) getURL(ctx context.Context, rawURL string, out any) error {
	path := strings.TrimPrefix(rawURL, c.baseURL)
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("spotify GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return parseAPIError(resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode spotify %s: %w", path, err)
	}
	return nil
}

func parseAPIError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	var envelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	msg := ""
	if json.Unmarshal(body, &envelope) == nil {
		msg = envelope.Error.Message
	}
	return &APIError{Status: resp.StatusCode, Message: msg}
}
