package cli

import (
	"io"
	"os"
	"testing"

	"github.com/bapatchirag/harharbinks/internal/config"
)

// TestMain redirects the user's home directory to a throwaway location for the
// whole package so exercising run — which ensures a config file exists on every
// launch — never reads or writes the developer's real configuration. It sets
// HOME, which os.UserHomeDir resolves on macOS and Linux.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "hhb-cli-test")
	if err != nil {
		panic(err)
	}
	os.Setenv("HOME", dir)
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// TestRunEnsuresConfig verifies that any launch — here the headless --version
// path — creates the config file, satisfying the "config file present on every
// launch" guarantee.
func TestRunEnsuresConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	if code := run([]string{"--version"}, "test", io.Discard, io.Discard); code != 0 {
		t.Fatalf("run(--version) exit code = %d, want 0", code)
	}
	path, err := config.Path()
	if err != nil {
		t.Fatalf("config.Path: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("run should create the config file at %s: %v", path, err)
	}
}
