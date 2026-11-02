package session

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/CharlyCeballos/soundsync/internal/fakeapi"
	"github.com/CharlyCeballos/soundsync/internal/oauth"
	"github.com/CharlyCeballos/soundsync/internal/spotify"
	"github.com/CharlyCeballos/soundsync/internal/transfer"
	"github.com/CharlyCeballos/soundsync/internal/youtube"

	"golang.org/x/oauth2"
)

func TestPlaylistsCachedUntilRefresh(t *testing.T) {
	lib := fakeapi.NewLibrary()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/me/playlists" {
			calls.Add(1)
		}
		lib.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	s := &Spotify{Client: spotify.NewClient(srv.Client()).WithBaseURL(srv.URL)}

	for _, refresh := range []bool{false, false, true} {
		got, err := s.Playlists(t.Context(), refresh)
		if err != nil || len(got) != 3 {
			t.Fatalf("playlists=%d err=%v", len(got), err)
		}
	}
	if calls.Load() != 2 {
		t.Errorf("library fetched %d times, want 2 (first use and refresh)", calls.Load())
	}
}

func TestPickRemembersForTheSession(t *testing.T) {
	y := &YouTube{}
	if _, ok := y.Pick("Queen Bohemian Rhapsody"); ok {
		t.Fatal("nothing remembered yet")
	}
	y.Remember("Queen Bohemian Rhapsody", transfer.Matched{Video: youtube.Video{ID: "v1"}, Found: true})
	if m, ok := y.Pick("Queen Bohemian Rhapsody"); !ok || m.Video.ID != "v1" {
		t.Errorf("Pick = %+v, %v", m, ok)
	}
}

func channelServer(t *testing.T, status int, body string) *youtube.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return youtube.NewClient(srv.Client()).WithBaseURL(srv.URL)
}

func TestLoadChannel(t *testing.T) {
	cases := []struct {
		name, body, hint string
		status           int
	}{
		{"ok", `{"items":[{"id":"UC1","snippet":{"title":"Carlos"}}]}`, "", http.StatusOK},
		{"no channel", `{"items":[]}`, "create_channel", http.StatusOK},
		{"api disabled", `{"error":{"code":403,"message":"off","errors":[{"reason":"accessNotConfigured"}]}}`, "enable the YouTube Data API v3", http.StatusForbidden},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			y := &YouTube{Client: channelServer(t, c.status, c.body)}
			err := y.LoadChannel(t.Context())
			if c.hint == "" {
				if err != nil || y.Channel.Title != "Carlos" {
					t.Fatalf("channel=%+v err=%v", y.Channel, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.hint) {
				t.Errorf("err = %v, want hint %q", err, c.hint)
			}
		})
	}
}

// Not parallel: youtube.RevokeURL is a package variable.
func TestRevokeSendsTheRefreshToken(t *testing.T) {
	rv := &fakeapi.Revoker{}
	srv := httptest.NewServer(rv)
	t.Cleanup(srv.Close)
	old := youtube.RevokeURL
	youtube.RevokeURL = srv.URL
	t.Cleanup(func() { youtube.RevokeURL = old })

	y := &YouTube{Token: &oauth2.Token{AccessToken: "a", RefreshToken: "r"}}
	if err := y.Revoke(); err != nil {
		t.Fatal(err)
	}
	if got := rv.Tokens(); len(got) != 1 || got[0] != "r" {
		t.Errorf("revoked %v, want [r]", got)
	}
}

func TestExplainLoginError(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{&oauth.ProviderError{Code: "access_denied"}, "Spotify login was cancelled or denied in the browser"},
		{fmt.Errorf("waiting: %w", context.DeadlineExceeded), "Spotify login timed out after 5m0s"},
		{fmt.Errorf("waiting: %w", context.Canceled), "Spotify login cancelled"},
		{&oauth2.RetrieveError{ErrorCode: "invalid_client"}, "does not match the OAuth client"},
	}
	for _, c := range cases {
		if got := explainLoginError("Spotify", c.err); !strings.Contains(got.Error(), c.want) {
			t.Errorf("explain(%v) = %q, want %q", c.err, got, c.want)
		}
	}
	other := errors.New("boom")
	if got := explainLoginError("Spotify", other); got != other {
		t.Errorf("unknown errors pass through, got %v", got)
	}
}
