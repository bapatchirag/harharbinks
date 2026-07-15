package update

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/bapatchirag/harharbinks/internal/config"
)

// unsetEnv removes key for the duration of the test, restoring any prior value on
// cleanup, so env-driven behavior is exercised deterministically regardless of
// the developer's shell.
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	if orig, ok := os.LookupEnv(key); ok {
		t.Cleanup(func() { _ = os.Setenv(key, orig) })
		_ = os.Unsetenv(key)
	}
}

// TestIsReleaseBuild checks that only pristine release versions are treated as
// release builds, while dev, dirty, describe-ahead, and pseudo-versions are not.
func TestIsReleaseBuild(t *testing.T) {
	cases := []struct {
		version string
		want    bool
	}{
		{"v1.2.3", true},
		{"1.2.3", true},
		{"v1.2.3-rc1", true},
		{"v0.0.1", true},
		{"dev", false},
		{"(devel)", false},
		{"", false},
		{"   ", false},
		{"v1.0.1-dirty", false},
		{"v1.0.1-3-g1a2b3c4", false},
		{"1.0.1-3-g1a2b3c4", false},
		{"0.0.0-20260705004817-2cc9a8fe1146", false},
		{"v0.0.0-20260705004817-2cc9a8fe1146", false},
		{"not-a-version", false},
	}
	for _, c := range cases {
		if got := IsReleaseBuild(c.version); got != c.want {
			t.Errorf("IsReleaseBuild(%q) = %v, want %v", c.version, got, c.want)
		}
	}
}

// TestEnabledFromConfig verifies that, without an env override, the persisted
// config.UpdateCheck flag decides whether checks run.
func TestEnabledFromConfig(t *testing.T) {
	unsetEnv(t, envVar)
	if !Enabled(config.Config{UpdateCheck: true}) {
		t.Error("Enabled should be true when config.UpdateCheck is set and no env override")
	}
	if Enabled(config.Config{UpdateCheck: false}) {
		t.Error("Enabled should be false when config.UpdateCheck is unset and no env override")
	}
}

// TestEnabledEnvOverride verifies HHB_UPDATE_CHECK overrides the persisted flag in
// both directions, and that a present but unparseable value opts in.
func TestEnabledEnvOverride(t *testing.T) {
	t.Setenv(envVar, "1")
	if !Enabled(config.Config{UpdateCheck: false}) {
		t.Error("HHB_UPDATE_CHECK=1 should enable checks over a false config")
	}
	t.Setenv(envVar, "0")
	if Enabled(config.Config{UpdateCheck: true}) {
		t.Error("HHB_UPDATE_CHECK=0 should disable checks over a true config")
	}
	t.Setenv(envVar, "yes")
	if !Enabled(config.Config{UpdateCheck: false}) {
		t.Error("a present, non-empty, unparseable value should opt in")
	}
}

// TestIsNewer covers the semantic-version comparison, including the leading-v
// tolerance and unparseable inputs (which are never newer).
func TestIsNewer(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"v1.0.0", "v1.0.1", true},
		{"1.0.0", "1.0.1", true},
		{"v1.0.0", "v2.0.0", true},
		{"v1.0.0", "v1.0.0", false},
		{"v2.0.0", "v1.0.0", false},
		{"dev", "v1.0.0", false},
		{"v1.0.0", "garbage", false},
	}
	for _, c := range cases {
		if got := isNewer(c.current, c.latest); got != c.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}

// TestStateRoundTrip verifies the on-disk cache saves and reloads.
func TestStateRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	want := state{CheckedAt: time.Now().Truncate(time.Second), LatestVersion: "v9.9.9", URL: "https://example.test/r"}
	if err := saveState(want); err != nil {
		t.Fatalf("saveState: %v", err)
	}
	got, ok := loadState()
	if !ok {
		t.Fatal("loadState reported no cache after saveState")
	}
	if got.LatestVersion != want.LatestVersion || got.URL != want.URL {
		t.Errorf("loadState = %+v, want %+v", got, want)
	}
}

// TestLoadStateMissing verifies a missing cache reports ok=false rather than
// failing, so a fresh install simply performs its first check.
func TestLoadStateMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, ok := loadState(); ok {
		t.Error("loadState should report ok=false when no cache file exists")
	}
}

// TestCheckUsesFreshCache verifies Check answers from a fresh cache without any
// network access, returning the cached comparison against the current version.
func TestCheckUsesFreshCache(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := saveState(state{CheckedAt: time.Now(), LatestVersion: "v2.0.0", URL: "https://example.test"}); err != nil {
		t.Fatalf("saveState: %v", err)
	}
	res, err := Check(context.Background(), "v1.0.0", false)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !res.Newer || res.Latest != "v2.0.0" {
		t.Errorf("Check = %+v, want Newer with Latest v2.0.0", res)
	}
	if res.Current != "v1.0.0" {
		t.Errorf("Check Current = %q, want v1.0.0", res.Current)
	}
}

// TestCachedReadsStaleWithoutNetwork verifies Cached returns a known result even
// when the cache is older than the TTL, since a hint tolerates staleness.
func TestCachedReadsStaleWithoutNetwork(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := saveState(state{CheckedAt: time.Now().Add(-72 * time.Hour), LatestVersion: "v3.0.0"}); err != nil {
		t.Fatalf("saveState: %v", err)
	}
	res, ok := Cached("v1.0.0")
	if !ok {
		t.Fatal("Cached should report ok=true when a cached version exists")
	}
	if !res.Newer || res.Latest != "v3.0.0" {
		t.Errorf("Cached = %+v, want Newer with Latest v3.0.0", res)
	}
}

// TestCachedMissing verifies Cached reports ok=false with no cache present.
func TestCachedMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, ok := Cached("v1.0.0"); ok {
		t.Error("Cached should report ok=false when no cache file exists")
	}
}
