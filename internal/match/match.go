package match

import (
	"strings"

	"github.com/CharlyCeballos/soundsync/internal/spotify"
	"github.com/CharlyCeballos/soundsync/internal/youtube"
)

// SearchResults is how many results to request per track. search.list costs
// the same for 1 or 50 results; a few give room to find an official upload.
const SearchResults = 5

// Reason explains why a video was chosen.
type Reason string

const (
	ReasonTopic Reason = "topic" // auto-generated "Artist - Topic" channel
	ReasonVEVO  Reason = "vevo"  // official VEVO channel
	ReasonFirst Reason = "first" // no official upload in the results: first result
)

// Query builds the search text from Spotify's original metadata, untouched
// (planV1 §6.2: no translation, no cleanup).
func Query(t spotify.Track) string {
	if len(t.Artists) == 0 {
		return t.Name
	}
	return t.Artists[0] + " " + t.Name
}

// Pick chooses a video from results: the first "Artist - Topic" upload
// anywhere in the results (album audio, faithful to the Spotify track),
// else the first VEVO upload (official, but a music video may add intros or
// edits), else the first result. ok is false when there are no results.
func Pick(results []youtube.Video) (v youtube.Video, reason Reason, ok bool) {
	if len(results) == 0 {
		return youtube.Video{}, "", false
	}
	for _, r := range results {
		if isTopic(r.ChannelTitle) {
			return r, ReasonTopic, true
		}
	}
	for _, r := range results {
		if isVEVO(r.ChannelTitle) {
			return r, ReasonVEVO, true
		}
	}
	return results[0], ReasonFirst, true
}

func isTopic(channel string) bool { return strings.HasSuffix(channel, " - Topic") }

// isVEVO matches "ArianaGrandeVevo", "MacMillerVEVO" and "VEVO".
func isVEVO(channel string) bool { return strings.HasSuffix(strings.ToLower(channel), "vevo") }
