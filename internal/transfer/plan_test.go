package transfer

import (
	"testing"

	"github.com/CharlyCeballos/soundsync/internal/spotify"
	"github.com/CharlyCeballos/soundsync/internal/youtube"
)

func m(pos int, videoID string) Matched {
	return named(pos, videoID, "t")
}

func named(pos int, videoID, track string) Matched {
	return Matched{
		Entry: spotify.Entry{Position: pos, Track: spotify.Track{Name: track}},
		Video: youtube.Video{ID: videoID},
		Found: videoID != "",
	}
}

// in builds playlist items; each item ID is "i" + its video ID and position.
func in(videoIDs ...string) []youtube.Item {
	var out []youtube.Item
	for i, id := range videoIDs {
		out = append(out, youtube.Item{ID: "i" + id + string(rune('0'+i)), VideoID: id, Title: "video " + id})
	}
	return out
}

func ids(ms []Matched) []string {
	var out []string
	for _, x := range ms {
		out = append(out, x.Video.ID)
	}
	return out
}

func videoIDs(items []youtube.Item) []string {
	var out []string
	for _, it := range items {
		out = append(out, it.VideoID)
	}
	return out
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestBuildPlanNewPlaylist(t *testing.T) {
	p := BuildPlan([]Matched{m(1, "A"), m(2, ""), m(3, "B"), m(4, "A")}, nil, true)
	if !eq(ids(p.ToAdd), []string{"A", "B"}) {
		t.Errorf("ToAdd = %v, want [A B] in Spotify order", ids(p.ToAdd))
	}
	if len(p.Unmatched) != 1 || p.Unmatched[0].Entry.Position != 2 {
		t.Errorf("Unmatched = %+v", p.Unmatched)
	}
	if len(p.SameVideo) != 1 || p.SameVideo[0].Entry.Position != 4 {
		t.Errorf("SameVideo = %+v (two tracks matching one video = one item)", p.SameVideo)
	}
	if p.InSync() {
		t.Error("new playlist cannot be in sync")
	}
}

func TestBuildPlanAppendsOnlyMissing(t *testing.T) {
	p := BuildPlan([]Matched{m(1, "A"), m(2, "B"), m(3, "C")}, in("B", "A"), true)
	if !eq(ids(p.ToAdd), []string{"C"}) || !eq(ids(p.Present), []string{"A", "B"}) {
		t.Errorf("ToAdd=%v Present=%v", ids(p.ToAdd), ids(p.Present))
	}
}

func TestBuildPlanSameSetDifferentOrderIsInSync(t *testing.T) {
	p := BuildPlan([]Matched{m(1, "A"), m(2, "B")}, in("B", "A"), true)
	if !p.InSync() {
		t.Errorf("same set in another order must be in sync: %+v", p)
	}
}

func TestBuildPlanExtraOnlyWhenComplete(t *testing.T) {
	existing := in("A", "X", "X")
	p := BuildPlan([]Matched{m(1, "A")}, existing, true)
	// Both copies of X go: each removal is one playlist item.
	if !eq(videoIDs(p.Extra), []string{"X", "X"}) || p.Extra[0].ID == p.Extra[1].ID || p.InSync() {
		t.Errorf("Extra = %+v, InSync = %v", p.Extra, p.InSync())
	}
	partial := BuildPlan([]Matched{m(1, "A")}, existing, false)
	if partial.Extra != nil || partial.ExtraKnown || partial.InSync() {
		t.Errorf("partial run must not judge extras: %+v", partial)
	}
}

// The author swapped the clip matched for "Mentiras" for its Topic upload by
// hand (todo 2.17): keep the Topic, do not add the clip back, remove nothing.
func TestBuildPlanKeepsManualReplacement(t *testing.T) {
	existing := []youtube.Item{
		{ID: "i1", VideoID: "hall", Title: "Hallelujah"},
		{ID: "i2", VideoID: "topic", Title: "Mentiras"},
		{ID: "i3", VideoID: "junk", Title: "Something else entirely"},
	}
	matched := []Matched{
		named(1, "hall", "Hallelujah"),
		named(2, "clip", "Mentiras (feat. Someone)"),
	}
	p := BuildPlan(matched, existing, true)
	if len(p.ToAdd) != 0 {
		t.Errorf("ToAdd = %v, the clip must not come back", ids(p.ToAdd))
	}
	if len(p.Replaced) != 1 || p.Replaced[0].Item.VideoID != "topic" || p.Replaced[0].Video.ID != "clip" {
		t.Errorf("Replaced = %+v", p.Replaced)
	}
	if !eq(videoIDs(p.Extra), []string{"junk"}) {
		t.Errorf("Extra = %v, want only the unrelated video", videoIDs(p.Extra))
	}
}

func TestBuildPlanReplacementNeedsAMeaningfulName(t *testing.T) {
	existing := []youtube.Item{{ID: "i1", VideoID: "x", Title: "OK Go - Here It Goes Again"}}
	p := BuildPlan([]Matched{named(1, "ok", "Ok")}, existing, true)
	if len(p.Replaced) != 0 || !eq(ids(p.ToAdd), []string{"ok"}) || len(p.Extra) != 1 {
		t.Errorf("a 2-letter name must not claim a video: %+v", p)
	}
}

func TestCoreTitle(t *testing.T) {
	for in, want := range map[string]string{
		"I'll Never Love Again - Film Version": "i'll never love again",
		"Déjenme Llorar (feat. Leonel García)": "déjenme llorar",
		"Hurt [Remastered]":                    "hurt",
		"Mentiras":                             "mentiras",
		"(What's the Story) Morning Glory?":    "(what's the story) morning glory?",
	} {
		if got := coreTitle(in); got != want {
			t.Errorf("coreTitle(%q) = %q, want %q", in, got, want)
		}
	}
}
