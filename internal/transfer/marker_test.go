package transfer

import (
	"strings"
	"testing"

	"github.com/CharlyCeballos/soundsync/internal/youtube"
)

const spID = "5ZwLvrI8d6VnEQXxB0bMms"

func TestDescriptionCarriesMarker(t *testing.T) {
	d := Description(spID)
	ids := MarkerIDs(d)
	if len(ids) != 1 || ids[0] != spID {
		t.Fatalf("MarkerIDs(Description) = %v", ids)
	}
	if len(d) > 5000 {
		t.Errorf("description too long for YouTube: %d", len(d))
	}
}

func TestMarkerIDsExactID(t *testing.T) {
	// A longer ID must not match a shorter one by prefix.
	ids := MarkerIDs("notes\nsoundsync:spotify:" + spID + "XYZ\nmore")
	if len(ids) != 1 || ids[0] != spID+"XYZ" {
		t.Errorf("got %v", ids)
	}
	if ids := MarkerIDs("no marker here"); len(ids) != 0 {
		t.Errorf("got %v", ids)
	}
}

func TestResolveDestination(t *testing.T) {
	title := "The Way to My Favorite Part"
	marked := youtube.Playlist{ID: "PL1", Title: "Renamed by user", Description: "x\n" + Marker(spID)}
	other := youtube.Playlist{ID: "PL2", Title: "Other", Description: Marker("SomeOtherPlaylistIDxx")}
	sameTitle := youtube.Playlist{ID: "PL3", Title: title}
	sameTitleOtherMarker := youtube.Playlist{ID: "PL4", Title: title, Description: Marker("SomeOtherPlaylistIDxx")}
	dup := youtube.Playlist{ID: "PL5", Title: title, Description: Marker(spID)}

	t.Run("marker wins even after rename", func(t *testing.T) {
		d := ResolveDestination([]youtube.Playlist{other, sameTitle, marked}, spID, title)
		if d.Existing == nil || d.Existing.ID != "PL1" || len(d.SameTitle) != 0 {
			t.Errorf("got %+v", d)
		}
	})
	t.Run("same title without marker is a candidate", func(t *testing.T) {
		d := ResolveDestination([]youtube.Playlist{other, sameTitle, sameTitleOtherMarker}, spID, title)
		if d.Existing != nil || len(d.SameTitle) != 1 || d.SameTitle[0].ID != "PL3" {
			t.Errorf("got %+v", d)
		}
	})
	t.Run("nothing found means create", func(t *testing.T) {
		d := ResolveDestination([]youtube.Playlist{other}, spID, title)
		if d.Existing != nil || len(d.SameTitle) != 0 || len(d.Duplicates) != 0 {
			t.Errorf("got %+v", d)
		}
	})
	t.Run("duplicates are reported", func(t *testing.T) {
		d := ResolveDestination([]youtube.Playlist{marked, dup}, spID, title)
		if d.Existing == nil || d.Existing.ID != "PL1" || len(d.Duplicates) != 1 || d.Duplicates[0].ID != "PL5" {
			t.Errorf("got %+v", d)
		}
	})
	if !strings.HasPrefix(Marker(spID), MarkerPrefix) {
		t.Error("marker prefix")
	}
}
