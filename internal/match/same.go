package match

import (
	"regexp"
	"strings"
	"time"

	"github.com/CharlyCeballos/soundsync/internal/spotify"
	"github.com/CharlyCeballos/soundsync/internal/youtube"
)

// LengthTolerance is how far a video's length may be from the track's for
// the two to count as the same recording. Album audio ("- Topic") lands
// within a second; a music video with an intro may miss, which only costs
// a search.
const LengthTolerance = 5 * time.Second

// minCoreLen keeps very short track names ("Ok", "21") from claiming
// unrelated videos.
const minCoreLen = 3

// CoreTitle lowercases a track name and drops suffixes such as
// "(feat. ...)", "- Film Version" or "[Remastered]", which video titles
// often write differently. ok is false when the core is too short to
// compare safely.
func CoreTitle(name string) (core string, ok bool) {
	s := strings.ToLower(name)
	for _, sep := range []string{" (", " - ", " ["} {
		if i := strings.Index(s, sep); i > 0 {
			s = s[:i]
		}
	}
	s = strings.TrimSpace(s)
	return s, len([]rune(s)) >= minCoreLen
}

// SameLength reports whether a video's length fits the track's; an unknown
// length (0) never fits.
func SameLength(trackMS int, video time.Duration) bool {
	if trackMS <= 0 || video <= 0 {
		return false
	}
	return (time.Duration(trackMS)*time.Millisecond - video).Abs() <= LengthTolerance
}

// Squash lowercases and drops all whitespace, for loose name comparisons
// ("Ariana Grande" against the channel "ArianaGrandeVevo").
func Squash(s string) string { return strings.Join(strings.Fields(strings.ToLower(s)), "") }

var liveRE = regexp.MustCompile(`\blive\b|\ben vivo\b`)

// OwnTopics returns, in ranking order, the results from the first artist's
// own "Artist - Topic" channel whose title contains the track's core name:
// album audio worth a length check (todo 2.18). Live versions are left out
// unless the track itself is live: measured on real searches, they are the
// usual wrong pick deep in the results.
func OwnTopics(t spotify.Track, results []youtube.Video) []youtube.Video {
	core, ok := CoreTitle(t.Name)
	if !ok || len(t.Artists) == 0 {
		return nil
	}
	channel := Squash(t.Artists[0] + " - Topic")
	trackIsLive := liveRE.MatchString(strings.ToLower(t.Name))
	var out []youtube.Video
	for _, r := range results {
		title := strings.ToLower(r.Title)
		if Squash(r.ChannelTitle) == channel && strings.Contains(title, core) && (trackIsLive || !liveRE.MatchString(title)) {
			out = append(out, r)
		}
	}
	return out
}
