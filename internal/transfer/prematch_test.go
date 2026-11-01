package transfer

import (
	"testing"
	"time"

	"github.com/CharlyCeballos/soundsync/internal/match"
	"github.com/CharlyCeballos/soundsync/internal/spotify"
	"github.com/CharlyCeballos/soundsync/internal/youtube"
)

func track(pos int, name, artist string, length time.Duration) spotify.Entry {
	return spotify.Entry{Position: pos, Track: spotify.Track{Name: name, Artists: []string{artist}, DurationMS: int(length.Milliseconds())}}
}

func TestPreMatchTitleArtistAndLength(t *testing.T) {
	mmss := func(m, s int) time.Duration { return time.Duration(m)*time.Minute + time.Duration(s)*time.Second }
	entries := []spotify.Entry{
		track(1, "The Way (feat. Mac Miller)", "Ariana Grande", mmss(3, 47)), // VEVO: artist in channel without spaces
		track(2, "Hallelujah", "Jeff Buckley", mmss(6, 53)),                  // Topic: artist only in the channel
		track(3, "Love", "Some Artist", mmss(3, 30)),                         // only "Love Story" is there: length differs
		track(4, "Hurt", "Johnny Cash", mmss(3, 38)),                         // a Hurt by another artist
		track(5, "Hallelujah", "Pentatonix", mmss(4, 33)),                    // same title, other artist, not there
	}
	items := []youtube.Item{
		{ID: "i1", VideoID: "way", Title: "The Way (Official Video) ft. Mac Miller", Channel: "ArianaGrandeVevo"},
		{ID: "i2", VideoID: "hall", Title: "Hallelujah", Channel: "Jeff Buckley - Topic"},
		{ID: "i3", VideoID: "story", Title: "Some Artist - Love Story", Channel: "Some Artist"},
		{ID: "i4", VideoID: "nin", Title: "Nine Inch Nails - Hurt", Channel: "NINEINCHNAILS"},
	}
	lengths := map[string]time.Duration{"way": mmss(3, 50), "hall": mmss(6, 53), "story": mmss(3, 58), "nin": mmss(3, 38)}

	if got := CandidateVideos(entries, items); len(got) != 3 {
		t.Errorf("CandidateVideos = %v, want way, hall, story (title + artist)", got)
	}
	got := PreMatch(entries, items, lengths)
	if len(got) != 2 || got[1].Video.ID != "way" || got[2].Video.ID != "hall" {
		t.Fatalf("PreMatch = %+v, want tracks 1 and 2 only", got)
	}
	if got[2].Reason != match.ReasonInPlaylist || !got[2].Found || got[2].Video.ChannelTitle != "Jeff Buckley - Topic" {
		t.Errorf("unexpected match %+v", got[2])
	}
}

func TestPreMatchClaimsEachVideoOnce(t *testing.T) {
	entries := []spotify.Entry{
		track(1, "Hallelujah", "Jeff Buckley", 6*time.Minute),
		track(2, "Hallelujah", "Jeff Buckley", 6*time.Minute), // the same track twice in Spotify
	}
	items := []youtube.Item{{ID: "i1", VideoID: "hall", Title: "Hallelujah", Channel: "Jeff Buckley - Topic"}}
	got := PreMatch(entries, items, map[string]time.Duration{"hall": 6 * time.Minute})
	if len(got) != 1 || got[1].Video.ID != "hall" {
		t.Errorf("PreMatch = %+v, want only the first track", got)
	}
}

func TestPreMatchNeedsKnownLengths(t *testing.T) {
	entries := []spotify.Entry{track(1, "Hallelujah", "Jeff Buckley", 0)} // Spotify sent no length
	items := []youtube.Item{{ID: "i1", VideoID: "hall", Title: "Hallelujah", Channel: "Jeff Buckley - Topic"}}
	if got := PreMatch(entries, items, map[string]time.Duration{"hall": 6 * time.Minute}); len(got) != 0 {
		t.Errorf("unknown length must not match: %+v", got)
	}
	entries = []spotify.Entry{track(1, "Hallelujah", "Jeff Buckley", 6*time.Minute)}
	if got := PreMatch(entries, items, nil); len(got) != 0 {
		t.Errorf("deleted video (no length) must not match: %+v", got)
	}
}
