package main

import (
	"github.com/spf13/cobra"
)

func newRootCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "soundsync",
		Short:        "Copy Spotify playlists to YouTube",
		Long:         "Soundsync copies your Spotify playlists to YouTube playlists (visible in YouTube Music).\nTokens stay in memory and are never written to disk.",
		SilenceUsage: true,
	}
	// Default to the current value: the shell rebuilds this tree for every
	// line, and a fixed ".env" default would undo a custom --env-file.
	cmd.PersistentFlags().StringVar(&a.envFile, "env-file", a.envFile, "path to the local .env file with your OAuth client IDs")

	cmd.AddCommand(
		newVersionCmd(),
		newConfigCmd(a),
		newLoginCmd(a),
		newSpotifyCmd(a),
		newYouTubeCmd(a),
		newShellCmd(a),
	)
	return cmd
}
