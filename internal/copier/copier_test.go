package copier

import (
	"context"
	"errors"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	"github.com/CharlyCeballos/soundsync/internal/fakeapi"
	"github.com/CharlyCeballos/soundsync/internal/session"
	"github.com/CharlyCeballos/soundsync/internal/spotify"
	"github.com/CharlyCeballos/soundsync/internal/youtube"
)

// world is a fake Spotify library and YouTube channel, served for one test.
type world struct {
	sp *spotify.Client
	yt *session.YouTube
	ch *fakeapi.YouTube
}

func newWorld(t *testing.T, ch *fakeapi.YouTube) world {
	t.Helper()
	spSrv := httptest.NewServer(fakeapi.NewLibrary())
	ytSrv := httptest.NewServer(ch)
	t.Cleanup(spSrv.Close)
	t.Cleanup(ytSrv.Close)
	return world{
		sp: spotify.NewClient(spSrv.Client()).WithBaseURL(spSrv.URL),
		yt: &session.YouTube{Client: youtube.NewClient(ytSrv.Client()).WithBaseURL(ytSrv.URL), Channel: youtube.Channel{Title: "Carlos"}},
		ch: ch,
	}
}

// sources returns the library's own playlists, in the given order.
func (w world) sources(t *testing.T, ids ...string) []spotify.Playlist {
	t.Helper()
	all, err := w.sp.MyPlaylists(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var out []spotify.Playlist
	for _, id := range ids {
		for _, p := range all {
			if p.ID == id {
				out = append(out, p)
			}
		}
	}
	return out
}

// plan resolves, prepares, looks up and matches, as both front ends do.
func (w world) plan(t *testing.T, ctx context.Context, opts Options, budget *Budget, obs Observer, ids ...string) []*Job {
	t.Helper()
	pls, err := w.yt.Client.MyPlaylists(ctx)
	if err != nil {
		t.Fatal(err)
	}
	jobs, err := Prepare(ctx, w.sp, Resolve(pls, w.sources(t, ids...)), opts)
	if err != nil {
		t.Fatal(err)
	}
	LookUp(ctx, w.yt, jobs, obs)
	Match(ctx, w.yt, jobs, budget, obs)
	return jobs
}

// recorder collects events from concurrent jobs.
type recorder struct {
	mu     sync.Mutex
	events []Event
}

func (r *recorder) observe(e Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
}

func (r *recorder) of(sourceID string) []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Event
	for _, e := range r.events {
		if e.SourceID == sourceID {
			out = append(out, e)
		}
	}
	return out
}

func TestResolveAndAdopt(t *testing.T) {
	srcs := []spotify.Playlist{{ID: fakeapi.WayID, Name: "The Way"}, {ID: fakeapi.CoversID, Name: "Covers"}, {ID: "new", Name: "Fresh"}}
	pls := []youtube.Playlist{
		{ID: "PLway", Title: "The Way", Description: "soundsync:spotify:" + fakeapi.WayID},
		{ID: "PLdup", Title: "The Way (old)", Description: "soundsync:spotify:" + fakeapi.WayID},
		{ID: "PLmine", Title: "Covers", Description: "my covers"},
	}
	dests := Resolve(pls, srcs)
	if d := dests[0]; d.Kind != Existing || d.Target.ID != "PLway" || len(d.Duplicates) != 1 || len(d.Candidates) != 0 {
		t.Errorf("marked: %+v", d)
	}
	if d := dests[1]; d.Kind != Create || len(d.Candidates) != 1 {
		t.Errorf("same title must wait for the user: %+v", d)
	}
	if d := dests[2]; d.Kind != Create || len(d.Candidates) != 0 {
		t.Errorf("nothing found: %+v", d)
	}

	if err := dests[1].Adopt("PLway"); !errors.Is(err, ErrNotACandidate) || dests[1].Kind != Create {
		t.Errorf("adopting a playlist that was not offered: err=%v kind=%v", err, dests[1].Kind)
	}
	if err := dests[1].Adopt("PLmine"); err != nil || dests[1].Kind != Adopt || dests[1].Target.Description != "my covers" {
		t.Errorf("adopt: err=%v dest=%+v", err, dests[1])
	}
	if dests[0].Cost() != 0 || dests[1].Cost() != youtube.WriteUnits {
		t.Error("only a new or adopted playlist costs a write")
	}
}

func TestPlanWriteAndEvents(t *testing.T) {
	w := newWorld(t, fakeapi.NewChannel())
	rec := &recorder{}
	jobs := w.plan(t, t.Context(), Options{}, NewBudget(100, 0), rec.observe, fakeapi.WayID, fakeapi.CoversID)

	changing, units := Changing(jobs)
	if len(changing) != 2 || units != 4*youtube.WriteUnits {
		t.Fatalf("changing=%d units=%d, want 2 playlists and 4 inserts", len(changing), units)
	}
	Write(t.Context(), w.yt, changing, youtube.PrivacyPrivate, rec.observe)
	if err := FirstErr(jobs); err != nil {
		t.Fatal(err)
	}

	var steps []string
	for _, e := range rec.of(fakeapi.WayID) {
		steps = append(steps, string(e.Step)+":"+e.Title)
	}
	want := []string{"lookup:", "search:", "search:", "add:The Way", "add:Mac Miller - My Favorite Part (feat. Ariana Grande) & more"}
	if !slices.Equal(steps, want) {
		t.Errorf("events of one job, in order:\n got %v\nwant %v", steps, want)
	}
	for _, j := range jobs {
		if !j.InSync() || j.Added != 2 {
			t.Errorf("%s: in sync=%v added=%d", j.Source.Name, j.InSync(), j.Added)
		}
	}

	// A second Write on the same plan resumes after what was done: nothing twice.
	Write(t.Context(), w.yt, changing, youtube.PrivacyPrivate, nil)
	if c := w.ch.Counts(); c.ItemInserts != 4 {
		t.Errorf("item inserts = %d after writing twice, want 4", c.ItemInserts)
	}
}

func TestQuotaStopsTheOtherPlaylist(t *testing.T) {
	ch := fakeapi.NewChannel()
	ch.FailItemAt = 2 // the second insert, whichever playlist sends it, hits the quota
	w := newWorld(t, ch)
	jobs := w.plan(t, t.Context(), Options{}, NewBudget(100, 0), nil, fakeapi.WayID, fakeapi.CoversID)
	changing, _ := Changing(jobs)
	Write(t.Context(), w.yt, changing, youtube.PrivacyPrivate, nil)

	quota, sibling := 0, 0
	for _, j := range jobs {
		switch {
		case IsQuota(j.Err):
			quota++
		case errors.Is(j.Err, ErrSiblingQuota):
			sibling++
		}
	}
	// Both playlists may reach YouTube's refusal before the cancel lands; what
	// matters is that neither keeps writing and none reports a bare cancel.
	if quota < 1 || quota+sibling != 2 {
		t.Fatalf("quota=%d sibling=%d; errors: %v / %v", quota, sibling, jobs[0].Err, jobs[1].Err)
	}
	if err := FirstErr(jobs); !IsQuota(err) {
		t.Errorf("FirstErr = %v, want the quota error (the root cause)", err)
	}
}

func TestBudgetLeavesTracksWaiting(t *testing.T) {
	w := newWorld(t, fakeapi.NewChannel())
	budget := NewBudget(1, 0)
	jobs := w.plan(t, t.Context(), Options{}, budget, nil, fakeapi.CoversID)
	if j := jobs[0]; len(j.Matched) != 1 || j.Waiting != 1 || j.Plan.ExtraKnown {
		t.Errorf("matched=%d waiting=%d extraKnown=%v, want 1, 1 and extras not judged", len(j.Matched), j.Waiting, j.Plan.ExtraKnown)
	}
	if budget.Left() != 0 {
		t.Errorf("Left = %d, must never go below zero", budget.Left())
	}
	if NewBudget(5, 9).Left() != 0 {
		t.Error("more used than allowed still shows zero")
	}
}

func TestSearchQuotaWaitsInsteadOfFailing(t *testing.T) {
	ch := fakeapi.NewChannel()
	ch.FailSearch = 1
	w := newWorld(t, ch)
	budget := NewBudget(100, 0)
	jobs := w.plan(t, t.Context(), Options{}, budget, nil, fakeapi.CoversID)
	if !budget.GoogleSaidNo() || jobs[0].Err != nil || jobs[0].Waiting != 2 {
		t.Errorf("googleSaidNo=%v err=%v waiting=%d", budget.GoogleSaidNo(), jobs[0].Err, jobs[0].Waiting)
	}
}

func TestCancelledRunIsNotASiblingQuota(t *testing.T) {
	w := newWorld(t, fakeapi.NewChannel())
	ctx, cancel := context.WithCancel(t.Context())
	jobs := w.plan(t, ctx, Options{}, NewBudget(100, 0), nil, fakeapi.WayID, fakeapi.CoversID)
	cancel()
	changing, _ := Changing(jobs)
	Write(ctx, w.yt, changing, youtube.PrivacyPrivate, nil)
	for _, j := range jobs {
		if !errors.Is(j.Err, context.Canceled) || errors.Is(j.Err, ErrSiblingQuota) {
			t.Errorf("%s: err = %v, want context.Canceled", j.Source.Name, j.Err)
		}
	}
}

func TestMaxItemsCapsTheTracks(t *testing.T) {
	w := newWorld(t, fakeapi.NewChannel())
	jobs := w.plan(t, t.Context(), Options{MaxItems: 1}, NewBudget(100, 0), nil, fakeapi.CoversID)
	if j := jobs[0]; !j.Capped || j.Complete || len(j.Tracks) != 1 || len(j.Plan.ToAdd) != 1 {
		t.Errorf("capped=%v complete=%v tracks=%d toAdd=%d", j.Capped, j.Complete, len(j.Tracks), len(j.Plan.ToAdd))
	}
}
