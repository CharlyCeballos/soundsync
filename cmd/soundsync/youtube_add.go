package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"text/tabwriter"

	"github.com/CharlyCeballos/soundsync/internal/i18n"
	"github.com/CharlyCeballos/soundsync/internal/match"
	"github.com/CharlyCeballos/soundsync/internal/session"
	"github.com/CharlyCeballos/soundsync/internal/spotify"
	"github.com/CharlyCeballos/soundsync/internal/transfer"
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

// addJob is one Spotify playlist being copied into its YouTube playlist.
type addJob struct {
	destination
	tracks   []spotify.Entry // searchable entries, limited
	skipped  []spotify.Entry // episodes and unavailable entries
	complete bool            // tracks covers the whole playlist
	capped   bool            // more than maxPlaylistItems tracks: the rest are left out

	existing []youtube.Item           // the destination's videos (none when it is created)
	known    map[int]transfer.Matched // tracks matched without a search, by position
	matched  []transfer.Matched
	plan     transfer.Plan
	waiting  int                // tracks not searched because the searches ran out
	leftOut  []transfer.Matched // matches dropped because the playlist would pass maxPlaylistItems
	ready    bool               // the destination exists and carries the marker
	added    int
	removed  int
	err      error
}

// hasWrites reports whether the job changes anything on YouTube.
func (j *addJob) hasWrites() bool {
	return j.kind != destExisting || len(j.plan.ToAdd) > 0 || len(j.plan.Extra) > 0
}

// cost is the quota the job's writes need.
func (j *addJob) cost() int {
	return j.destCost() + youtube.WriteUnits*(len(j.plan.ToAdd)+len(j.plan.Extra))
}

// actions describes the job's writes for the confirmation.
func (j *addJob) actions(privacy string) string {
	var parts []string
	if s := j.destAction(privacy); s != "" {
		parts = append(parts, s)
	}
	if n := len(j.plan.ToAdd); n > 0 {
		parts = append(parts, fmt.Sprintf(i18n.T("add %d video(s)"), n))
	}
	if n := len(j.plan.Extra); n > 0 {
		parts = append(parts, fmt.Sprintf(i18n.T("remove %d video(s)"), n))
	}
	return strings.Join(parts, ", ")
}

// title names the destination in progress lines and the summary.
func (j *addJob) title() string {
	if j.target.Title != "" {
		return j.target.Title
	}
	return j.src.Name
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
	runJobs(ctx, jobs, func(ctx context.Context, j *addJob) error { return j.lookUp(ctx, yt) })
	budget := &searchBudget{}
	budget.left.Store(int64(opts.maxSearches) - yt.Client.SearchesUsed())
	printSearchCount(out, jobs, budget.left.Load())
	runJobs(ctx, jobs, func(ctx context.Context, j *addJob) error { return j.match(ctx, yt, budget) })
	if budget.googleSaidNo.Load() {
		fmt.Fprintln(out, i18n.T("\nYouTube says today's searches are used up (they reset at midnight Pacific time). What was found so far can still be written."))
	}
	for _, j := range jobs {
		fmt.Fprintf(out, i18n.T("\n%q\n"), j.src.Name)
		printPlan(out, j)
		if j.err != nil {
			fmt.Fprintf(out, i18n.T("  stopped: %v\n"), firstLine(j.err))
		}
	}
	if err := firstErr(jobs); err != nil {
		printAddSummary(out, yt.Client, jobs)
		return err
	}
	return confirmAndWrite(ctx, out, a, yt, jobs, opts)
}

// confirmAndWrite shows every write, asks once (Policies III.C.2) and runs
// the writes concurrently across playlists.
func confirmAndWrite(ctx context.Context, out io.Writer, a *app, yt *session.YouTube, jobs []*addJob, opts *addOptions) error {
	units := 0
	var changing []*addJob
	for _, j := range jobs {
		if j.hasWrites() {
			units += j.cost()
			changing = append(changing, j)
		}
	}
	if len(changing) == 0 {
		fmt.Fprintln(out, i18n.T("\nNothing to write."))
		printAddSummary(out, yt.Client, jobs)
		return nil
	}

	fmt.Fprintf(out, i18n.T("\nAbout to change %d playlist(s) on channel %q: %d units (used so far: %d of %d).\n"),
		len(changing), yt.Channel.Title, units, yt.Client.UnitsUsed(), youtube.UnitsPerDay)
	for _, j := range changing {
		fmt.Fprintf(out, i18n.T("  %q: %s\n"), j.src.Name, j.actions(opts.privacy))
	}
	if !opts.yes && !a.confirm(out, "Go ahead?") {
		fmt.Fprintln(out, i18n.T("Cancelled. Nothing was changed."))
		printAddSummary(out, yt.Client, jobs)
		return nil
	}

	lw := &lockedWriter{w: out}
	runJobs(ctx, changing, func(ctx context.Context, j *addJob) error { return j.write(ctx, lw, yt.Client, opts.privacy) })
	fmt.Fprintln(out)
	printAddSummary(out, yt.Client, jobs)
	return firstErr(jobs)
}

// prepareAddJobs chooses each destination and reads the Spotify tracks,
// before any search.
func prepareAddJobs(ctx context.Context, out io.Writer, a *app, sp *session.Spotify, yt *session.YouTube, srcs []spotify.Playlist, opts *addOptions) ([]*addJob, error) {
	playlists, err := yt.Client.MyPlaylists(ctx)
	if err != nil {
		return nil, err
	}
	dests, err := chooseDestinations(out, a, playlists, srcs, opts.yes)
	if err != nil {
		return nil, err
	}

	jobs := make([]*addJob, len(dests))
	for i, d := range dests {
		j := &addJob{destination: d, ready: d.kind == destExisting}
		entries, err := sp.Client.PlaylistItems(ctx, d.src.ID)
		if err != nil {
			return nil, fmt.Errorf(i18n.T("%q: %w"), d.src.Name, err)
		}
		for _, e := range entries {
			if e.Skip != "" {
				j.skipped = append(j.skipped, e)
				continue
			}
			j.tracks = append(j.tracks, e)
		}
		if len(j.tracks) > maxPlaylistItems {
			j.tracks, j.capped = j.tracks[:maxPlaylistItems], true
		}
		j.complete = !j.capped
		if opts.limit > 0 && len(j.tracks) > opts.limit {
			j.tracks, j.complete = j.tracks[:opts.limit], false
		}
		jobs[i] = j
	}
	return jobs, nil
}

// printDestinations says where each Spotify playlist goes.
func printDestinations(out io.Writer, yt *session.YouTube, jobs []*addJob, limit int) {
	fmt.Fprintf(out, i18n.T("\nSpotify → YouTube on channel %q:\n"), yt.Channel.Title)
	for _, j := range jobs {
		dest := i18n.T("a new playlist")
		if j.kind != destCreate {
			dest = fmt.Sprintf(i18n.T("%q (%d videos)"), j.target.Title, j.target.ItemCount)
		}
		fmt.Fprintf(out, i18n.T("  %q → %s: %d track(s) to match\n"), j.src.Name, dest, len(j.tracks))
		switch {
		case j.capped:
			fmt.Fprintf(out, i18n.T("    note: only the first %d tracks: a YouTube playlist holds at most %d videos.\n"), maxPlaylistItems, maxPlaylistItems)
		case !j.complete:
			fmt.Fprintf(out, i18n.T("    note: only the first %d tracks (--limit).\n"), limit)
		}
	}
}

// printSearchCount tells, before searching, exactly how many of the daily
// searches the run spends.
func printSearchCount(out io.Writer, jobs []*addJob, budgetLeft int64) {
	known, searches, running := 0, 0, 0
	for _, j := range jobs {
		if j.err != nil {
			continue
		}
		running++
		known += len(j.known)
		searches += len(j.tracks) - len(j.known)
	}
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

// searchBudget is shared by the jobs of one run: the searches left before
// the run stops searching and writes what it found.
type searchBudget struct {
	left         atomic.Int64
	googleSaidNo atomic.Bool // YouTube answered quotaExceeded to a search
}

// take reserves one search; false means none are left.
func (b *searchBudget) take() bool { return b.left.Add(-1) >= 0 }

// exhaust ends searching for every job of the run after YouTube refused one.
func (b *searchBudget) exhaust() {
	b.left.Store(0)
	b.googleSaidNo.Store(true)
}

// runJobs runs fn for every job still running, concurrently across
// playlists. A quota error cancels the others: they would fail the same way.
func runJobs(parent context.Context, jobs []*addJob, fn func(context.Context, *addJob) error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	var wg sync.WaitGroup
	for _, j := range jobs {
		if j.err != nil {
			continue
		}
		wg.Add(1)
		go func(j *addJob) {
			defer wg.Done()
			if err := fn(ctx, j); err != nil {
				j.err = stopReason(parent, err, cancel)
			}
		}(j)
	}
	wg.Wait()
}

// lookUp reads the destination's videos and finds the tracks that need no
// search: those already in the playlist with the same title, artist and
// length (so a run on another day does not search them again), then those
// matched earlier in this session (todo 2.16).
func (j *addJob) lookUp(ctx context.Context, yt *session.YouTube) error {
	yc := yt.Client
	j.known = map[int]transfer.Matched{}
	if j.kind != destCreate {
		items, err := yc.PlaylistItems(ctx, j.target.ID)
		if err != nil {
			return err
		}
		j.existing = items
		if ids := transfer.CandidateVideos(j.tracks, items); len(ids) > 0 {
			lengths, err := yc.VideoLengths(ctx, ids)
			if err != nil {
				return err
			}
			j.known = transfer.PreMatch(j.tracks, items, lengths)
		}
	}
	for _, e := range j.tracks {
		if _, ok := j.known[e.Position]; ok {
			continue
		}
		if m, ok := yt.Pick(match.Query(e.Track)); ok {
			m.Entry = e
			j.known[e.Position] = m
		}
	}
	return nil
}

// match searches the remaining tracks and builds the plan. When the
// searches run out the plan covers what was found and the rest waits for
// the next run; on an error the plan covers what was matched before it, so
// the stop can be reported.
func (j *addJob) match(ctx context.Context, yt *session.YouTube, budget *searchBudget) error {
	var err error
	j.matched, j.waiting, err = searchAll(ctx, yt, j.tracks, j.known, budget)
	// An adopted playlist keeps its own videos this time: adopting combines
	// (planV1 §5); later runs remove what is not in Spotify.
	judgeExtras := j.complete && err == nil && j.waiting == 0 && j.kind != destAdopt
	j.plan = transfer.BuildPlan(j.matched, j.existing, judgeExtras)

	// shortcut: extras count as already removed, but write() inserts before
	// it deletes; at exactly the cap YouTube refuses the insert and the run
	// stops cleanly. Delete first if anyone gets that close.
	room := max(maxPlaylistItems-(len(j.existing)-len(j.plan.Extra)), 0)
	if len(j.plan.ToAdd) > room {
		j.leftOut = j.plan.ToAdd[room:]
		j.plan.ToAdd = j.plan.ToAdd[:room]
	}
	return err
}

// write gets the destination ready, appends the missing videos in Spotify
// order (no position: each insert appends), then removes the videos that
// are no longer in Spotify.
func (j *addJob) write(ctx context.Context, out io.Writer, yc *youtube.Client, privacy string) error {
	if !j.ready {
		if err := ensureDestination(ctx, yc, &j.destination, privacy); err != nil {
			return err
		}
		j.ready = true
		if j.kind == destCreate {
			fmt.Fprintf(out, i18n.T("  created [%s] (%s) %s\n"), j.target.Title, j.target.Privacy, j.target.MusicURL())
		} else {
			fmt.Fprintf(out, i18n.T("  marked [%s] as a Soundsync playlist\n"), j.target.Title)
		}
	}
	for i, m := range j.plan.ToAdd {
		if err := yc.AddVideo(ctx, j.target.ID, m.Video.ID); err != nil {
			return err
		}
		j.added++
		fmt.Fprintf(out, i18n.T("  + [%s] %d/%d %s\n"), j.target.Title, i+1, len(j.plan.ToAdd), m.Video.Title)
	}
	for i, it := range j.plan.Extra {
		if err := yc.DeleteItem(ctx, it.ID); err != nil && !isNotFound(err) {
			return err
		}
		j.removed++ // a 404 means someone removed it already: same outcome
		fmt.Fprintf(out, i18n.T("  - [%s] %d/%d %s\n"), j.target.Title, i+1, len(j.plan.Extra), it.Title)
	}
	return nil
}

// searchAll matches tracks in Spotify order, taking known ones as they are.
// Once the budget is spent (or YouTube refuses a search for quota), the
// remaining unknown tracks count as waiting, not failed. Any other error
// stops it, returning what it matched so far. Each found match is
// remembered for the session; a query with no result is not, so the next
// run tries it again (todo 2.6).
func searchAll(ctx context.Context, yt *session.YouTube, entries []spotify.Entry, known map[int]transfer.Matched, budget *searchBudget) (out []transfer.Matched, waiting int, err error) {
	for _, e := range entries {
		if m, ok := known[e.Position]; ok {
			out = append(out, m)
			continue
		}
		if !budget.take() {
			waiting++
			continue
		}
		q := match.Query(e.Track)
		results, err := yt.Client.SearchVideos(ctx, q, match.SearchResults)
		if isQuota(err) {
			budget.exhaust()
			waiting++
			continue
		}
		if err != nil {
			return out, waiting, err
		}
		v, reason, ok, err := pickBest(ctx, yt.Client, e.Track, results)
		if err != nil {
			return out, waiting, err
		}
		m := transfer.Matched{Entry: e, Video: v, Reason: reason, Found: ok}
		if ok {
			yt.Remember(q, m)
		}
		out = append(out, m)
	}
	return out, waiting, nil
}

// pickBest prefers the first artist's own Topic upload with the track's
// length anywhere in the results (todo 2.18), else the classic ranking
// over the top results only.
func pickBest(ctx context.Context, yc *youtube.Client, t spotify.Track, results []youtube.Video) (youtube.Video, match.Reason, bool, error) {
	if own := match.OwnTopics(t, results); len(own) > 0 {
		ids := make([]string, len(own))
		for i, v := range own {
			ids[i] = v.ID
		}
		lengths, err := yc.VideoLengths(ctx, ids)
		if err != nil {
			return youtube.Video{}, "", false, err
		}
		for _, v := range own {
			if match.SameLength(t.DurationMS, lengths[v.ID]) {
				return v, match.ReasonTopic, true, nil
			}
		}
	}
	v, reason, ok := match.Pick(results[:min(match.TopResults, len(results))])
	return v, reason, ok, nil
}

// printPlan shows one row per track, then the videos that would be removed.
func printPlan(out io.Writer, j *addJob) {
	plan := j.plan
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
	for _, m := range j.matched {
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

	if j.waiting > 0 {
		fmt.Fprintf(out, i18n.T("  %d track(s) not searched yet: no searches left; the next run picks them up.\n"), j.waiting)
	}
	if len(j.leftOut) > 0 {
		fmt.Fprintf(out, i18n.T("  %d match(es) left out: the playlist would pass YouTube's %d-video limit.\n"), len(j.leftOut), maxPlaylistItems)
	}
	if len(plan.Extra) > 0 {
		fmt.Fprintf(out, i18n.T("  To remove from %q (no Spotify track matches them):\n"), j.title())
		for _, it := range plan.Extra {
			fmt.Fprintf(out, i18n.T("    - %s %s\n"), it.Title, it.URL())
		}
	}
}

func printAddSummary(out io.Writer, yc *youtube.Client, jobs []*addJob) {
	fmt.Fprintln(out, i18n.T("Summary:"))
	for _, j := range jobs {
		p := j.plan
		state := ""
		switch {
		case j.err != nil:
			state = i18n.T(" (stopped)")
		case j.ready && j.added == len(p.ToAdd) && p.ExtraKnown && j.removed == len(p.Extra):
			// Everything in S is now in Y and Y has nothing extra.
			state = i18n.T(" (in sync)")
		}
		fmt.Fprintf(out, i18n.T("  %q%s: added %d, already there %d, removed %d, kept yours %d, no match %d, same video %d, not searchable %d.\n"),
			j.title(), state, j.added, len(p.Present), j.removed, len(p.Replaced), len(p.Unmatched), len(p.SameVideo), len(j.skipped))
		if j.waiting > 0 {
			fmt.Fprintf(out, i18n.T("    %d track(s) wait for the next run (no searches left).\n"), j.waiting)
		}
		if j.kind == destAdopt && j.ready {
			fmt.Fprintln(out, i18n.T("    Its other videos were kept. Later runs remove the ones no Spotify track matches, after asking."))
		}
		if j.ready && (j.added > 0 || j.removed > 0 || j.kind != destExisting) {
			fmt.Fprintf(out, i18n.T("    %s\n"), j.target.MusicURL())
		}
	}
	fmt.Fprintf(out, i18n.T("Quota used this session: %d of %d units, %d of %d searches.\n"),
		yc.UnitsUsed(), youtube.UnitsPerDay, yc.SearchesUsed(), youtube.SearchCallsPerDay)
	fmt.Fprintln(out, i18n.T("Data from Spotify. Data from YouTube."))
}

// errSiblingQuota marks a job cancelled because another one hit the quota.
var errSiblingQuota = i18n.Error("stopped because another playlist reached the daily YouTube quota")

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
				sibling = fmt.Errorf(i18n.T("%q: %w"), j.src.Name, j.err)
			}
		default:
			return fmt.Errorf(i18n.T("%q: %w"), j.src.Name, j.err)
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

func isNotFound(err error) bool {
	var apiErr *youtube.APIError
	return errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound
}

func explainQuota(err error) error {
	if isQuota(err) {
		return fmt.Errorf(i18n.T("%w\nhint: the daily YouTube quota is used up; it resets at midnight Pacific time. Run the same command again then: it only adds what is missing"), err)
	}
	return err
}
