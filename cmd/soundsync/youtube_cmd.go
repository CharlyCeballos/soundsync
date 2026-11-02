package main

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/CharlyCeballos/soundsync/internal/copier"
	"github.com/CharlyCeballos/soundsync/internal/i18n"
	"github.com/CharlyCeballos/soundsync/internal/transfer"
	"github.com/CharlyCeballos/soundsync/internal/youtube"

	"github.com/spf13/cobra"
)

// youtubeAttribution makes YouTube's role as the source clear (Policies III.F.2).
const youtubeAttribution = "Data from YouTube."

func newYouTubeCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "youtube",
		Short: i18n.T("Read and create playlists on your YouTube channel (token revoked on exit)"),
	}
	cmd.AddCommand(
		command(a, "playlists", nil, "List the playlists on your YouTube channel", cobra.NoArgs, runYouTubePlaylists),
		newYouTubeCreateCmd(a),
		newYouTubeAddCmd(a),
	)
	return cmd
}

func runYouTubePlaylists(ctx context.Context, out io.Writer, a *app, _ []string) error {
	yt, err := a.youtube(ctx, out)
	if err != nil {
		return err
	}
	playlists, err := yt.Client.MyPlaylists(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, i18n.T("\nPlaylists on channel %q: %d\n\n"), yt.Channel.Title, len(playlists))
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, i18n.T("  #\tTITLE\tVIDEOS\tPRIVACY\tSOUNDSYNC\tLINK"))
	for i, p := range playlists {
		source := "-"
		if ids := transfer.MarkerIDs(p.Description); len(ids) > 0 {
			source = "spotify:" + ids[0]
		}
		fmt.Fprintf(tw, i18n.T("  %d\t%s\t%d\t%s\t%s\t%s\n"), i+1, p.Title, p.ItemCount, p.Privacy, source, p.URL())
	}
	tw.Flush()
	fmt.Fprintf(out, i18n.T("\n%s Quota used this session: %d units.\n"), youtubeAttribution, yt.Client.UnitsUsed())
	return nil
}

type createOptions struct {
	privacy string
	yes     bool
}

func newYouTubeCreateCmd(a *app) *cobra.Command {
	opts := &createOptions{}
	cmd := command(a, "create <spotify playlist>...", nil,
		"Create empty YouTube playlists for Spotify playlists (reuses those Soundsync already created)",
		cobra.MinimumNArgs(1),
		func(ctx context.Context, out io.Writer, a *app, args []string) error {
			return runYouTubeCreate(ctx, out, a, args, opts)
		})
	cmd.Flags().StringVar(&opts.privacy, "privacy", youtube.PrivacyPrivate, i18n.T("privacy of new playlists: private, unlisted or public"))
	cmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, i18n.T("create without asking for confirmation"))
	return cmd
}

func runYouTubeCreate(ctx context.Context, out io.Writer, a *app, args []string, opts *createOptions) error {
	// Validate before any login so a typo costs nothing.
	if !youtube.ValidPrivacy(opts.privacy) {
		return fmt.Errorf(i18n.T("invalid --privacy %q (want private, unlisted or public)"), opts.privacy)
	}

	sp, err := a.spotify(ctx, out)
	if err != nil {
		return err
	}
	srcs, err := resolveSources(ctx, sp, args)
	if err != nil {
		return err
	}

	yt, err := a.youtube(ctx, out)
	if err != nil {
		return err
	}
	existing, err := yt.Client.MyPlaylists(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, i18n.T("\nYouTube channel: %q\n"), yt.Channel.Title)
	dests, err := chooseDestinations(out, a, existing, srcs, opts.yes)
	if err != nil {
		return err
	}
	var todo []*copier.Destination
	for i := range dests {
		d := &dests[i]
		fmt.Fprintf(out, i18n.T("\nSpotify playlist: %q (%s)\n"), d.Source.Name, d.Source.URL())
		switch d.Kind {
		case copier.Existing:
			p := d.Target
			fmt.Fprintf(out, i18n.T("Already on YouTube as %q (%s, %d videos).\n  %s\n"), p.Title, p.Privacy, p.ItemCount, p.MusicURL())
			continue
		case copier.Adopt:
			fmt.Fprintf(out, i18n.T("Will use %q (%s, %d videos).\n"), d.Target.Title, d.Target.Privacy, d.Target.ItemCount)
		default:
			fmt.Fprintln(out, i18n.T("Will be created."))
		}
		todo = append(todo, d)
	}

	if len(todo) == 0 {
		fmt.Fprintln(out, i18n.T("\nNothing to create."))
		fmt.Fprintf(out, i18n.T("%s Quota used this session: %d units.\n"), youtubeAttribution, yt.Client.UnitsUsed())
		return nil
	}

	fmt.Fprintf(out, i18n.T("\nAbout to change %d playlist(s) on channel %q: %d units (used so far: %d).\n"),
		len(todo), yt.Channel.Title, youtube.WriteUnits*len(todo), yt.Client.UnitsUsed())
	for _, d := range todo {
		fmt.Fprintf(out, i18n.T("  %q: %s\n"), d.Source.Name, destAction(*d, opts.privacy))
	}
	if !opts.yes && !a.confirm(out, "Go ahead?") {
		fmt.Fprintln(out, i18n.T("Cancelled. Nothing was changed."))
		return nil
	}

	for _, d := range todo {
		if err := d.Ensure(ctx, yt.Client, opts.privacy); err != nil {
			return copier.ExplainQuota(err)
		}
		if d.Kind == copier.Create {
			fmt.Fprintf(out, i18n.T("\nCreated %q (%s).\n  %s\n  %s\n"), d.Target.Title, d.Target.Privacy, d.Target.URL(), d.Target.MusicURL())
		} else {
			fmt.Fprintf(out, i18n.T("\n%q now carries the Soundsync line; Soundsync will keep it in sync.\n  %s\n"), d.Target.Title, d.Target.MusicURL())
		}
	}
	fmt.Fprintf(out, i18n.T("%s Quota used this session: %d units.\n"), youtubeAttribution, yt.Client.UnitsUsed())
	return nil
}
