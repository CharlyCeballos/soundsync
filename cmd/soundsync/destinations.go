package main

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/CharlyCeballos/soundsync/internal/spotify"
	"github.com/CharlyCeballos/soundsync/internal/transfer"
	"github.com/CharlyCeballos/soundsync/internal/youtube"
)

// destKind says what happens to a YouTube playlist before tracks go in.
type destKind int

const (
	destExisting destKind = iota // carries this playlist's marker: use it
	destCreate                   // nothing found (or it was deleted): create it (todo 2.12)
	destAdopt                    // same title, no marker, and the user chose it (todo 2.1)
)

// destination is the YouTube playlist chosen for one Spotify playlist.
type destination struct {
	src    spotify.Playlist
	kind   destKind
	target youtube.Playlist // zero for destCreate until it is created
}

// chooseDestinations finds each source's YouTube playlist. The only question
// asked is about identity (planV1 §13): whether an unmarked playlist with the
// same title is the one to use. --yes cannot answer it, so with --yes such a
// case stops everything before any quota is spent on searches.
func chooseDestinations(out io.Writer, a *app, playlists []youtube.Playlist, srcs []spotify.Playlist, yes bool) ([]destination, error) {
	dests := make([]destination, len(srcs))
	candidates := make([][]youtube.Playlist, len(srcs))
	var ambiguous []string
	for i, src := range srcs {
		d := transfer.ResolveDestination(playlists, src.ID, src.Name)
		for _, dup := range d.Duplicates {
			fmt.Fprintf(out, "warning: %q (%s) carries the same Soundsync marker as another playlist; only the first one is used.\n", dup.Title, dup.URL())
		}
		dests[i] = destination{src: src, kind: destCreate}
		switch {
		case d.Existing != nil:
			dests[i] = destination{src: src, kind: destExisting, target: *d.Existing}
		case len(d.SameTitle) > 0:
			candidates[i] = d.SameTitle
			ambiguous = append(ambiguous, src.Name)
		}
	}
	if yes && len(ambiguous) > 0 {
		return nil, fmt.Errorf("your channel already has a playlist called %s that Soundsync did not create; run the command without --yes to choose whether to use it", quoteAll(ambiguous))
	}
	for i := range dests {
		if len(candidates[i]) == 0 {
			continue
		}
		if p, ok := askAdopt(out, a, dests[i].src.Name, candidates[i]); ok {
			dests[i].kind, dests[i].target = destAdopt, p
		}
	}
	return dests, nil
}

// askAdopt lists the same-title playlists and lets the user pick one by
// number. Anything else means "create a new playlist"; the final
// confirmation still shows that before anything is written.
func askAdopt(out io.Writer, a *app, name string, cands []youtube.Playlist) (youtube.Playlist, bool) {
	fmt.Fprintf(out, "\n%q: your channel already has %d playlist(s) with this title that Soundsync did not create:\n", name, len(cands))
	for i, p := range cands {
		fmt.Fprintf(out, "  %d. %q (%s, %d videos) %s\n", i+1, p.Title, p.Privacy, p.ItemCount, p.URL())
	}
	fmt.Fprintln(out, "Use one? Soundsync adds a line to its description and copies the missing tracks into it; the videos already there stay.")
	choices := "1"
	if len(cands) > 1 {
		choices = "1-" + strconv.Itoa(len(cands))
	}
	fmt.Fprintf(out, "Type its number (%s), or press Enter for a new playlist: ", choices)
	line, err := a.readLine()
	if err != nil {
		fmt.Fprintln(out)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	if len(cands) == 1 && isYes(answer) {
		answer = "1"
	}
	if n, err := strconv.Atoi(answer); err == nil && n >= 1 && n <= len(cands) {
		return cands[n-1], true
	}
	fmt.Fprintln(out, "A new playlist will be created.")
	return youtube.Playlist{}, false
}

// ensureDestination creates or adopts the playlist; an existing one needs
// nothing. Privacy applies only to a new playlist (Policies III.E.3.f).
func ensureDestination(ctx context.Context, yc *youtube.Client, d *destination, privacy string) error {
	switch d.kind {
	case destCreate:
		p, err := yc.CreatePlaylist(ctx, d.src.Name, transfer.Description(d.src.ID), privacy)
		if err != nil {
			return err
		}
		d.target = p
	case destAdopt:
		desc := transfer.AdoptedDescription(d.target.Description, d.src.ID)
		if err := yc.SetDescription(ctx, d.target, desc); err != nil {
			return err
		}
		d.target.Description = desc
	}
	return nil
}

// destCost is the quota needed to get the destination ready.
func (d destination) destCost() int {
	if d.kind == destExisting {
		return 0
	}
	return youtube.WriteUnits // playlists.insert or playlists.update
}

// destAction describes, for the confirmation, what happens to the playlist.
func (d destination) destAction(privacy string) string {
	switch d.kind {
	case destCreate:
		return fmt.Sprintf("create it (%s)", privacy)
	case destAdopt:
		return "use the existing one and add the Soundsync line to its description"
	}
	return ""
}
