package transfer

import (
	"testing"

	"github.com/CharlyCeballos/soundsync/internal/spotify"
	"github.com/CharlyCeballos/soundsync/internal/youtube"
)

func m(pos int, videoID string) Matched {
	return Matched{
		Entry: spotify.Entry{Position: pos, Track: spotify.Track{Name: "t"}},
		Video: youtube.Video{ID: videoID},
		Found: videoID != "",
	}
}

func ids(ms []Matched) []string {
	var out []string
	for _, x := range ms {
		out = append(out, x.Video.ID)
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
	p := BuildPlan([]Matched{m(1, "A"), m(2, "B"), m(3, "C")}, []string{"B", "A"}, true)
	if !eq(ids(p.ToAdd), []string{"C"}) || !eq(ids(p.Present), []string{"A", "B"}) {
		t.Errorf("ToAdd=%v Present=%v", ids(p.ToAdd), ids(p.Present))
	}
}

func TestBuildPlanSameSetDifferentOrderIsInSync(t *testing.T) {
	p := BuildPlan([]Matched{m(1, "A"), m(2, "B")}, []string{"B", "A"}, true)
	if !p.InSync() {
		t.Errorf("same set in another order must be in sync: %+v", p)
	}
}

func TestBuildPlanExtraOnlyWhenComplete(t *testing.T) {
	existing := []string{"A", "X", "X"}
	p := BuildPlan([]Matched{m(1, "A")}, existing, true)
	if !eq(p.Extra, []string{"X"}) || p.InSync() {
		t.Errorf("Extra = %v, InSync = %v", p.Extra, p.InSync())
	}
	partial := BuildPlan([]Matched{m(1, "A")}, existing, false)
	if partial.Extra != nil || partial.ExtraKnown || partial.InSync() {
		t.Errorf("partial run must not judge extras: %+v", partial)
	}
}
