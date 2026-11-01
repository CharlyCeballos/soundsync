package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/CharlyCeballos/soundsync/internal/i18n"
	"github.com/CharlyCeballos/soundsync/internal/spotify"
)

// resolveSources turns each argument into one Spotify playlist the user can
// read. All problems are reported together, before anything costs quota.
// The same playlist given twice is kept once.
func resolveSources(ctx context.Context, sp *spotifySession, args []string) ([]spotify.Playlist, error) {
	var out []spotify.Playlist
	var problems []string
	seen := map[string]bool{}
	for _, arg := range args {
		p, err := resolvePlaylist(ctx, sp, arg)
		switch {
		case err != nil:
			problems = append(problems, err.Error())
		case p.Name == "":
			problems = append(problems, fmt.Sprintf(i18n.T("%q is not in your Spotify library; run 'spotify playlists' to see them"), arg))
		case p.AccessFor(sp.me.ID) == spotify.AccessFollowed:
			problems = append(problems, fmt.Sprintf(i18n.T("%q belongs to another user: Spotify does not let Development Mode apps read it"), p.Name))
		case seen[p.ID]:
			// Same playlist named twice: process it once.
		default:
			seen[p.ID] = true
			out = append(out, p)
		}
	}
	if len(problems) > 0 {
		msg := strings.Join(problems, "\n")
		if len(args) > 1 {
			msg += i18n.T("\ntip: each argument is one playlist; quote names with spaces, e.g. \"The Way to My Favorite Part\"")
		}
		return nil, errors.New(msg)
	}
	return out, nil
}

// quoteAll formats playlist names for a copy-pasteable command line.
func quoteAll(names []string) string {
	q := make([]string, len(names))
	for i, n := range names {
		q[i] = fmt.Sprintf(i18n.T("%q"), n)
	}
	return strings.Join(q, " ")
}

// lockedWriter serialises writes from concurrent goroutines. Each fmt.Fprintf
// call is a single Write, so whole lines never interleave.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}
