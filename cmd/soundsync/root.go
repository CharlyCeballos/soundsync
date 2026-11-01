package main

import (
	"github.com/CharlyCeballos/soundsync/internal/i18n"

	"github.com/spf13/cobra"
)

func newRootCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "soundsync",
		Short:        i18n.T("Copy Spotify playlists to YouTube"),
		Long:         i18n.T("Soundsync copies your Spotify playlists to YouTube playlists (visible in YouTube Music).\nTokens stay in memory and are never written to disk."),
		SilenceUsage: true,
	}
	// Default to the current value: the shell rebuilds this tree for every
	// line, and a fixed ".env" default would undo a custom --env-file.
	cmd.PersistentFlags().StringVar(&a.envFile, "env-file", a.envFile, i18n.T("path to the local .env file with your OAuth client IDs"))
	// Empty default: the language chosen earlier (system default, or a
	// previous --lang in the shell) stays until --lang says otherwise.
	var lang string
	cmd.PersistentFlags().StringVar(&lang, "lang", "", i18n.T("language: es or en (default: the system's, else es)"))
	cmd.PersistentPreRunE = func(*cobra.Command, []string) error {
		if lang == "" {
			return nil
		}
		return i18n.Set(lang)
	}
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
