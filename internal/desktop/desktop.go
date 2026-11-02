// Package desktop is the backend of Soundsync's desktop GUI. Its three
// services (Settings, Accounts, Copy) are bound to the frontend by the GUI
// shell in gui/, which also implements Host. The package never imports the
// GUI toolkit, so it is tested like any other package.
//
// Every exported method of a service is callable from the frontend:
// validate every argument here, never in the frontend alone.
package desktop

import (
	"context"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/CharlyCeballos/soundsync/internal/config"
	"github.com/CharlyCeballos/soundsync/internal/copier"
	"github.com/CharlyCeballos/soundsync/internal/session"
	"github.com/CharlyCeballos/soundsync/internal/update"
)

// Host is what the backend needs from the desktop toolkit.
type Host interface {
	// Emit sends an event to the frontend. It must be safe for concurrent use.
	Emit(event string, data any)
	// OpenExternal opens a URL or a folder with the OS default handler.
	OpenExternal(target string) error
	// SaveFile asks where to save a file; "" means the user cancelled.
	SaveFile(title, defaultName string) (string, error)
	// Focus brings the window to the front (after a browser login).
	Focus()
}

// Event names sent to the frontend.
const (
	EventLoginURL = "login:url"     // LoginURL: the browser login page, in case the browser did not open
	EventProgress = "copy:progress" // Progress
)

// LoginURL lets the user open the login page by hand.
type LoginURL struct {
	Provider string `json:"provider"` // "spotify" or "youtube"
	URL      string `json:"url"`
}

// Progress mirrors copier.Event for the frontend.
type Progress struct {
	SourceID string `json:"sourceId"`
	Step     string `json:"step"`
	Done     int    `json:"done"`
	Total    int    `json:"total"`
	Title    string `json:"title,omitempty"`
}

// LoginFunc logs in to one service; the GUI uses the real browser logins and
// tests replace them.
type (
	SpotifyLogin func(ctx, base context.Context, cfg config.Config, ui session.UI) (*session.Spotify, error)
	YouTubeLogin func(ctx, base context.Context, cfg config.Config, ui session.UI) (*session.YouTube, error)
)

// Options configure the backend.
type Options struct {
	Version   string // the build's version, e.g. v0.2.0 or dev
	ConfigDir string // where .env and the policy acceptance live; config.Dir()

	HTTP      *http.Client // for the update check; nil = http.DefaultClient
	UpdateURL string       // "" = update.LatestURL

	LoginSpotify SpotifyLogin // nil = the browser login
	LoginYouTube YouTubeLogin // nil = the browser login, channel read, revoke on failure
	Now          func() time.Time
}

// shutdownWait bounds how long closing the app waits for a running step.
const shutdownWait = 5 * time.Second

// App holds the services and the state they share. It is not bound to the
// frontend itself: only its services are.
type App struct {
	Settings *Settings
	Accounts *Accounts
	Copy     *Copy
	st       *state
}

// New builds the backend.
func New(host Host, opts Options) *App {
	if opts.HTTP == nil {
		opts.HTTP = http.DefaultClient
	}
	if opts.UpdateURL == "" {
		opts.UpdateURL = update.LatestURL
	}
	if opts.LoginSpotify == nil {
		opts.LoginSpotify = session.LoginSpotify
	}
	if opts.LoginYouTube == nil {
		opts.LoginYouTube = loginYouTube
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	base, cancel := context.WithCancel(context.Background())
	st := &state{host: host, opts: opts, base: base, cancelBase: cancel}
	return &App{Settings: &Settings{st}, Accounts: &Accounts{st}, Copy: &Copy{st}, st: st}
}

// Shutdown stops the running step, waits for it a little, and revokes
// Google's grant (YouTube API Policies III.D.2). Call it when the app quits.
func (a *App) Shutdown() {
	st := a.st
	st.cancelBase()
	st.mu.Lock()
	done := st.opDone
	st.mu.Unlock()
	if done != nil {
		select {
		case <-done:
		case <-time.After(shutdownWait):
		}
	}
	st.mu.Lock()
	yt := st.yt
	st.yt = nil
	st.mu.Unlock()
	if yt != nil {
		_ = yt.Revoke() // best effort: the process is going away
	}
}

// loginYouTube is the browser login plus the channel read; a session without
// a usable channel is revoked at once rather than kept around.
func loginYouTube(ctx, base context.Context, cfg config.Config, ui session.UI) (*session.YouTube, error) {
	y, err := session.AuthorizeYouTube(ctx, base, cfg, ui)
	if err != nil {
		return nil, err
	}
	if err := y.LoadChannel(ctx); err != nil {
		_ = y.Revoke()
		return nil, err
	}
	return y, nil
}

// phase is where the copy flow stands.
type phase int

const (
	phaseNone     phase = iota
	phaseResolved       // destinations known; same-title ones wait for a choice
	phasePlanned        // matches found; writes waiting for the user's consent
	phaseWritten        // the plan was written (or the write stopped): plan again to copy more
)

// state is shared by the services. mu guards every field; it is never held
// across a network call. busy names the step running, so a second login,
// plan or write is refused instead of racing (the login port is fixed).
type state struct {
	host Host
	opts Options

	base       context.Context // the app's lifetime; OAuth clients are built with it
	cancelBase context.CancelFunc

	mu             sync.Mutex
	busy           string
	opDone         chan struct{}
	stopOp         context.CancelFunc // ends the running step early (Copy.Stop)
	policyAccepted bool               // for this process, when the OS has no config folder

	sp *session.Spotify
	yt *session.YouTube

	phase   phase
	dests   []copier.Destination
	jobs    []*copier.Job
	privacy string
	budget  *copier.Budget
}

// begin marks a step as running. The returned context ends when the
// frontend cancels the call or the app quits; end must be called once.
func (st *state) begin(ctx context.Context, step string) (context.Context, func(), error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.busy != "" {
		return nil, nil, ErrBusy
	}
	opCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(st.base, cancel)
	done := make(chan struct{})
	st.busy, st.opDone, st.stopOp = step, done, cancel
	end := func() {
		stop()
		cancel()
		st.mu.Lock()
		st.busy, st.opDone, st.stopOp = "", nil, nil
		st.mu.Unlock()
		close(done)
	}
	return opCtx, end, nil
}

// envFile is where the GUI reads its OAuth clients: always the config
// folder, because a shortcut's working folder is unpredictable.
func (st *state) envFile() string {
	if st.opts.ConfigDir == "" {
		return ""
	}
	return filepath.Join(st.opts.ConfigDir, config.FileName)
}

// resetCopy forgets the copy flow, after an account changes.
func (st *state) resetCopy() {
	st.phase, st.dests, st.jobs, st.budget = phaseNone, nil, nil, nil
}
