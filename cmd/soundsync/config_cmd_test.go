package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// clearConfigEnv unsets every config variable for the duration of the test.
func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"SPOTIFY_CLIENT_ID", "SPOTIFY_REDIRECT_URI",
		"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "GOOGLE_REDIRECT_URI",
	} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
}

func TestConfigCheckValidNeverPrintsSecrets(t *testing.T) {
	clearConfigEnv(t)
	path := filepath.Join(t.TempDir(), ".env")
	env := "SPOTIFY_CLIENT_ID=spot-abc123\nGOOGLE_CLIENT_ID=goog-xyz789.apps.googleusercontent.com\nGOOGLE_CLIENT_SECRET=GOCSPX-supersecret\n"
	if err := os.WriteFile(path, []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := runConfigCheck(&out, path); err != nil {
		t.Fatalf("unexpected error: %v\n%s", err, out.String())
	}
	got := out.String()
	for _, secret := range []string{"spot-abc123", "goog-xyz789", "GOCSPX-supersecret"} {
		if strings.Contains(got, secret) {
			t.Errorf("output leaks %q:\n%s", secret, got)
		}
	}
	if !strings.Contains(got, "OK") {
		t.Errorf("expected OK in output:\n%s", got)
	}
}

func TestConfigCheckMissingFile(t *testing.T) {
	clearConfigEnv(t)
	var out bytes.Buffer
	err := runConfigCheck(&out, filepath.Join(t.TempDir(), "missing.env"))
	if err == nil {
		t.Fatal("expected error with no config")
	}
	got := out.String()
	for _, want := range []string{"not found", "SPOTIFY_CLIENT_ID is required", "GOOGLE_CLIENT_ID is required", "GOOGLE_CLIENT_SECRET is required"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}
