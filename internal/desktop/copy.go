package desktop

import (
	"context"
	"errors"
	"io/fs"
	"strings"

	"github.com/CharlyCeballos/soundsync/internal/copier"
	"github.com/CharlyCeballos/soundsync/internal/report"
	"github.com/CharlyCeballos/soundsync/internal/session"
	"github.com/CharlyCeballos/soundsync/internal/spotify"
	"github.com/CharlyCeballos/soundsync/internal/youtube"
)

// Copy copies the chosen Spotify playlists to YouTube in three calls:
// Resolve (where each one goes), Plan (look up and search) and Write (after
// the user agrees to the exact changes, Policies III.C.2).
type Copy struct{ st *state }

// PlaylistRef is a YouTube playlist.
type PlaylistRef struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Privacy  string `json:"privacy"`
	Videos   int    `json:"videos"`
	URL      string `json:"url"`
	MusicURL string `json:"musicUrl"`
}

func refOf(p youtube.Playlist) PlaylistRef {
	r := PlaylistRef{ID: p.ID, Title: p.Title, Privacy: p.Privacy, Videos: p.ItemCount}
	if p.ID != "" {
		r.URL, r.MusicURL = p.URL(), p.MusicURL()
	}
	return r
}

// DestinationView says where one Spotify playlist goes. Kind "choose" means
// the channel has playlists with the same title that Soundsync did not
// create: the user picks one of Candidates or a new playlist (todo 4.4).
type DestinationView struct {
	SourceID   string        `json:"sourceId"`
	SourceName string        `json:"sourceName"`
	SourceURL  string        `json:"sourceUrl"`
	Kind       string        `json:"kind"` // existing, create or choose
	Target     *PlaylistRef  `json:"target"`
	Candidates []PlaylistRef `json:"candidates"`
}

// Resolve checks the chosen playlists and finds where each one goes.
func (c *Copy) Resolve(ctx context.Context, sourceIDs []string) ([]DestinationView, error) {
	st := c.st
	if len(sourceIDs) == 0 {
		return nil, ErrNoSelection
	}
	ctx, end, err := st.begin(ctx, "resolve")
	if err != nil {
		return nil, err
	}
	defer end()
	sp, yt, err := st.sessions()
	if err != nil {
		return nil, err
	}
	srcs, err := chosen(ctx, sp, sourceIDs)
	if err != nil {
		return nil, err
	}
	pls, err := yt.Client.MyPlaylists(ctx)
	if err != nil {
		return nil, err
	}
	dests := copier.Resolve(pls, srcs)

	st.mu.Lock()
	st.resetCopy()
	st.phase, st.dests = phaseResolved, dests
	st.mu.Unlock()

	views := make([]DestinationView, len(dests))
	for i, d := range dests {
		views[i] = destinationView(d)
	}
	return views, nil
}

func destinationView(d copier.Destination) DestinationView {
	v := DestinationView{SourceID: d.Source.ID, SourceName: d.Source.Name, SourceURL: d.Source.URL(), Kind: "create"}
	switch {
	case d.Kind == copier.Existing:
		ref := refOf(d.Target)
		v.Kind, v.Target = "existing", &ref
	case len(d.Candidates) > 0:
		v.Kind = "choose"
		for _, p := range d.Candidates {
			v.Candidates = append(v.Candidates, refOf(p))
		}
	}
	return v
}

func (st *state) sessions() (*session.Spotify, *session.YouTube, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	switch {
	case st.sp == nil:
		return nil, nil, ErrSpotifyFirst
	case st.yt == nil:
		return nil, nil, ErrYouTubeFirst
	}
	return st.sp, st.yt, nil
}

// chosen maps IDs to the user's own or collaborative playlists, once each.
// The frontend only offers those, but every call is checked anyway.
func chosen(ctx context.Context, sp *session.Spotify, ids []string) ([]spotify.Playlist, error) {
	library, err := sp.Playlists(ctx, false)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]spotify.Playlist, len(library))
	for _, p := range library {
		byID[p.ID] = p
	}
	var out []spotify.Playlist
	seen := map[string]bool{}
	for _, id := range ids {
		p, ok := byID[id]
		switch {
		case !ok:
			return nil, errNotInLibrary(id)
		case p.AccessFor(sp.Me.ID) == spotify.AccessFollowed:
			return nil, errFollowed(p.Name)
		case !seen[id]:
			seen[id] = true
			out = append(out, p)
		}
	}
	return out, nil
}

// PlanRequest carries the user's answers. Adopt maps the ID of each Spotify
// playlist whose destination is "choose" to the YouTube playlist to use,
// or "" for a new playlist; every such playlist needs an answer.
type PlanRequest struct {
	Adopt   map[string]string `json:"adopt"`
	Privacy string            `json:"privacy"` // of new playlists: private, unlisted or public
}

// Plan reads the tracks, finds what is already on YouTube and searches the
// rest, sending copy:progress events. Nothing is written yet.
func (c *Copy) Plan(ctx context.Context, req PlanRequest) (*RunView, error) {
	st := c.st
	if !youtube.ValidPrivacy(req.Privacy) {
		return nil, ErrBadPrivacy
	}
	ctx, end, err := st.begin(ctx, "plan")
	if err != nil {
		return nil, err
	}
	defer end()
	sp, yt, err := st.sessions()
	if err != nil {
		return nil, err
	}
	st.mu.Lock()
	// A stopped or reviewed plan can be planned again; after a write the
	// channel changed, so the destinations must be resolved again.
	if st.phase != phaseResolved && st.phase != phasePlanned {
		st.mu.Unlock()
		return nil, ErrResolveFirst
	}
	dests := append([]copier.Destination(nil), st.dests...)
	st.mu.Unlock()

	for i := range dests {
		if err := answer(&dests[i], req.Adopt); err != nil {
			return nil, err
		}
	}
	jobs, err := copier.Prepare(ctx, sp.Client, dests, copier.Options{})
	if err != nil {
		if ctx.Err() != nil {
			// Stopped while reading Spotify: nothing planned, plan again.
			return &RunView{Phase: "planned", Channel: *channelOf(yt), Cancelled: true}, nil
		}
		return nil, err
	}
	observe := st.progress()
	copier.LookUp(ctx, yt, jobs, observe)
	budget := copier.NewBudget(youtube.SearchCallsPerDay, yt.Client.SearchesUsed())
	copier.Match(ctx, yt, jobs, budget, observe)

	st.mu.Lock()
	st.phase, st.jobs, st.privacy, st.budget = phasePlanned, jobs, req.Privacy, budget
	st.mu.Unlock()
	return runView(ctx, yt, jobs, budget, "planned"), nil
}

// answer applies the user's choice for one destination.
func answer(d *copier.Destination, adopt map[string]string) error {
	if len(d.Candidates) == 0 || d.Kind != copier.Create {
		return nil
	}
	id, answered := adopt[d.Source.ID]
	switch {
	case !answered:
		return errChoose(d.Source.Name)
	case id == "":
		return nil // a new playlist
	}
	return d.Adopt(id)
}

// Write makes the planned changes on YouTube. A plan is written once: after
// that (or after a stop), plan again to copy more.
func (c *Copy) Write(ctx context.Context) (*RunView, error) {
	st := c.st
	ctx, end, err := st.begin(ctx, "write")
	if err != nil {
		return nil, err
	}
	defer end()
	_, yt, err := st.sessions()
	if err != nil {
		return nil, err
	}
	st.mu.Lock()
	ph, jobs, privacy, budget := st.phase, st.jobs, st.privacy, st.budget
	if ph == phasePlanned {
		st.phase = phaseWritten // consumed even if the write stops
	}
	st.mu.Unlock()
	switch {
	case ph == phaseWritten:
		return nil, ErrAlreadyWritten
	case ph != phasePlanned:
		return nil, ErrPlanFirst
	case copier.FirstErr(jobs) != nil:
		return nil, ErrPlanIncomplete
	}
	changing, _ := copier.Changing(jobs)
	copier.Write(ctx, yt, changing, privacy, st.progress())
	return runView(ctx, yt, jobs, budget, "written"), nil
}

// Stop ends the running step early. A plan or a write then returns what it
// did so far, marked as cancelled; what was written stays.
func (c *Copy) Stop() {
	st := c.st
	st.mu.Lock()
	stop := st.stopOp
	st.mu.Unlock()
	if stop != nil {
		stop()
	}
}

// progress forwards the engine's events to the frontend.
func (st *state) progress() copier.Observer {
	return func(e copier.Event) {
		st.host.Emit(EventProgress, Progress{SourceID: e.SourceID, Step: string(e.Step), Done: e.Done, Total: e.Total, Title: e.Title})
	}
}

// SaveReport asks where to save the last run's report and writes it as CSV
// or JSON. It never replaces an existing file. "" means the user cancelled.
func (c *Copy) SaveReport(format string) (string, error) {
	st := c.st
	f := report.Format(strings.ToLower(format))
	if f != report.CSV && f != report.JSON {
		return "", ErrBadFormat
	}
	st.mu.Lock()
	busy, jobs := st.busy, st.jobs
	st.mu.Unlock()
	switch {
	case busy != "":
		return "", ErrBusy
	case jobs == nil:
		return "", ErrNoRun
	}

	name := "soundsync-report-" + st.opts.Now().Format("2006-01-02-1504") + "." + string(f)
	path, err := st.host.SaveFile(reportDialogTitle(), name)
	if err != nil || path == "" {
		return "", err
	}
	if _, err := report.FormatOf(path); err != nil {
		path += "." + string(f) // the dialog may drop the extension
	}
	err = report.Save(path, report.Rows(jobs))
	if errors.Is(err, fs.ErrExist) {
		return "", errReportExists(path)
	}
	return path, err
}
