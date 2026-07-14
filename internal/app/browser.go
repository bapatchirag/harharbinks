// This file implements the file browser screen: a full-window, filterable file
// browser used to choose a capture to open. It is the app-layer adapter around
// the generic FileBrowser component — it restricts selection to HAR files, and
// when the user picks one it loads the capture and hands off to the viewer. A
// "/" search filters the current directory; esc clears that filter (and, for a
// browser opened from the viewer, then steps back to it).
package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/bapatchirag/harharbinks/internal/har"
	"github.com/bapatchirag/harharbinks/internal/tui/component"
	"github.com/bapatchirag/harharbinks/internal/tui/keymap"
	"github.com/bapatchirag/harharbinks/internal/tui/layout"
	"github.com/bapatchirag/harharbinks/internal/tui/msg"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// Browser is the file browser screen. It composes the generic FileBrowser
// component with a status bar and adapts a chosen path into a loaded HAR viewer.
// When back is non-nil, esc returns to that screen (the viewer that opened it)
// rather than doing nothing.
type Browser struct {
	theme     theme.Theme
	keys      keymap.KeyMap
	browser   *component.FileBrowser
	search    *component.Search
	status    *component.StatusBar
	back      Screen
	searching bool
	err       string
	width     int
	height    int
}

// NewBrowser returns a file browser rooted at the working directory, restricted
// to HAR files. It is the entry screen when hhb is launched without a file.
func NewBrowser() *Browser { return newBrowser(nil) }

// NewBrowserReturning is like NewBrowser but remembers back as the screen to
// restore when the user presses esc, so opening the browser from the viewer is
// cancelable.
func NewBrowserReturning(back Screen) *Browser { return newBrowser(back) }

func newBrowser(back Screen) *Browser {
	th := theme.Default()
	km := keymap.Default()
	b := &Browser{
		theme:   th,
		keys:    km,
		browser: component.NewFileBrowser(th, []string{".har"}),
		search:  component.NewSearch(th, "filter files\u2026"),
		status:  component.NewStatusBar(th),
		back:    back,
	}
	b.browser.Focus()
	return b
}

// Title implements Screen.
func (b *Browser) Title() string { return "open file" }

// Help implements Screen, describing the browser's key bindings for the overlay.
func (b *Browser) Help() string {
	lines := []string{
		"Browse",
		"  up/down, j/k       move selection",
		"  right/l, enter     open directory / select file",
		"  left/h             parent directory",
		"  pgup/pgdn          page          g / G  top / bottom",
		"  /                  filter entries in this directory",
		"",
		"General",
	}
	if b.back != nil {
		lines = append(lines, "  esc                clear filter, then back to viewer")
	} else {
		lines = append(lines, "  esc                clear filter")
	}
	return strings.Join(lines, "\n")
}

// CapturesInput implements Screen: while the filter field is open it consumes
// every keystroke so characters are typed into the field rather than triggering
// global actions.
func (b *Browser) CapturesInput() bool { return b.searching }

// SetTheme implements Screen, swapping the browser's palette at runtime and
// propagating it to the picker, filter field, and status bar so the in-app theme
// selector recolors the screen live.
func (b *Browser) SetTheme(th theme.Theme) {
	b.theme = th
	b.browser.SetTheme(th)
	b.search.SetTheme(th)
	b.status.SetTheme(th)
}

// Init implements Screen, starting the initial directory read.
func (b *Browser) Init() tea.Cmd { return b.browser.Init() }

// Update implements Screen. It intercepts a completed selection (loading the
// capture and handing off to the viewer) and live-filter changes, then routes
// keys either to the open filter field or to the directory browser.
func (b *Browser) Update(tmsg tea.Msg) tea.Cmd {
	switch m := tmsg.(type) {
	case msg.FileSelectedMsg:
		return b.open(m.Path)
	case msg.SearchMsg:
		// Live filter as the user types in the "/" field.
		b.browser.SetFilter(m.Query)
		return nil
	case tea.KeyMsg:
		if b.searching {
			return b.handleSearchKey(m)
		}
		return b.handleKey(m)
	}
	return b.browser.Update(tmsg)
}

// handleKey routes a key while the filter field is closed: "/" opens the filter,
// esc clears an active filter (or steps back to the opener), and everything else
// drives the directory browser.
func (b *Browser) handleKey(k tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(k, b.keys.Search):
		b.startSearch()
		return nil
	case key.Matches(k, b.keys.Back):
		return b.handleBack()
	}
	return b.browser.Update(k)
}

// handleBack implements esc when the filter field is closed. It clears an active
// filter first; only with no filter to clear does it return to the screen that
// opened the browser. Esc never quits — use q to exit.
func (b *Browser) handleBack() tea.Cmd {
	if b.browser.Filter() != "" {
		b.browser.SetFilter("")
		return nil
	}
	if b.back != nil {
		return SwitchTo(b.back)
	}
	return nil
}

// handleSearchKey routes a key while the filter field is open: enter keeps the
// filter and closes the field, esc clears it and closes, and anything else edits
// the query (which live-filters through msg.SearchMsg).
func (b *Browser) handleSearchKey(k tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(k, b.keys.Enter):
		b.commitSearch()
		return nil
	case key.Matches(k, b.keys.Back):
		b.cancelSearch()
		return nil
	}
	return b.search.Update(k)
}

// startSearch opens the filter field, seeded with the active filter.
func (b *Browser) startSearch() {
	b.searching = true
	b.search.SetValue(b.browser.Filter())
	b.search.Focus()
}

// commitSearch closes the filter field, keeping the current filter applied.
func (b *Browser) commitSearch() {
	b.searching = false
	b.search.Blur()
}

// cancelSearch closes the filter field and clears the filter, restoring the full
// listing.
func (b *Browser) cancelSearch() {
	b.searching = false
	b.search.Blur()
	b.search.SetValue("")
	b.browser.SetFilter("")
}

// open loads the HAR at path and, on success, switches to a viewer over its
// entries. A load failure is surfaced in the status bar and the browser stays
// open so the user can pick another file.
func (b *Browser) open(path string) tea.Cmd {
	h, err := har.ParseFile(path)
	if err != nil {
		b.err = filepath.Base(path) + ": " + err.Error()
		return nil
	}
	b.err = ""
	return SwitchTo(NewViewer(h.Log.Entries, path))
}

// SetSize implements Screen, giving the picker the area between a one-line
// directory header and a one-line status bar.
func (b *Browser) SetSize(w, h int) {
	b.width, b.height = w, h
	bodyH := h - 2 // reserve the directory header and the status bar
	if bodyH < 1 {
		bodyH = 1
	}
	b.browser.SetSize(w, bodyH)
	b.status.SetSize(w, 1)
	b.search.SetSize(w, 1)
}

// View implements Screen, stacking a current-directory header over the file
// listing over the status bar of hints (or, while filtering, the "/" field).
func (b *Browser) View() string {
	if b.width == 0 || b.height == 0 {
		return ""
	}
	b.refreshStatus()
	bodyH := b.height - 2
	if bodyH < 1 {
		bodyH = 1
	}
	body := fitLines(strings.Split(b.browser.View(), "\n"), b.width, bodyH)
	bottom := b.status.View()
	if b.searching {
		bottom = b.search.View()
	}
	return layout.SplitVertical(
		b.dirHeader(),
		layout.SplitVertical(body, bottom),
	)
}

// dirHeader renders the picker's current directory as a themed full-width bar.
// The home directory is shortened to "~", and a long path is truncated from the
// left (with a leading ellipsis) so the current folder stays visible.
func (b *Browser) dirHeader() string {
	label := truncateLeftToWidth(" "+b.currentDir()+" ", b.width)
	return b.theme.Header().Render(padTo(label, b.width))
}

// currentDir returns the picker's current directory with the user's home
// directory shortened to "~" for a compact, familiar display.
func (b *Browser) currentDir() string {
	dir := b.browser.CurrentDir()
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if dir == home {
			return "~"
		}
		if strings.HasPrefix(dir, home+string(os.PathSeparator)) {
			return "~" + dir[len(home):]
		}
	}
	return dir
}

// refreshStatus sets the browser's status line: a load error when present,
// otherwise the selection prompt (or active filter) and context key hints.
func (b *Browser) refreshStatus() {
	if b.err != "" {
		b.status.SetLeft(" " + b.err + " ")
		b.status.SetCenter("")
		b.status.SetRight("")
		return
	}
	if q := b.browser.Filter(); q != "" {
		b.status.SetLeft(fmt.Sprintf(" filter: %s ", q))
	} else {
		b.status.SetLeft(" select a .har file ")
	}
	b.status.SetCenter("")
	// →/← enter and leave directories; enter opens a dir or selects a file.
	parts := []string{"\u2191/\u2193 move", "\u2192/\u2190 in/out dir"}
	if b.browser.Filter() != "" {
		parts = append(parts, "esc clear")
	} else {
		parts = append(parts, "/ find")
		if b.back != nil {
			parts = append(parts, "esc back")
		}
	}
	parts = append(parts, "enter open", "? help", "q quit")
	b.status.SetRight(" " + strings.Join(parts, " \u00b7 ") + " ")
}

// truncateLeftToWidth clips s from the left to at most w display cells, adding a
// leading ellipsis, so the trailing end of the string (here, the current folder)
// stays visible when the path is wider than the screen.
func truncateLeftToWidth(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= w {
		return s
	}
	if w == 1 {
		return "\u2026"
	}
	r := []rune(s)
	for len(r) > 0 && ansi.StringWidth(string(r))+1 > w {
		r = r[1:]
	}
	return "\u2026" + string(r)
}

var _ Screen = (*Browser)(nil)
