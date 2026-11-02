package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/CharlyCeballos/soundsync/internal/copier"
	"github.com/CharlyCeballos/soundsync/internal/i18n"
	"github.com/CharlyCeballos/soundsync/internal/session"
	"github.com/CharlyCeballos/soundsync/internal/spotify"
	"github.com/CharlyCeballos/soundsync/internal/youtube"

	"github.com/spf13/cobra"
)

// maxPlaylistItems is YouTube's cap on videos per playlist (planV1 §6.3).
// A variable so tests can lower it.
var maxPlaylistItems = youtube.MaxPlaylistItems

type addOptions struct {
	limit       int // 0 = every track
	maxSearches int
	yes         bool
	privacy     string
	report      string // file to save the run's report to; "" = none
}

func newYouTubeAddCmd(a *app) *cobra.Command {
	opts := &addOptions{}
	cmd := command(a, "add <spotify playlist>...", nil,
		"Copy Spotify playlists to YouTube: create or reuse each one, add what is missing, remove what is gone; several run in parallel",
		cobra.MinimumNArgs(1),
		func(ctx context.Context, out io.Writer, a *app, args []string) error {
			return runYouTubeAdd(ctx, out, a, args, opts)
		})
	cmd.Flags().IntVar(&opts.limit, "limit", 0, i18n.T("only look at the first N tracks of each playlist (0 = all)"))
	cmd.Flags().IntVar(&opts.maxSearches, "max-searches", youtube.SearchCallsPerDay, i18n.T("stop searching after N searches in this session; tracks left wait for the next run"))
	cmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, i18n.T("write without asking for confirmation"))
	cmd.Flags().StringVar(&opts.privacy, "privacy", youtube.PrivacyPrivate, i18n.T("privacy of playlists that get created: private, unlisted or public"))
	cmd.Flags().StringVar(&opts.report, "report", "", i18n.T("save a report of this run to a new .csv or .json file"))
	return cmd
}

// validate checks the options before any login, so a typo costs nothing.
func (o *addOptions) validate() error {
	switch {
	case o.limit < 0:
		return fmt.Errorf(i18n.T("--limit must be 0 (all tracks) or more, got %d"), o.limit)
	case o.maxSearches < 0:
		return fmt.Errorf(i18n.T("--max-searches must be 0 or more, got %d"), o.maxSearches)
	case !youtube.ValidPrivacy(o.privacy):
		return fmt.Errorf(i18n.T("invalid --privacy %q (want private, unlisted or public)"), o.privacy)
	case o.report != "":
		return checkReportPath(o.report)
	}
	return nil
}

// actions describes the job's writes for the confirmation.
func actions(j *copier.Job, privacy string) string {
	var parts []string
	if s := destAction(j.Destination, privacy); s != "" {
		parts = append(parts, s)
	}
	if n := len(j.Plan.ToAdd); n > 0 {
		parts = append(parts, fmt.Sprintf(i18n.T("add %d video(s)"), n))
	}
	if n := len(j.Plan.Extra); n > 0 {
		parts = append(parts, fmt.Sprintf(i18n.T("remove %d video(s)"), n))
	}
	return strings.Join(parts, ", ")
}

func runYouTubeAdd(ctx context.Context, out io.Writer, a *app, args []string, opts *addOptions) (err error) {
	if err := opts.validate(); err != nil {
		return err
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
	jobs, err := prepareAddJobs(ctx, out, a, sp, yt, srcs, opts)
	if err != nil {
		return err
	}
	if opts.report != "" {
		// Saved whatever happens next: a stopped or cancelled run is worth a report too.
		defer func() { err = errors.Join(err, saveReport(out, opts.report, jobs)) }()
	}

	printDestinations(out, yt, jobs, opts.limit)
	copier.LookUp(ctx, yt, jobs, nil)
	budget := copier.NewBudget(opts.maxSearches, yt.Client.SearchesUsed())
	printSearchCount(out, jobs, budget.Left())
	copier.Match(ctx, yt, jobs, budget, nil)
	if budget.GoogleSaidNo() {
		fmt.Fprintln(out, i18n.T("\nYouTube says today's searches are used up (they reset at midnight Pacific time). What was found so far can still be written."))
	}
	for _, j := range jobs {
		fmt.Fprintf(out, i18n.T("\n%q\n"), j.Source.Name)
		printPlan(out, j)
		if j.Err != nil {
			fmt.Fprintf(out, i18n.T("  stopped: %v\n"), firstLine(j.Err))
		}
	}
	if err := copier.FirstErr(jobs); err != nil {
		printAddSummary(out, yt.Client, jobs)
		return err
	}
	return confirmAndWrite(ctx, out, a, yt, jobs, opts)
}

// confirmAndWrite shows every write, asks once (Policies III.C.2) and runs
// the writes concurrently across playlists.
func confirmAndWrite(ctx context.Context, out io.Writer, a *app, yt *session.YouTube, jobs []*copier.Job, opts *addOptions) error {
	changing, units := copier.Changing(jobs)
	if len(changing) == 0 {
		fmt.Fprintln(out, i18n.T("\nNothing to write."))
		printAddSummary(out, yt.Client, jobs)
		return nil
	}

	fmt.Fprintf(out, i18n.T("\nAbout to change %d playlist(s) on channel %q: %d units (used so far: %d of %d).\n"),
		len(changing), yt.Channel.Title, units, yt.Client.UnitsUsed(), youtube.UnitsPerDay)
	for _, j := range changing {
		fmt.Fprintf(out, i18n.T("  %q: %s\n"), j.Source.Name, actions(j, opts.privacy))
	}
	if !opts.yes && !a.confirm(out, "Go ahead?") {
		fmt.Fprintln(out, i18n.T("Cancelled. Nothing was changed."))
		printAddSummary(out, yt.Client, jobs)
		return nil
	}

	copier.Write(ctx, yt, changing, opts.privacy, printWrites(&lockedWriter{w: out}))
	fmt.Fprintln(out)
	printAddSummary(out, yt.Client, jobs)
	return copier.FirstErr(jobs)
}

// printWrites prints one line per write as it happens.
func printWrites(out io.Writer) copier.Observer {
	return func(e copier.Event) {
		t := e.Target
		switch e.Step {
		case copier.StepCreate:
			fmt.Fprintf(out, i18n.T("  created [%s] (%s) %s\n"), t.Title, t.Privacy, t.MusicURL())
		case copier.StepMark:
			fmt.Fprintf(out, i18n.T("  marked [%s] as a Soundsync playlist\n"), t.Title)
		case copier.StepAdd:
			fmt.Fprintf(out, i18n.T("  + [%s] %d/%d %s\n"), t.Title, e.Done, e.Total, e.Title)
		case copier.StepRemove:
			fmt.Fprintf(out, i18n.T("  - [%s] %d/%d %s\n"), t.Title, e.Done, e.Total, e.Title)
		}
	}
}

// prepareAddJobs chooses each destination and reads the Spotify tracks,
// before any search.
func prepareAddJobs(ctx context.Context, out io.Writer, a *app, sp *session.Spotify, yt *session.YouTube, srcs []spotify.Playlist, opts *addOptions) ([]*copier.Job, error) {
	playlists, err := yt.Client.MyPlaylists(ctx)
	if err != nil {
		return nil, err
	}
	dests, err := chooseDestinations(out, a, playlists, srcs, opts.yes)
	if err != nil {
		return nil, err
	}
	return copier.Prepare(ctx, sp.Client, dests, copier.Options{Limit: opts.limit, MaxItems: maxPlaylistItems})
}

// printDestinations says where each Spotify playlist goes.
func printDestinations(out io.Writer, yt *session.YouTube, jobs []*copier.Job, limit int) {
	fmt.Fprintf(out, i18n.T("\nSpotify → YouTube on channel %q:\n"), yt.Channel.Title)
	for _, j := range jobs {
		dest := i18n.T("a new playlist")
		if j.Kind != copier.Create {
			dest = fmt.Sprintf(i18n.T("%q (%d videos)"), j.Target.Title, j.Target.ItemCount)
		}
		fmt.Fprintf(out, i18n.T("  %q → %s: %d track(s) to match\n"), j.Source.Name, dest, len(j.Tracks))
		switch {
		case j.Capped:
			fmt.Fprintf(out, i18n.T("    note: only the first %d tracks: a YouTube playlist holds at most %d videos.\n"), maxPlaylistItems, maxPlaylistItems)
		case !j.Complete:
			fmt.Fprintf(out, i18n.T("    note: only the first %d tracks (--limit).\n"), limit)
		}
	}
}

// printSearchCount tells, before searching, exactly how many of the daily
// searches the run spends.
func printSearchCount(out io.Writer, jobs []*copier.Job, budgetLeft int64) {
	known, searches, running := copier.Searches(jobs)
	if known > 0 {
		fmt.Fprintf(out, i18n.T("%d track(s) need no search: already in the YouTube playlist, or matched earlier in this session.\n"), known)
	}
	if searches == 0 {
		return
	}
	how := ""
	if running > 1 {
		how = i18n.T(" in parallel")
	}
	if budgetLeft <= 0 {
		fmt.Fprintf(out, i18n.T("No searches left in this session (--max-searches): %d track(s) wait for the next run.\n"), searches)
		return
	}
	if int64(searches) > budgetLeft {
		fmt.Fprintf(out, i18n.T("Searching %d of %d track(s)%s: %d search(es) left in this session (--max-searches); the rest wait for the next run.\n"),
			budgetLeft, searches, how, budgetLeft)
		return
	}
	fmt.Fprintf(out, i18n.T("Searching %d track(s)%s: %d of the %d daily searches.\n"), searches, how, searches, youtube.SearchCallsPerDay)
}

// printPlan shows one row per track, then the videos that would be removed.
func printPlan(out io.Writer, j *copier.Job) {
	plan := j.Plan
	status := map[int]string{}
	for _, m := range plan.ToAdd {
		status[m.Entry.Position] = i18n.T("add")
	}
	for _, m := range plan.Present {
		status[m.Entry.Position] = i18n.T("already in playlist")
	}
	for _, m := range plan.SameVideo {
		status[m.Entry.Position] = i18n.T("same video as an earlier track")
	}
	replaced := map[int]youtube.Item{}
	for _, r := range plan.Replaced {
		replaced[r.Entry.Position] = r.Item
	}

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, i18n.T("  #\tSPOTIFY\tYOUTUBE\tWHY\tACTION"))
	for _, m := range j.Matched {
		t := m.Entry.Track
		left := strings.Join(t.Artists, ", ") + " — " + t.Name
		if it, ok := replaced[m.Entry.Position]; ok {
			fmt.Fprintf(tw, i18n.T("  %d\t%s\t%s %s\tyours\tkeep your video (Soundsync found %s)\n"), m.Entry.Position, left, it.Title, it.URL(), m.Video.URL())
			continue
		}
		if !m.Found {
			fmt.Fprintf(tw, i18n.T("  %d\t%s\t(no result)\t-\tskip\n"), m.Entry.Position, left)
			continue
		}
		right := fmt.Sprintf(i18n.T("%s [%s] %s"), m.Video.Title, m.Video.ChannelTitle, m.Video.URL())
		fmt.Fprintf(tw, i18n.T("  %d\t%s\t%s\t%s\t%s\n"), m.Entry.Position, left, right, i18n.T(string(m.Reason)), status[m.Entry.Position])
	}
	tw.Flush()

	if j.Waiting > 0 {
		fmt.Fprintf(out, i18n.T("  %d track(s) not searched yet: no searches left; the next run picks them up.\n"), j.Waiting)
	}
	if len(j.LeftOut) > 0 {
		fmt.Fprintf(out, i18n.T("  %d match(es) left out: the playlist would pass YouTube's %d-video limit.\n"), len(j.LeftOut), maxPlaylistItems)
	}
	if len(plan.Extra) > 0 {
		fmt.Fprintf(out, i18n.T("  To remove from %q (no Spotify track matches them):\n"), j.Title())
		for _, it := range plan.Extra {
			fmt.Fprintf(out, i18n.T("    - %s %s\n"), it.Title, it.URL())
		}
	}
}

func printAddSummary(out io.Writer, yc *youtube.Client, jobs []*copier.Job) {
	fmt.Fprintln(out, i18n.T("Summary:"))
	for _, j := range jobs {
		p := j.Plan
		state := ""
		switch {
		case j.Err != nil:
			state = i18n.T(" (stopped)")
		case j.InSync():
			// Everything in S is now in Y and Y has nothing extra.
			state = i18n.T(" (in sync)")
		}
		fmt.Fprintf(out, i18n.T("  %q%s: added %d, already there %d, removed %d, kept yours %d, no match %d, same video %d, not searchable %d.\n"),
			j.Title(), state, j.Added, len(p.Present), j.Removed, len(p.Replaced), len(p.Unmatched), len(p.SameVideo), len(j.Skipped))
		if j.Waiting > 0 {
			fmt.Fprintf(out, i18n.T("    %d track(s) wait for the next run (no searches left).\n"), j.Waiting)
		}
		if j.Kind == copier.Adopt && j.Ready {
			fmt.Fprintln(out, i18n.T("    Its other videos were kept. Later runs remove the ones no Spotify track matches, after asking."))
		}
		if j.Ready && (j.Added > 0 || j.Removed > 0 || j.Kind != copier.Existing) {
			fmt.Fprintf(out, i18n.T("    %s\n"), j.Target.MusicURL())
		}
	}
	fmt.Fprintf(out, i18n.T("Quota used this session: %d of %d units, %d of %d searches.\n"),
		yc.UnitsUsed(), youtube.UnitsPerDay, yc.SearchesUsed(), youtube.SearchCallsPerDay)
	fmt.Fprintln(out, i18n.T("Data from Spotify. Data from YouTube."))
}

func firstLine(err error) string {
	s := err.Error()
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
