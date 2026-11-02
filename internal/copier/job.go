package copier

import (
	"context"
	"fmt"

	"github.com/CharlyCeballos/soundsync/internal/i18n"
	"github.com/CharlyCeballos/soundsync/internal/spotify"
	"github.com/CharlyCeballos/soundsync/internal/transfer"
	"github.com/CharlyCeballos/soundsync/internal/youtube"
)

// Options shape a run.
type Options struct {
	Limit int // only the first Limit tracks of each playlist; 0 = all
	// MaxItems caps the tracks considered per playlist; 0 = YouTube's cap
	// on videos per playlist (planV1 §6.3).
	MaxItems int
}

func (o Options) maxItems() int {
	if o.MaxItems > 0 {
		return o.MaxItems
	}
	return youtube.MaxPlaylistItems
}

// Job is one Spotify playlist being copied into its YouTube playlist. The
// engine fills its fields step by step; callers read them between steps.
type Job struct {
	Destination
	Tracks   []spotify.Entry // searchable entries, limited
	Skipped  []spotify.Entry // episodes and unavailable entries
	Complete bool            // Tracks covers the whole playlist
	Capped   bool            // more than MaxItems tracks: the rest are left out

	Existing []youtube.Item           // the destination's videos (none when it is created)
	Known    map[int]transfer.Matched // tracks matched without a search, by position
	Matched  []transfer.Matched
	Plan     transfer.Plan
	Waiting  int                // tracks not searched because the searches ran out
	LeftOut  []transfer.Matched // matches dropped because the playlist would pass MaxItems
	Ready    bool               // the destination exists and carries the marker
	Added    int
	Removed  int
	Err      error

	maxItems int
}

// HasWrites reports whether the job changes anything on YouTube.
func (j *Job) HasWrites() bool {
	return j.Kind != Existing || len(j.Plan.ToAdd) > 0 || len(j.Plan.Extra) > 0
}

// Cost is the quota the job's writes need.
func (j *Job) Cost() int {
	return j.Destination.Cost() + youtube.WriteUnits*(len(j.Plan.ToAdd)+len(j.Plan.Extra))
}

// Title names the destination: the YouTube title once known, else the
// Spotify name it will get.
func (j *Job) Title() string {
	if j.Target.Title != "" {
		return j.Target.Title
	}
	return j.Source.Name
}

// InSync reports whether, after writing, everything in Spotify is in the
// YouTube playlist and the playlist has nothing extra.
func (j *Job) InSync() bool {
	p := j.Plan
	return j.Err == nil && j.Ready && j.Added == len(p.ToAdd) && p.ExtraKnown && j.Removed == len(p.Extra)
}

// Prepare reads each source's tracks from Spotify, before any search.
func Prepare(ctx context.Context, sc *spotify.Client, dests []Destination, opts Options) ([]*Job, error) {
	jobs := make([]*Job, len(dests))
	for i, d := range dests {
		j := &Job{Destination: d, Ready: d.Kind == Existing, maxItems: opts.maxItems()}
		entries, err := sc.PlaylistItems(ctx, d.Source.ID)
		if err != nil {
			return nil, fmt.Errorf(i18n.T("%q: %w"), d.Source.Name, err)
		}
		for _, e := range entries {
			if e.Skip != "" {
				j.Skipped = append(j.Skipped, e)
				continue
			}
			j.Tracks = append(j.Tracks, e)
		}
		if len(j.Tracks) > j.maxItems {
			j.Tracks, j.Capped = j.Tracks[:j.maxItems], true
		}
		j.Complete = !j.Capped
		if opts.Limit > 0 && len(j.Tracks) > opts.Limit {
			j.Tracks, j.Complete = j.Tracks[:opts.Limit], false
		}
		jobs[i] = j
	}
	return jobs, nil
}
