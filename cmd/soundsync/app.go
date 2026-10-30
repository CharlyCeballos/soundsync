package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"soundsync/internal/config"
	"soundsync/internal/oauth"
	"soundsync/internal/spotify"
	"soundsync/internal/youtube"

	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

const (
	// loginTimeout bounds how long we wait for the user in the browser.
	// It covers the login only, never the work done afterwards.
	loginTimeout = 5 * time.Minute
	// revokeTimeout bounds the best-effort token revocation on exit.
	revokeTimeout = 10 * time.Second
)

// app holds the state of one Soundsync process: configuration and the
// in-memory sessions. A one-shot command uses it once; the shell reuses it
// for every line, so each service asks for a browser login only once.
// Tokens never leave this struct and are never written to disk.
type app struct {
	envFile string

	// baseCtx outlives individual commands. OAuth clients are built with it
	// because token refreshes use the context the client was created with.
	baseCtx context.Context

	cfg *config.Config
	sp  *spotifySession
	yt  *youtubeSession

	// readLine reads one line of user input. The shell points it at its own
	// scanner so confirmations do not steal lines from the command stream.
	readLine func() (string, error)

	// Session constructors; tests replace them to avoid real logins.
	newSpotify func(ctx context.Context, out io.Writer, a *app) (*spotifySession, error)
	newYouTube func(ctx context.Context, out io.Writer, a *app) (*youtubeSession, error)
}

type spotifySession struct {
	client *spotify.Client
	me     spotify.User
	// playlists caches the library listing for name lookups; nil until fetched.
	playlists []spotify.Playlist
}

type youtubeSession struct {
	client  *youtube.Client
	channel youtube.Channel
	tok     *oauth2.Token
}

func newApp() *app {
	stdin := bufio.NewReader(os.Stdin)
	return &app{
		envFile: ".env",
		baseCtx: context.Background(),
		readLine: func() (string, error) {
			line, err := stdin.ReadString('\n')
			if err == io.EOF && line != "" {
				err = nil
			}
			return line, err
		},
		newSpotify: loginSpotify,
		newYouTube: loginYouTube,
	}
}

// confirm asks a yes/no question; anything but an explicit yes means no,
// including end of input.
func (a *app) confirm(out io.Writer, question string) bool {
	fmt.Fprintf(out, "%s [y/N]: ", question)
	line, err := a.readLine()
	if err != nil {
		fmt.Fprintln(out)
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes", "s", "si", "sí":
		return true
	}
	return false
}

// config loads and validates .env once per process.
func (a *app) config() (config.Config, error) {
	if a.cfg != nil {
		return *a.cfg, nil
	}
	if _, err := config.LoadDotEnv(a.envFile); err != nil {
		return config.Config{}, err
	}
	cfg, err := config.FromEnv(os.Getenv)
	if err != nil {
		return config.Config{}, fmt.Errorf("%w\nrun 'soundsync config check' for details", err)
	}
	a.cfg = &cfg
	return cfg, nil
}

// spotify returns the Spotify session, logging in on first use.
func (a *app) spotify(ctx context.Context, out io.Writer) (*spotifySession, error) {
	if a.sp == nil {
		s, err := a.newSpotify(ctx, out, a)
		if err != nil {
			return nil, err
		}
		a.sp = s
	}
	return a.sp, nil
}

// youtube returns the YouTube session, logging in on first use.
func (a *app) youtube(ctx context.Context, out io.Writer) (*youtubeSession, error) {
	if a.yt == nil {
		s, err := a.newYouTube(ctx, out, a)
		if err != nil {
			return nil, err
		}
		a.yt = s
	}
	return a.yt, nil
}

// close ends the process's sessions: the Google token is revoked
// (YouTube API Policies III.D.2). Spotify has no revocation endpoint; its
// token simply disappears with the process.
func (a *app) close(out io.Writer) {
	if a.yt == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), revokeTimeout)
	defer cancel()
	err := youtube.Revoke(ctx, &http.Client{Timeout: revokeTimeout}, a.yt.tok)
	a.yt = nil
	if err != nil {
		fmt.Fprintf(out, "warning: %v; revoke it manually at https://myaccount.google.com/permissions\n", err)
		return
	}
	fmt.Fprintln(out, "Google access revoked. Nothing was stored on disk.")
}

func loginSpotify(ctx context.Context, out io.Writer, a *app) (*spotifySession, error) {
	cfg, err := a.config()
	if err != nil {
		return nil, err
	}
	flow := spotify.NewFlow(cfg.SpotifyClientID, cfg.SpotifyRedirectURI)
	flow.Logf = noteLogger(out)

	loginCtx, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()
	tok, err := flow.Login(loginCtx, browserOpener(out, "Spotify"))
	if err != nil {
		return nil, explainLoginError("Spotify", err)
	}

	client := spotify.NewClient(flow.Client(a.baseCtx, tok))
	me, err := client.Me(ctx)
	if err != nil {
		var apiErr *spotify.APIError
		if errors.As(err, &apiErr) && apiErr.Status == 403 {
			return nil, fmt.Errorf("%w\nhint: add this Spotify account under User Management in your app's dashboard, and check that the app owner's Premium is active", err)
		}
		return nil, err
	}
	return &spotifySession{client: client, me: me}, nil
}

func loginYouTube(ctx context.Context, out io.Writer, a *app) (*youtubeSession, error) {
	cfg, err := a.config()
	if err != nil {
		return nil, err
	}
	flow := youtube.NewFlow(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURI)
	flow.Logf = noteLogger(out)

	loginCtx, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()
	tok, err := flow.Login(loginCtx, browserOpener(out, "YouTube"))
	if err != nil {
		return nil, explainLoginError("YouTube", err)
	}
	s := &youtubeSession{client: youtube.NewClient(flow.Client(a.baseCtx, tok)), tok: tok}

	ch, err := s.client.MyChannel(ctx)
	if err != nil {
		// The token exists but the session is unusable: revoke it now rather
		// than keep a half-built session around.
		a.yt = s
		a.close(out)
		var apiErr *youtube.APIError
		switch {
		case errors.Is(err, youtube.ErrNoChannel):
			return nil, fmt.Errorf("%w\nhint: create a channel at https://www.youtube.com/create_channel, or log in with another Google account", err)
		case errors.As(err, &apiErr) && apiErr.Reason == "accessNotConfigured":
			return nil, fmt.Errorf("%w\nhint: enable the YouTube Data API v3 in your Google Cloud project (docs/setup.md, step 2)", err)
		}
		return nil, err
	}
	s.channel = ch
	fmt.Fprintln(out, "Quota used: 1 unit (channels.list).")
	return s, nil
}

// runner is the body of a command that may need logins.
type runner func(ctx context.Context, out io.Writer, a *app, args []string) error

// command builds a subcommand whose context is cancelled by Ctrl+C.
func command(a *app, use string, aliases []string, short string, args cobra.PositionalArgs, run runner) *cobra.Command {
	return &cobra.Command{
		Use:     use,
		Aliases: aliases,
		Short:   short,
		Args:    args,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer stop()
			return run(ctx, cmd.OutOrStdout(), a, args)
		},
	}
}

// browserOpener prints the URL (fallback) and tries to open it.
// A failure to launch the browser is not fatal: the user can paste the URL.
func browserOpener(out io.Writer, provider string) func(string) error {
	return func(authURL string) error {
		fmt.Fprintf(out, "Opening your browser to log in to %s.\nIf it does not open, paste this URL into it:\n\n  %s\n\nWaiting for the login (Ctrl+C to cancel)...\n", provider, authURL)
		if err := oauth.OpenBrowser(authURL); err != nil {
			fmt.Fprintf(out, "Could not open the browser automatically: %v\n", err)
		}
		return nil
	}
}

func noteLogger(out io.Writer) func(string, ...any) {
	return func(format string, args ...any) { fmt.Fprintf(out, "note: "+format+"\n", args...) }
}

func explainLoginError(provider string, err error) error {
	var pe *oauth.ProviderError
	var re *oauth2.RetrieveError
	switch {
	case errors.As(err, &pe) && pe.Code == "access_denied":
		return fmt.Errorf("%s login was cancelled or denied in the browser", provider)
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("%s login timed out after %s", provider, loginTimeout)
	case errors.Is(err, context.Canceled):
		return fmt.Errorf("%s login cancelled", provider)
	case errors.As(err, &re) && re.ErrorCode == "invalid_client":
		return fmt.Errorf("%w\nhint: the client ID or secret in .env does not match the OAuth client in the console", err)
	}
	return err
}
