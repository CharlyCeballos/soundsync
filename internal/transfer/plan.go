package transfer

import (
	"strings"

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

// Replacement is a video the user put in the playlist in place of the one
// Soundsync matched for a track (todo 2.17): it is kept and the match is
// not added.
type Replacement struct {
	Matched
	Item youtube.Item
}

// Plan is the diff between the matched Spotify tracks (S) and the videos
// already in the YouTube playlist (Y), following planV1 §6.2.
type Plan struct {
	ToAdd      []Matched      // videos in S but not in Y, Spotify order, each once
	Present    []Matched      // already in Y: no write
	Replaced   []Replacement  // the user swapped the match for another video: no write
	SameVideo  []Matched      // matched a video an earlier track already uses
	Unmatched  []Matched      // no search result: skip and report
	Extra      []youtube.Item // in Y, not in S and not a replacement: to remove (todo 2.5)
	ExtraKnown bool           // false when only part of the playlist was matched
}

// InSync reports whether S equals Y as sets, so nothing needs writing.
// Order is ignored on purpose (planV1 §4: never reorder an equal set).
func (p Plan) InSync() bool { return len(p.ToAdd) == 0 && p.ExtraKnown && len(p.Extra) == 0 }

// BuildPlan computes the diff. complete says whether matched covers the
// whole Spotify playlist; if not, Y's extra videos cannot be judged.
//
// A track whose match is missing from Y, while Y holds an unmatched video
// whose title contains the track's name, is taken as a manual correction
// by the user: that video stays and the match is not added back.
func BuildPlan(matched []Matched, existing []youtube.Item, complete bool) Plan {
	inY := make(map[string]bool, len(existing))
	for _, it := range existing {
		inY[it.VideoID] = true
	}

	var p Plan
	var missing []Matched
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
			missing = append(missing, m)
		}
	}

	var loose []youtube.Item // in Y but matched by no track
	for _, it := range existing {
		if !inS[it.VideoID] {
			loose = append(loose, it)
		}
	}
	kept := map[string]bool{} // video IDs kept as the user's replacements
	for _, m := range missing {
		if it, ok := replacementFor(m.Entry.Track.Name, loose, kept); ok {
			kept[it.VideoID] = true
			p.Replaced = append(p.Replaced, Replacement{Matched: m, Item: it})
			continue
		}
		p.ToAdd = append(p.ToAdd, m)
	}

	p.ExtraKnown = complete
	if complete {
		for _, it := range loose {
			if !kept[it.VideoID] {
				p.Extra = append(p.Extra, it)
			}
		}
	}
	return p
}

// minCoreLen keeps very short track names ("Ok", "21") from claiming
// unrelated videos as replacements.
const minCoreLen = 3

// replacementFor finds a loose item whose title contains the core of the
// track name, skipping videos already kept for another track.
func replacementFor(trackName string, loose []youtube.Item, kept map[string]bool) (youtube.Item, bool) {
	core := coreTitle(trackName)
	if len([]rune(core)) < minCoreLen {
		return youtube.Item{}, false
	}
	for _, it := range loose {
		if !kept[it.VideoID] && strings.Contains(strings.ToLower(it.Title), core) {
			return it, true
		}
	}
	return youtube.Item{}, false
}

// coreTitle lowercases a track name and drops suffixes such as
// "(feat. ...)", "- Film Version" or "[Remastered]", which video titles
// often write differently.
func coreTitle(name string) string {
	s := strings.ToLower(name)
	for _, sep := range []string{" (", " - ", " ["} {
		if i := strings.Index(s, sep); i > 0 {
			s = s[:i]
		}
	}
	return strings.TrimSpace(s)
}
