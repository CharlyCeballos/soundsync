package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/CharlyCeballos/soundsync/internal/config"
	"github.com/CharlyCeballos/soundsync/internal/i18n"
	"github.com/CharlyCeballos/soundsync/internal/oauth"
	"github.com/CharlyCeballos/soundsync/internal/session"

	"github.com/spf13/cobra"
)

// app holds the state of one Soundsync process: configuration and the
// in-memory sessions. A one-shot command uses it once; the shell reuses it
// for every line, so each service asks for a browser login only once.
// Tokens never leave this struct and are never written to disk.
type app struct {
	envFile string
	// configDir holds the privacy-policy acceptance, the only file Soundsync
	// writes on its own; "" when the OS has no config folder.
	configDir      string
	policyAccepted bool

	// baseCtx outlives individual commands. OAuth clients are built with it
	// because token refreshes use the context the client was created with.
	baseCtx context.Context

	cfg *config.Config
	sp  *session.Spotify
	yt  *session.YouTube

	// readLine reads one line of user input. The shell points it at its own
	// scanner so confirmations do not steal lines from the command stream.
	readLine func() (string, error)

	// Session constructors; tests replace them to avoid real logins.
	newSpotify func(ctx context.Context, out io.Writer, a *app) (*session.Spotify, error)
	newYouTube func(ctx context.Context, out io.Writer, a *app) (*session.YouTube, error)
}

func newApp() *app {
	stdin := bufio.NewReader(os.Stdin)
	configDir := config.Dir()
	return &app{
		envFile:   defaultEnvFile(configDir),
		configDir: configDir,
		baseCtx:   context.Background(),
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

// defaultEnvFile prefers .env in the current folder (running from a clone)
// and falls back to the user's config folder, where an installed binary
// looks for it.
func defaultEnvFile(configDir string) string {
	const name = ".env"
	if _, err := os.Stat(name); err == nil || configDir == "" {
		return name
	}
	return filepath.Join(configDir, name)
}

// confirm asks a yes/no question; anything but an explicit yes means no,
// including end of input.
func (a *app) confirm(out io.Writer, question string) bool {
	fmt.Fprintf(out, i18n.T("%s [y/N]: "), i18n.T(question))
	line, err := a.readLine()
	if err != nil {
		fmt.Fprintln(out)
		return false
	}
	return isYes(strings.ToLower(strings.TrimSpace(line)))
}

// isYes accepts English and Spanish yes; answer must be trimmed and lowercase.
func isYes(answer string) bool {
	switch answer {
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
	f, err := config.ReadFile(a.envFile)
	if err != nil {
		return config.Config{}, err
	}
	cfg, err := config.FromEnv(f.Getenv)
	if err != nil {
		return config.Config{}, fmt.Errorf(i18n.T("%w\nrun 'soundsync config check' for details"), err)
	}
	a.cfg = &cfg
	return cfg, nil
}

// spotify returns the Spotify session, logging in on first use.
func (a *app) spotify(ctx context.Context, out io.Writer) (*session.Spotify, error) {
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
func (a *app) youtube(ctx context.Context, out io.Writer) (*session.YouTube, error) {
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
	err := a.yt.Revoke()
	a.yt = nil
	if err != nil {
		fmt.Fprintf(out, i18n.T("warning: %v; revoke it manually at https://myaccount.google.com/permissions\n"), err)
		return
	}
	fmt.Fprintln(out, i18n.T("Google access revoked. No Spotify or YouTube data was kept on disk, except reports you asked for."))
}

func loginSpotify(ctx context.Context, out io.Writer, a *app) (*session.Spotify, error) {
	cfg, err := a.config()
	if err != nil {
		return nil, err
	}
	if err := a.ensurePolicyAccepted(out); err != nil {
		return nil, err
	}
	return session.LoginSpotify(ctx, a.baseCtx, cfg, terminalUI(out, "Spotify"))
}

func loginYouTube(ctx context.Context, out io.Writer, a *app) (*session.YouTube, error) {
	cfg, err := a.config()
	if err != nil {
		return nil, err
	}
	if err := a.ensurePolicyAccepted(out); err != nil {
		return nil, err
	}
	s, err := session.AuthorizeYouTube(ctx, a.baseCtx, cfg, terminalUI(out, "YouTube"))
	if err != nil {
		return nil, err
	}
	if err := s.LoadChannel(ctx); err != nil {
		// The token exists but the session is unusable: revoke it now rather
		// than keep a half-built session around.
		a.yt = s
		a.close(out)
		return nil, err
	}
	fmt.Fprintln(out, i18n.T("Quota used: 1 unit (channels.list)."))
	return s, nil
}

// terminalUI prints the login URL and notes to the terminal.
func terminalUI(out io.Writer, provider string) session.UI {
	return session.UI{Open: browserOpener(out, provider), Logf: noteLogger(out)}
}

// runner is the body of a command that may need logins.
type runner func(ctx context.Context, out io.Writer, a *app, args []string) error

// command builds a subcommand whose context is cancelled by Ctrl+C.
func command(a *app, use string, aliases []string, short string, args cobra.PositionalArgs, run runner) *cobra.Command {
	return &cobra.Command{
		Use:     use,
		Aliases: aliases,
		Short:   i18n.T(short),
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
		fmt.Fprintf(out, i18n.T("Opening your browser to log in to %s.\nIf it does not open, paste this URL into it:\n\n  %s\n\nWaiting for the login (Ctrl+C to cancel)...\n"), provider, authURL)
		if err := oauth.OpenBrowser(authURL); err != nil {
			fmt.Fprintf(out, i18n.T("Could not open the browser automatically: %v\n"), err)
		}
		return nil
	}
}

func noteLogger(out io.Writer) func(string, ...any) {
	return func(format string, args ...any) { fmt.Fprintf(out, i18n.T("note: ")+format+"\n", args...) }
}
