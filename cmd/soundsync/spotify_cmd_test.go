package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/CharlyCeballos/soundsync/internal/spotify"
)

func TestParsePlaylistRef(t *testing.T) {
	const id = "37i9dQZF1DXcBWIGoYBM5M"
	ok := []string{
		id,
		"  " + id + "  ",
		"spotify:playlist:" + id,
		"https://open.spotify.com/playlist/" + id,
		"https://open.spotify.com/playlist/" + id + "?si=abc123&pt=xyz",
		"https://open.spotify.com/intl-es/playlist/" + id,
	}
	for _, ref := range ok {
		got, err := parsePlaylistRef(ref)
		if err != nil || got != id {
			t.Errorf("parsePlaylistRef(%q) = %q, %v; want %q", ref, got, err, id)
		}
	}
	bad := []string{
		"",
		"short",
		"https://open.spotify.com/track/" + id,
		"https://evil.example.com/playlist/" + id,
		"spotify:track:" + id,
		id + "/../../me",
	}
	for _, ref := range bad {
		if got, err := parsePlaylistRef(ref); err == nil {
			t.Errorf("parsePlaylistRef(%q) = %q, want error", ref, got)
		}
	}
}

func TestPrintPlaylists(t *testing.T) {
	var out bytes.Buffer
	printPlaylists(&out, []spotify.Playlist{
		{ID: "p1", Name: "Rolas pa'l camino", OwnerID: "me", Total: 12},
		{ID: "p2", Name: "Top 50 México", OwnerID: "spotify", Total: -1},
	}, "me")
	got := out.String()
	for _, want := range []string{
		"Rolas pa'l camino", "Top 50 México", "owned", "followed", "?",
		"https://open.spotify.com/playlist/p1",
		"1 followed playlist(s)", attribution,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}

func TestPrintEntries(t *testing.T) {
	var out bytes.Buffer
	printEntries(&out, spotify.Playlist{ID: "p1", Name: "Tristes 😢😓"}, []spotify.Entry{
		{Position: 1, Track: spotify.Track{ID: "t1", Name: "Oye Mi Amor", Artists: []string{"Maná"}, ISRC: "USWB19200001"}},
		{Position: 2, Skip: spotify.SkipEpisode},
		{Position: 3, Track: spotify.Track{Name: "Demo casera", Artists: []string{"Charly"}, IsLocal: true}},
		{Position: 4, Skip: spotify.SkipUnavailable},
	})
	got := out.String()
	for _, want := range []string{
		`"Tristes 😢😓" (https://open.spotify.com/playlist/p1)`,
		"Maná — Oye Mi Amor", "USWB19200001", "https://open.spotify.com/track/t1",
		"(skipped: podcast episode)", "(local file)", "(skipped: unavailable)",
		"Tracks: 2 (local files: 1, without ISRC: 1). Skipped: 1 episode(s), 1 unavailable.",
		attribution,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}
