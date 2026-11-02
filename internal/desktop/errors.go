package desktop

import (
	"errors"
	"fmt"

	"github.com/CharlyCeballos/soundsync/internal/copier"
	"github.com/CharlyCeballos/soundsync/internal/i18n"
)

// Errors the frontend shows as they are, in the language chosen.
var (
	ErrBusy              = i18n.Error("Soundsync is still working on the previous step; wait for it to finish or stop it")
	ErrNoConfigDir       = i18n.Error("this system has no folder for app settings, so Soundsync cannot find its configuration file")
	ErrPolicyNotAccepted = i18n.Error("accept the privacy policy to let Soundsync connect to Spotify and YouTube")
	ErrSpotifyFirst      = i18n.Error("connect Spotify first")
	ErrYouTubeFirst      = i18n.Error("connect YouTube first")
	ErrNoSelection       = i18n.Error("choose at least one Spotify playlist")
	ErrResolveFirst      = i18n.Error("choose your playlists again: the previous choice is no longer current")
	ErrPlanFirst         = i18n.Error("look for matches first")
	ErrPlanIncomplete    = i18n.Error("the search stopped before every playlist was checked; look for matches again")
	ErrAlreadyWritten    = i18n.Error("this plan was already written; look for matches again to copy more")
	ErrNoRun             = i18n.Error("there is no copy to report yet")
	ErrURLNotAllowed     = i18n.Error("Soundsync only opens links to Spotify, YouTube, Google and its own page")
	ErrConfigExists      = i18n.Error("the configuration file already exists; open its folder to edit it")
	ErrBadPrivacy        = i18n.Error("choose private, unlisted or public for new playlists")
	ErrBadFormat         = i18n.Error("choose CSV or JSON for the report")
)

func errNotInLibrary(id string) error {
	return fmt.Errorf(i18n.T("playlist %s is not in your Spotify library; refresh the list and choose again"), id)
}

func errFollowed(name string) error {
	return fmt.Errorf(i18n.T("%q belongs to another user: Spotify does not let Development Mode apps read it"), name)
}

func errChoose(name string) error {
	return fmt.Errorf(i18n.T("%q: choose whether to use your existing playlist or create a new one"), name)
}

func errReportExists(path string) error {
	return fmt.Errorf(i18n.T("%q already exists; choose another name (Soundsync never overwrites files)"), path)
}

// explain rewords engine errors for the GUI: the CLI's quota hint talks
// about running a command again.
func explain(err error) string {
	switch {
	case err == nil:
		return ""
	case copier.IsQuota(err):
		return i18n.T("YouTube's daily quota is used up; it resets at midnight Pacific time. Copy again then: Soundsync only adds what is missing.")
	case errors.Is(err, copier.ErrSiblingQuota):
		return copier.ErrSiblingQuota.Error()
	}
	return err.Error()
}
