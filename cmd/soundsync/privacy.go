package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/CharlyCeballos/soundsync/internal/consent"
	"github.com/CharlyCeballos/soundsync/internal/i18n"
)

// privacyNotice must keep naming every consent link (a test checks it).
const privacyNotice = `
Before Soundsync connects to Spotify and YouTube, please read how it handles your data:
  Privacy policy:           https://github.com/CharlyCeballos/soundsync/blob/main/PRIVACY.md
  YouTube Terms of Service: https://www.youtube.com/t/terms
  Google Privacy Policy:    https://policies.google.com/privacy
Soundsync uses YouTube API Services: by continuing you agree to the YouTube Terms of Service.
Nothing from Spotify or YouTube is saved on this computer. Revoke Google access any time at
https://myaccount.google.com/permissions.
`

// errPolicyDeclined stops a login the user did not consent to.
var errPolicyDeclined = i18n.Error("accept the privacy policy to let Soundsync connect to Spotify and YouTube")

// ensurePolicyAccepted asks once, before the first login, that the user
// accept the privacy policy and the YouTube Terms of Service. The accepted
// version is remembered in the user's config folder; a new version asks again.
func (a *app) ensurePolicyAccepted(out io.Writer) error {
	if a.policyAccepted || consent.Accepted(a.configDir) {
		a.policyAccepted = true
		return nil
	}
	fmt.Fprint(out, i18n.T(privacyNotice))
	if !a.confirm(out, "Do you accept?") {
		return errPolicyDeclined
	}
	a.policyAccepted = true
	switch err := consent.Remember(a.configDir); {
	case errors.Is(err, consent.ErrNoConfigDir):
		fmt.Fprintln(out, i18n.T("note: no config folder found; Soundsync will ask again next time."))
	case err != nil:
		fmt.Fprintf(out, i18n.T("note: could not remember your answer (%v); Soundsync will ask again next time.\n"), err)
	}
	return nil
}
