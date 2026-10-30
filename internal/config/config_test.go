package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestFromEnvValid(t *testing.T) {
	cfg, err := FromEnv(envFrom(map[string]string{
		EnvSpotifyClientID:    " spotify-id ",
		EnvGoogleClientID:     "google-id",
		EnvGoogleClientSecret: "google-secret",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SpotifyClientID != "spotify-id" {
		t.Errorf("client ID not trimmed: %q", cfg.SpotifyClientID)
	}
	if got := cfg.SpotifyRedirectURI.String(); got != DefaultSpotifyRedirectURI {
		t.Errorf("spotify redirect default = %q", got)
	}
	if got := cfg.GoogleRedirectURI.String(); got != DefaultGoogleRedirectURI {
		t.Errorf("google redirect default = %q", got)
	}
	if cfg.GoogleClientSecret != "google-secret" {
		t.Errorf("secret not loaded: %q", cfg.GoogleClientSecret)
	}
}

func TestFromEnvReportsAllErrors(t *testing.T) {
	_, err := FromEnv(envFrom(map[string]string{
		EnvGoogleRedirectURI: "https://example.com/callback",
	}))
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{EnvSpotifyClientID, EnvGoogleClientID, EnvGoogleClientSecret, EnvGoogleRedirectURI} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q does not mention %s", msg, want)
		}
	}
}

func TestValidateLoopbackRedirect(t *testing.T) {
	cases := []struct {
		raw string
		ok  bool
	}{
		{"http://127.0.0.1:8888/callback", true},
		{"http://[::1]:8889/callback", true},
		{"http://localhost:8888/callback", false},
		{"https://127.0.0.1:8888/callback", false},
		{"http://127.0.0.1/callback", false},
		{"http://127.0.0.1:8888", false},
		{"http://127.0.0.1:8888/", false},
		{"http://127.0.0.1:8888/callback?x=1", false},
		{"http://192.168.1.10:8888/callback", false},
		{"http://127.0.0.1:99999/callback", false},
	}
	for _, c := range cases {
		_, err := ValidateLoopbackRedirect(c.raw)
		if (err == nil) != c.ok {
			t.Errorf("%s: want ok=%v, got err=%v", c.raw, c.ok, err)
		}
	}
}

func TestLoadDotEnvDoesNotOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	content := "SOUNDSYNC_TEST_A=from-file\nSOUNDSYNC_TEST_B=from-file\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SOUNDSYNC_TEST_A", "from-env")
	// Register B for cleanup, then make sure it is truly unset.
	t.Setenv("SOUNDSYNC_TEST_B", "")
	os.Unsetenv("SOUNDSYNC_TEST_B")

	loaded, err := LoadDotEnv(path)
	if err != nil || !loaded {
		t.Fatalf("loaded=%v err=%v", loaded, err)
	}
	if got := os.Getenv("SOUNDSYNC_TEST_A"); got != "from-env" {
		t.Errorf("env var overridden by file: %q", got)
	}
	if got := os.Getenv("SOUNDSYNC_TEST_B"); got != "from-file" {
		t.Errorf("file value not loaded: %q", got)
	}
}

func TestLoadDotEnvMissingFile(t *testing.T) {
	loaded, err := LoadDotEnv(filepath.Join(t.TempDir(), "missing.env"))
	if err != nil || loaded {
		t.Fatalf("loaded=%v err=%v", loaded, err)
	}
}
