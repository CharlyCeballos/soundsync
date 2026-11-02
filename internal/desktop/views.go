package desktop

import (
	"context"
	"errors"

	"github.com/CharlyCeballos/soundsync/internal/copier"
	"github.com/CharlyCeballos/soundsync/internal/i18n"
	"github.com/CharlyCeballos/soundsync/internal/report"
	"github.com/CharlyCeballos/soundsync/internal/session"
	"github.com/CharlyCeballos/soundsync/internal/youtube"
)

// RunView is a plan, or its outcome once written.
type RunView struct {
	Phase     string        `json:"phase"` // planned or written
	Channel   Channel       `json:"channel"`
	Playlists []PlaylistRun `json:"playlists"`
	// Units is the quota the remaining writes need; Changing counts the
	// playlists those writes touch.
	Units    int `json:"units"`
	Changing int `json:"changing"`

	UnitsUsed      int64 `json:"unitsUsed"`
	UnitsPerDay    int64 `json:"unitsPerDay"`
	SearchesUsed   int64 `json:"searchesUsed"`
	SearchesPerDay int64 `json:"searchesPerDay"`
	// SearchesLeft is what this session may still search (never below 0).
	SearchesLeft int64 `json:"searchesLeft"`
	// SearchesExhausted means YouTube refused a search: today's are gone.
	SearchesExhausted bool `json:"searchesExhausted"`

	Cancelled bool   `json:"cancelled"` // the user stopped the step
	Error     string `json:"error"`     // the root cause of a stop, "" if none
}

// PlaylistRun is one Spotify playlist and its YouTube destination.
type PlaylistRun struct {
	SourceID   string       `json:"sourceId"`
	SourceName string       `json:"sourceName"`
	SourceURL  string       `json:"sourceUrl"`
	Kind       string       `json:"kind"` // existing, create or adopt
	Target     PlaylistRef  `json:"target"`
	Rows       []report.Row `json:"rows"` // one per track, then removed videos

	Add       int  `json:"add"`    // videos the plan adds
	Remove    int  `json:"remove"` // videos the plan removes
	Added     int  `json:"added"`
	Removed   int  `json:"removed"`
	Units     int  `json:"units"`
	HasWrites bool `json:"hasWrites"`
	Ready     bool `json:"ready"`  // the YouTube playlist exists and carries the marker
	InSync    bool `json:"inSync"` // after writing: nothing missing, nothing extra

	Present   int    `json:"present"`
	KeptYours int    `json:"keptYours"`
	NoMatch   int    `json:"noMatch"`
	SameVideo int    `json:"sameVideo"`
	Skipped   int    `json:"skipped"`
	Waiting   int    `json:"waiting"`
	LeftOut   int    `json:"leftOut"`
	Capped    bool   `json:"capped"`
	Error     string `json:"error"`
}

var kindNames = map[copier.Kind]string{copier.Existing: "existing", copier.Create: "create", copier.Adopt: "adopt"}

func runView(ctx context.Context, yt *session.YouTube, jobs []*copier.Job, budget *copier.Budget, phase string) *RunView {
	v := &RunView{
		Phase:             phase,
		Channel:           *channelOf(yt),
		UnitsUsed:         yt.Client.UnitsUsed(),
		UnitsPerDay:       youtube.UnitsPerDay,
		SearchesUsed:      yt.Client.SearchesUsed(),
		SearchesPerDay:    youtube.SearchCallsPerDay,
		SearchesLeft:      budget.Left(),
		SearchesExhausted: budget.GoogleSaidNo(),
		Cancelled:         ctx.Err() != nil,
	}
	for _, j := range jobs {
		v.Playlists = append(v.Playlists, playlistRun(j))
		if u := remainingUnits(j); u > 0 {
			v.Units += u
			v.Changing++
		}
	}
	if err := copier.FirstErr(jobs); err != nil && !v.Cancelled {
		v.Error = explain(err)
	}
	return v
}

// remainingUnits is the quota the job's writes still need.
func remainingUnits(j *copier.Job) int {
	units := youtube.WriteUnits * (len(j.Plan.ToAdd) - j.Added + len(j.Plan.Extra) - j.Removed)
	if !j.Ready {
		units += j.Destination.Cost()
	}
	return units
}

func playlistRun(j *copier.Job) PlaylistRun {
	p := j.Plan
	r := PlaylistRun{
		SourceID:   j.Source.ID,
		SourceName: j.Source.Name,
		SourceURL:  j.Source.URL(),
		Kind:       kindNames[j.Kind],
		Target:     refOf(j.Target),
		Rows:       report.Rows([]*copier.Job{j}),
		Add:        len(p.ToAdd),
		Remove:     len(p.Extra),
		Added:      j.Added,
		Removed:    j.Removed,
		Units:      j.Cost(),
		HasWrites:  j.HasWrites(),
		Ready:      j.Ready,
		InSync:     j.InSync(),
		Present:    len(p.Present),
		KeptYours:  len(p.Replaced),
		NoMatch:    len(p.Unmatched),
		SameVideo:  len(p.SameVideo),
		Skipped:    len(j.Skipped),
		Waiting:    j.Waiting,
		LeftOut:    len(j.LeftOut),
		Capped:     j.Capped,
	}
	if r.Target.Title == "" {
		r.Target.Title = j.Title() // a playlist still to be created gets the Spotify name
	}
	if j.Err != nil && !errors.Is(j.Err, context.Canceled) {
		r.Error = explain(j.Err)
	}
	return r
}

func reportDialogTitle() string { return i18n.T("Save the Soundsync report") }
