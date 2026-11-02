// Package session holds the logins of one Soundsync process: the Spotify and
// YouTube clients, who is logged in, and caches that live in RAM only.
// Tokens never leave these structs and are never written to disk. Callers
// show messages and ask questions; this package does neither.
package session

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/CharlyCeballos/soundsync/internal/config"
	"github.com/CharlyCeballos/soundsync/internal/i18n"
	"github.com/CharlyCeballos/soundsync/internal/oauth"
	"github.com/CharlyCeballos/soundsync/internal/spotify"
	"github.com/CharlyCeballos/soundsync/internal/transfer"
	"github.com/CharlyCeballos/soundsync/internal/youtube"

	"golang.org/x/oauth2"
)

const (
	// LoginTimeout bounds how long a login waits for the user in the
	// browser. It covers the login only, never the work done afterwards.
	LoginTimeout = 5 * time.Minute
	// revokeTimeout bounds the best-effort token revocation.
	revokeTimeout = 10 * time.Second
)

// UI is how a login reaches the user.
type UI struct {
	// Open shows the authorization URL: it opens the browser and gives the
	// user a way to open the URL by hand.
	Open func(authURL string) error
	// Logf, if set, receives diagnostic notes (never tokens or codes).
	Logf func(format string, args ...any)
}

// Spotify is a logged-in Spotify session.
type Spotify struct {
	Client *spotify.Client
	Me     spotify.User

	mu      sync.Mutex
	library []spotify.Playlist // nil until fetched
}

// Playlists returns the user's library. The first listing is kept for name
// lookups; refresh fetches it again (the library may have changed).
func (s *Spotify) Playlists(ctx context.Context, refresh bool) ([]spotify.Playlist, error) {
	s.mu.Lock()
	cached := s.library
	s.mu.Unlock()
	if cached != nil && !refresh {
		return cached, nil
	}
	playlists, err := s.Client.MyPlaylists(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.library = playlists
	s.mu.Unlock()
	return playlists, nil
}

// YouTube is a Google login with access to the user's YouTube channel.
type YouTube struct {
	Client  *youtube.Client
	Channel youtube.Channel
	Token   *oauth2.Token

	// picks remembers the video found for each search query (todo 2.16):
	// running a copy again in the same session costs no searches. It lives
	// in RAM and dies with the session.
	picks sync.Map // query -> transfer.Matched
}

// Pick returns the match found earlier in this session for a search query.
func (y *YouTube) Pick(query string) (transfer.Matched, bool) {
	m, ok := y.picks.Load(query)
	if !ok {
		return transfer.Matched{}, false
	}
	return m.(transfer.Matched), true
}

// Remember keeps a match for the rest of the session.
func (y *YouTube) Remember(query string, m transfer.Matched) { y.picks.Store(query, m) }

// Revoke ends Google's grant (YouTube API Policies III.D.2). Spotify has no
// revocation endpoint; its token simply disappears with the process.
func (y *YouTube) Revoke() error {
	ctx, cancel := context.WithTimeout(context.Background(), revokeTimeout)
	defer cancel()
	return youtube.Revoke(ctx, &http.Client{Timeout: revokeTimeout}, y.Token)
}

// LoginSpotify runs the browser login and reads who logged in. base outlives
// the login: token refreshes use the context the client was built with.
func LoginSpotify(ctx, base context.Context, cfg config.Config, ui UI) (*Spotify, error) {
	flow := spotify.NewFlow(cfg.SpotifyClientID, cfg.SpotifyRedirectURI)
	flow.Logf = ui.Logf

	loginCtx, cancel := context.WithTimeout(ctx, LoginTimeout)
	defer cancel()
	tok, err := flow.Login(loginCtx, ui.Open)
	if err != nil {
		return nil, explainLoginError("Spotify", err)
	}

	client := spotify.NewClient(flow.Client(base, tok))
	me, err := client.Me(ctx)
	if err != nil {
		var apiErr *spotify.APIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusForbidden {
			return nil, fmt.Errorf(i18n.T("%w\nhint: add this Spotify account under User Management in your app's dashboard, and check that the app owner's Premium is active"), err)
		}
		return nil, err
	}
	return &Spotify{Client: client, Me: me}, nil
}

// AuthorizeYouTube runs the Google browser login. The session has no
// channel yet: call LoadChannel, and Revoke if that fails.
func AuthorizeYouTube(ctx, base context.Context, cfg config.Config, ui UI) (*YouTube, error) {
	flow := youtube.NewFlow(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURI)
	flow.Logf = ui.Logf

	loginCtx, cancel := context.WithTimeout(ctx, LoginTimeout)
	defer cancel()
	tok, err := flow.Login(loginCtx, ui.Open)
	if err != nil {
		return nil, explainLoginError("YouTube", err)
	}
	return &YouTube{Client: youtube.NewClient(flow.Client(base, tok)), Token: tok}, nil
}

// LoadChannel reads the channel playlists will be written to (1 unit).
func (y *YouTube) LoadChannel(ctx context.Context) error {
	ch, err := y.Client.MyChannel(ctx)
	if err != nil {
		var apiErr *youtube.APIError
		switch {
		case errors.Is(err, youtube.ErrNoChannel):
			return fmt.Errorf(i18n.T("%w\nhint: create a channel at https://www.youtube.com/create_channel, or log in with another Google account"), err)
		case errors.As(err, &apiErr) && apiErr.Reason == "accessNotConfigured":
			return fmt.Errorf(i18n.T("%w\nhint: enable the YouTube Data API v3 in your Google Cloud project (docs/setup.md, step 2)"), err)
		}
		return err
	}
	y.Channel = ch
	return nil
}

func explainLoginError(provider string, err error) error {
	var pe *oauth.ProviderError
	var re *oauth2.RetrieveError
	switch {
	case errors.As(err, &pe) && pe.Code == "access_denied":
		return fmt.Errorf(i18n.T("%s login was cancelled or denied in the browser"), provider)
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf(i18n.T("%s login timed out after %s"), provider, LoginTimeout)
	case errors.Is(err, context.Canceled):
		return fmt.Errorf(i18n.T("%s login cancelled"), provider)
	case errors.As(err, &re) && re.ErrorCode == "invalid_client":
		return fmt.Errorf(i18n.T("%w\nhint: the client ID or secret in .env does not match the OAuth client in the console"), err)
	}
	return err
}
