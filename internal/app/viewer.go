// This file implements the HAR viewer screen: a scrollable table of
// request/response entries on top and a live summary of the highlighted entry
// below. It is the app-layer adapter that maps har.Entry values into the generic
// Table component through per-column render functions, so the component itself
// stays unaware of HAR. The full tabbed detail inspector arrives in a later
// milestone; this screen shows a compact summary.
package app

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/bapatchirag/harharbinks/internal/har"
	"github.com/bapatchirag/harharbinks/internal/tui/component"
	"github.com/bapatchirag/harharbinks/internal/tui/keymap"
	"github.com/bapatchirag/harharbinks/internal/tui/layout"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// Viewer is the HAR viewer screen. It composes a Table of entries with a summary
// pane and a status bar, adapting the HAR domain into the generic components.
type Viewer struct {
	theme   theme.Theme
	keys    keymap.KeyMap
	table   *component.Table[har.Entry]
	status  *component.StatusBar
	entries []har.Entry
	title   string

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
		{Title: "METHOD", Width: 7, Render: func(e har.Entry) string { return e.Request.Method }},
		{Title: "STATUS", Width: 6, Render: func(e har.Entry) string { return fmt.Sprintf("%d", e.Response.Status) }},
		{Title: "TYPE", Width: 10, Render: func(e har.Entry) string { return shortType(e.Response.Content.MimeType) }},
		{Title: "URL", Width: 46, Render: func(e har.Entry) string { return e.Request.URL }},
		{Title: "SIZE", Width: 9, Render: func(e har.Entry) string { return humanSize(e.Response.Content.Size) }},
		{Title: "TIME", Width: 8, Render: func(e har.Entry) string { return humanMS(e.Time) }},
	}, th, km)
	table.SetRows(entries)
	table.Focus()

	title := "HAR"
	if source != "" && source != "-" {
		title = filepath.Base(source)
	}

	v := &Viewer{
		theme:   th,
		keys:    km,
		table:   table,
		status:  component.NewStatusBar(th),
		entries: entries,
		title:   title,
	}
	v.refreshStatus()
	return v
}

// Title implements Screen.
func (v *Viewer) Title() string { return v.title }

// Init implements Screen.
func (v *Viewer) Init() tea.Cmd { return v.table.Init() }

// Update implements Screen. Navigation is delegated to the table; the summary
// pane and status bar always reflect the currently highlighted entry.
func (v *Viewer) Update(msg tea.Msg) tea.Cmd {
	cmd := v.table.Update(msg)
	v.refreshStatus()
	return cmd
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
	v.status.SetSize(w, 1)
}

// View implements Screen, stacking the table over the summary over the status bar.
func (v *Viewer) View() string {
	if v.width == 0 || v.height == 0 {
		return ""
	}
	return layout.SplitVertical(
		v.table.View(),
		layout.SplitVertical(v.detailView(v.width, v.detailH), v.status.View()),
	)
}

// detailView renders a compact summary of the highlighted entry, fitted to
// exactly height lines.
func (v *Viewer) detailView(width, height int) string {
	lines := []string{sectionHeader(v.theme, "Detail", width)}
	if e, ok := v.table.Selected(); ok {
		lines = append(lines,
			field("Method", e.Request.Method),
			field("URL", e.Request.URL),
			field("Status", fmt.Sprintf("%d %s", e.Response.Status, e.Response.StatusText)),
			field("Type", nonEmpty(e.Response.Content.MimeType, "-")),
			field("Size", humanSize(e.Response.Content.Size)),
			field("Time", humanMS(e.Time)),
		)
		if e.ServerIPAddress != "" {
			lines = append(lines, field("Server", e.ServerIPAddress))
		}
	} else {
		lines = append(lines, v.theme.MutedText().Render("  No entries to display."))
	}
	return fitLines(lines, width, height)
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
	v.status.SetRight(" ↑/↓ move · q quit ")
}

// field formats one aligned "Label: value" summary line.
func field(name, value string) string {
	return fmt.Sprintf("  %-8s %s", name+":", value)
}

// sectionHeader renders a full-width styled section title bar.
func sectionHeader(th theme.Theme, title string, width int) string {
	return th.Header().Render(padTo(" "+title+" ", width))
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
