//go:build e2e

// The e2e build serves the app over HTTP (with the server tag) against fake
// Spotify and YouTube APIs, for the Playwright tests in frontend/e2e. Each
// test starts its own process with a -scenario. Never shipped.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/CharlyCeballos/soundsync/internal/config"
	"github.com/CharlyCeballos/soundsync/internal/consent"
	"github.com/CharlyCeballos/soundsync/internal/desktop"
	"github.com/CharlyCeballos/soundsync/internal/fakeapi"
	"github.com/CharlyCeballos/soundsync/internal/session"
	"github.com/CharlyCeballos/soundsync/internal/spotify"
	"github.com/CharlyCeballos/soundsync/internal/youtube"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/oauth2"
)

var scenario = flag.String("scenario", "ready", "noconfig, nopolicy, ready, samename, quota or many")

// loginDelay lets the tests see the "waiting for the browser" state.
const loginDelay = 400 * time.Millisecond

const e2eEnv = "SPOTIFY_CLIENT_ID=0123456789abcdef0123456789abcdef\n" +
	"GOOGLE_CLIENT_ID=123-abc.apps.googleusercontent.com\n" +
	"GOOGLE_CLIENT_SECRET=GOCSPX-e2e\n"

// e2eHost records what a real host would hand to the OS.
type e2eHost struct {
	wailsHost
	dir    string
	mu     sync.Mutex
	opened []string
}

func (h *e2eHost) OpenExternal(target string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.opened = append(h.opened, target)
	return nil
}

// SaveFile answers like a user who accepts the suggested name.
func (h *e2eHost) SaveFile(_, defaultName string) (string, error) {
	return filepath.Join(h.dir, defaultName), nil
}

func (h *e2eHost) Focus() {}

type world struct {
	host    *e2eHost
	opts    desktop.Options
	channel *fakeapi.YouTube
	revoker *fakeapi.Revoker
}

var (
	setupOnce sync.Once
	e2e       world
)

func setup() {
	setupOnce.Do(func() {
		dir, err := os.MkdirTemp("", "soundsync-e2e-*")
		must(err)
		if *scenario != "noconfig" {
			must(os.WriteFile(filepath.Join(dir, config.FileName), []byte(e2eEnv), 0o600))
		}
		if *scenario != "noconfig" && *scenario != "nopolicy" {
			must(consent.Remember(dir))
		}

		lib, ch := fakeapi.NewLibrary(), fakeapi.NewChannel()
		switch *scenario {
		case "samename":
			ch.Playlists[1] = fakeapi.UnmarkedPlaylist("PLmine", "Covers", "my covers", 1)
			ch.Seed("PLmine", "otherV", "Some Other Song")
		case "quota":
			ch.FailItemAt = 2
		case "many":
			for i := 1; i <= 22; i++ {
				id := fmt.Sprintf("Mix%02dxxxxxxxxxxxxxxx", i)
				lib.Playlists = append(lib.Playlists, fakeapi.SpotifyPlaylist(id, fmt.Sprintf("Mix %02d", i), fakeapi.UserID, 1))
				lib.Entries[id] = []map[string]any{fakeapi.Track("t"+id, "Song "+id, 200000, "Artist")}
			}
		}
		rv := &fakeapi.Revoker{}
		spSrv, ytSrv, rvSrv := httptest.NewServer(lib), httptest.NewServer(ch), httptest.NewServer(rv)
		youtube.RevokeURL = rvSrv.URL

		e2e = world{host: &e2eHost{dir: dir}, channel: ch, revoker: rv}
		e2e.opts = desktop.Options{
			Version:   "v0.0.0-e2e",
			ConfigDir: dir,
			LoginSpotify: func(ctx, _ context.Context, _ config.Config, ui session.UI) (*session.Spotify, error) {
				if err := waitLogin(ctx, ui, "https://accounts.spotify.com/authorize?client_id=e2e"); err != nil {
					return nil, err
				}
				return &session.Spotify{Client: spotify.NewClient(spSrv.Client()).WithBaseURL(spSrv.URL), Me: spotify.User{ID: fakeapi.UserID, DisplayName: "Charly"}}, nil
			},
			LoginYouTube: func(ctx, _ context.Context, _ config.Config, ui session.UI) (*session.YouTube, error) {
				if err := waitLogin(ctx, ui, "https://accounts.google.com/o/oauth2/auth?client_id=e2e"); err != nil {
					return nil, err
				}
				return &session.YouTube{
					Client:  youtube.NewClient(ytSrv.Client()).WithBaseURL(ytSrv.URL),
					Channel: youtube.Channel{ID: "UCe2e", Title: "Carlos Ceballos"},
					Token:   &oauth2.Token{AccessToken: "a", RefreshToken: "e2e-refresh"},
				}, nil
			},
		}
	})
}

func waitLogin(ctx context.Context, ui session.UI, url string) error {
	if err := ui.Open(url); err != nil {
		return err
	}
	select {
	case <-time.After(loginDelay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func backendOptions() desktop.Options { setup(); return e2e.opts }

func newHost() shell { setup(); return e2e.host }

// assetMiddleware exposes /__e2e/state so the tests can check what reached
// the fake world: links opened, files saved, writes and revocations.
func assetMiddleware() application.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/__e2e/state" {
				next.ServeHTTP(w, r)
				return
			}
			e2e.host.mu.Lock()
			opened := append([]string(nil), e2e.host.opened...)
			e2e.host.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"dir":     e2e.host.dir,
				"opened":  opened,
				"counts":  e2e.channel.Counts(),
				"revoked": e2e.revoker.Tokens(),
			})
		})
	}
}
