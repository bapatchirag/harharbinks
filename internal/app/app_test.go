package app

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/muesli/termenv"

	"github.com/bapatchirag/harharbinks/internal/config"
	"github.com/bapatchirag/harharbinks/internal/har"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// TestMain forces a color-free profile so golden output is deterministic across
// environments (it captures layout and content, not ANSI color codes).
func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.Ascii)
	os.Exit(m.Run())
}

// demoEntries is a small, stable fixture so the golden frame does not depend on
// the evolving sample capture.
func demoEntries() []har.Entry {
	return []har.Entry{
		{
			Time:            132,
			Request:         har.Request{Method: "GET", URL: "https://api.example.com/users?page=2"},
			Response:        har.Response{Status: 200, StatusText: "OK", Content: har.Content{Size: 4096, MimeType: "application/json"}},
			ServerIPAddress: "93.184.216.34",
		},
		{
			Time:     88,
			Request:  har.Request{Method: "GET", URL: "https://cdn.example.com/app.js"},
			Response: har.Response{Status: 200, StatusText: "OK", Content: har.Content{Size: 88213, MimeType: "application/javascript"}},
		},
		{
			Time:     512,
			Request:  har.Request{Method: "POST", URL: "https://api.example.com/login"},
			Response: har.Response{Status: 302, StatusText: "Found", Content: har.Content{Size: 0, MimeType: "text/html"}},
		},
		{
			Time:     14,
			Request:  har.Request{Method: "GET", URL: "https://example.com/favicon.ico"},
			Response: har.Response{Status: 404, StatusText: "Not Found", Content: har.Content{Size: 512, MimeType: "text/html"}},
		},
		{
			Time:     1240,
			Request:  har.Request{Method: "DELETE", URL: "https://api.example.com/users/7"},
			Response: har.Response{Status: 204, StatusText: "No Content", Content: har.Content{Size: 0}},
		},
	}
}

func newTestApp() *App { return New(NewViewer(demoEntries(), "sample.har")) }

func keyDown() tea.Msg { return tea.KeyMsg{Type: tea.KeyDown} }

// TestUpdateNoticeInHeader verifies that once the launch check reports a newer
// release, the passive notice appears in the app header telling the user how to
// update. It drives the model directly and touches no network.
func TestUpdateNoticeInHeader(t *testing.T) {
	var m tea.Model = newTestApp()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	if before := m.View(); strings.Contains(before, "hhb update") {
		t.Fatalf("header should not advertise an update before a check reports one; view:\n%s", before)
	}
	m, _ = m.Update(updateAvailableMsg{version: "v9.9.9"})
	view := m.View()
	if !strings.Contains(view, "v9.9.9") {
		t.Errorf("header should advertise the available version; view:\n%s", view)
	}
	if !strings.Contains(view, "hhb update") {
		t.Errorf("header should tell the user how to update; view:\n%s", view)
	}
}

// TestViewerGolden renders the full app frame after a couple of moves and
// compares it to a checked-in golden (regenerate with -update).
func TestViewerGolden(t *testing.T) {
	var m tea.Model = newTestApp()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m, _ = m.Update(keyDown())
	m, _ = m.Update(keyDown())
	golden.RequireEqual(t, []byte(m.View()))
}

// TestViewerTeatest drives the app through key events and asserts the highlighted
// row advanced accordingly.
func TestViewerTeatest(t *testing.T) {
	a := newTestApp()
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(100, 24))
	tm.Send(keyDown())
	tm.Send(keyDown())
	tm.Send(keyDown())
	if err := tm.Quit(); err != nil {
		t.Fatalf("quit: %v", err)
	}
	v := tm.FinalModel(t).(*App).screen.(*Viewer)
	if got := v.table.Cursor(); got != 3 {
		t.Errorf("after three downs, cursor = %d, want 3", got)
	}
}

// TestAppQuitKey verifies the router turns the global quit key into tea.Quit.
func TestAppQuitKey(t *testing.T) {
	var m tea.Model = newTestApp()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("q should return a command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("q did not produce tea.QuitMsg")
	}
}

// TestViewerEmpty ensures an empty capture renders an explicit empty state.
func TestViewerEmpty(t *testing.T) {
	v := NewViewer(nil, "empty.har")
	v.SetSize(80, 23)
	if out := v.View(); !strings.Contains(out, "No entries") {
		t.Errorf("empty viewer should indicate no entries; got:\n%s", out)
	}
}

// TestFocusToggle verifies Tab moves focus between the list and the detail
// inspector, changing which pane responds to navigation keys.
func TestFocusToggle(t *testing.T) {
	a := newTestApp()
	var m tea.Model = a
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	v := a.screen.(*Viewer)

	if v.detail.Focused() {
		t.Fatal("detail should start blurred (list focused)")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !v.detail.Focused() {
		t.Fatal("tab should focus the detail inspector")
	}
	before := v.detail.Active()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if v.detail.Active() == before {
		t.Errorf("right should switch the detail tab while focused")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if v.detail.Focused() {
		t.Fatal("tab should return focus to the list")
	}
	cur := v.table.Cursor()
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if v.table.Cursor() != cur+1 {
		t.Errorf("down should move the list selection when focused; cursor=%d want %d", v.table.Cursor(), cur+1)
	}
}

// TestHelpOverlay verifies ? opens the help overlay and a dismiss key closes it.
func TestHelpOverlay(t *testing.T) {
	a := newTestApp()
	var m tea.Model = a
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})

	if strings.Contains(a.View(), "\u2014 keys") {
		t.Fatal("help should be hidden initially")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if !strings.Contains(a.View(), "\u2014 keys") {
		t.Errorf("? should open the help overlay")
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if strings.Contains(a.View(), "\u2014 keys") {
		t.Errorf("esc should close the help overlay")
	}
}

// TestWithConfig verifies a persisted configuration restored at construction
// sets the app's starting theme and is propagated to the screen's components
// before the first frame, so a saved choice colors the UI immediately on launch.
func TestWithConfig(t *testing.T) {
	want := theme.Gruvbox()
	a := New(NewViewer(demoEntries(), "sample.har"), WithConfig(config.Config{Theme: want.Name}))
	if a.theme.Name != want.Name {
		t.Errorf("app theme = %q, want %q", a.theme.Name, want.Name)
	}
	if v := a.screen.(*Viewer); v.theme.Name != want.Name {
		t.Errorf("screen theme = %q, want %q", v.theme.Name, want.Name)
	}
}

// TestSettingsEditorOpens verifies c opens the tabbed configuration editor,
// showing the category tab and the Theme field with its current value.
func TestSettingsEditorOpens(t *testing.T) {
	a := newTestApp()
	var m tea.Model = a
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})

	if a.settingsVisible {
		t.Fatal("settings editor should be hidden initially")
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if !a.settingsVisible {
		t.Fatal("c should open the settings editor")
	}
	view := a.View()
	for _, want := range []string{"settings", "Appearance", "Theme", "Kanagawa"} {
		if !strings.Contains(view, want) {
			t.Errorf("settings view should contain %q; got:\n%s", want, view)
		}
	}
}

// TestSettingsEditorEditsAndPersists verifies that cycling the Theme field in the
// editor recolors the UI live, updates the live config, and persists the change
// immediately (write-on-change), and that esc closes the overlay.
func TestSettingsEditorEditsAndPersists(t *testing.T) {
	var saved []config.Config
	a := New(NewViewer(demoEntries(), "sample.har"), WithConfigSaver(func(c config.Config) {
		saved = append(saved, c)
	}))
	var m tea.Model = a
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	start := a.theme.Name
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})

	if a.theme.Name == start {
		t.Errorf("right should change the theme; still %q", a.theme.Name)
	}
	if a.cfg.Theme != a.theme.Name {
		t.Errorf("live config theme = %q, want %q", a.cfg.Theme, a.theme.Name)
	}
	if v := a.screen.(*Viewer); v.theme.Name != a.theme.Name {
		t.Errorf("editing should recolor the screen live; screen=%q want %q", v.theme.Name, a.theme.Name)
	}
	if len(saved) != 1 {
		t.Fatalf("editing should persist exactly once; saved = %v", saved)
	}
	if saved[0].Theme != a.theme.Name {
		t.Errorf("persisted theme = %q, want %q", saved[0].Theme, a.theme.Name)
	}

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if a.settingsVisible {
		t.Errorf("esc should close the settings editor")
	}
}

// TestSettingsEditorGolden snapshots the settings editor overlay (regenerate with
// -update).
func TestSettingsEditorGolden(t *testing.T) {
	var m tea.Model = newTestApp()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	golden.RequireEqual(t, []byte(m.View()))
}
