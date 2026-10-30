package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"text/tabwriter"

	"soundsync/internal/match"
	"soundsync/internal/spotify"
	"soundsync/internal/transfer"
	"soundsync/internal/youtube"

	"github.com/spf13/cobra"
)

// defaultAddLimit keeps spike runs small (todo 1.6: at most 5 tracks).
const defaultAddLimit = 5

type addOptions struct {
	limit int
	yes   bool
}

func newYouTubeAddCmd(a *app) *cobra.Command {
	opts := &addOptions{}
	cmd := command(a, "add <spotify playlist>...", nil,
		"Find each track on YouTube and add the missing ones; several playlists run in parallel",
		cobra.MinimumNArgs(1),
		func(ctx context.Context, out io.Writer, a *app, args []string) error {
			return runYouTubeAdd(ctx, out, a, args, opts)
		})
	cmd.Flags().IntVar(&opts.limit, "limit", defaultAddLimit, "search at most this many tracks per playlist (each search uses 1 of the 100 daily searches)")
	cmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, "add without asking for confirmation")
	return cmd
}

// addJob is one Spotify playlist being copied into its YouTube playlist.
type addJob struct {
	src      spotify.Playlist
	target   youtube.Playlist
	tracks   []spotify.Entry // searchable entries, limited
	skipped  int             // episodes and unavailable entries
	complete bool            // tracks covers the whole playlist

	matched []transfer.Matched
	plan    transfer.Plan
	added   int
	err     error
}

func runYouTubeAdd(ctx context.Context, out io.Writer, a *app, args []string, opts *addOptions) error {
	if opts.limit < 1 {
		return fmt.Errorf("--limit must be at least 1, got %d", opts.limit)
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
	jobs, err := prepareAddJobs(ctx, sp, yt, srcs, opts.limit)
	if err != nil {
		return err
	}

	searches := 0
	fmt.Fprintf(out, "\nSpotify → YouTube on channel %q:\n", yt.channel.Title)
	for _, j := range jobs {
		fmt.Fprintf(out, "  %q → %q (%d videos): %d track(s) to search\n", j.src.Name, j.target.Title, j.target.ItemCount, len(j.tracks))
		if !j.complete {
			fmt.Fprintf(out, "    note: only the first %d tracks; use --limit to search more.\n", opts.limit)
		}
		searches += len(j.tracks)
	}
	how := ""
	if len(jobs) > 1 {
		how = " in parallel"
	}
	fmt.Fprintf(out, "Searching %d track(s)%s: %d of the %d daily searches.\n", searches, how, searches, youtube.SearchCallsPerDay)

	matchAll(ctx, yt.client, jobs)
	for _, j := range jobs {
		fmt.Fprintf(out, "\n%q\n", j.src.Name)
		printPlan(out, j.plan, j.matched)
		if j.err != nil {
			fmt.Fprintf(out, "  stopped: %v\n", firstLine(j.err))
		}
	}
	if err := firstErr(jobs); err != nil {
		printAddSummary(out, yt.client, jobs)
		return err
	}

	total := 0
	for _, j := range jobs {
		total += len(j.plan.ToAdd)
	}
	if total == 0 {
		fmt.Fprintln(out, "\nNothing to write.")
		printAddSummary(out, yt.client, jobs)
		return nil
	}

	fmt.Fprintf(out, "\nAbout to add %d video(s) on channel %q: %d units (used so far: %d of %d).\n",
		total, yt.channel.Title, 50*total, yt.client.UnitsUsed(), youtube.UnitsPerDay)
	for _, j := range jobs {
		if n := len(j.plan.ToAdd); n > 0 {
			fmt.Fprintf(out, "  %q: %d\n", j.target.Title, n)
		}
	}
	if !opts.yes && !a.confirm(out, "Add them?") {
		fmt.Fprintln(out, "Cancelled. Nothing was added.")
		printAddSummary(out, yt.client, jobs)
		return nil
	}

	insertAll(ctx, &lockedWriter{w: out}, yt.client, jobs)
	fmt.Fprintln(out)
	printAddSummary(out, yt.client, jobs)
	return firstErr(jobs)
}

// prepareAddJobs finds each destination and reads the Spotify tracks.
// Every missing destination is reported at once, before any search.
func prepareAddJobs(ctx context.Context, sp *spotifySession, yt *youtubeSession, srcs []spotify.Playlist, limit int) ([]*addJob, error) {
	playlists, err := yt.client.MyPlaylists(ctx)
	if err != nil {
		return nil, err
	}
	var jobs []*addJob
	var missing []string
	for _, src := range srcs {
		dest := transfer.ResolveDestination(playlists, src.ID, src.Name)
		if dest.Existing == nil {
			missing = append(missing, src.Name)
			continue
		}
		jobs = append(jobs, &addJob{src: src, target: *dest.Existing})
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("no YouTube playlist yet for %s; run: youtube create %s", quoteAll(missing), quoteAll(missing))
	}

	for _, j := range jobs {
		entries, err := sp.client.PlaylistItems(ctx, j.src.ID)
		if err != nil {
			return nil, fmt.Errorf("%q: %w", j.src.Name, err)
		}
		for _, e := range entries {
			if e.Skip != "" {
				j.skipped++
				continue
			}
			j.tracks = append(j.tracks, e)
		}
		j.complete = len(j.tracks) <= limit
		if !j.complete {
			j.tracks = j.tracks[:limit]
		}
	}
	return jobs, nil
}

// matchAll searches every job concurrently (tracks within a job in Spotify
// order) and builds each plan. A quota error cancels the other jobs: they
// would fail the same way.
func matchAll(parent context.Context, yc *youtube.Client, jobs []*addJob) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Add(1)
		go func(j *addJob) {
			defer wg.Done()
			existing, err := yc.PlaylistVideoIDs(ctx, j.target.ID)
			if err == nil {
				j.matched, err = searchAll(ctx, yc, j.tracks)
			}
			j.plan = transfer.BuildPlan(j.matched, existing, j.complete && err == nil)
			if err != nil {
				j.err = stopReason(parent, err, cancel)
			}
		}(j)
	}
	wg.Wait()
}

// insertAll adds each job's videos concurrently across playlists and in
// Spotify order within one playlist (no position: each insert appends).
func insertAll(parent context.Context, out io.Writer, yc *youtube.Client, jobs []*addJob) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	var wg sync.WaitGroup
	for _, j := range jobs {
		if len(j.plan.ToAdd) == 0 {
			continue
		}
		wg.Add(1)
		go func(j *addJob) {
			defer wg.Done()
			for i, m := range j.plan.ToAdd {
				if err := yc.AddVideo(ctx, j.target.ID, m.Video.ID); err != nil {
					j.err = stopReason(parent, err, cancel)
					return
				}
				j.added++
				fmt.Fprintf(out, "  + [%s] %d/%d %s\n", j.target.Title, i+1, len(j.plan.ToAdd), m.Video.Title)
			}
		}(j)
	}
	wg.Wait()
}

// searchAll matches tracks in Spotify order. It stops at the first error
// and returns what it matched so far, so a quota stop still gets reported.
func searchAll(ctx context.Context, yc *youtube.Client, entries []spotify.Entry) ([]transfer.Matched, error) {
	var out []transfer.Matched
	for _, e := range entries {
		results, err := yc.SearchVideos(ctx, match.Query(e.Track), match.SearchResults)
		if err != nil {
			return out, err
		}
		v, reason, ok := match.Pick(results)
		out = append(out, transfer.Matched{Entry: e, Video: v, Reason: reason, Found: ok})
	}
	return out, nil
}

func printPlan(out io.Writer, plan transfer.Plan, matched []transfer.Matched) {
	status := map[int]string{}
	for _, m := range plan.ToAdd {
		status[m.Entry.Position] = "add"
	}
	for _, m := range plan.Present {
		status[m.Entry.Position] = "already in playlist"
	}
	for _, m := range plan.SameVideo {
		status[m.Entry.Position] = "same video as an earlier track"
	}

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  #\tSPOTIFY\tYOUTUBE\tWHY\tACTION")
	for _, m := range matched {
		t := m.Entry.Track
		left := strings.Join(t.Artists, ", ") + " — " + t.Name
		if !m.Found {
			fmt.Fprintf(tw, "  %d\t%s\t(no result)\t-\tskip\n", m.Entry.Position, left)
			continue
		}
		right := fmt.Sprintf("%s [%s] %s", m.Video.Title, m.Video.ChannelTitle, m.Video.URL())
		fmt.Fprintf(tw, "  %d\t%s\t%s\t%s\t%s\n", m.Entry.Position, left, right, m.Reason, status[m.Entry.Position])
	}
	tw.Flush()
}

func printAddSummary(out io.Writer, yc *youtube.Client, jobs []*addJob) {
	fmt.Fprintln(out, "Summary:")
	for _, j := range jobs {
		p := j.plan
		state := ""
		switch {
		case j.err != nil:
			state = " (stopped)"
		case j.added == len(p.ToAdd) && p.ExtraKnown && len(p.Extra) == 0:
			// Everything in S is now in Y and Y has nothing extra.
			state = " (in sync)"
		}
		fmt.Fprintf(out, "  %q%s: added %d, already there %d, no match %d, same video %d, not searchable %d.\n",
			j.target.Title, state, j.added, len(p.Present), len(p.Unmatched), len(p.SameVideo), j.skipped)
		if p.ExtraKnown && len(p.Extra) > 0 {
			fmt.Fprintf(out, "    %d video(s) are no longer in Spotify; removing them comes in a later version.\n", len(p.Extra))
		}
		if j.added > 0 {
			fmt.Fprintf(out, "    %s\n", j.target.MusicURL())
		}
	}
	fmt.Fprintf(out, "Quota used this session: %d of %d units, %d of %d searches.\n",
		yc.UnitsUsed(), youtube.UnitsPerDay, yc.SearchesUsed(), youtube.SearchCallsPerDay)
	fmt.Fprintln(out, "Data from Spotify. Data from YouTube.")
}

// errSiblingQuota marks a job cancelled because another one hit the quota.
var errSiblingQuota = errors.New("stopped because another playlist reached the daily YouTube quota")

// stopReason explains why a job stopped. A quota error cancels the other
// jobs (they would fail the same way); they then report errSiblingQuota
// instead of a bare "context canceled".
func stopReason(parent context.Context, err error, cancelOthers context.CancelFunc) error {
	switch {
	case isQuota(err):
		cancelOthers()
		return explainQuota(err)
	case errors.Is(err, context.Canceled) && parent.Err() == nil:
		return errSiblingQuota
	}
	return err
}

// firstErr returns the root cause among the jobs' errors: a job cancelled
// because of another job's quota stop only matters if nothing else failed.
func firstErr(jobs []*addJob) error {
	var sibling error
	for _, j := range jobs {
		switch {
		case j.err == nil:
		case errors.Is(j.err, errSiblingQuota):
			if sibling == nil {
				sibling = fmt.Errorf("%q: %w", j.src.Name, j.err)
			}
		default:
			return fmt.Errorf("%q: %w", j.src.Name, j.err)
		}
	}
	return sibling
}

func firstLine(err error) string {
	s := err.Error()
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func isQuota(err error) bool {
	var apiErr *youtube.APIError
	return errors.As(err, &apiErr) && (apiErr.Reason == "quotaExceeded" || apiErr.Reason == "dailyLimitExceeded")
}

func explainQuota(err error) error {
	if isQuota(err) {
		return fmt.Errorf("%w\nhint: the daily YouTube quota is used up; it resets at midnight Pacific time. Run the same command again then: it only adds what is missing", err)
	}
	return err
}
