package consent

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRememberThenAccepted(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "soundsync") // does not exist yet
	if Accepted(dir) {
		t.Fatal("nothing accepted yet")
	}
	if err := Remember(dir); err != nil {
		t.Fatal(err)
	}
	if !Accepted(dir) {
		t.Fatal("acceptance not remembered")
	}
}

func TestOlderVersionIsNotAccepted(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, fileName), []byte("2000-01-01\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if Accepted(dir) {
		t.Error("a new policy version must be accepted again")
	}
}

// The version users accept must be the one PRIVACY.md shows.
func TestVersionMatchesThePolicy(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "PRIVACY.md"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "Version " + Version + "."; !strings.Contains(string(b), want) {
		t.Errorf("PRIVACY.md does not say %q: bump both together", want)
	}
}

func TestNoConfigDir(t *testing.T) {
	if Accepted("") {
		t.Error("no folder means nothing was remembered")
	}
	if err := Remember(""); !errors.Is(err, ErrNoConfigDir) {
		t.Errorf("err = %v, want ErrNoConfigDir", err)
	}
}
