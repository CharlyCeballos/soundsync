// Package copier copies Spotify playlists into YouTube playlists: it picks
// each destination, plans the changes (look up what is already there,
// search the rest, diff) and writes them. It prints nothing and asks
// nothing: callers render its results, answer its questions and report its
// progress through an Observer.
package copier

import (
	"context"

	"github.com/CharlyCeballos/soundsync/internal/i18n"
	"github.com/CharlyCeballos/soundsync/internal/spotify"
	"github.com/CharlyCeballos/soundsync/internal/transfer"
	"github.com/CharlyCeballos/soundsync/internal/youtube"
)

// Kind says what happens to a YouTube playlist before tracks go in.
type Kind int

const (
	Existing Kind = iota // carries this playlist's marker: use it
	Create               // nothing found (or it was deleted): create it (todo 2.12)
	Adopt                // same title, no marker, and the user chose it (todo 2.1)
)

// Destination is the YouTube playlist chosen for one Spotify playlist.
type Destination struct {
	Source spotify.Playlist
	Kind   Kind
	Target youtube.Playlist // zero for Create until it is created

	// Candidates are the user's playlists with the same title and no marker.
	// Using one needs the user's consent (planV1 §6.1 step 4, todo 2.1):
	// the destination stays Create until Adopt is called.
	Candidates []youtube.Playlist
	// Duplicates carry the same marker as Target; only Target is used.
	Duplicates []youtube.Playlist
}

// ErrNotACandidate rejects an adoption the user was not offered.
var ErrNotACandidate = i18n.Error("that playlist is not one of the playlists offered for this Spotify playlist")

// Resolve finds each source's YouTube playlist among the channel's
// playlists. The marker settles identity (planV1 §5); same-title playlists
// without one come back as Candidates for the caller to ask about.
func Resolve(playlists []youtube.Playlist, srcs []spotify.Playlist) []Destination {
	dests := make([]Destination, len(srcs))
	for i, src := range srcs {
		r := transfer.ResolveDestination(playlists, src.ID, src.Name)
		dests[i] = Destination{Source: src, Kind: Create, Candidates: r.SameTitle, Duplicates: r.Duplicates}
		if r.Existing != nil {
			dests[i].Kind, dests[i].Target = Existing, *r.Existing
		}
	}
	return dests
}

// Adopt makes the candidate with playlistID the destination. Only an ID
// travels from the user: the playlist itself comes from the channel listing,
// so fields the user never sees (its default language) are kept as they are.
func (d *Destination) Adopt(playlistID string) error {
	for _, p := range d.Candidates {
		if p.ID == playlistID {
			d.Kind, d.Target = Adopt, p
			return nil
		}
	}
	return ErrNotACandidate
}

// Ensure creates or adopts the playlist; an existing one needs nothing.
// Privacy applies only to a new playlist (Policies III.E.3.f).
func (d *Destination) Ensure(ctx context.Context, yc *youtube.Client, privacy string) error {
	switch d.Kind {
	case Create:
		p, err := yc.CreatePlaylist(ctx, d.Source.Name, transfer.Description(d.Source.ID), privacy)
		if err != nil {
			return err
		}
		d.Target = p
	case Adopt:
		desc := transfer.AdoptedDescription(d.Target.Description, d.Source.ID)
		if err := yc.SetDescription(ctx, d.Target, desc); err != nil {
			return err
		}
		d.Target.Description = desc
	}
	return nil
}

// Cost is the quota needed to get the destination ready.
func (d Destination) Cost() int {
	if d.Kind == Existing {
		return 0
	}
	return youtube.WriteUnits // playlists.insert or playlists.update
}
