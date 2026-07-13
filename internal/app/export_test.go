package app

import (
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/bapatchirag/harharbinks/internal/har"
	"github.com/bapatchirag/harharbinks/internal/tui/msg"
)

// TestViewerMenuOpenClose verifies "e" opens the export menu (capturing input
// and rendering the box) and esc closes it.
func TestViewerMenuOpenClose(t *testing.T) {
	v := sizedViewer()
	if v.CapturesInput() {
		t.Fatal("viewer should not capture input before the menu opens")
	}

	v.Update(runeKey('e'))
	if !v.menuOpen || !v.CapturesInput() {
		t.Fatal("\"e\" should open the export menu and capture input")
	}
	if out := v.View(); !strings.Contains(out, "Export") {
		t.Errorf("open menu should render the Export box; view:\n%s", out)
	}

	v.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if v.menuOpen || v.CapturesInput() {
		t.Error("esc should close the export menu")
	}
}

// TestViewerMenuShortcutTriggersAction verifies an item's shortcut key (here
// "u" for Copy URL) activates it directly while the menu is open, then closes it.
func TestViewerMenuShortcutTriggersAction(t *testing.T) {
	var got string
	restore := stubClipboard(func(s string) error { got = s; return nil })
	defer restore()

	v := sizedViewer()
	v.Update(runeKey('e'))        // open the export menu
	cmd := v.Update(runeKey('u')) // "u" -> Copy URL
	if cmd == nil {
		t.Fatal("a shortcut key should emit a menu action command")
	}
	v.Update(cmd()) // deliver the resulting MenuActionMsg back to the viewer

	if want := "https://api.example.com/users?page=2"; got != want {
		t.Errorf("copied URL = %q, want %q", got, want)
	}
	if v.menuOpen {
		t.Error("activating an item should close the menu")
	}
}

// TestViewerCopyURLAction verifies the copy-URL action copies the selected
// entry's URL and raises a success toast.
func TestViewerCopyURLAction(t *testing.T) {
	var got string
	restore := stubClipboard(func(s string) error { got = s; return nil })
	defer restore()

	v := sizedViewer() // cursor starts on the first demo entry
	cmd := v.Update(msg.MenuActionMsg{Action: actionCopyURL})

	if want := "https://api.example.com/users?page=2"; got != want {
		t.Errorf("copied URL = %q, want %q", got, want)
	}
	if !v.toast.Visible() {
		t.Error("a successful copy should show a toast")
	}
	if cmd == nil {
		t.Error("showing a toast should return its auto-dismiss command")
	}
	if out := v.View(); !strings.Contains(out, "URL copied") {
		t.Errorf("toast text should appear in the view; view:\n%s", out)
	}
}

// TestViewerCopyCurlAction verifies the copy-cURL action copies the entry
// rendered by har.Curl.
func TestViewerCopyCurlAction(t *testing.T) {
	var got string
	restore := stubClipboard(func(s string) error { got = s; return nil })
	defer restore()

	v := sizedViewer()
	e, _ := v.table.Selected()
	v.Update(msg.MenuActionMsg{Action: actionCopyCurl})

	if want := har.Curl(e); got != want {
		t.Errorf("copied cURL = %q, want %q", got, want)
	}
}

// TestViewerCopyFailureToast verifies a clipboard error is surfaced as an error
// toast rather than being swallowed.
func TestViewerCopyFailureToast(t *testing.T) {
	restore := stubClipboard(func(string) error { return errors.New("no clipboard") })
	defer restore()

	v := sizedViewer()
	v.Update(msg.MenuActionMsg{Action: actionCopyURL})

	if !v.toast.Visible() {
		t.Fatal("a failed copy should still show a toast")
	}
	if out := v.View(); !strings.Contains(out, "Copy failed") {
		t.Errorf("failed copy should surface an error toast; view:\n%s", out)
	}
}

// TestViewerSaveBodyAction verifies the save action writes the body and reports
// success, redirecting the write to a temp dir.
func TestViewerSaveBodyAction(t *testing.T) {
	dir := t.TempDir()
	orig := bodySaveDir
	bodySaveDir = dir
	defer func() { bodySaveDir = orig }()

	v := sizedViewer() // entry 0: /users?page=2 -> file name "users"
	v.Update(msg.MenuActionMsg{Action: actionSaveBody})

	if !v.toast.Visible() {
		t.Fatal("save should show a toast")
	}
	if _, err := os.Stat(filepath.Join(dir, "users")); err != nil {
		t.Errorf("save should write the response body file: %v", err)
	}
}

// TestWriteBody verifies a plain-text body is written under the URL's base name.
func TestWriteBody(t *testing.T) {
	dir := t.TempDir()
	e := har.Entry{
		Request:  har.Request{URL: "https://example.com/assets/app.js"},
		Response: har.Response{Content: har.Content{Text: "console.log(1)", MimeType: "application/javascript"}},
	}

	name, err := writeBody(dir, e)
	if err != nil {
		t.Fatal(err)
	}
	if name != "app.js" {
		t.Errorf("name = %q, want app.js", name)
	}
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "console.log(1)" {
		t.Errorf("body = %q, want console.log(1)", b)
	}
}

// TestWriteBodyBase64 verifies a base64-encoded body is decoded before writing.
func TestWriteBodyBase64(t *testing.T) {
	dir := t.TempDir()
	e := har.Entry{
		Request: har.Request{URL: "https://example.com/data"},
		Response: har.Response{Content: har.Content{
			Text:     base64.StdEncoding.EncodeToString([]byte("hello")),
			Encoding: "base64",
		}},
	}

	name, err := writeBody(dir, e)
	if err != nil {
		t.Fatal(err)
	}
	if name != "data" {
		t.Errorf("name = %q, want data", name)
	}
	b, _ := os.ReadFile(filepath.Join(dir, name))
	if string(b) != "hello" {
		t.Errorf("body = %q, want hello", b)
	}
}

// TestBodyFileNameFallback verifies URLs with no usable final segment fall back
// to "response" rather than a directory-like or empty name.
func TestBodyFileNameFallback(t *testing.T) {
	cases := map[string]string{
		"https://example.com/":             "response",
		"https://example.com":              "response",
		"https://example.com/img/logo.png": "logo.png",
	}
	for u, want := range cases {
		got := bodyFileName(har.Entry{Request: har.Request{URL: u}})
		if got != want {
			t.Errorf("bodyFileName(%q) = %q, want %q", u, got, want)
		}
	}
}

// TestViewerMenuToastTeatest drives the whole program: open the menu, activate
// the first item, and confirm the confirmation toast renders.
func TestViewerMenuToastTeatest(t *testing.T) {
	restore := stubClipboard(func(string) error { return nil })
	defer restore()

	a := New(NewViewer(demoEntries(), "sample.har"))
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(100, 24))
	tm.Send(runeKey('e'))                   // open the export menu
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter}) // activate "Copy URL"

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("URL copied"))
	}, teatest.WithDuration(3*time.Second))

	if err := tm.Quit(); err != nil {
		t.Fatalf("quit: %v", err)
	}
}

// stubClipboard swaps the package clipboard writer for the duration of a test,
// returning a function that restores the original.
func stubClipboard(fn func(string) error) func() {
	orig := copyToClipboard
	copyToClipboard = fn
	return func() { copyToClipboard = orig }
}
