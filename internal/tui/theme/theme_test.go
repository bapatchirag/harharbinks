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
