// Package consent remembers that the user accepted Soundsync's privacy
// policy and the YouTube Terms of Service before the first login (YouTube
// ToS §7, Developer Policies III.A.1–A.2, todo 2.15). The acceptance is the
// only file Soundsync writes on its own.
package consent

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Version matches the version line of PRIVACY.md. Changing it makes
// everyone accept the policy again.
const Version = "2026-11-03"

// Links every consent notice shows (Policies III.A.1–A.2).
const (
	PolicyURL            = "https://github.com/CharlyCeballos/soundsync/blob/main/PRIVACY.md"
	YouTubeTermsURL      = "https://www.youtube.com/t/terms"
	GooglePrivacyURL     = "https://policies.google.com/privacy"
	GooglePermissionsURL = "https://myaccount.google.com/permissions"
)

const fileName = "privacy-accepted"

// ErrNoConfigDir means the OS has no config folder, so the acceptance cannot
// be remembered and the user is asked again next time.
var ErrNoConfigDir = errors.New("no config folder")

// Accepted reports whether the current Version was accepted. An empty dir
// (no config folder) is never accepted.
func Accepted(dir string) bool {
	if dir == "" {
		return false
	}
	b, err := os.ReadFile(filepath.Join(dir, fileName))
	return err == nil && strings.TrimSpace(string(b)) == Version
}

// Remember stores the acceptance of the current Version in dir, readable by
// the user only.
func Remember(dir string) error {
	if dir == "" {
		return ErrNoConfigDir
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, fileName), []byte(Version+"\n"), 0o600)
}
