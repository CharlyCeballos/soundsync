package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// privacyPolicyVersion matches the version line of PRIVACY.md. Changing it
// makes everyone accept the policy again.
const privacyPolicyVersion = "2026-11-01"

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
var errPolicyDeclined = errors.New("accept the privacy policy to let Soundsync connect to Spotify and YouTube")

// ensurePolicyAccepted asks once, before the first login, that the user
// accept the privacy policy and the YouTube Terms of Service (YouTube ToS
// §7, Developer Policies III.A.1–A.2, todo 2.15). The accepted version is
// remembered in the user's config folder; a new version asks again.
func (a *app) ensurePolicyAccepted(out io.Writer) error {
	if a.policyAccepted {
		return nil
	}
	path := ""
	if a.configDir != "" {
		path = filepath.Join(a.configDir, "privacy-accepted")
		if b, err := os.ReadFile(path); err == nil && strings.TrimSpace(string(b)) == privacyPolicyVersion {
			a.policyAccepted = true
			return nil
		}
	}

	fmt.Fprint(out, privacyNotice)
	if !a.confirm(out, "Do you accept?") {
		return errPolicyDeclined
	}
	a.policyAccepted = true
	if path == "" {
		fmt.Fprintln(out, "note: no config folder found; Soundsync will ask again next time.")
		return nil
	}
	err := os.MkdirAll(a.configDir, 0o700)
	if err == nil {
		err = os.WriteFile(path, []byte(privacyPolicyVersion+"\n"), 0o600)
	}
	if err != nil {
		fmt.Fprintf(out, "note: could not remember your answer (%v); Soundsync will ask again next time.\n", err)
	}
	return nil
}

// defaultConfigDir is Soundsync's folder under the OS config directory
// (%APPDATA% on Windows, ~/.config on Linux), or "" if there is none.
func defaultConfigDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "soundsync")
}
