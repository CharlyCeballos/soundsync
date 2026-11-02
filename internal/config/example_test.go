package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExampleMatchesRepositoryFile(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", ".env.example"))
	if err != nil {
		t.Fatal(err)
	}
	_, repoBody, _ := strings.Cut(string(b), "\n") // its first line tells to copy the file
	if repoBody != exampleBody {
		t.Errorf(".env.example and config.Example drifted apart:\n--- .env.example\n%s\n--- Example\n%s", repoBody, exampleBody)
	}
}

func TestExampleParsesWithMissingIDs(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte(Example), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{EnvSpotifyClientID, EnvGoogleClientID, EnvGoogleClientSecret, EnvSpotifyRedirectURI, EnvGoogleRedirectURI} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	f, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := FromEnv(f.Getenv)
	if err == nil || cfg.SpotifyRedirectURI == nil || cfg.GoogleRedirectURI == nil {
		t.Errorf("the example must parse, keep valid redirect URIs and still ask for the IDs: cfg=%+v err=%v", cfg, err)
	}
}
