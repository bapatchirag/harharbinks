package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/bapatchirag/harharbinks/internal/tui/msg"
)

// sampleHAR is the shared fixture capture at the repository root, addressed
// relative to this package's directory.
const sampleHAR = "../../testdata/sample.har"

// TestBrowserOpensViewerOnSelect verifies that choosing a file loads the capture
// and hands off to the viewer through the router's switch message.
func TestBrowserOpensViewerOnSelect(t *testing.T) {
	a := New(NewBrowser())
	var m tea.Model = a
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})

	// The file browser emits FileSelectedMsg once the user picks a file.
	_, cmd := m.Update(msg.FileSelectedMsg{Path: sampleHAR})
	if cmd == nil {
		t.Fatal("selecting a file should return a switch command")
	}
	sw, ok := cmd().(SwitchScreenMsg)
	if !ok {
		t.Fatalf("expected SwitchScreenMsg, got %T", cmd())
	}
	if _, ok := sw.Screen.(*Viewer); !ok {
		t.Fatalf("selection should switch to a *Viewer, got %T", sw.Screen)
	}

	// Feed the switch back through the router and confirm the active screen changed.
	m, _ = m.Update(sw)
	if _, ok := a.screen.(*Viewer); !ok {
		t.Errorf("router should now show the viewer, got %T", a.screen)
	}
}

// TestBrowserTeatestOpensViewer drives the whole program: an injected selection
// switches to the viewer, whose list-focused status hints then appear.
func TestBrowserTeatestOpensViewer(t *testing.T) {
	a := New(NewBrowser())
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(100, 24))
	tm.Send(msg.FileSelectedMsg{Path: sampleHAR})

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("tab detail"))
	}, teatest.WithDuration(3*time.Second))

	if err := tm.Quit(); err != nil {
		t.Fatalf("quit: %v", err)
	}
	if _, ok := tm.FinalModel(t).(*App).screen.(*Viewer); !ok {
		t.Errorf("after selecting a file the active screen should be the viewer")
	}
}

// TestBrowserOpenErrorStays verifies a failed load keeps the browser open and
// surfaces the error in its status line rather than switching screens.
func TestBrowserOpenErrorStays(t *testing.T) {
	b := NewBrowser()
	b.SetSize(80, 24)

	if cmd := b.open("does-not-exist.har"); cmd != nil {
		t.Fatal("a failed load should not switch screens")
	}
	if b.err == "" {
		t.Error("a failed load should record an error for the status bar")
	}
	if out := b.View(); !strings.Contains(out, "does-not-exist.har") {
		t.Errorf("browser view should surface the load error; got:\n%s", out)
	}
}

// TestBrowserEscReturnsToViewer verifies a browser opened from the viewer returns
// to that exact viewer on esc, preserving its state.
func TestBrowserEscReturnsToViewer(t *testing.T) {
	v := NewViewer(demoEntries(), "sample.har")
	b := NewBrowserReturning(v)
	b.SetSize(80, 24)

	cmd := b.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("esc with a return screen should switch back")
	}
	sw, ok := cmd().(SwitchScreenMsg)
	if !ok {
		t.Fatalf("expected SwitchScreenMsg, got %T", cmd())
	}
	if sw.Screen != v {
		t.Error("esc should return to the originating viewer instance")
	}
}

// TestBrowserRootEscNoFilterNoop verifies esc in the root browser with no active
// filter is a no-op: it no longer quits (use q) and does not navigate up.
func TestBrowserRootEscNoFilterNoop(t *testing.T) {
	b := NewBrowser()
	b.SetSize(80, 24)
	if cmd := b.Update(tea.KeyMsg{Type: tea.KeyEsc}); cmd != nil {
		t.Errorf("esc with no filter in the root browser should be a no-op, got a command")
	}
}

// TestBrowserFilterNarrowsAndEscClears verifies "/" opens the filter, typing
// narrows the listing to matching entries, and esc clears it and closes the field.
func TestBrowserFilterNarrowsAndEscClears(t *testing.T) {
	b := NewBrowser() // rooted at this package dir (internal/app)
	b.SetSize(100, 24)

	b.Update(runeKey('/'))
	if !b.CapturesInput() {
		t.Fatal("/ should open the filter field")
	}
	b.Update(msg.SearchMsg{Query: "browser"})
	if got := b.browser.Filter(); got != "browser" {
		t.Fatalf("filter = %q, want browser", got)
	}
	out := b.View()
	if !strings.Contains(out, "browser.go") {
		t.Errorf("filter should keep browser.go; view:\n%s", out)
	}
	if strings.Contains(out, "app.go") {
		t.Errorf("filter should hide app.go; view:\n%s", out)
	}

	b.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if b.CapturesInput() {
		t.Error("esc should close the filter field")
	}
	if b.browser.Filter() != "" {
		t.Errorf("esc should clear the filter, got %q", b.browser.Filter())
	}
}

// TestBrowserEscClearsCommittedFilter verifies that after committing a filter
// with enter (field closed), esc clears the still-applied filter.
func TestBrowserEscClearsCommittedFilter(t *testing.T) {
	b := NewBrowser()
	b.SetSize(100, 24)

	b.Update(runeKey('/'))
	b.Update(msg.SearchMsg{Query: "browser"})
	b.Update(tea.KeyMsg{Type: tea.KeyEnter}) // commit: field closes, filter stays
	if b.CapturesInput() {
		t.Fatal("enter should close the filter field")
	}
	if b.browser.Filter() != "browser" {
		t.Fatalf("commit should keep the filter, got %q", b.browser.Filter())
	}

	b.Update(tea.KeyMsg{Type: tea.KeyEsc}) // esc clears the committed filter
	if b.browser.Filter() != "" {
		t.Errorf("esc should clear the committed filter, got %q", b.browser.Filter())
	}
}

// TestBrowserShowsCurrentDir verifies the browser renders its current directory
// in the header bar.
func TestBrowserShowsCurrentDir(t *testing.T) {
	b := NewBrowser()
	b.SetSize(100, 24)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	leaf := filepath.Base(cwd)
	if out := b.View(); !strings.Contains(out, leaf) {
		t.Errorf("browser header should show the current directory %q; view:\n%s", leaf, out)
	}
}

// TestViewerOpenKeyOpensBrowser verifies the viewer's "o" key opens the file
// browser, remembering the viewer so esc can cancel back to it.
func TestViewerOpenKeyOpensBrowser(t *testing.T) {
	v := sizedViewer()
	cmd := v.Update(runeKey('o'))
	if cmd == nil {
		t.Fatal("o should return a switch command")
	}
	sw, ok := cmd().(SwitchScreenMsg)
	if !ok {
		t.Fatalf("expected SwitchScreenMsg, got %T", cmd())
	}
	br, ok := sw.Screen.(*Browser)
	if !ok {
		t.Fatalf("o should open the file browser, got %T", sw.Screen)
	}
	if br.back != v {
		t.Error("the browser opened from the viewer should return to it on esc")
	}
}
