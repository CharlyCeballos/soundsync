package transfer

import (
	"github.com/CharlyCeballos/soundsync/internal/match"
	"github.com/CharlyCeballos/soundsync/internal/spotify"
	"github.com/CharlyCeballos/soundsync/internal/youtube"
)

// Matched is one Spotify track with the YouTube video chosen for it.
type Matched struct {
	Entry  spotify.Entry
	Video  youtube.Video
	Reason match.Reason
	Found  bool // false: the search returned nothing usable
}

// Plan is the diff between the matched Spotify tracks (S) and the videos
// already in the YouTube playlist (Y), following planV1 §6.2.
type Plan struct {
	ToAdd      []Matched // videos in S but not in Y, Spotify order, each once
	Present    []Matched // already in Y: no write
	SameVideo  []Matched // matched a video an earlier track already uses
	Unmatched  []Matched // no search result: skip and report
	Extra      []string  // video IDs in Y but not in S (removal: todo 2.5)
	ExtraKnown bool      // false when only part of the playlist was matched
}

// InSync reports whether S equals Y as sets, so nothing needs writing.
// Order is ignored on purpose (planV1 §4: never reorder an equal set).
func (p Plan) InSync() bool { return len(p.ToAdd) == 0 && p.ExtraKnown && len(p.Extra) == 0 }

// BuildPlan computes the diff. complete says whether matched covers the
// whole Spotify playlist; if not, Y's extra videos cannot be judged.
func BuildPlan(matched []Matched, existing []string, complete bool) Plan {
	inY := make(map[string]bool, len(existing))
	for _, id := range existing {
		inY[id] = true
	}

	var p Plan
	inS := map[string]bool{}
	for _, m := range matched {
		switch {
		case !m.Found:
			p.Unmatched = append(p.Unmatched, m)
		case inS[m.Video.ID]:
			p.SameVideo = append(p.SameVideo, m)
		case inY[m.Video.ID]:
			inS[m.Video.ID] = true
			p.Present = append(p.Present, m)
		default:
			inS[m.Video.ID] = true
			p.ToAdd = append(p.ToAdd, m)
		}
	}

	p.ExtraKnown = complete
	if complete {
		seen := map[string]bool{}
		for _, id := range existing {
			if !inS[id] && !seen[id] {
				seen[id] = true
				p.Extra = append(p.Extra, id)
			}
		}
	}
	return p
}
