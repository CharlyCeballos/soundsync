package main

import (
	"context"
	"fmt"
	"io"

	"github.com/CharlyCeballos/soundsync/internal/i18n"

	"github.com/spf13/cobra"
)

func newLoginCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: i18n.T("Log in to a service and show the connected account (token kept in memory only)"),
	}
	cmd.AddCommand(
		command(a, "spotify", nil, "Log in to Spotify and show the connected account", cobra.NoArgs, runSpotifyLogin),
		command(a, "youtube", []string{"google"}, "Log in to Google and show the YouTube channel (token revoked on exit)", cobra.NoArgs, runYouTubeLogin),
	)
	return cmd
}

func runSpotifyLogin(ctx context.Context, out io.Writer, a *app, _ []string) error {
	s, err := a.spotify(ctx, out)
	if err != nil {
		return err
	}
	name := s.me.DisplayName
	if name == "" {
		name = s.me.ID
	}
	fmt.Fprintf(out, i18n.T("Logged in to Spotify as %s (user id %s). The token stays in memory only.\n"), name, s.me.ID)
	return nil
}

func runYouTubeLogin(ctx context.Context, out io.Writer, a *app, _ []string) error {
	s, err := a.youtube(ctx, out)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, i18n.T("Logged in to YouTube. Playlists would be created on channel %q (id %s).\n"), s.channel.Title, s.channel.ID)
	return nil
}
