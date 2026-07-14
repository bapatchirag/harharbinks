package app

import (
	"strings"
	"testing"

	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// TestWindowRows exercises the pane scrolling core: it returns a fixed-height
// window that follows the cursor and pads short lists, which is how both the
// category sidebar and the field pane stay navigable as they grow.
func TestWindowRows(t *testing.T) {
	rows := []string{"0", "1", "2", "3", "4"}

	// Cursor at the top: the first `height` rows, no scrolling yet.
	scroll := 0
	if got := windowRows(rows, 0, 3, &scroll); scroll != 0 || strings.Join(got, ",") != "0,1,2" {
		t.Fatalf("top window = %v scroll=%d, want [0 1 2] scroll=0", got, scroll)
	}

	// Cursor past the window: scrolls down so the cursor row is the last visible.
	if got := windowRows(rows, 4, 3, &scroll); scroll != 2 || strings.Join(got, ",") != "2,3,4" {
		t.Fatalf("scrolled window = %v scroll=%d, want [2 3 4] scroll=2", got, scroll)
	}

	// Cursor back at the top: scrolls up to reveal it again.
	if got := windowRows(rows, 0, 3, &scroll); scroll != 0 || strings.Join(got, ",") != "0,1,2" {
		t.Fatalf("scroll-up window = %v scroll=%d, want [0 1 2] scroll=0", got, scroll)
	}

	// Fewer rows than the height: padded with blanks to exactly height lines.
	scroll = 0
	got := windowRows([]string{"only"}, 0, 3, &scroll)
	if len(got) != 3 || got[0] != "only" || got[1] != "" || got[2] != "" {
		t.Fatalf("padded window = %q, want [only, \"\", \"\"]", got)
	}
}

// TestSettingsNavigationWraps verifies category and field navigation with several
// categories: the field cursor wraps within the active category, and tab switches
// category (wrapping) while resetting the field cursor and its scroll offset.
func TestSettingsNavigationWraps(t *testing.T) {
	s := &settingsModel{tabs: []settingsTab{
		{name: "A", fields: []settingField{{label: "x"}, {label: "y"}, {label: "z"}}},
		{name: "B", fields: []settingField{{label: "p"}}},
	}}

	s.moveCursor(1)
	s.moveCursor(1)
	if s.cursor != 2 {
		t.Fatalf("cursor after two downs = %d, want 2", s.cursor)
	}
	s.moveCursor(1)
	if s.cursor != 0 {
		t.Errorf("cursor should wrap to the first field; got %d", s.cursor)
	}
	s.moveCursor(-1)
	if s.cursor != 2 {
		t.Errorf("cursor should wrap back to the last field; got %d", s.cursor)
	}

	s.cursor = 2
	s.scroll = 5
	s.switchTab(1)
	if s.activeTab != 1 || s.cursor != 0 || s.scroll != 0 {
		t.Fatalf("after switchTab: tab=%d cursor=%d scroll=%d, want 1,0,0", s.activeTab, s.cursor, s.scroll)
	}
	s.switchTab(1)
	if s.activeTab != 0 {
		t.Errorf("switchTab should wrap to the first category; got %d", s.activeTab)
	}
}

// TestCategoryTabsScroll verifies the horizontal tab bar scrolls when the
// category headings overflow the available width: the active tab stays visible
// and an overflow marker appears on whichever side has hidden tabs.
func TestCategoryTabsScroll(t *testing.T) {
	th := theme.Default()
	names := []string{"Alpha", "Bravo", "Charlie", "Delta", "Echo", "Foxtrot"}
	tabs := make([]settingsTab, len(names))
	for i, n := range names {
		tabs[i] = settingsTab{name: n}
	}
	s := &settingsModel{tabs: tabs}

	// Active tab at the start: it is visible, a right marker shows more to the
	// right, and there is no left marker.
	s.activeTab = 0
	first := s.categoryTabs(th, 24)
	if !strings.Contains(first, "Alpha") {
		t.Errorf("active first tab should be visible; got %q", first)
	}
	if !strings.Contains(first, "\u203a") {
		t.Errorf("expected a right overflow marker; got %q", first)
	}
	if strings.Contains(first, "\u2039") {
		t.Errorf("did not expect a left marker at the start; got %q", first)
	}

	// Active tab at the end: it is visible and a left marker shows more to the left.
	s.activeTab = len(names) - 1
	last := s.categoryTabs(th, 24)
	if !strings.Contains(last, "Foxtrot") {
		t.Errorf("active last tab should be visible; got %q", last)
	}
	if !strings.Contains(last, "\u2039") {
		t.Errorf("expected a left overflow marker; got %q", last)
	}

	// A wide bar fits everything: every heading shows and no markers appear.
	all := s.categoryTabs(th, 200)
	for _, n := range names {
		if !strings.Contains(all, n) {
			t.Errorf("wide bar should show %q; got %q", n, all)
		}
	}
	if strings.ContainsAny(all, "\u2039\u203a") {
		t.Errorf("no overflow markers expected when everything fits; got %q", all)
	}
}
