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
