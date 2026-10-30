package transfer

import (
	"regexp"

	"soundsync/internal/youtube"
)

// MarkerPrefix tags a YouTube playlist with the Spotify playlist it mirrors
// (planV1 §5). The marker lives in the playlist description and is the
// destination's identity: Soundsync keeps no local mapping file.
const MarkerPrefix = "soundsync:spotify:"

var markerRE = regexp.MustCompile(`(?m)` + regexp.QuoteMeta(MarkerPrefix) + `([0-9A-Za-z]+)`)

// Marker returns the marker for a Spotify playlist ID.
func Marker(spotifyID string) string { return MarkerPrefix + spotifyID }

// MarkerIDs returns every Spotify playlist ID marked in a description.
func MarkerIDs(description string) []string {
	var ids []string
	for _, m := range markerRE.FindAllStringSubmatch(description, -1) {
		ids = append(ids, m[1])
	}
	return ids
}

// Description is the text Soundsync writes on playlists it creates.
func Description(spotifyID string) string {
	return "Copied from a Spotify playlist with Soundsync.\n" +
		"Keep the next line: it lets Soundsync update this playlist instead of creating a new one.\n" +
		Marker(spotifyID)
}

// Destination is where a Spotify playlist should go on YouTube.
type Destination struct {
	// Existing is the playlist already marked for this Spotify playlist, if any.
	Existing *youtube.Playlist
	// SameTitle lists unmarked playlists with the same title. Adopting one of
	// them needs the user's consent (planV1 §6.1 step 4, todo 2.1).
	SameTitle []youtube.Playlist
	// Duplicates lists further marked playlists beyond Existing; they point to
	// the same source and should not exist.
	Duplicates []youtube.Playlist
}

// ResolveDestination finds the YouTube playlist for spotifyID among the
// user's playlists: the marker wins; otherwise same-title candidates are
// reported so the caller can ask.
func ResolveDestination(playlists []youtube.Playlist, spotifyID, title string) Destination {
	var d Destination
	for i := range playlists {
		p := playlists[i]
		marked := false
		for _, id := range MarkerIDs(p.Description) {
			if id == spotifyID {
				marked = true
				break
			}
		}
		switch {
		case marked && d.Existing == nil:
			d.Existing = &p
		case marked:
			d.Duplicates = append(d.Duplicates, p)
		case p.Title == title && len(MarkerIDs(p.Description)) == 0:
			d.SameTitle = append(d.SameTitle, p)
		}
	}
	if d.Existing != nil {
		d.SameTitle = nil // the marker already settles identity
	}
	return d
}
