package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/CharlyCeballos/soundsync/internal/copier"
	"github.com/CharlyCeballos/soundsync/internal/i18n"
	"github.com/CharlyCeballos/soundsync/internal/spotify"
	"github.com/CharlyCeballos/soundsync/internal/youtube"
)

// chooseDestinations finds each source's YouTube playlist. The only question
// asked is about identity (planV1 §13): whether an unmarked playlist with the
// same title is the one to use. --yes cannot answer it, so with --yes such a
// case stops everything before any quota is spent on searches.
func chooseDestinations(out io.Writer, a *app, playlists []youtube.Playlist, srcs []spotify.Playlist, yes bool) ([]copier.Destination, error) {
	dests := copier.Resolve(playlists, srcs)
	var ambiguous []string
	for _, d := range dests {
		for _, dup := range d.Duplicates {
			fmt.Fprintf(out, i18n.T("warning: %q (%s) carries the same Soundsync marker as another playlist; only the first one is used.\n"), dup.Title, dup.URL())
		}
		if len(d.Candidates) > 0 {
			ambiguous = append(ambiguous, d.Source.Name)
		}
	}
	if yes && len(ambiguous) > 0 {
		return nil, fmt.Errorf(i18n.T("your channel already has a playlist called %s that Soundsync did not create; run the command without --yes to choose whether to use it"), quoteAll(ambiguous))
	}
	for i := range dests {
		d := &dests[i]
		if len(d.Candidates) == 0 {
			continue
		}
		if p, ok := askAdopt(out, a, d.Source.Name, d.Candidates); ok {
			if err := d.Adopt(p.ID); err != nil {
				return nil, err
			}
		}
	}
	return dests, nil
}

// askAdopt lists the same-title playlists and lets the user pick one by
// number. Anything else means "create a new playlist"; the final
// confirmation still shows that before anything is written.
func askAdopt(out io.Writer, a *app, name string, cands []youtube.Playlist) (youtube.Playlist, bool) {
	fmt.Fprintf(out, i18n.T("\n%q: your channel already has %d playlist(s) with this title that Soundsync did not create:\n"), name, len(cands))
	for i, p := range cands {
		fmt.Fprintf(out, i18n.T("  %d. %q (%s, %d videos) %s\n"), i+1, p.Title, p.Privacy, p.ItemCount, p.URL())
	}
	fmt.Fprintln(out, i18n.T("Use one? Soundsync adds a line to its description and copies the missing tracks into it; the videos already there stay."))
	choices := "1"
	if len(cands) > 1 {
		choices = "1-" + strconv.Itoa(len(cands))
	}
	fmt.Fprintf(out, i18n.T("Type its number (%s), or press Enter for a new playlist: "), choices)
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
	fmt.Fprintln(out, i18n.T("A new playlist will be created."))
	return youtube.Playlist{}, false
}

// destAction describes, for the confirmation, what happens to the playlist.
func destAction(d copier.Destination, privacy string) string {
	switch d.Kind {
	case copier.Create:
		return fmt.Sprintf(i18n.T("create it (%s)"), privacy)
	case copier.Adopt:
		return i18n.T("use the existing one and add the Soundsync line to its description")
	}
	return ""
}
