package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/CharlyCeballos/soundsync/internal/fakeapi"
	"github.com/CharlyCeballos/soundsync/internal/session"
	"github.com/CharlyCeballos/soundsync/internal/spotify"
	"github.com/CharlyCeballos/soundsync/internal/transfer"
	"github.com/CharlyCeballos/soundsync/internal/youtube"

	"golang.org/x/oauth2"
)

// spotifyClient serves a fake Spotify library for the length of the test.
func spotifyClient(t *testing.T, lib *fakeapi.Spotify) *spotify.Client {
	t.Helper()
	srv := httptest.NewServer(lib)
	t.Cleanup(srv.Close)
	return spotify.NewClient(srv.Client()).WithBaseURL(srv.URL)
}

// counts returns the searches and playlist item inserts made so far.
func counts(yt *fakeapi.YouTube) (searches, itemInserts int) {
	c := yt.Counts()
	return c.Searches, c.ItemInserts
}

func newTestApp(t *testing.T, yt *fakeapi.YouTube) *app {
	ytSrv := httptest.NewServer(yt)
	t.Cleanup(ytSrv.Close)
	a := newApp()
	a.newSpotify = func(context.Context, io.Writer, *app) (*session.Spotify, error) {
		return &session.Spotify{Client: spotifyClient(t, fakeapi.NewLibrary()), Me: spotify.User{ID: "me"}}, nil
	}
	a.newYouTube = func(context.Context, io.Writer, *app) (*session.YouTube, error) {
		return &session.YouTube{
			Client:  youtube.NewClient(ytSrv.Client()).WithBaseURL(ytSrv.URL),
			Channel: youtube.Channel{ID: "UC1", Title: "Carlos Ceballos"},
			Token:   &oauth2.Token{AccessToken: "x"},
		}, nil
	}
	return a
}

func TestYouTubeCreateFlow(t *testing.T) {
	yt := &fakeapi.YouTube{}
	a := newTestApp(t, yt)
	in := strings.NewReader(strings.Join([]string{
		"youtube create Way", // asks...
		"n",                  // ...declined
		"youtube create Way", // asks...
		"y",                  // ...accepted
		"youtube create Way", // already exists: must not ask
		"youtube playlists",  // so this line runs as a command
		"youtube create Pentatonix",
		"exit",
	}, "\n"))
	var out, errOut bytes.Buffer
	if err := runShell(a, in, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	got := out.String()

	if yt.Inserts != 1 {
		t.Fatalf("inserts = %d, want exactly 1\n%s", yt.Inserts, got)
	}
	snippet := yt.Playlists[0]["snippet"].(map[string]string)
	status := yt.Playlists[0]["status"].(map[string]string)
	if snippet["title"] != "The Way to My Favorite Part" {
		t.Errorf("title altered: %q", snippet["title"])
	}
	if ids := transfer.MarkerIDs(snippet["description"]); len(ids) != 1 || ids[0] != "5ZwLvrI8d6VnEQXxB0bMms" {
		t.Errorf("marker missing from description %q", snippet["description"])
	}
	if status["privacyStatus"] != "private" {
		t.Errorf("privacy = %q, want private by default", status["privacyStatus"])
	}

	for _, want := range []string{
		"Cancelled. Nothing was changed.",
		`Created "The Way to My Favorite Part" (private).`,
		"https://music.youtube.com/playlist?list=PLfake1",
		"Already on YouTube as",
		"Nothing to create.",
		"spotify:5ZwLvrI8d6VnEQXxB0bMms", // youtube playlists shows the marker
		youtubeAttribution,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("stdout missing %q:\n%s", want, got)
		}
	}
	if n := strings.Count(got, "Go ahead?"); n != 2 {
		t.Errorf("confirmation asked %d times, want 2:\n%s", n, got)
	}
	if !strings.Contains(errOut.String(), "belongs to another user") {
		t.Errorf("followed playlist not rejected:\n%s", errOut.String())
	}
}

func TestYouTubeCreateYesSkipsPromptAndHonoursPrivacy(t *testing.T) {
	yt := &fakeapi.YouTube{}
	a := newTestApp(t, yt)
	in := strings.NewReader("youtube create Way --yes --privacy unlisted\nexit\n")
	var out, errOut bytes.Buffer
	if err := runShell(a, in, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if yt.Inserts != 1 || strings.Contains(out.String(), "Go ahead?") {
		t.Fatalf("want one insert without prompt:\n%s\n%s", out.String(), errOut.String())
	}
	if p := yt.Playlists[0]["status"].(map[string]string)["privacyStatus"]; p != "unlisted" {
		t.Errorf("privacy = %q, want unlisted", p)
	}
}

func TestYouTubeCreateBadPrivacyNeedsNoLogin(t *testing.T) {
	a := newApp()
	a.newSpotify = func(context.Context, io.Writer, *app) (*session.Spotify, error) {
		t.Error("must not log in to Spotify")
		return nil, fmt.Errorf("no")
	}
	a.newYouTube = func(context.Context, io.Writer, *app) (*session.YouTube, error) {
		t.Error("must not log in to YouTube")
		return nil, fmt.Errorf("no")
	}
	in := strings.NewReader("youtube create Way --privacy secret\nexit\n")
	var out, errOut bytes.Buffer
	runShell(a, in, &out, &errOut)
	if !strings.Contains(errOut.String(), `invalid --privacy "secret"`) {
		t.Errorf("want privacy error:\n%s", errOut.String())
	}
}

func TestConfirmAnswers(t *testing.T) {
	for answer, want := range map[string]bool{
		"y": true, "YES": true, " sí ": true, "s": true,
		"": false, "n": false, "no": false, "yep": false,
	} {
		a := newApp()
		a.readLine = func() (string, error) { return answer, nil }
		var out bytes.Buffer
		if got := a.confirm(&out, "Go?"); got != want {
			t.Errorf("confirm(%q) = %v, want %v", answer, got, want)
		}
	}
	a := newApp()
	a.readLine = func() (string, error) { return "", io.EOF }
	var out bytes.Buffer
	if a.confirm(&out, "Go?") {
		t.Error("EOF must mean no")
	}
}
