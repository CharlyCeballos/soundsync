package main

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestYouTubeAddReportCSV(t *testing.T) {
	yt := newAddFake()
	yt.seed("PLway", "junkV", "Totally Unrelated Song")
	yt.seed("PLcov", "ninV", "Nine Inch Nails - Hurt (Official Video)")
	path := filepath.Join(t.TempDir(), "run.csv")
	out, errOut := runLines(t, newTestApp(t, yt), "youtube add Way Covers -y --report "+path, "exit")
	if errOut != "" {
		t.Fatalf("stderr: %s", errOut)
	}
	if !strings.Contains(out, "Report saved to "+path+" (6 rows).") {
		t.Errorf("no save confirmation:\n%s", out)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(records[0], ",") != strings.Join(reportHeader, ",") {
		t.Errorf("header = %v", records[0])
	}
	// playlist, position, track, status, youtube url
	want := [][5]string{
		{"The Way to My Favorite Part", "1", "The Way (feat. Mac Miller)", "added", "https://www.youtube.com/watch?v=topicV"},
		{"The Way to My Favorite Part", "2", "", "not searchable: podcast episode", ""},
		{"The Way to My Favorite Part", "3", "My Favorite Part", "added", "https://www.youtube.com/watch?v=firstV"},
		{"The Way to My Favorite Part", "", "", "removed", "https://www.youtube.com/watch?v=junkV"},
		{"Covers", "1", "Hallelujah", "added", "https://www.youtube.com/watch?v=hallV"},
		{"Covers", "2", "Hurt", "kept yours", "https://www.youtube.com/watch?v=ninV"},
	}
	if len(records) != len(want)+1 {
		t.Fatalf("rows = %d, want %d:\n%v", len(records)-1, len(want), records)
	}
	for i, w := range want {
		r := records[i+1]
		got := [5]string{r[0], r[1], r[3], r[5], r[8]}
		if w[2] == "" {
			got[2] = "" // the episode's name is not checked
		}
		if got != w {
			t.Errorf("row %d = %v, want %v", i+1, got, w)
		}
		if r[10] != reportAttribution {
			t.Errorf("row %d attribution = %q", i+1, r[10])
		}
	}
	if records[1][4] != "https://open.spotify.com/track/1Onx" {
		t.Errorf("Spotify link back missing: %q", records[1][4])
	}
}

func TestYouTubeAddReportJSONEvenWhenCancelled(t *testing.T) {
	yt := newAddFake()
	path := filepath.Join(t.TempDir(), "run.json")
	runLines(t, newTestApp(t, yt), "youtube add Covers --report "+path, "n", "exit")
	var got struct {
		Attribution string      `json:"attribution"`
		Rows        []reportRow `json:"rows"`
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Attribution != reportAttribution || len(got.Rows) != 2 || got.Rows[0].Status != "to add (not done)" {
		t.Errorf("unexpected report %+v", got)
	}
}

func TestYouTubeAddReportPathCheckedBeforeLogin(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "old.csv")
	os.WriteFile(existing, []byte("keep me"), 0o600)
	yt := newAddFake()
	_, errOut := runLines(t, newTestApp(t, yt),
		"youtube add Covers --report "+existing,
		"youtube add Covers --report "+filepath.Join(dir, "run.txt"),
		"exit")
	mustContain(t, "stderr", errOut, "already exists; choose another file name", "use a .csv or .json file name")
	if searches, _ := yt.counts(); searches != 0 {
		t.Errorf("searches = %d, want 0", searches)
	}
	if b, _ := os.ReadFile(existing); string(b) != "keep me" {
		t.Error("an existing file must never be overwritten")
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
