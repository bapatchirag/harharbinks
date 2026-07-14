package theme

import (
	"strings"
	"testing"
)

func TestDefault(t *testing.T) {
	th := Default()
	if th.Name == "" {
		t.Error("theme name should not be empty")
	}
	if got := th.Title().Render("x"); !strings.Contains(got, "x") {
		t.Errorf("Title().Render(%q) = %q, want it to contain the input", "x", got)
	}
}

func TestNamedThemes(t *testing.T) {
	seen := make(map[string]bool)
	for _, th := range []Theme{Gruvbox(), Everforest(), Kanagawa(), Zenburn()} {
		if th.Name == "" {
			t.Errorf("named theme has empty name")
		}
		if seen[th.Name] {
			t.Errorf("duplicate theme name %q", th.Name)
		}
		seen[th.Name] = true
		if got := th.Selected().Render("x"); !strings.Contains(got, "x") {
			t.Errorf("%s Selected().Render(%q) = %q, want it to contain the input", th.Name, "x", got)
		}
	}
}

// TestThemes verifies the selector's palette list is the built-in set, with the
// default first and no duplicates.
func TestThemes(t *testing.T) {
	all := Themes()
	if len(all) != 4 {
		t.Fatalf("Themes() returned %d palettes, want 4", len(all))
	}
	if all[0].Name != Default().Name {
		t.Errorf("Themes()[0] = %q, want the default %q", all[0].Name, Default().Name)
	}
	seen := make(map[string]bool)
	for _, th := range all {
		if seen[th.Name] {
			t.Errorf("duplicate palette %q in Themes()", th.Name)
		}
		seen[th.Name] = true
	}
}

// TestDisplayName checks the human-facing name derived for the theme selector.
func TestDisplayName(t *testing.T) {
	want := map[string]string{
		"harharbinks-kanagawa":   "Kanagawa",
		"harharbinks-gruvbox":    "Gruvbox",
		"harharbinks-everforest": "Everforest",
		"harharbinks-zenburn":    "Zenburn",
	}
	for _, th := range Themes() {
		if got := th.DisplayName(); got != want[th.Name] {
			t.Errorf("%s DisplayName() = %q, want %q", th.Name, got, want[th.Name])
		}
	}
}
