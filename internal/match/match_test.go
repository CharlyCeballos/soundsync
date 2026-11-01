package match

import (
	"testing"

	"github.com/CharlyCeballos/soundsync/internal/spotify"
	"github.com/CharlyCeballos/soundsync/internal/youtube"
)

func TestQueryUsesOriginalMetadata(t *testing.T) {
	cases := []struct {
		track spotify.Track
		want  string
	}{
		{spotify.Track{Name: "The Way (feat. Mac Miller)", Artists: []string{"Ariana Grande", "Mac Miller"}}, "Ariana Grande The Way (feat. Mac Miller)"},
		{spotify.Track{Name: "Déjenme Llorar (feat. Leonel García)", Artists: []string{"Carla Morrison"}}, "Carla Morrison Déjenme Llorar (feat. Leonel García)"},
		{spotify.Track{Name: "Demo casera"}, "Demo casera"},
	}
	for _, c := range cases {
		if got := Query(c.track); got != c.want {
			t.Errorf("Query = %q, want %q", got, c.want)
		}
	}
}

func TestPick(t *testing.T) {
	lyric := youtube.Video{ID: "lyric", ChannelTitle: "Lyrics Hub"}
	vevo := youtube.Video{ID: "vevo", ChannelTitle: "ArianaGrandeVevo"}
	topic := youtube.Video{ID: "topic", ChannelTitle: "Ariana Grande - Topic"}
	cover := youtube.Video{ID: "cover", ChannelTitle: "Some Cover Band"}
	fakeTopic := youtube.Video{ID: "fake", ChannelTitle: "Topic Talks"}

	cases := []struct {
		name    string
		results []youtube.Video
		wantID  string
		reason  Reason
		ok      bool
	}{
		{"official later in ranking still wins", []youtube.Video{lyric, cover, vevo}, "vevo", ReasonVEVO, true},
		{"topic beats vevo ranked above it", []youtube.Video{vevo, lyric, topic}, "topic", ReasonTopic, true},
		{"topic beats vevo ranked below it", []youtube.Video{lyric, topic, vevo}, "topic", ReasonTopic, true},
		{"no official: first result", []youtube.Video{cover, lyric}, "cover", ReasonFirst, true},
		{"'Topic' must be the suffix", []youtube.Video{fakeTopic}, "fake", ReasonFirst, true},
		{"no results", nil, "", "", false},
	}
	for _, c := range cases {
		v, reason, ok := Pick(c.results)
		if v.ID != c.wantID || reason != c.reason || ok != c.ok {
			t.Errorf("%s: got (%s, %s, %v), want (%s, %s, %v)", c.name, v.ID, reason, ok, c.wantID, c.reason, c.ok)
		}
	}
}
