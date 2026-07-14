package main

import "testing"

// TestResolveVersionPrefersLdflags verifies an injected (release) version takes
// precedence over the build-info fallback.
func TestResolveVersionPrefersLdflags(t *testing.T) {
	old := version
	t.Cleanup(func() { version = old })
	version = "1.2.3"
	if got := resolveVersion(); got != "1.2.3" {
		t.Errorf("resolveVersion() = %q, want %q", got, "1.2.3")
	}
}

// TestResolveVersionFallsBack verifies that without an injected version the
// resolver returns a non-empty string (the module version when available, else
// the "dev" default) and never the empty string.
func TestResolveVersionFallsBack(t *testing.T) {
	old := version
	t.Cleanup(func() { version = old })
	version = "dev"
	if got := resolveVersion(); got == "" {
		t.Error("resolveVersion() returned empty string")
	}
}
