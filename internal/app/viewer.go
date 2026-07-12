// This file implements the HAR viewer screen: a scrollable table of
// request/response entries on top and a tabbed detail inspector for the
// highlighted entry below. It is the app-layer adapter that maps har.Entry
// values into the generic Table component through per-column render functions,
// so the component itself stays unaware of HAR. Row navigation drives the table;
// the inspector (see detail.go) tracks the selection and switches its own tabs.
package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/bapatchirag/harharbinks/internal/har"
	"github.com/bapatchirag/harharbinks/internal/tui/component"
	"github.com/bapatchirag/harharbinks/internal/tui/focus"
	"github.com/bapatchirag/harharbinks/internal/tui/keymap"
	"github.com/bapatchirag/harharbinks/internal/tui/layout"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// Viewer is the HAR viewer screen. It composes a Table of entries with a tabbed
// detail inspector and a status bar, adapting the HAR domain into the generic
// components. Tab moves focus between the list and the inspector.
type Viewer struct {
	theme   theme.Theme
	keys    keymap.KeyMap
	table   *component.Table[har.Entry]
	detail  *Detail
	status  *component.StatusBar
	focus   *focus.Manager
	entries []har.Entry
	title   string
	curIdx  int

	width   int
	height  int
	tableH  int
	detailH int
}

// NewViewer builds a HAR viewer for the given entries. source is the file path
// the entries were loaded from; its base name is shown in the header.
func NewViewer(entries []har.Entry, source string) *Viewer {
	th := theme.Default()
	km := keymap.Default()

	table := component.NewTable([]component.Column[har.Entry]{
		{Title: "METHOD", Width: 7,
			Render: func(e har.Entry) string { return e.Request.Method },
			Color:  func(e har.Entry) lipgloss.Color { return methodColor(th, e.Request.Method) }},
		{Title: "STATUS", Width: 6, Render: func(e har.Entry) string { return fmt.Sprintf("%d", e.Response.Status) }},
		{Title: "TYPE", Width: 10, Render: func(e har.Entry) string { return shortType(e.Response.Content.MimeType) }},
		{Title: "SIZE", Width: 9, Render: func(e har.Entry) string { return humanSize(e.Response.Content.Size) }},
		{Title: "TIME", Width: 8, Render: func(e har.Entry) string { return humanMS(e.Time) }},
		{Title: "URL", Width: 40, Flex: true, Render: func(e har.Entry) string { return e.Request.URL }},
	}, th, km)
	table.SetRows(entries)

	title := "HAR"
	if source != "" && source != "-" {
		title = filepath.Base(source)
	}

	v := &Viewer{
		theme:   th,
		keys:    km,
		table:   table,
		detail:  NewDetail(th, km),
		status:  component.NewStatusBar(th),
		entries: entries,
		title:   title,
		curIdx:  -1,
	}
	// The list starts focused; Tab hands focus to the inspector and back.
	v.focus = focus.New(v.table, v.detail)
	v.syncDetail()
	v.refreshStatus()
	return v
}

// Title implements Screen.
func (v *Viewer) Title() string { return v.title }

// Help implements Screen, describing the viewer's key bindings for the overlay.
func (v *Viewer) Help() string {
	return strings.Join([]string{
		"Panes",
		"  tab / shift+tab    switch focused pane (list / detail)",
		"",
		"List focused",
		"  up/down, j/k       move selection",
		"  pgup/pgdn, b/f     page",
		"  g / G              top / bottom",
		"",
		"Detail focused",
		"  left/right, h/l    previous / next tab",
		"  up/down, j/k       scroll body",
		"  pgup/pgdn          page       g / G  top / bottom",
		"",
		"General",
		"  ?                  toggle this help",
		"  q                  quit",
	}, "\n")
}

// Init implements Screen.
func (v *Viewer) Init() tea.Cmd { return v.table.Init() }

// Update implements Screen. Tab/Shift+Tab move focus between the request list and
// the detail inspector; the focused pane then receives navigation keys. The list
// moves its selection (re-syncing the inspector), while the inspector switches
// tabs (left/right) and scrolls (up/down).
func (v *Viewer) Update(msg tea.Msg) tea.Cmd {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return v.table.Update(msg)
	}
	switch {
	case key.Matches(k, v.keys.Tab):
		v.focus.Next()
		v.refreshStatus()
		return nil
	case key.Matches(k, v.keys.ShiftTab):
		v.focus.Prev()
		v.refreshStatus()
		return nil
	}
	if v.detail.Focused() {
		v.detail.HandleKey(k)
		v.refreshStatus()
		return nil
	}
	prev := v.table.Cursor()
	cmd := v.table.Update(msg)
	if v.table.Cursor() != prev {
		v.syncDetail()
	}
	v.refreshStatus()
	return cmd
}

// syncDetail points the inspector at the currently highlighted entry, or clears
// it when the table is empty.
func (v *Viewer) syncDetail() {
	if e, ok := v.table.Selected(); ok {
		v.detail.SetEntry(e)
		v.curIdx = v.table.Cursor()
	} else {
		v.detail.Clear()
		v.curIdx = -1
	}
}

// SetSize implements Screen, splitting the area between the table and the detail
// pane on a 40/60 ratio (the detail pane taking the larger share, since it will
// grow to hold scrollable per-entry information), above a one-line status bar.
// The split is proportional to the window height.
func (v *Viewer) SetSize(w, h int) {
	v.width, v.height = w, h

	avail := h - 1 // reserve the status line
	if avail < 2 {
		avail = 2
	}
	// 60% of the available height to the detail pane, 40% to the table.
	detailH := avail * 3 / 5
	tableH := avail - detailH
	if tableH < 2 {
		tableH = 2 // keep the header plus at least one row
		detailH = avail - tableH
	}
	if detailH < 1 {
		detailH = 1
		tableH = avail - detailH
	}
	v.tableH, v.detailH = tableH, detailH

	v.table.SetSize(w, tableH)
	v.detail.SetSize(w, detailH)
	v.status.SetSize(w, 1)
}

// View implements Screen, stacking the table over the detail inspector over the
// status bar.
func (v *Viewer) View() string {
	if v.width == 0 || v.height == 0 {
		return ""
	}
	return layout.SplitVertical(
		v.table.View(),
		layout.SplitVertical(v.detail.View(), v.status.View()),
	)
}

// refreshStatus updates the status bar with the cursor position, source, and hints.
func (v *Viewer) refreshStatus() {
	n := len(v.entries)
	pos := 0
	if n > 0 {
		pos = v.table.Cursor() + 1
	}
	v.status.SetLeft(fmt.Sprintf(" %d/%d ", pos, n))
	v.status.SetCenter(v.title)
	if v.detail.Focused() {
		v.status.SetRight(fmt.Sprintf(" detail:%s · ←/→ tabs · ↑/↓ scroll · tab list · ? help ", tabNames[v.detail.Active()]))
	} else {
		v.status.SetRight(" ↑/↓ rows · tab detail · ? help · q quit ")
	}
}

// field formats one aligned "Label: value" line, styling the label as a bold,
// colored key.
func field(th theme.Theme, name, value string) string {
	return "  " + th.Key().Render(padTo(name+":", 8)) + " " + value
}

// methodColor maps an HTTP request method to a theme color so the METHOD column
// and the overview read at a glance (blue read, green create, red delete, ...).
func methodColor(th theme.Theme, method string) lipgloss.Color {
	switch strings.ToUpper(method) {
	case "GET":
		return th.Info
	case "POST":
		return th.Success
	case "PUT":
		return th.Warning
	case "PATCH":
		return th.Secondary
	case "DELETE":
		return th.Error
	case "HEAD", "OPTIONS":
		return th.Muted
	default:
		return th.Fg
	}
}

// methodText renders an HTTP method in its method color for inline use (e.g. the
// overview field), where no surrounding row style applies.
func methodText(th theme.Theme, method string) string {
	return th.Base().Foreground(methodColor(th, method)).Render(method)
}

// fitLines truncates each line to width and pads or trims the slice to exactly
// height lines, so the pane occupies a fixed area.
func fitLines(lines []string, width, height int) string {
	out := make([]string, height)
	for i := 0; i < height; i++ {
		if i < len(lines) {
			out[i] = ansi.Truncate(lines[i], width, "…")
		}
	}
	return strings.Join(out, "\n")
}

var _ Screen = (*Viewer)(nil)
