package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/CharlyCeballos/soundsync/internal/i18n"
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
		items: map[string][]fakeItem{},
		// Lengths match the Spotify fixture's, except the VEVO video's intro.
		lengths: map[string]string{"topicV": "PT3M47S", "vevoV": "PT4M10S", "firstV": "PT3M35S", "hallV": "PT6M53S", "hurtV": "PT3M38S"},
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
	// The second run finds both tracks in the playlist: no new searches.
	if searches, inserts := yt.counts(); inserts != 2 || searches != 2 {
		t.Errorf("inserts=%d searches=%d, want 2 and 2", inserts, searches)
	}
	mustContain(t, "stdout", out,
		"Searching 2 track(s): 2 of the 100 daily searches.",
		"2 track(s) need no search: already in the YouTube playlist, or matched earlier in this session.",
		"Ariana Grande, Mac Miller — The Way (feat. Mac Miller)",
		"[Ariana Grande - Topic]",
		"My Favorite Part (feat. Ariana Grande) & more", // HTML unescaped
		"About to change 1 playlist(s)",
		`"The Way to My Favorite Part": add 2 video(s)`,
		"+ [The Way to My Favorite Part] 2/2",
		"not searchable 1.",
		"Nothing to write.",
		`"The Way to My Favorite Part" (in sync): added 0, already there 2`,
		"Data from Spotify. Data from YouTube.",
	)
	if n := strings.Count(out, "Go ahead?"); n != 1 {
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
	if n := strings.Count(out, "Go ahead?"); n != 1 {
		t.Errorf("one confirmation for both playlists, got %d", n)
	}
	mustContain(t, "stdout", out,
		"Searching 4 track(s) in parallel: 4 of the 100 daily searches.",
		"About to change 2 playlist(s)",
		`"The Way to My Favorite Part": add 2 video(s)`,
		`"Covers": add 2 video(s)`,
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

func TestYouTubeAddRetriesMisses(t *testing.T) {
	yt := newAddFake()
	delete(yt.search, "Johnny Cash Hurt") // no result: must be searched again next run
	a := newTestApp(t, yt)
	out, _ := runLines(t, a,
		"youtube add Covers -y",
		"youtube add Covers -y", // Hallelujah is in the playlist now, Hurt is retried
		"exit",
	)
	if searches, _ := yt.counts(); searches != 3 {
		t.Errorf("searches = %d, want 3 (2 + the retried miss)\n%s", searches, out)
	}
	mustContain(t, "stdout", out,
		"1 track(s) need no search",
		"Searching 1 track(s): 1 of the 100 daily searches.",
	)
}

// Another day means a new process with an empty session cache: tracks already
// in the playlist (same title, artist and length) cost no search, so a long
// playlist keeps advancing at 100 searches a day.
func TestYouTubeAddNewSessionSkipsTracksAlreadyCopied(t *testing.T) {
	yt := newAddFake()
	runLines(t, newTestApp(t, yt), "youtube add Covers -y", "exit")
	yt.lengths["hurtV"] = "PT4M30S" // say the upload got a long intro: length no longer fits

	out, _ := runLines(t, newTestApp(t, yt), "youtube add Covers -y", "exit")
	if searches, inserts := yt.counts(); searches != 3 || inserts != 2 {
		t.Errorf("searches=%d inserts=%d, want 2+1 and 2 (Hurt searched again, found present)", searches, inserts)
	}
	mustContain(t, "stdout", out,
		"1 track(s) need no search",
		"Searching 1 track(s)",
		"Hallelujah [Jeff Buckley - Topic] https://www.youtube.com/watch?v=hallV  playlist",
		`"Covers" (in sync): added 0, already there 2`,
	)
}

func TestYouTubeAddDecline(t *testing.T) {
	yt := newAddFake()
	a := newTestApp(t, yt)
	out, _ := runLines(t, a, "youtube add Way Covers", "n", "exit")
	if _, inserts := yt.counts(); inserts != 0 || !strings.Contains(out, "Cancelled. Nothing was changed.") {
		t.Errorf("inserts = %d\n%s", inserts, out)
	}
}

// todo 2.12 + one-command flow: a missing (or deleted) destination is
// created in the same run, under the same confirmation.
func TestYouTubeAddCreatesMissingPlaylist(t *testing.T) {
	yt := newAddFake()
	yt.playlists = yt.playlists[:1] // only "The Way..." exists on YouTube
	a := newTestApp(t, yt)
	out, errOut := runLines(t, a, "youtube add Way Covers --privacy unlisted", "y", "exit")
	if yt.inserts != 1 {
		t.Fatalf("playlists created = %d, want 1\n%s\n%s", yt.inserts, out, errOut)
	}
	if got := yt.videos("PLfake1"); len(got) != 2 || got[0] != "hallV" || got[1] != "hurtV" {
		t.Errorf("new Covers videos = %v, want [hallV hurtV]", got)
	}
	if p := yt.playlists[len(yt.playlists)-1]["status"].(map[string]string)["privacyStatus"]; p != "unlisted" {
		t.Errorf("privacy = %q, want unlisted", p)
	}
	mustContain(t, "stdout", out,
		`"Covers" → a new playlist: 2 track(s) to match`,
		`"Covers": create it (unlisted), add 2 video(s)`,
		"250 units", // 50 create + 4 inserts
		"created [Covers] (unlisted)",
		`"Covers" (in sync): added 2`,
	)
	if n := strings.Count(out, "Go ahead?"); n != 1 {
		t.Errorf("asked %d times, want 1", n)
	}
}

// todo 2.5: a video no Spotify track matches is removed, after the same
// confirmation; a partial run (--limit) never removes anything.
func TestYouTubeAddRemovesExtras(t *testing.T) {
	yt := newAddFake()
	yt.seed("PLway", "topicV", "The Way", "junkV", "Totally Unrelated Song", "firstV", "My Favorite Part")
	a := newTestApp(t, yt)
	out, _ := runLines(t, a, "youtube add Way --limit 1 -y", "youtube add Way", "y", "exit")
	if got := yt.videos("PLway"); len(got) != 2 || got[0] != "topicV" || got[1] != "firstV" {
		t.Errorf("videos = %v, want junkV removed and order kept", got)
	}
	if yt.deletes != 1 {
		t.Errorf("deletes = %d, want 1", yt.deletes)
	}
	mustContain(t, "stdout", out,
		`To remove from "The Way to My Favorite Part" (no Spotify track matches them):`,
		"- Totally Unrelated Song https://www.youtube.com/watch?v=junkV",
		`"The Way to My Favorite Part": remove 1 video(s)`,
		"- [The Way to My Favorite Part] 1/1 Totally Unrelated Song",
		`"The Way to My Favorite Part" (in sync): added 0, already there 2, removed 1`,
	)
	if first := out[:strings.Index(out, "soundsync> \nSpotify")+1]; strings.Contains(first, "To remove") {
		t.Errorf("the --limit run must not judge extras")
	}
}

// todo 2.17: the author swapped a match for another upload by hand. The
// swap stays, the match is not added back, nothing is removed.
func TestYouTubeAddKeepsManualReplacement(t *testing.T) {
	yt := newAddFake()
	yt.seed("PLcov", "hallV", "Hallelujah", "ninV", "Nine Inch Nails - Hurt (Official Video)")
	a := newTestApp(t, yt)
	out, _ := runLines(t, a, "youtube add Covers", "exit")
	if got := yt.videos("PLcov"); len(got) != 2 || got[1] != "ninV" {
		t.Errorf("videos = %v, want the user's ninV kept", got)
	}
	if _, inserts := yt.counts(); inserts != 0 || yt.deletes != 0 {
		t.Errorf("inserts=%d deletes=%d, want none", inserts, yt.deletes)
	}
	mustContain(t, "stdout", out,
		"keep your video (Soundsync found https://www.youtube.com/watch?v=hurtV)",
		"Nothing to write.",
		`"Covers" (in sync): added 0, already there 1, removed 0, kept yours 1`,
	)
}

// todo 2.1: an unmarked playlist with the same title is adopted only if
// the user picks it; its videos stay that run and go on later runs.
func TestYouTubeAddAdoptsSameTitle(t *testing.T) {
	yt := newAddFake()
	yt.playlists[1] = map[string]any{
		"id":      "PLmine",
		"snippet": map[string]string{"title": "Covers", "description": "My own covers"},
		"status":  map[string]string{"privacyStatus": "public"},
	}
	yt.seed("PLmine", "otherV", "Some Other Song")
	a := newTestApp(t, yt)
	out, errOut := runLines(t, a,
		"youtube add Covers", "1", "y", // adopt, then confirm
		"youtube add Covers -y", // now marked: the old video goes
		"exit",
	)
	desc := yt.playlists[1]["snippet"].(map[string]string)["description"]
	if !strings.HasPrefix(desc, "My own covers\n\n") || len(transfer.MarkerIDs(desc)) != 1 {
		t.Errorf("description = %q, want user text + marker", desc)
	}
	if yt.inserts != 0 || yt.updates != 1 {
		t.Errorf("created=%d updated=%d, want 0 and 1", yt.inserts, yt.updates)
	}
	if got := yt.videos("PLmine"); len(got) != 2 || got[0] != "hallV" || got[1] != "hurtV" {
		t.Errorf("videos = %v, want [hallV hurtV] after the second run", got)
	}
	mustContain(t, "stdout", out,
		`"Covers": your channel already has 1 playlist(s) with this title that Soundsync did not create:`,
		`1. "Covers" (public, 0 videos)`,
		`"Covers": use the existing one and add the Soundsync line to its description, add 2 video(s)`,
		"marked [Covers] as a Soundsync playlist",
		"Its other videos were kept.",
		"- [Covers] 1/1 Some Other Song",
	)
	if errOut != "" {
		t.Errorf("unexpected stderr: %s", errOut)
	}
}

func TestYouTubeAddSameTitleWithYesStopsBeforeSearching(t *testing.T) {
	yt := newAddFake()
	yt.playlists[1]["snippet"] = map[string]string{"title": "Covers", "description": "no marker"}
	a := newTestApp(t, yt)
	_, errOut := runLines(t, a, "youtube add Way Covers -y", "exit")
	mustContain(t, "stderr", errOut, `already has a playlist called "Covers" that Soundsync did not create; run the command without --yes`)
	if searches, inserts := yt.counts(); searches != 0 || inserts != 0 {
		t.Errorf("searches=%d inserts=%d, want 0 and 0", searches, inserts)
	}
}

func TestYouTubeAddSameTitleDeclinedCreatesNew(t *testing.T) {
	yt := newAddFake()
	yt.playlists[1]["snippet"] = map[string]string{"title": "Covers", "description": "no marker"}
	a := newTestApp(t, yt)
	out, _ := runLines(t, a, "youtube add Covers", "", "y", "exit")
	if yt.inserts != 1 || yt.updates != 0 {
		t.Errorf("created=%d updated=%d, want a new playlist and no adoption", yt.inserts, yt.updates)
	}
	mustContain(t, "stdout", out, "A new playlist will be created.", `"Covers": create it (private), add 2 video(s)`)
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
	out, errOut := runLines(t, a, "youtube add Way --limit 1 --yes", "youtube add Way --limit -1", "youtube add Way --max-searches -1", "exit")
	if searches, _ := yt.counts(); searches != 1 || len(yt.videos("PLway")) != 1 {
		t.Errorf("searches=%d videos=%v, want 1 and 1", searches, yt.videos("PLway"))
	}
	mustContain(t, "stdout", out, "only the first 1 tracks (--limit)")
	mustContain(t, "stderr", errOut, "--limit must be 0 (all tracks) or more", "--max-searches must be 0 or more")
	if strings.Contains(out, "To remove") || strings.Contains(out, "(in sync)") {
		t.Errorf("a partial run must not judge extras or claim sync:\n%s", out)
	}
}

// todo 2.8: when the searches run out, what was found is still written and
// the rest waits; extras are not judged on a partial run.
func TestYouTubeAddMaxSearches(t *testing.T) {
	yt := newAddFake()
	yt.seed("PLcov", "junkV", "Unrelated")
	a := newTestApp(t, yt)
	out, _ := runLines(t, a, "youtube add Covers --max-searches 1 -y", "exit")
	searches, inserts := yt.counts()
	if searches != 1 || inserts != 1 || yt.deletes != 0 {
		t.Errorf("searches=%d inserts=%d deletes=%d, want 1, 1, 0", searches, inserts, yt.deletes)
	}
	mustContain(t, "stdout", out,
		"Searching 1 of 2 track(s): 1 search(es) left in this session (--max-searches)",
		"1 track(s) not searched yet",
		"1 track(s) wait for the next run",
	)
	if strings.Contains(out, "To remove") {
		t.Errorf("a run that left tracks unsearched must not remove anything:\n%s", out)
	}
}

// YouTube refusing a search for quota no longer throws the found matches away.
func TestYouTubeAddSearchQuotaStillWritesWhatWasFound(t *testing.T) {
	yt := newAddFake()
	yt.failSearch = 2 // the 2nd search hits an empty bucket
	a := newTestApp(t, yt)
	out, errOut := runLines(t, a, "youtube add Covers -y", "exit")
	if got := yt.videos("PLcov"); len(got) != 1 || got[0] != "hallV" {
		t.Errorf("videos = %v, want the one match found before the quota ran out", got)
	}
	mustContain(t, "stdout", out, "YouTube says today's searches are used up", "1 track(s) wait for the next run")
	if errOut != "" {
		t.Errorf("a search quota stop is not an error now:\n%s", errOut)
	}
}

// todo 2.7: a YouTube playlist holds at most maxPlaylistItems videos.
func TestYouTubeAddCapsAtPlaylistLimit(t *testing.T) {
	old := maxPlaylistItems
	maxPlaylistItems = 1
	t.Cleanup(func() { maxPlaylistItems = old })
	yt := newAddFake()
	a := newTestApp(t, yt)
	out, _ := runLines(t, a, "youtube add Covers -y", "exit")
	if got := yt.videos("PLcov"); len(got) != 1 {
		t.Errorf("videos = %v, want 1 (the cap)", got)
	}
	mustContain(t, "stdout", out, "only the first 1 tracks: a YouTube playlist holds at most 1 videos.")
}

func TestYouTubeCreateTwoPlaylistsOneExisting(t *testing.T) {
	yt := newAddFake()
	yt.playlists = yt.playlists[:1] // "The Way..." exists, "Covers" does not
	a := newTestApp(t, yt)
	out, _ := runLines(t, a, "youtube create Way Covers", "y", "exit")
	if yt.inserts != 1 {
		t.Fatalf("playlists created = %d, want 1 (Covers only)\n%s", yt.inserts, out)
	}
	mustContain(t, "stdout", out, "Already on YouTube as", "Go ahead?", `Created "Covers" (private).`)
}

// todo 2.18: the artist's own Topic deep in the results wins when its length
// fits; otherwise only the top results are ranked, as before.
func TestYouTubeAddFindsOwnTopicBeyondTopResults(t *testing.T) {
	for _, c := range []struct {
		length, want string
	}{
		{"PT3M38S", "cashTopic"}, // album length: the Topic upload wins
		{"PT5M02S", "hurtV"},     // another recording: classic ranking over the top 5
	} {
		yt := newAddFake()
		results := []map[string]any{video("hurtV", "Hurt", "JohnnyCashVEVO")}
		for i := range 5 {
			results = append(results, video(fmt.Sprintf("x%d", i), "Hurt cover", "Someone"))
		}
		yt.search["Johnny Cash Hurt"] = append(results, video("cashTopic", "Hurt", "Johnny Cash - Topic"))
		yt.lengths["cashTopic"] = c.length
		runLines(t, newTestApp(t, yt), "youtube add Covers -y", "exit")
		if got := yt.videos("PLcov"); len(got) != 2 || got[1] != c.want {
			t.Errorf("Topic length %s: videos = %v, want %s second", c.length, got, c.want)
		}
	}
}

// todo 2.9: --lang es switches the whole flow, and "s" confirms.
func TestYouTubeAddInSpanish(t *testing.T) {
	t.Cleanup(func() { i18n.Set(i18n.EN) })
	yt := newAddFake()
	out, _ := runLines(t, newTestApp(t, yt), "youtube add Covers --lang es", "s", "exit")
	if _, inserts := yt.counts(); inserts != 2 {
		t.Errorf("inserts = %d, want 2: \"s\" must confirm in Spanish", inserts)
	}
	mustContain(t, "stdout", out,
		"Spotify → YouTube en el canal",
		"Buscando 2 track(s)",
		"POR QUÉ",
		"¿Continuar? [s/N]",
		"Resumen:",
		`"Covers" (sincronizada): agregados 2`,
	)
}
