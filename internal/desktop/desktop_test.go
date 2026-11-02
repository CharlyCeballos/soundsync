package desktop

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CharlyCeballos/soundsync/internal/config"
	"github.com/CharlyCeballos/soundsync/internal/consent"
	"github.com/CharlyCeballos/soundsync/internal/fakeapi"
	"github.com/CharlyCeballos/soundsync/internal/i18n"
	"github.com/CharlyCeballos/soundsync/internal/report"
	"github.com/CharlyCeballos/soundsync/internal/session"
	"github.com/CharlyCeballos/soundsync/internal/spotify"
	"github.com/CharlyCeballos/soundsync/internal/youtube"

	"golang.org/x/oauth2"
)

// host records what the backend asks of the GUI shell.
type host struct {
	mu       sync.Mutex
	events   []string
	payloads []any
	opened   []string
	focused  int
	savePath string // what the save dialog answers
}

func (h *host) Emit(event string, data any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, event)
	h.payloads = append(h.payloads, data)
}

func (h *host) OpenExternal(target string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.opened = append(h.opened, target)
	return nil
}

func (h *host) SaveFile(string, string) (string, error) { return h.savePath, nil }

func (h *host) Focus() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.focused++
}

func (h *host) count(event string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, e := range h.events {
		if e == event {
			n++
		}
	}
	return n
}

const validEnv = "SPOTIFY_CLIENT_ID=0123456789abcdef0123456789abcdef\n" +
	"GOOGLE_CLIENT_ID=123-abc.apps.googleusercontent.com\n" +
	"GOOGLE_CLIENT_SECRET=GOCSPX-test\n"

// clearEnv keeps the developer's real environment out of the config checks.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{config.EnvSpotifyClientID, config.EnvSpotifyRedirectURI, config.EnvGoogleClientID, config.EnvGoogleClientSecret, config.EnvGoogleRedirectURI} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
}

// fixture is a backend whose logins reach fake Spotify and YouTube APIs.
type fixture struct {
	app  *App
	host *host
	ch   *fakeapi.YouTube
	dir  string
}

func newFixture(t *testing.T, ch *fakeapi.YouTube) *fixture {
	t.Helper()
	clearEnv(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(validEnv), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := consent.Remember(dir); err != nil {
		t.Fatal(err)
	}
	spSrv, ytSrv := httptest.NewServer(fakeapi.NewLibrary()), httptest.NewServer(ch)
	t.Cleanup(spSrv.Close)
	t.Cleanup(ytSrv.Close)
	h := &host{savePath: filepath.Join(dir, "report.csv")}
	app := New(h, Options{
		Version:   "dev",
		ConfigDir: dir,
		LoginSpotify: func(ctx, base context.Context, cfg config.Config, ui session.UI) (*session.Spotify, error) {
			if err := ui.Open("https://accounts.spotify.com/authorize?x=1"); err != nil {
				return nil, err
			}
			return &session.Spotify{Client: spotify.NewClient(spSrv.Client()).WithBaseURL(spSrv.URL), Me: spotify.User{ID: fakeapi.UserID, DisplayName: "Charly"}}, nil
		},
		LoginYouTube: func(ctx, base context.Context, cfg config.Config, ui session.UI) (*session.YouTube, error) {
			return &session.YouTube{
				Client:  youtube.NewClient(ytSrv.Client()).WithBaseURL(ytSrv.URL),
				Channel: youtube.Channel{ID: "UC1", Title: "Carlos Ceballos"},
				Token:   &oauth2.Token{AccessToken: "a", RefreshToken: "r"},
			}, nil
		},
		Now: func() time.Time { return time.Date(2026, 11, 2, 15, 30, 0, 0, time.UTC) },
	})
	return &fixture{app: app, host: h, ch: ch, dir: dir}
}

func (f *fixture) connect(t *testing.T) {
	t.Helper()
	if _, err := f.app.Accounts.ConnectSpotify(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.Accounts.ConnectYouTube(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestFirstRunGates(t *testing.T) {
	clearEnv(t)
	dir := filepath.Join(t.TempDir(), "soundsync") // does not exist yet
	h := &host{}
	app := New(h, Options{ConfigDir: dir})

	st := app.Settings.Status()
	if st.PolicyAccepted || st.Config.Found || st.Config.OK || st.Config.File != filepath.Join(dir, ".env") {
		t.Fatalf("first run status = %+v", st)
	}
	if _, err := app.Accounts.ConnectSpotify(t.Context()); !errors.Is(err, ErrPolicyNotAccepted) {
		t.Fatalf("connect before consent: err = %v", err)
	}
	if err := app.Settings.AcceptPolicy(); err != nil || !consent.Accepted(dir) {
		t.Fatalf("accept: err=%v remembered=%v", err, consent.Accepted(dir))
	}
	if _, err := app.Accounts.ConnectSpotify(t.Context()); err == nil || !strings.Contains(err.Error(), "SPOTIFY_CLIENT_ID is required") {
		t.Fatalf("connect without config: err = %v", err)
	}

	if err := app.Settings.CreateConfigFile(); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, ".env")); string(b) != config.Example {
		t.Error("the created file must be the example")
	}
	if len(h.opened) != 1 || h.opened[0] != dir {
		t.Errorf("opened %v, want the config folder", h.opened)
	}
	if err := app.Settings.CreateConfigFile(); !errors.Is(err, ErrConfigExists) {
		t.Errorf("second create: err = %v, want ErrConfigExists", err)
	}
	st = app.Settings.Status()
	if !st.Config.Found || st.Config.OK || st.Config.Fields[0].State != "missing" || st.Config.Fields[1].State != "set" {
		t.Errorf("status after creating the file = %+v", st.Config)
	}
	for _, f := range st.Config.Fields {
		if strings.Contains(f.State, "GOCSPX") {
			t.Fatal("secrets never reach the frontend")
		}
	}
}

func TestFullCopyFlow(t *testing.T) {
	f := newFixture(t, fakeapi.NewChannel())
	f.connect(t)
	if f.host.count(EventLoginURL) != 1 || f.host.focused != 2 {
		t.Errorf("login url events=%d focus=%d", f.host.count(EventLoginURL), f.host.focused)
	}

	pls, err := f.app.Accounts.Playlists(t.Context(), false)
	if err != nil || len(pls) != 3 || pls[2].Access != "followed" || pls[2].Tracks != -1 {
		t.Fatalf("playlists=%+v err=%v", pls, err)
	}

	dests, err := f.app.Copy.Resolve(t.Context(), []string{fakeapi.WayID, fakeapi.CoversID, fakeapi.WayID})
	if err != nil || len(dests) != 2 || dests[0].Kind != "existing" || dests[0].Target.Title != "The Way to My Favorite Part" {
		t.Fatalf("dests=%+v err=%v", dests, err)
	}

	plan, err := f.app.Copy.Plan(t.Context(), PlanRequest{Privacy: youtube.PrivacyPrivate})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Phase != "planned" || plan.Units != 4*youtube.WriteUnits || plan.Changing != 2 || plan.SearchesUsed != 4 || plan.SearchesLeft != 96 {
		t.Fatalf("plan = %+v", plan)
	}
	if way := plan.Playlists[0]; way.Add != 2 || way.Skipped != 1 || len(way.Rows) != 3 || way.Rows[0].Status != report.ToAdd {
		t.Errorf("way = %+v", way)
	}
	if f.host.count(EventProgress) < 6 {
		t.Errorf("progress events = %d, want lookups and searches", f.host.count(EventProgress))
	}
	if c := f.ch.Counts(); c.ItemInserts != 0 {
		t.Fatal("planning must not write")
	}

	done, err := f.app.Copy.Write(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if done.Phase != "written" || done.Units != 0 || done.Changing != 0 || !done.Playlists[0].InSync || done.Playlists[0].Added != 2 {
		t.Fatalf("written = %+v", done)
	}
	if _, err := f.app.Copy.Write(t.Context()); !errors.Is(err, ErrAlreadyWritten) {
		t.Errorf("second write: err = %v", err)
	}
	if c := f.ch.Counts(); c.ItemInserts != 4 {
		t.Errorf("inserts = %d, want 4", c.ItemInserts)
	}

	path, err := f.app.Copy.SaveReport("CSV")
	if err != nil || path != f.host.savePath {
		t.Fatalf("save: path=%q err=%v", path, err)
	}
	if b, _ := os.ReadFile(path); !strings.Contains(string(b), "added") {
		t.Errorf("report = %s", b)
	}
	if _, err := f.app.Copy.SaveReport("csv"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("saving over a file: err = %v", err)
	}
}

func TestAdoptSameTitle(t *testing.T) {
	ch := fakeapi.NewChannel()
	ch.Playlists[1] = fakeapi.UnmarkedPlaylist("PLmine", "Covers", "my covers", 1)
	ch.Seed("PLmine", "otherV", "Some Other Song")
	f := newFixture(t, ch)
	f.connect(t)

	dests, err := f.app.Copy.Resolve(t.Context(), []string{fakeapi.CoversID})
	if err != nil || dests[0].Kind != "choose" || len(dests[0].Candidates) != 1 || dests[0].Candidates[0].Videos != 1 {
		t.Fatalf("dests=%+v err=%v", dests, err)
	}
	if _, err := f.app.Copy.Plan(t.Context(), PlanRequest{Privacy: youtube.PrivacyPrivate}); err == nil || !strings.Contains(err.Error(), "choose whether") {
		t.Fatalf("plan without an answer: err = %v", err)
	}
	if _, err := f.app.Copy.Plan(t.Context(), PlanRequest{Privacy: youtube.PrivacyPrivate, Adopt: map[string]string{fakeapi.CoversID: "PLway"}}); err == nil {
		t.Fatal("adopting a playlist that was not offered must fail")
	}
	plan, err := f.app.Copy.Plan(t.Context(), PlanRequest{Privacy: youtube.PrivacyPrivate, Adopt: map[string]string{fakeapi.CoversID: "PLmine"}})
	if err != nil || plan.Playlists[0].Kind != "adopt" || plan.Playlists[0].Remove != 0 {
		t.Fatalf("plan=%+v err=%v (an adopted playlist keeps its videos this time)", plan, err)
	}
	if _, err := f.app.Copy.Write(t.Context()); err != nil {
		t.Fatal(err)
	}
	if c := f.ch.Counts(); c.Updates != 1 || c.Inserts != 0 {
		t.Errorf("updates=%d inserts=%d, want the description updated and nothing created", c.Updates, c.Inserts)
	}
}

func TestCreateWhenTheUserSaysNew(t *testing.T) {
	ch := fakeapi.NewChannel()
	ch.Playlists[1] = fakeapi.UnmarkedPlaylist("PLmine", "Covers", "", 0)
	f := newFixture(t, ch)
	f.connect(t)
	if _, err := f.app.Copy.Resolve(t.Context(), []string{fakeapi.CoversID}); err != nil {
		t.Fatal(err)
	}
	plan, err := f.app.Copy.Plan(t.Context(), PlanRequest{Privacy: youtube.PrivacyUnlisted, Adopt: map[string]string{fakeapi.CoversID: ""}})
	if err != nil || plan.Playlists[0].Kind != "create" || plan.Playlists[0].Target.Title != "Covers" {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
	if _, err := f.app.Copy.Write(t.Context()); err != nil {
		t.Fatal(err)
	}
	if c := f.ch.Counts(); c.Inserts != 1 {
		t.Errorf("playlists created = %d, want 1", c.Inserts)
	}
}

func TestResolveRejects(t *testing.T) {
	f := newFixture(t, fakeapi.NewChannel())
	if _, err := f.app.Copy.Resolve(t.Context(), []string{fakeapi.WayID}); !errors.Is(err, ErrSpotifyFirst) {
		t.Errorf("before connecting: err = %v", err)
	}
	f.connect(t)
	cases := map[string]string{fakeapi.FollowedID: "belongs to another user", "nope": "not in your Spotify library"}
	for id, want := range cases {
		if _, err := f.app.Copy.Resolve(t.Context(), []string{id}); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Resolve(%s): err = %v, want %q", id, err, want)
		}
	}
	if _, err := f.app.Copy.Resolve(t.Context(), nil); !errors.Is(err, ErrNoSelection) {
		t.Errorf("empty selection: err = %v", err)
	}
	if _, err := f.app.Copy.Plan(t.Context(), PlanRequest{Privacy: "everyone"}); !errors.Is(err, ErrBadPrivacy) {
		t.Errorf("bad privacy: err = %v", err)
	}
	if _, err := f.app.Copy.Write(t.Context()); !errors.Is(err, ErrPlanFirst) {
		t.Errorf("write without a plan: err = %v", err)
	}
	if _, err := f.app.Copy.SaveReport("xml"); !errors.Is(err, ErrBadFormat) {
		t.Errorf("bad format: err = %v", err)
	}
}

func TestOneStepAtATime(t *testing.T) {
	f := newFixture(t, fakeapi.NewChannel())
	entered := make(chan struct{})
	f.app.st.opts.LoginSpotify = func(ctx, base context.Context, cfg config.Config, ui session.UI) (*session.Spotify, error) {
		close(entered)
		<-ctx.Done() // the user never finishes in the browser
		return nil, ctx.Err()
	}
	ctx, cancel := context.WithCancel(t.Context())
	errc := make(chan error, 1)
	go func() {
		_, err := f.app.Accounts.ConnectSpotify(ctx)
		errc <- err
	}()
	<-entered
	if _, err := f.app.Accounts.ConnectYouTube(t.Context()); !errors.Is(err, ErrBusy) {
		t.Errorf("second login while one waits: err = %v", err)
	}
	if err := f.app.Accounts.DisconnectYouTube(); !errors.Is(err, ErrBusy) {
		t.Errorf("disconnect while busy: err = %v", err)
	}
	cancel()
	if err := <-errc; !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled login: err = %v", err)
	}
	if _, err := f.app.Accounts.ConnectYouTube(t.Context()); err != nil {
		t.Errorf("after the cancel the next step runs: %v", err)
	}
}

func TestCancelledPlanCannotBeWritten(t *testing.T) {
	f := newFixture(t, fakeapi.NewChannel())
	f.connect(t)
	if _, err := f.app.Copy.Resolve(t.Context(), []string{fakeapi.WayID}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	plan, err := f.app.Copy.Plan(ctx, PlanRequest{Privacy: youtube.PrivacyPrivate})
	if err == nil && !plan.Cancelled {
		t.Fatalf("plan = %+v, want cancelled", plan)
	}
	if _, err := f.app.Copy.Write(t.Context()); err == nil {
		t.Error("an incomplete plan must not be written")
	}
}

func TestOpenURLAllowlist(t *testing.T) {
	f := newFixture(t, fakeapi.NewChannel())
	for link, ok := range map[string]bool{
		"https://open.spotify.com/playlist/x":                              true,
		"https://music.youtube.com/playlist?list=PL1":                      true,
		"https://github.com/CharlyCeballos/soundsync/blob/main/PRIVACY.md": true,
		"http://open.spotify.com/playlist/x":                               false,
		"https://open.spotify.com.evil.example/":                           false,
		"https://open.spotify.com@evil.example/":                           false,
		"https://github.com/CharlyCeballos/soundsync-evil":                 false,
		"file:///etc/passwd":                                               false,
		"javascript:alert(1)":                                              false,
	} {
		err := f.app.Settings.OpenURL(link)
		if (err == nil) != ok {
			t.Errorf("OpenURL(%q) err = %v, want allowed=%v", link, err, ok)
		}
	}
}

// Not parallel: youtube.RevokeURL is a package variable.
func TestDisconnectAndShutdownRevoke(t *testing.T) {
	rv := &fakeapi.Revoker{}
	srv := httptest.NewServer(rv)
	t.Cleanup(srv.Close)
	old := youtube.RevokeURL
	youtube.RevokeURL = srv.URL
	t.Cleanup(func() { youtube.RevokeURL = old })

	f := newFixture(t, fakeapi.NewChannel())
	f.connect(t)
	if err := f.app.Accounts.DisconnectYouTube(); err != nil {
		t.Fatal(err)
	}
	if f.app.Settings.Status().YouTube != nil || len(rv.Tokens()) != 1 {
		t.Fatalf("after disconnect: revoked %v", rv.Tokens())
	}
	f.connect(t)
	f.app.Shutdown()
	if got := rv.Tokens(); len(got) != 2 || got[1] != "r" {
		t.Errorf("revoked %v, want the second session revoked on shutdown", got)
	}
}

func TestLanguageSwitchDuringARun(t *testing.T) {
	t.Cleanup(func() { i18n.Set(i18n.EN) })
	f := newFixture(t, fakeapi.NewChannel())
	f.connect(t)
	if _, err := f.app.Copy.Resolve(t.Context(), []string{fakeapi.WayID, fakeapi.CoversID}); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
				f.app.Settings.SetLanguage([]string{i18n.ES, i18n.EN}[i%2])
			}
		}
	}()
	_, err := f.app.Copy.Plan(t.Context(), PlanRequest{Privacy: youtube.PrivacyPrivate})
	close(stop)
	wg.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if err := f.app.Settings.SetLanguage("fr"); err == nil {
		t.Error("unsupported language must fail")
	}
}
