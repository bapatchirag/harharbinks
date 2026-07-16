package app

import (
	"bytes"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/bapatchirag/harharbinks/internal/har"
)

// runeKey builds a key message for a single typed rune.
func runeKey(r rune) tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

// sizedViewer returns a viewer over demoEntries, sized like the golden frame.
func sizedViewer() *Viewer {
	v := NewViewer(demoEntries(), "sample.har")
	v.SetSize(100, 23)
	return v
}

// setFilter opens the filter, applies the query string, and rebuilds the view,
// mirroring what typing into the field and committing does.
func setFilter(v *Viewer, q string) {
	v.startSearch()
	v.search.SetValue(q)
	v.query = q
	v.applyView()
}

// TestViewerFilter drives the live filter through the full app: "/" opens the
// field, typing narrows the list, and only the matching host remains.
func TestViewerFilter(t *testing.T) {
	a := New(NewViewer(demoEntries(), "sample.har"))
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(100, 24))

	tm.Send(runeKey('/'))
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("cdn")})

	// teatest.WaitFor tees every frame ever rendered into one growing buffer that
	// is never reset, so a negative assertion (the "users?page=2" row is gone) can
	// never come true once the initial unfiltered frame has been flushed — which is
	// exactly what happens under load on slower CI runners, making the old check a
	// guaranteed timeout there. Wait instead for a positive, monotonic marker: the
	// open filter line echoing the typed query ("/ cdn", unique since it is not a
	// substring of "cdn.example.com"). Because bubbletea serializes Update calls,
	// the frame that prints "/ cdn" has already processed the SearchMsg that
	// applies the filter; the authoritative row count is asserted on the final
	// model below.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("/ cdn"))
	}, teatest.WithDuration(10*time.Second))

	if err := tm.Quit(); err != nil {
		t.Fatalf("quit: %v", err)
	}
	v := tm.FinalModel(t).(*App).screen.(*Viewer)
	if got := len(v.table.Rows()); got != 1 {
		t.Errorf("filtered rows = %d, want 1", got)
	}
	if !v.CapturesInput() {
		t.Error("viewer should still be capturing input while the filter is open")
	}
}

// TestViewerFilterAllFields verifies the filter matches across any entry field,
// not just the URL.
func TestViewerFilterAllFields(t *testing.T) {
	cases := []struct {
		q    string
		want int
	}{
		{"POST", 1},        // request method (entry 2)
		{"404", 1},         // response status (entry 3)
		{"javascript", 1},  // response MIME type (entry 1)
		{"DELETE", 1},      // request method (entry 4)
		{"example.com", 5}, // host substring shared by every URL
		{"zzznope", 0},     // matches nothing
	}
	for _, c := range cases {
		v := sizedViewer()
		setFilter(v, c.q)
		if got := len(v.table.Rows()); got != c.want {
			t.Errorf("filter %q rows = %d, want %d", c.q, got, c.want)
		}
	}
}

// TestViewerScopedMethodFilter verifies method:POST scopes to the method field:
// a GET whose body contains "postMessage" is excluded, though a free "post"
// search still matches it.
func TestViewerScopedMethodFilter(t *testing.T) {
	entries := []har.Entry{
		{Request: har.Request{Method: "POST", URL: "https://x/login"}},
		{
			Request:  har.Request{Method: "GET", URL: "https://x/sdk.js"},
			Response: har.Response{Content: har.Content{MimeType: "application/javascript", Text: "a.postMessage(1)"}},
		},
	}
	v := NewViewer(entries, "x.har")
	v.SetSize(100, 23)

	setFilter(v, "method:POST")
	if got := len(v.table.Rows()); got != 1 {
		t.Fatalf("method:POST rows = %d, want 1 (the POST only)", got)
	}
	if sel, _ := v.table.Selected(); sel.Request.Method != "POST" {
		t.Errorf("method:POST selected %q method, want POST", sel.Request.Method)
	}

	setFilter(v, "post")
	if got := len(v.table.Rows()); got != 2 {
		t.Errorf("free \"post\" rows = %d, want 2 (method + postMessage body)", got)
	}
}

// TestViewerCancelFilterRestores verifies esc clears the filter and restores the
// full list.
func TestViewerCancelFilterRestores(t *testing.T) {
	v := sizedViewer()
	v.startSearch()
	v.search.SetValue("cdn")
	v.query = "cdn"
	v.applyView()
	if len(v.table.Rows()) != 1 {
		t.Fatalf("filtered rows = %d, want 1", len(v.table.Rows()))
	}
	v.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if v.CapturesInput() {
		t.Error("esc should close the filter field")
	}
	if got := len(v.table.Rows()); got != len(demoEntries()) {
		t.Errorf("rows after cancel = %d, want %d", got, len(demoEntries()))
	}
}

// TestViewerEscClearsFilter verifies esc clears an applied filter directly, with
// the filter field already closed (no need to reopen it with "/").
func TestViewerEscClearsFilter(t *testing.T) {
	v := sizedViewer()
	// Apply a filter and commit it: the field closes but the filter stays.
	v.startSearch()
	v.search.SetValue("cdn")
	v.query = "cdn"
	v.applyView()
	v.commitSearch()
	if v.CapturesInput() {
		t.Fatal("filter field should be closed after commit")
	}
	if len(v.table.Rows()) != 1 {
		t.Fatalf("committed filter rows = %d, want 1", len(v.table.Rows()))
	}
	// Esc on the closed, filtered list clears the filter without reopening it.
	v.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if v.query != "" {
		t.Errorf("esc should clear the query, got %q", v.query)
	}
	if got := len(v.table.Rows()); got != len(demoEntries()) {
		t.Errorf("rows after esc = %d, want %d", got, len(demoEntries()))
	}
}

// TestViewerSort verifies the sort key cycles and reorders the table.
func TestViewerSort(t *testing.T) {
	v := sizedViewer()
	// Original order keeps the first row as the first GET.
	if first, _ := v.table.Selected(); first.Request.Method != "GET" {
		t.Fatalf("initial first row method = %q, want GET", first.Request.Method)
	}
	v.Update(runeKey('s')) // none -> method (ascending)
	first := v.table.Rows()[0]
	if first.Request.Method != "DELETE" {
		t.Errorf("after method sort, first row = %q, want DELETE", first.Request.Method)
	}
}

// TestViewerSortReverse verifies Shift+S steps the sort cycle backwards, wrapping
// from the initial "none" to the last preset.
func TestViewerSortReverse(t *testing.T) {
	v := sizedViewer()
	v.Update(runeKey('S')) // none -> url (wraps backward), ascending
	if got := v.table.Rows()[0].Request.URL; got != "https://api.example.com/login" {
		t.Errorf("reverse-sort first row = %q, want the login URL", got)
	}
	v.Update(runeKey('s')) // url -> none (forward), original order restored
	if got := v.table.Rows()[0].Request.URL; got != "https://api.example.com/users?page=2" {
		t.Errorf("after wrapping back to none, first row = %q, want the users URL", got)
	}
}

// TestViewerFollowSession verifies enter opens the selected entry's session
// (grouped by host here, since the fixture has no connection ids) and esc
// restores the full list with the original selection.
func TestViewerFollowSession(t *testing.T) {
	v := sizedViewer()
	v.Update(tea.KeyMsg{Type: tea.KeyDown}) // -> index 1
	v.Update(tea.KeyMsg{Type: tea.KeyDown}) // -> index 2 (POST login, api.example.com)
	v.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !v.sessionMode {
		t.Fatal("enter should open session mode")
	}
	if got := len(v.table.Rows()); got != 3 {
		t.Errorf("session rows = %d, want 3 (api.example.com)", got)
	}
	if sel, _ := v.table.Selected(); sel.Request.URL != "https://api.example.com/login" {
		t.Errorf("session highlight = %q, want the login exchange", sel.Request.URL)
	}

	v.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if v.sessionMode {
		t.Fatal("esc should leave session mode")
	}
	if got := len(v.table.Rows()); got != len(demoEntries()) {
		t.Errorf("rows after leaving session = %d, want %d", got, len(demoEntries()))
	}
	if sel, _ := v.table.Selected(); sel.Request.URL != "https://api.example.com/login" {
		t.Errorf("selection after leaving session = %q, want the login exchange", sel.Request.URL)
	}
}

// TestViewerSessionGolden renders the full app frame in follow-session mode.
func TestViewerSessionGolden(t *testing.T) {
	var m tea.Model = New(NewViewer(demoEntries(), "sample.har"))
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	golden.RequireEqual(t, []byte(m.View()))
}

// TestViewerSearchGolden renders the full app frame with the filter field open
// and a query applied. The field is blurred so the golden omits the blinking
// cursor while still exercising the search-line layout.
func TestViewerSearchGolden(t *testing.T) {
	v := NewViewer(demoEntries(), "sample.har")
	var m tea.Model = New(v)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	v.startSearch()
	v.search.SetValue("api")
	v.query = "api"
	v.applyView()
	v.search.Blur()
	golden.RequireEqual(t, []byte(m.View()))
}
