package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/CharlyCeballos/soundsync/internal/transfer"
)

const (
	wayID    = "5ZwLvrI8d6VnEQXxB0bMms"
	coversID = "3yQi6hff0Ic45VJfJoaXIn"
)

func video(id, title, channel string) map[string]any {
	return map[string]any{
		"id":      map[string]string{"videoId": id},
		"snippet": map[string]string{"title": title, "channelTitle": channel},
	}
}

func ytPlaylist(id, title, spotifyID string) map[string]any {
	return map[string]any{
		"id":      id,
		"snippet": map[string]string{"title": title, "description": transfer.Description(spotifyID)},
		"status":  map[string]string{"privacyStatus": "private"},
	}
}

// newAddFake seeds a channel that already has both Soundsync playlists.
func newAddFake() *fakeYouTube {
	return &fakeYouTube{
		playlists: []map[string]any{
			ytPlaylist("PLway", "The Way to My Favorite Part", wayID),
			ytPlaylist("PLcov", "Covers", coversID),
		},
		search: map[string][]map[string]any{
			"Ariana Grande The Way (feat. Mac Miller)": {
				video("vevoV", "The Way (Official Video)", "ArianaGrandeVevo"),
				video("lyricV", "The Way lyrics", "Lyrics Hub"),
				video("topicV", "The Way", "Ariana Grande - Topic"),
			},
			"Mac Miller My Favorite Part": {
				video("firstV", "Mac Miller - My Favorite Part (feat. Ariana Grande) &amp; more", "Some Uploader"),
			},
			"Jeff Buckley Hallelujah": {video("hallV", "Hallelujah", "Jeff Buckley - Topic")},
			"Johnny Cash Hurt":        {video("hurtV", "Hurt", "JohnnyCashVEVO")},
		},
		items: map[string][]string{},
	}
}

func runLines(t *testing.T, a *app, lines ...string) (string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	if err := runShell(a, strings.NewReader(strings.Join(lines, "\n")), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	return out.String(), errOut.String()
}

func mustContain(t *testing.T, label, got string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("%s missing %q:\n%s", label, w, got)
		}
	}
}

func TestYouTubeAddFlowAndIdempotence(t *testing.T) {
	yt := newAddFake()
	a := newTestApp(t, yt)
	out, errOut := runLines(t, a,
		"youtube add Way",
		"y",
		"youtube add Way", // second run: in sync, must not ask
		"youtube playlists",
		"exit",
	)

	// Topic wins over the VEVO video ranked above it.
	if got := yt.videos("PLway"); len(got) != 2 || got[0] != "topicV" || got[1] != "firstV" {
		t.Fatalf("playlist videos = %v, want [topicV firstV] in Spotify order\n%s\n%s", got, out, errOut)
	}
	if searches, inserts := yt.counts(); inserts != 2 || searches != 4 {
		t.Errorf("inserts=%d searches=%d, want 2 and 4", inserts, searches)
	}
	mustContain(t, "stdout", out,
		"Searching 2 track(s): 2 of the 100 daily searches.",
		"Ariana Grande, Mac Miller — The Way (feat. Mac Miller)",
		"[Ariana Grande - Topic]",
		"My Favorite Part (feat. Ariana Grande) & more", // HTML unescaped
		"About to add 2 video(s)",
		"+ [The Way to My Favorite Part] 2/2",
		"not searchable 1.",
		"Nothing to write.",
		`"The Way to My Favorite Part" (in sync): added 0, already there 2`,
		"Data from Spotify. Data from YouTube.",
	)
	if n := strings.Count(out, "Add them?"); n != 1 {
		t.Errorf("asked %d times, want 1", n)
	}
	if !strings.Contains(out, "Playlists on channel") {
		t.Errorf("line after an in-sync run was swallowed:\n%s", out)
	}
}

func TestYouTubeAddTwoPlaylistsInParallel(t *testing.T) {
	yt := newAddFake()
	a := newTestApp(t, yt)
	out, errOut := runLines(t, a, "youtube add Way Covers", "y", "exit")

	if got := yt.videos("PLway"); len(got) != 2 || got[0] != "topicV" || got[1] != "firstV" {
		t.Errorf("Way videos = %v, want [topicV firstV]", got)
	}
	if got := yt.videos("PLcov"); len(got) != 2 || got[0] != "hallV" || got[1] != "hurtV" {
		t.Errorf("Covers videos = %v, want [hallV hurtV]", got)
	}
	if n := strings.Count(out, "Add them?"); n != 1 {
		t.Errorf("one confirmation for both playlists, got %d", n)
	}
	mustContain(t, "stdout", out,
		"Searching 4 track(s) in parallel: 4 of the 100 daily searches.",
		"About to add 4 video(s)",
		`"The Way to My Favorite Part": 2`,
		`"Covers": 2`,
		"+ [Covers] 2/2 Hurt",
		`"Covers" (in sync): added 2`,
		"4 of 100 searches",
	)
	// Tables come out whole, in the order the playlists were given.
	if wayAt, covAt := strings.Index(out, "\n\"The Way to My Favorite Part\"\n"), strings.Index(out, "\n\"Covers\"\n"); wayAt < 0 || covAt < 0 || wayAt > covAt {
		t.Errorf("tables out of order (way %d, covers %d):\n%s", wayAt, covAt, out)
	}
	if errOut != "" {
		t.Errorf("unexpected stderr: %s", errOut)
	}
}

func TestYouTubeAddDecline(t *testing.T) {
	yt := newAddFake()
	a := newTestApp(t, yt)
	out, _ := runLines(t, a, "youtube add Way Covers", "n", "exit")
	if _, inserts := yt.counts(); inserts != 0 || !strings.Contains(out, "Cancelled. Nothing was added.") {
		t.Errorf("inserts = %d\n%s", inserts, out)
	}
}

func TestYouTubeAddNeedsCreateFirst(t *testing.T) {
	yt := newAddFake()
	yt.playlists = yt.playlists[:1] // only "The Way..." exists on YouTube
	a := newTestApp(t, yt)
	_, errOut := runLines(t, a, "youtube add Way Covers", "exit")
	mustContain(t, "stderr", errOut, `no YouTube playlist yet for "Covers"; run: youtube create "Covers"`)
	if searches, _ := yt.counts(); searches != 0 {
		t.Errorf("searches = %d, want 0 before every destination exists", searches)
	}
}

func TestYouTubeAddReportsAllBadArgsAtOnce(t *testing.T) {
	yt := newAddFake()
	a := newTestApp(t, yt)
	_, errOut := runLines(t, a, "youtube add Way nothing-like-this Pentatonix", "exit")
	mustContain(t, "stderr", errOut,
		`no playlist in your library matches "nothing-like-this"`,
		`"Pentatonix Evolution of Music" belongs to another user`,
		"tip: each argument is one playlist",
	)
	if searches, _ := yt.counts(); searches != 0 {
		t.Errorf("searches = %d, want 0", searches)
	}
}

func TestYouTubeAddSamePlaylistTwiceRunsOnce(t *testing.T) {
	yt := newAddFake()
	a := newTestApp(t, yt)
	runLines(t, a, "youtube add Way 5ZwLvrI8d6VnEQXxB0bMms -y", "exit")
	if searches, _ := yt.counts(); searches != 2 || len(yt.videos("PLway")) != 2 {
		t.Errorf("searches=%d videos=%v, want one run", searches, yt.videos("PLway"))
	}
}

func TestYouTubeAddQuotaStopsTheOtherPlaylist(t *testing.T) {
	yt := newAddFake()
	yt.failItemAt = 2 // quota runs out after the first insert, whichever playlist sends it
	a := newTestApp(t, yt)
	out, errOut := runLines(t, a, "youtube add Way Covers --yes", "exit")
	mustContain(t, "stdout", out, "(stopped)")
	mustContain(t, "stderr", errOut, "quotaExceeded", "it only adds what is missing")
	if total := len(yt.videos("PLway")) + len(yt.videos("PLcov")); total != 1 {
		t.Errorf("only the insert made before the quota ran out may land, got %d videos", total)
	}
}

func TestYouTubeAddLimit(t *testing.T) {
	yt := newAddFake()
	a := newTestApp(t, yt)
	out, errOut := runLines(t, a, "youtube add Way --limit 1 --yes", "youtube add Way --limit 0", "exit")
	if searches, _ := yt.counts(); searches != 1 || len(yt.videos("PLway")) != 1 {
		t.Errorf("searches=%d videos=%v, want 1 and 1", searches, yt.videos("PLway"))
	}
	mustContain(t, "stdout", out, "only the first 1 tracks")
	mustContain(t, "stderr", errOut, "--limit must be at least 1")
	if strings.Contains(out, "no longer in Spotify") || strings.Contains(out, "(in sync)") {
		t.Errorf("a partial run must not judge extras or claim sync:\n%s", out)
	}
}

func TestYouTubeCreateTwoPlaylistsOneExisting(t *testing.T) {
	yt := newAddFake()
	yt.playlists = yt.playlists[:1] // "The Way..." exists, "Covers" does not
	a := newTestApp(t, yt)
	out, _ := runLines(t, a, "youtube create Way Covers", "y", "exit")
	if yt.inserts != 1 {
		t.Fatalf("playlists created = %d, want 1 (Covers only)\n%s", yt.inserts, out)
	}
	mustContain(t, "stdout", out, "Already on YouTube as", "Create it?", `Created "Covers" (private).`)
}
