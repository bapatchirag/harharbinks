package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bapatchirag/harharbinks/internal/config"
)

// useTempConfig points the user's home directory at a throwaway location for the
// test so Load/Save operate on an isolated ~/.config/hhb file instead of the
// developer's real config. It sets HOME, which os.UserHomeDir resolves on macOS
// and Linux.
func useTempConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	path, err := config.Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	return path
}

// TestDefault verifies the built-in defaults are populated (not zero), so a
// fresh config file is written with real values for every field.
func TestDefault(t *testing.T) {
	if config.Default().Theme == "" {
		t.Error("Default().Theme should be a non-empty palette name")
	}
}

// TestLoadMissing verifies a missing config file loads as the built-in defaults
// rather than failing, so a fresh install starts fully configured.
func TestLoadMissing(t *testing.T) {
	useTempConfig(t)
	if got := config.Load(); got != config.Default() {
		t.Errorf("Load() with no file = %+v, want defaults %+v", got, config.Default())
	}
}

// TestSaveLoadRoundTrip verifies a saved theme is read back unchanged and that
// Save creates the parent directory when it does not yet exist.
func TestSaveLoadRoundTrip(t *testing.T) {
	useTempConfig(t)
	want := config.Config{Theme: "harharbinks-gruvbox"}
	if err := config.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if got := config.Load(); got != want {
		t.Errorf("Load() after Save = %+v, want %+v", got, want)
	}
}

// TestLoadMalformed verifies invalid JSON on disk loads as the built-in defaults
// instead of propagating an error, keeping startup resilient to a corrupt file.
func TestLoadMalformed(t *testing.T) {
	path := useTempConfig(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if got := config.Load(); got != config.Default() {
		t.Errorf("Load() with malformed file = %+v, want defaults %+v", got, config.Default())
	}
}

// TestEnsureCreatesFile verifies Ensure writes a config file for a fresh install
// and returns the defaults, so the file is present after any launch.
func TestEnsureCreatesFile(t *testing.T) {
	path := useTempConfig(t)
	got, err := config.Ensure()
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if got != config.Default() {
		t.Errorf("Ensure() = %+v, want defaults %+v", got, config.Default())
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("Ensure() should create the config file: %v", err)
	}
}

// TestSaveOverwrites verifies a second Save replaces the first, so re-selecting a
// theme updates rather than appends.
func TestSaveOverwrites(t *testing.T) {
	useTempConfig(t)
	if err := config.Save(config.Config{Theme: "harharbinks-zenburn"}); err != nil {
		t.Fatalf("first Save: %v", err)
	}
	if err := config.Save(config.Config{Theme: "harharbinks-everforest"}); err != nil {
		t.Fatalf("second Save: %v", err)
	}
	if got := config.Load(); got.Theme != "harharbinks-everforest" {
		t.Errorf("Load() after overwrite = %q, want %q", got.Theme, "harharbinks-everforest")
	}
}

// TestDefaultUpdateCheckOff verifies update checks are opt-in: the built-in
// default keeps them disabled so a fresh install stays fully offline.
func TestDefaultUpdateCheckOff(t *testing.T) {
	if config.Default().UpdateCheck {
		t.Error("Default().UpdateCheck should be false (opt-in)")
	}
}

// TestUpdateCheckRoundTrip verifies the opt-in update flag is persisted and read
// back unchanged alongside the theme.
func TestUpdateCheckRoundTrip(t *testing.T) {
	useTempConfig(t)
	want := config.Config{Theme: config.Default().Theme, UpdateCheck: true}
	if err := config.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if got := config.Load(); got != want {
		t.Errorf("Load() after Save = %+v, want %+v", got, want)
	}
}

// TestDirIsConfigParent verifies Dir returns the parent directory of the config
// file, the shared ~/.config/hhb location used for the config and update cache.
func TestDirIsConfigParent(t *testing.T) {
	dir, err := config.Dir()
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	path, err := config.Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if got := filepath.Dir(path); got != dir {
		t.Errorf("Dir() = %q, want parent of Path() %q", dir, got)
	}
}
