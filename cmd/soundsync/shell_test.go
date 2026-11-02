package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/CharlyCeballos/soundsync/internal/session"
	"github.com/CharlyCeballos/soundsync/internal/spotify"
	"github.com/CharlyCeballos/soundsync/internal/youtube"

	"golang.org/x/oauth2"
)

func TestSplitArgs(t *testing.T) {
	cases := []struct {
		line string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{"spotify playlists", []string{"spotify", "playlists"}},
		{"  spotify   tracks   Tristes  ", []string{"spotify", "tracks", "Tristes"}},
		{`spotify tracks "The Way to My Favorite Part"`, []string{"spotify", "tracks", "The Way to My Favorite Part"}},
		{`spotify tracks 'Para regresar 😈💕💝'`, []string{"spotify", "tracks", "Para regresar 😈💕💝"}},
		{`spotify tracks "Surfin' 🏄"`, []string{"spotify", "tracks", "Surfin' 🏄"}},
		{`config check --env-file "C:\Users\me\my env\.env"`, []string{"config", "check", "--env-file", `C:\Users\me\my env\.env`}},
		{`x ""`, []string{"x", ""}},
	}
	for _, c := range cases {
		got, err := splitArgs(c.line)
		if err != nil {
			t.Errorf("splitArgs(%q): %v", c.line, err)
			continue
		}
		if strings.Join(got, "|") != strings.Join(c.want, "|") || len(got) != len(c.want) {
			t.Errorf("splitArgs(%q) = %q, want %q", c.line, got, c.want)
		}
	}
	if _, err := splitArgs(`spotify tracks "unclosed`); err == nil {
		t.Error("want error for unclosed quote")
	}
}

// fakeSpotifyAPI serves a tiny library: one owned playlist and one followed.
func fakeSpotifyAPI(t *testing.T) *spotify.Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/me/playlists":
			w.Write([]byte(`{"items":[
				{"id":"5zb6y1KrutMp4L1HasRRo7","name":"Tristes 😢😓","owner":{"id":"me"},"items":{"total":1}},
				{"id":"3REKkLasecJIJnNwk8Kzeg","name":"Pentatonix Evolution of Music","owner":{"id":"someone"}}
			],"next":null}`))
		case "/playlists/5zb6y1KrutMp4L1HasRRo7/items":
			w.Write([]byte(`{"items":[{"item":{"type":"track","id":"t1","name":"Without You","artists":[{"name":"Air Supply"}],"external_ids":{"isrc":"USGI10000048"}}}],"next":null}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return spotify.NewClient(srv.Client()).WithBaseURL(srv.URL)
}

func TestShellLogsInOncePerService(t *testing.T) {
	var spotifyLogins, youtubeLogins atomic.Int32
	a := newApp()
	a.newSpotify = func(ctx context.Context, out io.Writer, a *app) (*session.Spotify, error) {
		spotifyLogins.Add(1)
		return &session.Spotify{Client: fakeSpotifyAPI(t), Me: spotify.User{ID: "me", DisplayName: "Charly"}}, nil
	}
	a.newYouTube = func(ctx context.Context, out io.Writer, a *app) (*session.YouTube, error) {
		youtubeLogins.Add(1)
		return &session.YouTube{Channel: youtube.Channel{ID: "UC1", Title: "Carlos Ceballos"}, Token: &oauth2.Token{AccessToken: "yt-access"}}, nil
	}

	in := strings.NewReader(strings.Join([]string{
		"login spotify",
		"spotify playlists",
		"spotify tracks Tristes",
		`spotify tracks "Pentatonix Evolution"`,
		"spotify tracks nothing-like-this",
		"login youtube",
		"login youtube",
		"shell",
		"exit",
		"spotify playlists", // never reached
	}, "\n"))
	var out, errOut bytes.Buffer
	if err := runShell(a, in, &out, &errOut); err != nil {
		t.Fatal(err)
	}

	if n := spotifyLogins.Load(); n != 1 {
		t.Errorf("Spotify logins = %d, want 1", n)
	}
	if n := youtubeLogins.Load(); n != 1 {
		t.Errorf("YouTube logins = %d, want 1", n)
	}
	got := out.String()
	for _, want := range []string{
		"Logged in to Spotify as Charly",
		"Playlists in your Spotify library: 2",
		`Playlist "Tristes 😢😓"`,
		"Air Supply — Without You",
		`channel "Carlos Ceballos"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("stdout missing %q:\n%s", want, got)
		}
	}
	if strings.Count(got, "Playlists in your Spotify library") != 1 {
		t.Errorf("commands after 'exit' must not run:\n%s", got)
	}
	gotErr := errOut.String()
	for _, want := range []string{
		"belongs to another user",
		`no playlist in your library matches "nothing-like-this"`,
		"already in the shell",
	} {
		if !strings.Contains(gotErr, want) {
			t.Errorf("stderr missing %q:\n%s", want, gotErr)
		}
	}
}

func TestCloseRevokesGoogleOnce(t *testing.T) {
	var revoked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		revoked = append(revoked, r.PostForm.Get("token"))
	}))
	defer srv.Close()
	old := youtube.RevokeURL
	youtube.RevokeURL = srv.URL
	defer func() { youtube.RevokeURL = old }()

	a := newApp()
	var out bytes.Buffer
	a.close(&out) // no session: nothing to revoke
	if len(revoked) != 0 {
		t.Fatalf("revoked without a session: %v", revoked)
	}

	a.yt = &session.YouTube{Token: &oauth2.Token{AccessToken: "a", RefreshToken: "r"}}
	a.close(&out)
	a.close(&out) // second close is a no-op
	if len(revoked) != 1 || revoked[0] != "r" {
		t.Errorf("revoked = %v, want exactly one refresh-token revocation", revoked)
	}
	if !strings.Contains(out.String(), "Google access revoked") {
		t.Errorf("missing confirmation:\n%s", out.String())
	}
}

func TestShellKeepsCustomEnvFile(t *testing.T) {
	dir := t.TempDir()
	custom := filepath.Join(dir, "custom.env")
	if err := os.WriteFile(custom, []byte("SPOTIFY_CLIENT_ID=x\nGOOGLE_CLIENT_ID=y\nGOOGLE_CLIENT_SECRET=z\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	clearConfigEnv(t)

	a := newApp()
	a.envFile = custom
	in := strings.NewReader("config check\nconfig check\nexit\n")
	var out, errOut bytes.Buffer
	if err := runShell(a, in, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(out.String(), custom+" (loaded)"); n != 2 {
		t.Errorf("custom env file used %d times, want 2:\n%s\n%s", n, out.String(), errOut.String())
	}
}
