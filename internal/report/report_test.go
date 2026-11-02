package report

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CharlyCeballos/soundsync/internal/copier"
	"github.com/CharlyCeballos/soundsync/internal/fakeapi"
	"github.com/CharlyCeballos/soundsync/internal/session"
	"github.com/CharlyCeballos/soundsync/internal/spotify"
	"github.com/CharlyCeballos/soundsync/internal/youtube"
)

// plannedRun plans "The Way to My Favorite Part" against a channel whose
// playlist holds one unrelated video, so the plan adds two and removes one.
func plannedRun(t *testing.T) (*session.YouTube, []*copier.Job) {
	t.Helper()
	ch := fakeapi.NewChannel()
	ch.Seed("PLway", "junkV", "Totally Unrelated Song")
	spSrv, ytSrv := httptest.NewServer(fakeapi.NewLibrary()), httptest.NewServer(ch)
	t.Cleanup(spSrv.Close)
	t.Cleanup(ytSrv.Close)
	sp := spotify.NewClient(spSrv.Client()).WithBaseURL(spSrv.URL)
	yt := &session.YouTube{Client: youtube.NewClient(ytSrv.Client()).WithBaseURL(ytSrv.URL)}

	ctx := t.Context()
	pls, err := yt.Client.MyPlaylists(ctx)
	if err != nil {
		t.Fatal(err)
	}
	src := spotify.Playlist{ID: fakeapi.WayID, Name: "The Way to My Favorite Part"}
	jobs, err := copier.Prepare(ctx, sp, copier.Resolve(pls, []spotify.Playlist{src}), copier.Options{})
	if err != nil {
		t.Fatal(err)
	}
	copier.LookUp(ctx, yt, jobs, nil)
	copier.Match(ctx, yt, jobs, copier.NewBudget(100, 0), nil)
	return yt, jobs
}

func statuses(rows []Row) []Status {
	out := make([]Status, len(rows))
	for i, r := range rows {
		out[i] = r.Status
	}
	return out
}

func TestRowsBeforeAndAfterWriting(t *testing.T) {
	yt, jobs := plannedRun(t)
	before := Rows(jobs)
	want := []Status{ToAdd, Episode, ToAdd, ToRemove}
	if got := statuses(before); !equal(got, want) {
		t.Fatalf("before writing: %v, want %v", got, want)
	}
	if r := before[0]; r.Position != 1 || r.SpotifyURL != "https://open.spotify.com/track/1Onx" || r.YouTubeURL != "https://www.youtube.com/watch?v=topicV" || r.Why != "topic" {
		t.Errorf("first row: %+v", r)
	}
	if r := before[3]; r.Position != 0 || r.SpotifyTrack != "" || r.YouTubeTitle != "Totally Unrelated Song" {
		t.Errorf("a removed video has no Spotify side: %+v", r)
	}

	copier.Write(t.Context(), yt, jobs, youtube.PrivacyPrivate, nil)
	if got := statuses(Rows(jobs)); !equal(got, []Status{Added, Episode, Added, Removed}) {
		t.Errorf("after writing: %v", got)
	}
}

func equal(a, b []Status) bool {
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

func TestWriteFormats(t *testing.T) {
	rows := []Row{{Playlist: "=cmd|' /C calc'!A0", Position: 1, Status: Added, YouTubeTitle: "@title"}}

	var csvOut bytes.Buffer
	if err := Write(&csvOut, CSV, rows); err != nil {
		t.Fatal(err)
	}
	records, err := csv.NewReader(&csvOut).ReadAll()
	if err != nil || len(records) != 2 {
		t.Fatalf("records=%v err=%v", records, err)
	}
	if got := records[1]; got[0] != "'=cmd|' /C calc'!A0" || got[6] != "'@title" || got[10] != Attribution {
		t.Errorf("csv row = %q", got)
	}

	var jsonOut bytes.Buffer
	if err := Write(&jsonOut, JSON, rows); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Attribution string
		Rows        []Row
	}
	if err := json.Unmarshal(jsonOut.Bytes(), &doc); err != nil || doc.Attribution != Attribution || doc.Rows[0].Status != Added {
		t.Errorf("json = %s, err %v", jsonOut.String(), err)
	}
}

func TestCellDefusesFormulas(t *testing.T) {
	for in, want := range map[string]string{
		"=HYPERLINK(\"x\")": "'=HYPERLINK(\"x\")",
		"+1":                "'+1",
		"-Topic":            "'-Topic",
		"@cmd":              "'@cmd",
		"Hallelujah":        "Hallelujah",
		"":                  "",
	} {
		if got := cell(in); got != want {
			t.Errorf("cell(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSaveNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "run.CSV")
	if err := Save(path, nil); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, nil); !errors.Is(err, fs.ErrExist) {
		t.Errorf("second save: err = %v, want fs.ErrExist", err)
	}
	if b, _ := os.ReadFile(path); !strings.HasPrefix(string(b), "playlist,position") {
		t.Errorf("file = %q", b)
	}
	if err := Save(filepath.Join(dir, "run.txt"), nil); !errors.Is(err, ErrFormat) {
		t.Errorf("txt: err = %v, want ErrFormat", err)
	}
}
