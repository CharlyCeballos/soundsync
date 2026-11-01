package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func answering(t *testing.T, a *app, answers ...string) *int {
	asked := 0
	a.readLine = func() (string, error) {
		if asked >= len(answers) {
			t.Fatal("asked more than expected")
		}
		asked++
		return answers[asked-1], nil
	}
	return &asked
}

func TestPrivacyAcceptedOnceAndRemembered(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "soundsync") // does not exist yet
	a := newApp()
	a.configDir = dir
	asked := answering(t, a, "y")
	var out bytes.Buffer
	if err := a.ensurePolicyAccepted(&out); err != nil {
		t.Fatal(err)
	}
	if err := a.ensurePolicyAccepted(&out); err != nil || *asked != 1 {
		t.Fatalf("same process must not ask twice: err=%v asked=%d", err, *asked)
	}
	for _, want := range []string{"PRIVACY.md", "https://www.youtube.com/t/terms", "https://policies.google.com/privacy", "YouTube API Services", "myaccount.google.com/permissions"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("notice missing %q", want)
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, "privacy-accepted")); err != nil || strings.TrimSpace(string(b)) != privacyPolicyVersion {
		t.Fatalf("acceptance file = %q, %v", b, err)
	}

	// Next run (new process): remembered, no question.
	b := newApp()
	b.configDir = dir
	answering(t, b) // any question fails the test
	if err := b.ensurePolicyAccepted(&out); err != nil {
		t.Fatal(err)
	}
}

func TestPrivacyDeclinedBlocksLogin(t *testing.T) {
	dir := t.TempDir()
	a := newApp()
	a.configDir = dir
	a.envFile = writeEnv(t)
	answering(t, a, "n")
	var out bytes.Buffer
	if _, err := loginSpotify(t.Context(), &out, a); !errors.Is(err, errPolicyDeclined) {
		t.Fatalf("err = %v, want errPolicyDeclined before any browser login", err)
	}
	if strings.Contains(out.String(), "Opening your browser") {
		t.Error("must not open the browser without consent")
	}
	if _, err := os.Stat(filepath.Join(dir, "privacy-accepted")); !os.IsNotExist(err) {
		t.Error("a refusal must not be remembered as acceptance")
	}
}

func TestPrivacyNewVersionAsksAgain(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "privacy-accepted"), []byte("2000-01-01\n"), 0o600)
	a := newApp()
	a.configDir = dir
	asked := answering(t, a, "sí")
	var out bytes.Buffer
	if err := a.ensurePolicyAccepted(&out); err != nil || *asked != 1 {
		t.Fatalf("err=%v asked=%d, want one question for the new version", err, *asked)
	}
}

// writeEnv creates a valid .env so loginSpotify gets past config loading.
func writeEnv(t *testing.T) string {
	t.Helper()
	for _, k := range []string{"SPOTIFY_CLIENT_ID", "SPOTIFY_REDIRECT_URI", "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "GOOGLE_REDIRECT_URI"} {
		t.Setenv(k, "") // LoadDotEnv does not override variables already set
		os.Unsetenv(k)
	}
	path := filepath.Join(t.TempDir(), ".env")
	env := "SPOTIFY_CLIENT_ID=0123456789abcdef0123456789abcdef\n" +
		"SPOTIFY_REDIRECT_URI=http://127.0.0.1:8888/callback\n" +
		"GOOGLE_CLIENT_ID=123-abc.apps.googleusercontent.com\n" +
		"GOOGLE_CLIENT_SECRET=GOCSPX-test\n" +
		"GOOGLE_REDIRECT_URI=http://127.0.0.1:8889/callback\n"
	if err := os.WriteFile(path, []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
