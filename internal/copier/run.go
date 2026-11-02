package copier

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/CharlyCeballos/soundsync/internal/i18n"
	"github.com/CharlyCeballos/soundsync/internal/match"
	"github.com/CharlyCeballos/soundsync/internal/session"
	"github.com/CharlyCeballos/soundsync/internal/spotify"
	"github.com/CharlyCeballos/soundsync/internal/transfer"
	"github.com/CharlyCeballos/soundsync/internal/youtube"
)

// Step names the progress an Event reports.
type Step string

const (
	StepLookUp Step = "lookup" // the destination's videos were read
	StepSearch Step = "search" // one more track has its match, or waits
	StepCreate Step = "create" // the destination was created
	StepMark   Step = "mark"   // an adopted destination got the marker
	StepAdd    Step = "add"    // a video was added
	StepRemove Step = "remove" // a video was removed
)

// Event is one piece of progress of one job. It carries values only, so an
// observer can keep it or hand it to another goroutine.
type Event struct {
	SourceID string // the Spotify playlist of the job
	Step     Step
	Done     int              // tracks or videos done so far in this step
	Total    int              // tracks or videos this step handles
	Title    string           // the video added or removed
	Target   youtube.Playlist // the destination as it is now
}

// Observer receives progress. Jobs run concurrently, so it may be called
// from several goroutines at once. A nil Observer is fine.
type Observer func(Event)

func (o Observer) emit(e Event) {
	if o != nil {
		o(e)
	}
}

// Budget is shared by the jobs of one run: the searches left before the run
// stops searching and writes what it found.
type Budget struct {
	left         atomic.Int64
	googleSaidNo atomic.Bool // YouTube answered quotaExceeded to a search
}

// NewBudget allows maxSearches searches per session, used of them spent.
func NewBudget(maxSearches int, used int64) *Budget {
	b := &Budget{}
	b.left.Store(int64(maxSearches) - used)
	return b
}

// Left is how many searches remain, never below zero.
func (b *Budget) Left() int64 { return max(b.left.Load(), 0) }

// GoogleSaidNo reports whether YouTube refused a search for quota.
func (b *Budget) GoogleSaidNo() bool { return b.googleSaidNo.Load() }

// take reserves one search; false means none are left.
func (b *Budget) take() bool { return b.left.Add(-1) >= 0 }

// exhaust ends searching for every job of the run after YouTube refused one.
func (b *Budget) exhaust() {
	b.left.Store(0)
	b.googleSaidNo.Store(true)
}

// LookUp reads each destination's videos and finds the tracks that need no
// search: those already in the playlist with the same title, artist and
// length (so a run on another day does not search them again), then those
// matched earlier in this session (todo 2.16).
func LookUp(ctx context.Context, yt *session.YouTube, jobs []*Job, obs Observer) {
	runJobs(ctx, jobs, func(ctx context.Context, j *Job) error {
		if err := j.lookUp(ctx, yt); err != nil {
			return err
		}
		obs.emit(Event{SourceID: j.Source.ID, Step: StepLookUp, Done: 1, Total: 1, Target: j.Target})
		return nil
	})
}

func (j *Job) lookUp(ctx context.Context, yt *session.YouTube) error {
	j.Known = map[int]transfer.Matched{}
	if j.Kind != Create {
		items, err := yt.Client.PlaylistItems(ctx, j.Target.ID)
		if err != nil {
			return err
		}
		j.Existing = items
		if ids := transfer.CandidateVideos(j.Tracks, items); len(ids) > 0 {
			lengths, err := yt.Client.VideoLengths(ctx, ids)
			if err != nil {
				return err
			}
			j.Known = transfer.PreMatch(j.Tracks, items, lengths)
		}
	}
	for _, e := range j.Tracks {
		if _, ok := j.Known[e.Position]; ok {
			continue
		}
		if m, ok := yt.Pick(match.Query(e.Track)); ok {
			m.Entry = e
			j.Known[e.Position] = m
		}
	}
	return nil
}

// Searches counts what Match will do: tracks already known, tracks still to
// search, and jobs still running.
func Searches(jobs []*Job) (known, searches, running int) {
	for _, j := range jobs {
		if j.Err != nil {
			continue
		}
		running++
		known += len(j.Known)
		searches += len(j.Tracks) - len(j.Known)
	}
	return known, searches, running
}

// Match searches the remaining tracks and builds each plan. When the
// searches run out the plan covers what was found and the rest waits for the
// next run; on an error the plan covers what was matched before it, so the
// stop can be reported.
func Match(ctx context.Context, yt *session.YouTube, jobs []*Job, budget *Budget, obs Observer) {
	runJobs(ctx, jobs, func(ctx context.Context, j *Job) error { return j.match(ctx, yt, budget, obs) })
}

func (j *Job) match(ctx context.Context, yt *session.YouTube, budget *Budget, obs Observer) error {
	var err error
	j.Matched, j.Waiting, err = j.searchAll(ctx, yt, budget, obs)
	// An adopted playlist keeps its own videos this time: adopting combines
	// (planV1 §5); later runs remove what is not in Spotify.
	judgeExtras := j.Complete && err == nil && j.Waiting == 0 && j.Kind != Adopt
	j.Plan = transfer.BuildPlan(j.Matched, j.Existing, judgeExtras)

	// shortcut: extras count as already removed, but Write inserts before
	// it deletes; at exactly the cap YouTube refuses the insert and the run
	// stops cleanly. Delete first if anyone gets that close.
	room := max(j.maxVideos()-(len(j.Existing)-len(j.Plan.Extra)), 0)
	if len(j.Plan.ToAdd) > room {
		j.LeftOut = j.Plan.ToAdd[room:]
		j.Plan.ToAdd = j.Plan.ToAdd[:room]
	}
	return err
}

func (j *Job) maxVideos() int {
	if j.maxItems > 0 {
		return j.maxItems
	}
	return youtube.MaxPlaylistItems
}

// searchAll matches tracks in Spotify order, taking known ones as they are.
// Once the budget is spent (or YouTube refuses a search for quota), the
// remaining unknown tracks count as waiting, not failed. Any other error
// stops it, returning what it matched so far. Each found match is
// remembered for the session; a query with no result is not, so the next
// run tries it again (todo 2.6).
func (j *Job) searchAll(ctx context.Context, yt *session.YouTube, budget *Budget, obs Observer) (out []transfer.Matched, waiting int, err error) {
	progress := func(done int) {
		obs.emit(Event{SourceID: j.Source.ID, Step: StepSearch, Done: done, Total: len(j.Tracks), Target: j.Target})
	}
	for i, e := range j.Tracks {
		if m, ok := j.Known[e.Position]; ok {
			out = append(out, m)
			progress(i + 1)
			continue
		}
		if !budget.take() {
			waiting++
			progress(i + 1)
			continue
		}
		q := match.Query(e.Track)
		results, err := yt.Client.SearchVideos(ctx, q, match.SearchResults)
		if IsQuota(err) {
			budget.exhaust()
			waiting++
			progress(i + 1)
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
		progress(i + 1)
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

// Changing returns the jobs with writes and the quota they need together.
func Changing(jobs []*Job) (changing []*Job, units int) {
	for _, j := range jobs {
		if j.HasWrites() {
			units += j.Cost()
			changing = append(changing, j)
		}
	}
	return changing, units
}

// Write runs the writes of each job concurrently across playlists.
func Write(ctx context.Context, yt *session.YouTube, jobs []*Job, privacy string, obs Observer) {
	runJobs(ctx, jobs, func(ctx context.Context, j *Job) error { return j.write(ctx, yt.Client, privacy, obs) })
}

// write gets the destination ready, appends the missing videos in Spotify
// order (no position: each insert appends), then removes the videos that
// are no longer in Spotify. It resumes after what was already done, so
// calling it again never adds a video twice.
func (j *Job) write(ctx context.Context, yc *youtube.Client, privacy string, obs Observer) error {
	if !j.Ready {
		if err := j.Ensure(ctx, yc, privacy); err != nil {
			return err
		}
		j.Ready = true
		step := StepMark
		if j.Kind == Create {
			step = StepCreate
		}
		obs.emit(Event{SourceID: j.Source.ID, Step: step, Done: 1, Total: 1, Title: j.Target.Title, Target: j.Target})
	}
	for i := j.Added; i < len(j.Plan.ToAdd); i++ {
		m := j.Plan.ToAdd[i]
		if err := yc.AddVideo(ctx, j.Target.ID, m.Video.ID); err != nil {
			return err
		}
		j.Added++
		obs.emit(Event{SourceID: j.Source.ID, Step: StepAdd, Done: j.Added, Total: len(j.Plan.ToAdd), Title: m.Video.Title, Target: j.Target})
	}
	for i := j.Removed; i < len(j.Plan.Extra); i++ {
		it := j.Plan.Extra[i]
		if err := yc.DeleteItem(ctx, it.ID); err != nil && !IsNotFound(err) {
			return err
		}
		j.Removed++ // a 404 means someone removed it already: same outcome
		obs.emit(Event{SourceID: j.Source.ID, Step: StepRemove, Done: j.Removed, Total: len(j.Plan.Extra), Title: it.Title, Target: j.Target})
	}
	return nil
}

// runJobs runs fn for every job still running, concurrently across
// playlists. A quota error cancels the others: they would fail the same way.
func runJobs(parent context.Context, jobs []*Job, fn func(context.Context, *Job) error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	var wg sync.WaitGroup
	for _, j := range jobs {
		if j.Err != nil {
			continue
		}
		wg.Add(1)
		go func(j *Job) {
			defer wg.Done()
			if err := fn(ctx, j); err != nil {
				j.Err = stopReason(parent, err, cancel)
			}
		}(j)
	}
	wg.Wait()
}

// ErrSiblingQuota marks a job cancelled because another one hit the quota.
var ErrSiblingQuota = i18n.Error("stopped because another playlist reached the daily YouTube quota")

// stopReason explains why a job stopped. A quota error cancels the other
// jobs (they would fail the same way); they then report ErrSiblingQuota
// instead of a bare "context canceled".
func stopReason(parent context.Context, err error, cancelOthers context.CancelFunc) error {
	switch {
	case IsQuota(err):
		cancelOthers()
		return ExplainQuota(err)
	case errors.Is(err, context.Canceled) && parent.Err() == nil:
		return ErrSiblingQuota
	}
	return err
}

// FirstErr returns the root cause among the jobs' errors: a job cancelled
// because of another job's quota stop only matters if nothing else failed.
func FirstErr(jobs []*Job) error {
	var sibling error
	for _, j := range jobs {
		switch {
		case j.Err == nil:
		case errors.Is(j.Err, ErrSiblingQuota):
			if sibling == nil {
				sibling = fmt.Errorf(i18n.T("%q: %w"), j.Source.Name, j.Err)
			}
		default:
			return fmt.Errorf(i18n.T("%q: %w"), j.Source.Name, j.Err)
		}
	}
	return sibling
}

// IsQuota reports whether YouTube refused a call for quota.
func IsQuota(err error) bool {
	var apiErr *youtube.APIError
	return errors.As(err, &apiErr) && (apiErr.Reason == "quotaExceeded" || apiErr.Reason == "dailyLimitExceeded")
}

// IsNotFound reports whether YouTube answered 404.
func IsNotFound(err error) bool {
	var apiErr *youtube.APIError
	return errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound
}

// ExplainQuota adds what to do when the daily quota runs out.
func ExplainQuota(err error) error {
	if IsQuota(err) {
		return fmt.Errorf(i18n.T("%w\nhint: the daily YouTube quota is used up; it resets at midnight Pacific time. Run the same command again then: it only adds what is missing"), err)
	}
	return err
}
