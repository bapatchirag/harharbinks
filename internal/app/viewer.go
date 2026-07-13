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
	"sort"
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
	"github.com/bapatchirag/harharbinks/internal/tui/msg"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// Viewer is the HAR viewer screen. It composes a Table of entries with a tabbed
// detail inspector and a status bar, adapting the HAR domain into the generic
// components. Tab moves focus between the list and the inspector.
type Viewer struct {
	theme     theme.Theme
	keys      keymap.KeyMap
	table     *component.Table[har.Entry]
	detail    *Detail
	status    *component.StatusBar
	search    *component.Search
	focus     *focus.Manager
	entries   []har.Entry
	order     []int    // table row -> index into entries, for the current view
	haystacks []string // per-entry lowercased searchable text, indexed like entries
	title     string
	curIdx    int

	// Live filter and sort state driving the table view.
	query     string
	sortIdx   int
	searching bool

	// Follow-session state: while active the table shows one session's exchanges.
	sessionMode   bool
	sessionLabel  string
	sessionReturn int // entries index to re-select when leaving session mode

	width   int
	height  int
	tableH  int
	detailH int
}

// sortSpec is one preset in the sort cycle stepped through by the sort key.
type sortSpec struct {
	key   har.SortKey
	desc  bool
	label string
}

// sortCycle is the ordered set of sorts the "s" key steps through, beginning at
// the capture's original order. Sizes and times default to descending, since the
// largest and slowest exchanges are usually the ones worth finding first.
var sortCycle = []sortSpec{
	{har.SortNone, false, "none"},
	{har.SortMethod, false, "method"},
	{har.SortStatus, false, "status"},
	{har.SortSize, true, "size\u2193"},
	{har.SortTime, true, "time\u2193"},
	{har.SortURL, false, "url"},
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
		search:  component.NewSearch(th, "filter (text or field:value)\u2026"),
		entries: entries,
		title:   title,
		curIdx:  -1,
	}
	// The list starts focused; Tab hands focus to the inspector and back.
	v.focus = focus.New(v.table, v.detail)
	// Precompute each entry's searchable text once so the live filter (which runs
	// on every keystroke) is a cheap substring check over any field.
	v.haystacks = make([]string, len(entries))
	for i, e := range entries {
		v.haystacks[i] = strings.ToLower(har.SearchText(e))
	}
	v.applyView()
	return v
}

// Title implements Screen. In follow-session mode it names the session so the
// header reflects the narrowed view.
func (v *Viewer) Title() string {
	if v.sessionMode {
		return "session \u00b7 " + v.sessionLabel
	}
	return v.title
}

// Help implements Screen, describing the viewer's key bindings for the overlay.
func (v *Viewer) Help() string {
	return strings.Join([]string{
		"Panes",
		"  tab / shift+tab    switch focused pane (list / detail)",
		"",
		"List focused",
		"  up/down, j/k       move selection",
		"  pgup/pgdn, b/f     page          g / G  top / bottom",
		"  /                  filter \u2014 text, or field:value (esc clears)",
		"                       fields: method url host status header cookie",
		"                               mime body query server conn",
		"  s / S              cycle sort forward / reverse",
		"  enter              follow session (esc to leave)",
		"  o                  open another file",
		"",
		"Detail focused",
		"  left/right, h/l    previous / next tab",
		"  up/down, j/k       scroll body   pgup/pgdn page",
		"",
		"General",
		"  ?                  toggle this help",
		"  q                  quit",
	}, "\n")
}

// CapturesInput implements Screen: while the filter field is open it consumes
// every keystroke so characters are typed into the field.
func (v *Viewer) CapturesInput() bool { return v.searching }

// Init implements Screen.
func (v *Viewer) Init() tea.Cmd { return v.table.Init() }

// Update implements Screen. It routes a message to the active mode: live-filter
// updates rebuild the view; keys go to the open filter field, the session
// overlay, the focused inspector, or the request list, depending on state.
func (v *Viewer) Update(tmsg tea.Msg) tea.Cmd {
	switch m := tmsg.(type) {
	case msg.SearchMsg:
		v.query = m.Query
		v.applyView()
		return nil
	case tea.KeyMsg:
		return v.handleKey(m)
	default:
		return v.table.Update(tmsg)
	}
}

// handleKey dispatches a key to the active mode. Tab always toggles the focused
// pane; the filter field, session overlay, and inspector each capture keys while
// active, otherwise the request list handles navigation, search, sort, and
// follow.
func (v *Viewer) handleKey(k tea.KeyMsg) tea.Cmd {
	if v.searching {
		return v.handleSearchKey(k)
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
	case key.Matches(k, v.keys.Open):
		// Open the file browser to switch captures; esc there returns here.
		return SwitchTo(NewBrowserReturning(v))
	}
	if v.detail.Focused() {
		v.detail.HandleKey(k)
		v.refreshStatus()
		return nil
	}
	if v.sessionMode {
		if key.Matches(k, v.keys.Back) {
			v.closeSession()
			return nil
		}
		return v.navigate(k)
	}
	switch {
	case key.Matches(k, v.keys.Search):
		v.startSearch()
		return nil
	case key.Matches(k, v.keys.Sort):
		v.cycleSort(1)
		return nil
	case key.Matches(k, v.keys.SortRev):
		v.cycleSort(-1)
		return nil
	case key.Matches(k, v.keys.Enter):
		v.openSession()
		return nil
	case key.Matches(k, v.keys.Back):
		v.clearFilter()
		return nil
	}
	return v.navigate(k)
}

// navigate forwards a key to the request list, re-syncing the inspector when the
// selection moves.
func (v *Viewer) navigate(k tea.KeyMsg) tea.Cmd {
	prev := v.table.Cursor()
	cmd := v.table.Update(k)
	if v.table.Cursor() != prev {
		v.syncDetail()
	}
	v.refreshStatus()
	return cmd
}

// handleSearchKey routes keys while the filter field is open: enter commits the
// filter and returns to the list, esc cancels and clears it, and anything else
// edits the query (which live-filters the list through msg.SearchMsg).
func (v *Viewer) handleSearchKey(k tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(k, v.keys.Enter):
		v.commitSearch()
		return nil
	case key.Matches(k, v.keys.Back):
		v.cancelSearch()
		return nil
	}
	return v.search.Update(k)
}

// startSearch opens the filter field, seeded with the current query.
func (v *Viewer) startSearch() {
	v.searching = true
	v.search.SetValue(v.query)
	v.search.Focus()
	v.refreshStatus()
}

// commitSearch closes the filter field, keeping the current filter applied.
func (v *Viewer) commitSearch() {
	v.searching = false
	v.search.Blur()
	v.refreshStatus()
}

// cancelSearch closes the filter field and clears the filter, restoring the full
// list.
func (v *Viewer) cancelSearch() {
	v.searching = false
	v.search.Blur()
	v.search.SetValue("")
	if v.query != "" {
		v.query = ""
		v.applyView()
	}
	v.refreshStatus()
}

// clearFilter drops an applied filter while the filter field is closed, restoring
// the full list. It is a no-op when no filter is active, so esc is harmless on an
// unfiltered list.
func (v *Viewer) clearFilter() {
	if v.query == "" {
		return
	}
	v.query = ""
	v.search.SetValue("")
	v.applyView()
}

// cycleSort steps the sort preset by dir (+1 forward, -1 reverse), wrapping
// around, and re-applies the view.
func (v *Viewer) cycleSort(dir int) {
	n := len(sortCycle)
	v.sortIdx = ((v.sortIdx+dir)%n + n) % n
	v.applyView()
}

// openSession switches the list to show every exchange in the selected entry's
// session (grouped by connection, or by host and time when none is recorded),
// highlighting the entry it was opened from. The session spans the full capture,
// not just the currently filtered view.
func (v *Viewer) openSession() {
	if v.sessionMode || len(v.order) == 0 {
		return
	}
	origIdx := v.order[v.table.Cursor()]
	s, pos, ok := har.SessionAt(v.entries, origIdx)
	if !ok {
		return
	}
	v.sessionMode = true
	v.sessionLabel = s.Label
	v.sessionReturn = origIdx
	v.table.SetRows(s.Entries)
	v.table.SetCursor(pos)
	v.syncDetail()
	v.refreshStatus()
}

// closeSession leaves session mode, rebuilding the filtered/sorted list and
// re-selecting the entry the session was opened from.
func (v *Viewer) closeSession() {
	v.sessionMode = false
	v.sessionLabel = ""
	v.applyView()
	for row, idx := range v.order {
		if idx == v.sessionReturn {
			v.table.SetCursor(row)
			break
		}
	}
	v.syncDetail()
	v.refreshStatus()
}

// applyView rebuilds the table rows from the full entry set by applying the
// active filter then the sort, recording the row-to-source-index mapping in
// v.order so the follow-session view can map a row back to the original capture.
// The filter is a parsed query: free text matches any field, while field:value
// terms (e.g. method:POST) match one field, and multiple terms are conjunctive.
func (v *Viewer) applyView() {
	q := har.ParseQuery(v.query)
	order := make([]int, 0, len(v.entries))
	for i := range v.entries {
		if q.Empty() || q.MatchText(v.entries[i], v.haystacks[i]) {
			order = append(order, i)
		}
	}
	if spec := sortCycle[v.sortIdx]; spec.key != har.SortNone {
		if less := har.Less(spec.key); less != nil {
			sort.SliceStable(order, func(a, b int) bool {
				if spec.desc {
					return less(v.entries[order[b]], v.entries[order[a]])
				}
				return less(v.entries[order[a]], v.entries[order[b]])
			})
		}
	}
	v.order = order
	rows := make([]har.Entry, len(order))
	for i, idx := range order {
		rows[i] = v.entries[idx]
	}
	v.table.SetRows(rows)
	v.syncDetail()
	v.refreshStatus()
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
	v.search.SetSize(w, 1)
}

// View implements Screen, stacking the table over the detail inspector over the
// status bar. While the filter field is open it replaces the status bar so the
// query is editable in place.
func (v *Viewer) View() string {
	if v.width == 0 || v.height == 0 {
		return ""
	}
	bottom := v.status.View()
	if v.searching {
		bottom = v.search.View()
	}
	return layout.SplitVertical(
		v.table.View(),
		layout.SplitVertical(v.detail.View(), bottom),
	)
}

// refreshStatus updates the status bar with the cursor position (over the shown
// count), the source, and context-sensitive key hints.
func (v *Viewer) refreshStatus() {
	shown := len(v.table.Rows())
	pos := 0
	if shown > 0 {
		pos = v.table.Cursor() + 1
	}
	left := fmt.Sprintf(" %d/%d ", pos, shown)
	if v.query != "" && !v.sessionMode {
		left = fmt.Sprintf(" %d/%d of %d ", pos, shown, len(v.entries))
	}
	v.status.SetLeft(left)
	// The header already names the file/session, so the center stays empty to
	// give the right-hand key hints room.
	v.status.SetCenter("")

	switch {
	case v.sessionMode:
		v.status.SetRight(" esc leave session \u00b7 \u2191/\u2193 rows \u00b7 tab detail \u00b7 ? help ")
	case v.detail.Focused():
		v.status.SetRight(fmt.Sprintf(" detail:%s \u00b7 \u2190/\u2192 tabs \u00b7 \u2191/\u2193 scroll \u00b7 tab list \u00b7 ? help ", tabNames[v.detail.Active()]))
	default:
		parts := []string{"/ filter", "s/S sort:" + sortCycle[v.sortIdx].label, "enter follow"}
		if v.query != "" {
			parts = append([]string{"esc clear"}, parts...)
		}
		parts = append(parts, "o open", "tab detail", "? help")
		v.status.SetRight(" " + strings.Join(parts, " \u00b7 ") + " ")
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
