package match

import (
	"testing"
	"time"

	"github.com/CharlyCeballos/soundsync/internal/spotify"
	"github.com/CharlyCeballos/soundsync/internal/youtube"
)

func TestCoreTitle(t *testing.T) {
	for in, want := range map[string]string{
		"I'll Never Love Again - Film Version": "i'll never love again",
		"Déjenme Llorar (feat. Leonel García)": "déjenme llorar",
		"Hurt [Remastered]":                    "hurt",
		"Mentiras":                             "mentiras",
		"(What's the Story) Morning Glory?":    "(what's the story) morning glory?",
	} {
		if got, ok := CoreTitle(in); got != want || !ok {
			t.Errorf("CoreTitle(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	if _, ok := CoreTitle("Ok"); ok {
		t.Error("a 2-letter core is too short to compare")
	}
}

func TestSameLength(t *testing.T) {
	if !SameLength(227000, 3*time.Minute+50*time.Second) || SameLength(227000, 4*time.Minute) || SameLength(0, time.Minute) || SameLength(60000, 0) {
		t.Error("SameLength tolerance or unknown-length handling is off")
	}
}

// Cases taken from the 2.18 measurement on real searches.
func TestOwnTopics(t *testing.T) {
	queen := spotify.Track{Name: "Bohemian Rhapsody", Artists: []string{"Queen"}}
	results := []youtube.Video{
		{ID: "official", Title: "Queen – Bohemian Rhapsody (Official Video Remastered)", ChannelTitle: "Queen Official"},
		{ID: "liveaid", Title: "Bohemian Rhapsody (Live Aid)", ChannelTitle: "Queen - Topic"},
		{ID: "other", Title: "Don't Stop Me Now (...Revisited)", ChannelTitle: "Queen - Topic"},
		{ID: "studio", Title: "Bohemian Rhapsody - Remastered 2011", ChannelTitle: "Queen - Topic"},
	}
	if got := OwnTopics(queen, results); len(got) != 1 || got[0].ID != "studio" {
		t.Errorf("OwnTopics = %+v, want only the studio Topic (not live, not another song)", got)
	}

	cover := spotify.Track{Name: "Déjenme Llorar (feat. Leonel García)", Artists: []string{"Carla Morrison"}}
	other := []youtube.Video{{ID: "morel", Title: "Dejenme Llorar Un Rato", ChannelTitle: "Jorge Morel - Topic"}}
	if got := OwnTopics(cover, other); len(got) != 0 {
		t.Errorf("another artist's Topic must not count: %+v", got)
	}

	live := spotify.Track{Name: "Without You - Live", Artists: []string{"Air Supply"}}
	liveTopic := []youtube.Video{{ID: "l", Title: "Without You (Live)", ChannelTitle: "Air Supply - Topic"}}
	if got := OwnTopics(live, liveTopic); len(got) != 1 {
		t.Errorf("a live track may match a live Topic: %+v", got)
	}
}
