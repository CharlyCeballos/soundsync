package transfer

import (
	"strings"
	"time"

	"github.com/CharlyCeballos/soundsync/internal/match"
	"github.com/CharlyCeballos/soundsync/internal/spotify"
	"github.com/CharlyCeballos/soundsync/internal/youtube"
)

// CandidateVideos lists the IDs of playlist videos that look like one of
// the tracks by title and artist; only those need a length check.
func CandidateVideos(entries []spotify.Entry, items []youtube.Item) []string {
	var ids []string
	seen := map[string]bool{}
	for _, e := range entries {
		for _, it := range items {
			if !seen[it.VideoID] && looksLike(e.Track, it) {
				seen[it.VideoID] = true
				ids = append(ids, it.VideoID)
			}
		}
	}
	return ids
}

// PreMatch finds tracks that are already in the playlist, so a run on
// another day does not spend searches on them again (nothing is stored
// between runs). A track pairs with the first unclaimed video that looks
// like it (title and artist) and has its length. Keys are track positions.
func PreMatch(entries []spotify.Entry, items []youtube.Item, lengths map[string]time.Duration) map[int]Matched {
	out := map[int]Matched{}
	claimed := map[string]bool{} // playlist item IDs
	for _, e := range entries {
		for _, it := range items {
			if claimed[it.ID] || !looksLike(e.Track, it) || !match.SameLength(e.Track.DurationMS, lengths[it.VideoID]) {
				continue
			}
			claimed[it.ID] = true
			out[e.Position] = Matched{
				Entry:  e,
				Video:  youtube.Video{ID: it.VideoID, Title: it.Title, ChannelTitle: it.Channel},
				Reason: match.ReasonInPlaylist,
				Found:  true,
			}
			break
		}
	}
	return out
}

// looksLike: the video title contains the track's core name, and its title
// or channel contains the first artist (spaces ignored, so "Ariana Grande"
// matches the channel "ArianaGrandeVevo").
func looksLike(t spotify.Track, it youtube.Item) bool {
	core, ok := match.CoreTitle(t.Name)
	if !ok || len(t.Artists) == 0 || !strings.Contains(strings.ToLower(it.Title), core) {
		return false
	}
	artist := match.Squash(t.Artists[0])
	return artist != "" && (strings.Contains(match.Squash(it.Title), artist) || strings.Contains(match.Squash(it.Channel), artist))
}
