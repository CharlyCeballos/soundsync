package main

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"text/tabwriter"

	"github.com/CharlyCeballos/soundsync/internal/config"
	"github.com/CharlyCeballos/soundsync/internal/i18n"

	"github.com/spf13/cobra"
)

func newConfigCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: i18n.T("Inspect local configuration"),
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "check",
		Short: i18n.T("Validate the .env file without showing secret values"),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runConfigCheck(cmd.OutOrStdout(), a.envFile)
		},
	})
	return cmd
}

func runConfigCheck(out io.Writer, envFile string) error {
	loaded, err := config.LoadDotEnv(envFile)
	if err != nil {
		return err
	}
	source := envFile + " (loaded)"
	if !loaded {
		source = envFile + " (not found, using environment only)"
	}
	fmt.Fprintf(out, i18n.T("Config: %s\n\n"), source)

	cfg, cfgErr := config.FromEnv(os.Getenv)

	tw := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	// Client IDs and secrets are shown only as set/missing: never echo them.
	fmt.Fprintf(tw, i18n.T("  %s\t%s\n"), config.EnvSpotifyClientID, presence(cfg.SpotifyClientID))
	fmt.Fprintf(tw, i18n.T("  %s\t%s\n"), config.EnvSpotifyRedirectURI, uriOrInvalid(cfg.SpotifyRedirectURI))
	fmt.Fprintf(tw, i18n.T("  %s\t%s\n"), config.EnvGoogleClientID, presence(cfg.GoogleClientID))
	fmt.Fprintf(tw, i18n.T("  %s\t%s\n"), config.EnvGoogleClientSecret, presence(cfg.GoogleClientSecret))
	fmt.Fprintf(tw, i18n.T("  %s\t%s\n"), config.EnvGoogleRedirectURI, uriOrInvalid(cfg.GoogleRedirectURI))
	tw.Flush()
	fmt.Fprintln(out)

	if cfgErr != nil {
		fmt.Fprintln(out, i18n.T("Problems:"))
		for _, e := range unwrapAll(cfgErr) {
			fmt.Fprintf(out, i18n.T("  - %v\n"), e)
		}
		return errors.New(i18n.T("configuration is invalid"))
	}
	fmt.Fprintln(out, i18n.T("OK"))
	return nil
}

func presence(v string) string {
	if v == "" {
		return i18n.T("missing")
	}
	return i18n.T("set")
}

// uriOrInvalid shows a redirect URI; nil means validation failed and the
// problem is listed separately.
func uriOrInvalid(u *url.URL) string {
	if u == nil {
		return i18n.T("invalid")
	}
	return u.String()
}

func unwrapAll(err error) []error {
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		return joined.Unwrap()
	}
	return []error{err}
}
