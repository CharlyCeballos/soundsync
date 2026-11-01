package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"text/tabwriter"

	"github.com/CharlyCeballos/soundsync/internal/i18n"
	"github.com/CharlyCeballos/soundsync/internal/spotify"

	"github.com/spf13/cobra"
)

// attribution is required wherever Spotify data is shown (Developer Policy II.4.1).
const attribution = "Data from Spotify."

func newSpotifyCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spotify",
		Short: i18n.T("Read your Spotify playlists (token kept in memory only)"),
	}
	cmd.AddCommand(
		command(a, "playlists", nil, "List the playlists in your Spotify library", cobra.NoArgs, runSpotifyPlaylists),
		command(a, "tracks <playlist>", nil, "List the tracks of one playlist: name, ID, open.spotify.com link or spotify: URI", cobra.MinimumNArgs(1), runSpotifyTracks),
	)
	return cmd
}

func runSpotifyPlaylists(ctx context.Context, out io.Writer, a *app, _ []string) error {
	s, err := a.spotify(ctx, out)
	if err != nil {
		return err
	}
	// Always refresh: the library may have changed since the last listing.
	playlists, err := s.client.MyPlaylists(ctx)
	if err != nil {
		return err
	}
	s.playlists = playlists
	printPlaylists(out, playlists, s.me.ID)
	return nil
}

func runSpotifyTracks(ctx context.Context, out io.Writer, a *app, args []string) error {
	ref := joinArgs(args)
	s, err := a.spotify(ctx, out)
	if err != nil {
		return err
	}
	p, err := resolvePlaylist(ctx, s, ref)
	if err != nil {
		return err
	}
	if p.Name != "" && p.AccessFor(s.me.ID) == spotify.AccessFollowed {
		return fmt.Errorf(i18n.T("%q belongs to another user: Spotify does not let Development Mode apps read it"), p.Name)
	}
	entries, err := s.client.PlaylistItems(ctx, p.ID)
	if err != nil {
		var apiErr *spotify.APIError
		if errors.As(err, &apiErr) && (apiErr.Status == 403 || apiErr.Status == 404) {
			return fmt.Errorf(i18n.T("%w\nhint: Spotify only lets Development Mode apps read playlists you own or collaborate on"), err)
		}
		return err
	}
	printEntries(out, p, entries)
	return nil
}

// joinArgs rebuilds a playlist name typed without quotes, which arrives as
// several arguments.
func joinArgs(args []string) string { return strings.Join(args, " ") }

// resolvePlaylist turns a name, ID, link or URI into a playlist. The library
// listing is fetched once per session and reused for later lookups.
func resolvePlaylist(ctx context.Context, s *spotifySession, ref string) (spotify.Playlist, error) {
	if s.playlists == nil {
		playlists, err := s.client.MyPlaylists(ctx)
		if err != nil {
			return spotify.Playlist{}, err
		}
		s.playlists = playlists
	}

	id, idErr := parsePlaylistRef(ref)
	if idErr == nil {
		for _, p := range s.playlists {
			if p.ID == id {
				return p, nil
			}
		}
	}

	matches := spotify.FindPlaylists(s.playlists, ref)
	switch {
	case len(matches) == 1:
		return matches[0], nil
	case len(matches) > 1:
		var b strings.Builder
		fmt.Fprintf(&b, i18n.T("%q matches %d playlists; use a longer name or the link:"), ref, len(matches))
		for _, m := range matches {
			fmt.Fprintf(&b, i18n.T("\n  %s  %s"), m.Name, m.URL())
		}
		return spotify.Playlist{}, errors.New(b.String())
	case idErr == nil:
		// A valid ID that is not in the library (e.g. a link someone shared).
		return spotify.Playlist{ID: id}, nil
	}
	return spotify.Playlist{}, fmt.Errorf(i18n.T("no playlist in your library matches %q; run 'spotify playlists' to see them"), ref)
}

func printPlaylists(out io.Writer, playlists []spotify.Playlist, userID string) {
	fmt.Fprintf(out, i18n.T("\nPlaylists in your Spotify library: %d\n\n"), len(playlists))
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, i18n.T("  #\tNAME\tTRACKS\tACCESS\tLINK"))
	followed := 0
	for i, p := range playlists {
		access := p.AccessFor(userID)
		if access == spotify.AccessFollowed {
			followed++
		}
		total := "?"
		if p.Total >= 0 {
			total = fmt.Sprint(p.Total)
		}
		fmt.Fprintf(tw, i18n.T("  %d\t%s\t%s\t%s\t%s\n"), i+1, p.Name, total, access, p.URL())
	}
	tw.Flush()
	if followed > 0 {
		fmt.Fprintf(out, i18n.T("\n%d followed playlist(s) belong to other users: Spotify does not let Development Mode apps read them, so they will be skipped.\n"), followed)
	}
	fmt.Fprintln(out, i18n.T("\n")+attribution)
}

func printEntries(out io.Writer, p spotify.Playlist, entries []spotify.Entry) {
	title := p.ID
	if p.Name != "" {
		title = fmt.Sprintf(i18n.T("%q (%s)"), p.Name, p.URL())
	}
	fmt.Fprintf(out, i18n.T("\nPlaylist %s: %d entries\n\n"), title, len(entries))
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	var tracks, local, episodes, unavailable, noISRC int
	for _, e := range entries {
		if e.Skip != "" {
			if e.Skip == spotify.SkipEpisode {
				episodes++
			} else {
				unavailable++
			}
			fmt.Fprintf(tw, i18n.T("  %d\t(skipped: %s)\t\t\n"), e.Position, e.Skip)
			continue
		}
		t := e.Track
		tracks++
		note := t.URL()
		if t.IsLocal {
			local++
			note = "(local file)"
		}
		isrc := t.ISRC
		if isrc == "" {
			isrc = "-"
			noISRC++
		}
		fmt.Fprintf(tw, i18n.T("  %d\t%s — %s\t%s\t%s\n"), e.Position, strings.Join(t.Artists, ", "), t.Name, isrc, note)
	}
	tw.Flush()
	fmt.Fprintf(out, i18n.T("\nTracks: %d (local files: %d, without ISRC: %d). Skipped: %d episode(s), %d unavailable.\n"),
		tracks, local, noISRC, episodes, unavailable)
	fmt.Fprintln(out, attribution)
}

// parsePlaylistRef accepts a bare ID, an open.spotify.com link (with or
// without query string or locale prefix) or a spotify:playlist: URI.
func parsePlaylistRef(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	var id string
	switch {
	case strings.HasPrefix(ref, "spotify:playlist:"):
		id = strings.TrimPrefix(ref, "spotify:playlist:")
	case strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://"):
		u, err := url.Parse(ref)
		if err != nil || u.Hostname() != "open.spotify.com" {
			return "", fmt.Errorf(i18n.T("%q is not an open.spotify.com playlist link"), ref)
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		for i := 0; i+1 < len(parts); i++ {
			if parts[i] == "playlist" {
				id = parts[i+1]
			}
		}
	default:
		id = ref
	}
	if !isBase62(id) {
		return "", fmt.Errorf(i18n.T("%q is not a Spotify playlist ID, link or URI"), ref)
	}
	return id, nil
}

func isBase62(s string) bool {
	if len(s) < 16 || len(s) > 32 {
		return false
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
			return false
		}
	}
	return true
}
