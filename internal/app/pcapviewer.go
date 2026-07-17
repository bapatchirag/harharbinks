// This file implements the PCAP viewer screen: a scrollable, Wireshark-style
// packet list for a loaded capture over a packet-detail inspector, plus the
// capture-wide views layered on top of it — a display filter and sort over the
// list, a conversations (flows) table, and a capture-statistics panel, plus a
// follow view that scopes the list to one conversation's frames. It is the
// app-layer adapter that maps pcap.Packet
// values into the generic Table component through per-column render functions, so
// the component itself stays unaware of PCAP. The list, the detail's layer tree,
// and its hex view form a three-pane focus ring cycled with Tab; moving through
// the list drives the inspector (see pcapdetail.go), and moving through the layer
// tree highlights the matching bytes. The flows and stats views (see
// pcapflows.go and pcapstats.go) are modes the viewer switches into and leaves
// with esc; "enter" scopes the list to the selected packet's conversation (its
// whole path of frames), the packet-capture analogue of the HAR follow-session
// view.
package app

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bapatchirag/harharbinks/internal/pcap"
	"github.com/bapatchirag/harharbinks/internal/tui/component"
	"github.com/bapatchirag/harharbinks/internal/tui/focus"
	"github.com/bapatchirag/harharbinks/internal/tui/keymap"
	"github.com/bapatchirag/harharbinks/internal/tui/layout"
	"github.com/bapatchirag/harharbinks/internal/tui/msg"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// pcapMode is the viewer's current top-level view. The packet list is the home
// mode; the others are capture-wide views reached from it and left with esc.
type pcapMode int

const (
	modePackets pcapMode = iota // the packet list plus its detail inspector
	modeFlows                   // the conversations (flows) table
	modeStats                   // the capture-statistics panel
)

// pcapSortSpec is one preset in the packet-list sort cycle.
type pcapSortSpec struct {
	key   pcap.SortKey
	desc  bool
	label string
}

// pcapSortCycle is the ordered set of sorts the "s" key steps through, beginning
// at the capture's original order. Length defaults to descending, since the
// largest frames are usually the ones worth finding first.
var pcapSortCycle = []pcapSortSpec{
	{pcap.SortNone, false, "none"},
	{pcap.SortTime, false, "time"},
	{pcap.SortProto, false, "proto"},
	{pcap.SortLen, true, "len\u2193"},
	{pcap.SortSrc, false, "src"},
	{pcap.SortDst, false, "dst"},
}

// PcapViewer is the PCAP viewer screen. Its home view composes a Table of packets
// over a packet-detail inspector (a layer tree synced to a hex view) with a
// status bar; Tab cycles focus across the list, the layer tree, and the hex view.
// A display filter ("/") and sort ("s") narrow and reorder the list, "enter"
// follows the selected packet's conversation (scoping the list to its frames),
// and a views menu ("e") opens the conversations and statistics views. It adapts
// the PCAP domain into the generic components.
type PcapViewer struct {
	theme   theme.Theme
	keys    keymap.KeyMap
	table   *component.Table[pcap.Packet]
	detail  *pcapDetail
	status  *component.StatusBar
	search  *component.Search
	menu    *component.Menu
	flows   *pcapFlows
	stats   *pcapStats
	focus   *focus.Manager
	packets []pcap.Packet
	start   time.Time
	title   string
	notice  string // capture caveat (truncated / unsupported link type), or ""
	curIdx  int    // table row currently mirrored in the detail, or -1 when none

	// View state driving the packet list and the mode the viewer is in.
	mode      pcapMode
	query     string
	haystacks []string // per-packet lowercased SearchText, indexed like packets
	sortIdx   int
	searching bool
	menuOpen  bool

	// Follow state: while active the packet list is scoped to one conversation's
	// frames (its whole path, the frames before and after the selected packet).
	// followFromFlows records that it was opened from the flows table so esc returns
	// there; followReturn is the packet Index to re-select when returning to the
	// full list.
	followMode      bool
	followFromFlows bool
	followLabel     string
	followReturn    int

	width     int
	height    int
	hasDetail bool
}

// NewPcapViewer builds a PCAP viewer for the given packets. source is the file
// path they were loaded from; its base name is shown in the header. Times in the
// list are shown relative to the first packet, so the capture begins at zero.
func NewPcapViewer(packets []pcap.Packet, source string) *PcapViewer {
	th := theme.Default()
	km := keymap.Default()

	title := "PCAP"
	if source != "" && source != "-" {
		title = filepath.Base(source)
	}

	var start time.Time
	if len(packets) > 0 {
		start = packets[0].Timestamp
	}

	v := &PcapViewer{
		theme:   th,
		keys:    km,
		detail:  newPcapDetail(th, km),
		status:  component.NewStatusBar(th),
		search:  component.NewSearch(th, "filter \u2014 text, or field:value\u2026"),
		menu:    newPcapMenu(th, km),
		flows:   newPcapFlows(th, km),
		stats:   newPcapStats(th),
		packets: packets,
		start:   start,
		title:   title,
		curIdx:  -1,
	}
	// The PROTO column's color reads v.theme (rather than the construction-time
	// palette) so the settings editor recolors the column live along with the
	// rest of the UI.
	v.table = component.NewTable([]component.Column[pcap.Packet]{
		{Title: "#", Width: 5, Render: func(p pcap.Packet) string { return strconv.Itoa(p.Index) }},
		{Title: "TIME", Width: 9, Render: func(p pcap.Packet) string { return relTime(p.Timestamp, v.start) }},
		{Title: "SOURCE", Width: 15, Render: func(p pcap.Packet) string { return nonEmpty(p.Source(), "-") }},
		{Title: "DEST", Width: 15, Render: func(p pcap.Packet) string { return nonEmpty(p.Dest(), "-") }},
		{Title: "PROTO", Width: 6,
			Render: func(p pcap.Packet) string { return p.Protocol() },
			Color:  func(p pcap.Packet) lipgloss.Color { return protocolColor(v.theme, p.Protocol()) }},
		{Title: "LEN", Width: 6, Render: func(p pcap.Packet) string { return strconv.Itoa(p.OrigLen) }},
		{Title: "INFO", Width: 40, Flex: true, Render: func(p pcap.Packet) string { return p.Info() }},
	}, th, km)
	// The list, layer tree, and hex view form a three-pane focus ring cycled with
	// Tab; the list starts focused.
	v.focus = focus.New(v.table, v.detail.tree, v.detail.hex)
	// Precompute each packet's searchable text once so the live filter (which runs
	// on every keystroke) is a cheap substring check for free-text terms.
	v.haystacks = make([]string, len(packets))
	for i, p := range packets {
		v.haystacks[i] = strings.ToLower(pcap.SearchText(p))
	}
	v.applyView()
	return v
}

// newPcapMenu builds the views menu: the capture-wide conversations and
// statistics views, reachable by arrow keys plus enter or by mnemonic.
func newPcapMenu(th theme.Theme, km keymap.KeyMap) *component.Menu {
	m := component.NewMenu([]component.MenuItem{
		{Key: "f", Title: "Conversations (flows)", Action: "flows"},
		{Key: "s", Title: "Capture statistics", Action: "stats"},
	}, th, km)
	m.SetTitle("Views")
	return m
}

// SetNotice records a short caveat about the open capture (for example that it
// was truncated or uses a link type that cannot be decoded), shown in the status
// bar and in the empty-capture message. An empty string clears it.
func (v *PcapViewer) SetNotice(note string) {
	v.notice = note
	v.refreshStatus()
}

// Title implements Screen, naming the open capture and the active view in the
// app header.
func (v *PcapViewer) Title() string {
	switch {
	case v.followMode:
		return "flow \u00b7 " + v.followLabel
	case v.mode == modeFlows:
		return v.title + " \u00b7 flows"
	case v.mode == modeStats:
		return v.title + " \u00b7 stats"
	default:
		return v.title
	}
}

// Help implements Screen, describing the viewer's key bindings for the overlay.
func (v *PcapViewer) Help() string {
	return strings.Join([]string{
		"Packet list",
		"  up/down, j/k       move selection",
		"  ctrl+u/ctrl+d      page          g / G  top / bottom",
		"  /                  filter \u2014 text, or field:value (esc clears)",
		"                       fields: proto src dst port info",
		"  s / S              cycle sort forward / reverse",
		"  enter              follow the packet's conversation (its frames)",
		"  e                  views menu (conversations, statistics)",
		"  tab / shift+tab    switch focused pane (list / layers / bytes)",
		"",
		"Layer tree",
		"  up/down, j/k       move          left/right  collapse / expand",
		"  enter              toggle a layer's fields",
		"",
		"Hex view",
		"  left/right         previous / next byte",
		"  up/down            previous / next row     ctrl+u/ctrl+d  page",
		"",
		"Following a conversation",
		"  up/down, j/k       move through the flow's frames",
		"  esc                back to the full list",
		"",
		"Conversations / statistics",
		"  up/down, j/k       move / scroll",
		"  enter              follow a conversation (flows list)",
		"  esc                back to the packet list",
	}, "\n")
}

// CapturesInput implements Screen: while the filter field or the views menu is
// open it consumes every keystroke, so characters are typed into the field and
// menu navigation is not intercepted by global bindings.
func (v *PcapViewer) CapturesInput() bool { return v.searching || v.menuOpen }

// SetTheme implements Screen, swapping the viewer's palette at runtime and
// propagating it to every component so the settings editor recolors the whole
// screen live.
func (v *PcapViewer) SetTheme(th theme.Theme) {
	v.theme = th
	v.table.SetTheme(th)
	v.detail.SetTheme(th)
	v.status.SetTheme(th)
	v.search.SetTheme(th)
	v.menu.SetTheme(th)
	v.flows.SetTheme(th)
	v.stats.SetTheme(th)
	v.refreshStatus()
}

// Init implements Screen.
func (v *PcapViewer) Init() tea.Cmd { return v.table.Init() }

// Update implements Screen. Live-filter updates rebuild the packet list and menu
// selections open a view; every key is dispatched by the current mode.
func (v *PcapViewer) Update(tmsg tea.Msg) tea.Cmd {
	switch m := tmsg.(type) {
	case msg.SearchMsg:
		v.query = m.Query
		v.applyView()
		return nil
	case msg.MenuActionMsg:
		return v.runMenuAction(m.Action)
	case tea.KeyMsg:
		return v.handleKey(m)
	default:
		return nil
	}
}

// handleKey dispatches a key to the active mode. The filter field and views menu
// capture keys while open; otherwise the packet list, flows table, stats panel,
// or follow-stream handles it depending on the current mode.
func (v *PcapViewer) handleKey(k tea.KeyMsg) tea.Cmd {
	if v.searching {
		return v.handleSearchKey(k)
	}
	if v.menuOpen {
		return v.handleMenuKey(k)
	}
	switch v.mode {
	case modeFlows:
		return v.handleFlowsKey(k)
	case modeStats:
		return v.handleStatsKey(k)
	default:
		return v.handlePacketsKey(k)
	}
}

// handlePacketsKey drives the packet-list mode: Tab cycles the three panes, "e"
// opens the views menu, and — while the list itself is focused — "/", "s"/"S",
// enter, and esc filter, sort, follow, and clear. Otherwise the key goes to the
// focused pane: the list re-syncs the detail, the layer tree re-points the hex
// highlight, and the hex view moves its byte cursor.
func (v *PcapViewer) handlePacketsKey(k tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(k, v.keys.Tab):
		if v.hasDetail {
			v.focus.Next()
		}
		v.refreshHighlight()
		v.refreshStatus()
		return nil
	case key.Matches(k, v.keys.ShiftTab):
		if v.hasDetail {
			v.focus.Prev()
		}
		v.refreshHighlight()
		v.refreshStatus()
		return nil
	case key.Matches(k, v.keys.Menu) && !v.followMode:
		v.openMenu()
		return nil
	}

	// Actions on the list itself are handled only while the list pane holds focus;
	// in the tree or hex pane these keys fall through to the pane. While following a
	// conversation the list is scoped to it, so esc leaves the follow view and the
	// filter, sort, and follow actions are disabled.
	if v.table.Focused() {
		if v.followMode {
			if key.Matches(k, v.keys.Back) {
				v.closeFollow()
				return nil
			}
		} else {
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
				v.followSelected()
				return nil
			case key.Matches(k, v.keys.Back):
				v.clearFilter()
				return nil
			}
		}
	}

	var cmd tea.Cmd
	switch {
	case v.detail.tree.Focused():
		cmd = v.detail.tree.Update(k)
		v.detail.syncHighlight()
	case v.detail.hex.Focused():
		cmd = v.detail.hex.Update(k)
	default:
		cmd = v.table.Update(k)
		v.syncPacket()
	}
	v.refreshStatus()
	return cmd
}

// handleSearchKey routes keys while the filter field is open: enter commits the
// filter and returns to the list, esc cancels and clears it, and anything else
// edits the query (which live-filters the list through msg.SearchMsg).
func (v *PcapViewer) handleSearchKey(k tea.KeyMsg) tea.Cmd {
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

// handleMenuKey routes keys while the views menu is open: esc closes it, and
// everything else drives the menu (which emits msg.MenuActionMsg on selection).
func (v *PcapViewer) handleMenuKey(k tea.KeyMsg) tea.Cmd {
	if key.Matches(k, v.keys.Back) {
		v.closeMenu()
		return nil
	}
	return v.menu.Update(k)
}

// handleFlowsKey drives the conversations table: esc returns to the packet list,
// enter follows the selected conversation (scoping the list to its frames), and
// everything else navigates.
func (v *PcapViewer) handleFlowsKey(k tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(k, v.keys.Back):
		v.showPackets()
		return nil
	case key.Matches(k, v.keys.Enter):
		v.followFlow()
		return nil
	}
	cmd := v.flows.Update(k)
	v.refreshStatus()
	return cmd
}

// handleStatsKey drives the statistics panel: esc returns to the packet list and
// everything else scrolls the pane.
func (v *PcapViewer) handleStatsKey(k tea.KeyMsg) tea.Cmd {
	if key.Matches(k, v.keys.Back) {
		v.showPackets()
		return nil
	}
	cmd := v.stats.Update(k)
	v.refreshStatus()
	return cmd
}

// syncPacket mirrors the table's selected packet into the detail inspector,
// rebuilding its layer tree and hex view only when the selection actually moves,
// then refreshes the hex highlight for the current focus.
func (v *PcapViewer) syncPacket() {
	p, ok := v.table.Selected()
	if !ok {
		v.curIdx = -1
		v.detail.Clear()
		return
	}
	if idx := v.table.Cursor(); idx != v.curIdx {
		v.curIdx = idx
		v.detail.SetPacket(p)
	}
	v.refreshHighlight()
}

// refreshHighlight shows the layer tree's selected byte range in the hex view
// only while the detail (the tree or the hex) is focused, and clears it while the
// packet list is focused. This keeps a layer highlight from lingering in the
// bytes pane after Tab moves focus back to the list.
func (v *PcapViewer) refreshHighlight() {
	if v.detail.tree.Focused() || v.detail.hex.Focused() {
		v.detail.syncHighlight()
	} else {
		v.detail.hex.ClearHighlight()
	}
}

// applyView rebuilds the packet list from the full capture by applying the active
// display filter then the active sort, and re-syncs the detail to the new
// selection. The filter is a parsed query: free text matches any searchable
// field, while field:value terms (e.g. proto:tls) match one field, and multiple
// terms are conjunctive.
func (v *PcapViewer) applyView() {
	q := pcap.ParseQuery(v.query)
	rows := make([]pcap.Packet, 0, len(v.packets))
	for i, p := range v.packets {
		if q.Empty() || q.MatchText(p, v.haystacks[i]) {
			rows = append(rows, p)
		}
	}
	if spec := pcapSortCycle[v.sortIdx]; spec.key != pcap.SortNone {
		pcap.Sort(rows, spec.key, spec.desc)
	}
	v.curIdx = -1
	v.table.SetRows(rows)
	v.syncPacket()
	v.refreshStatus()
}

// startSearch opens the filter field, seeded with the current query.
func (v *PcapViewer) startSearch() {
	v.searching = true
	v.search.SetValue(v.query)
	v.search.Focus()
	v.refreshStatus()
}

// commitSearch closes the filter field, keeping the current filter applied.
func (v *PcapViewer) commitSearch() {
	v.searching = false
	v.search.Blur()
	v.refreshStatus()
}

// cancelSearch closes the filter field and clears the filter, restoring the full
// list.
func (v *PcapViewer) cancelSearch() {
	v.searching = false
	v.search.Blur()
	v.search.SetValue("")
	if v.query != "" {
		v.query = ""
		v.applyView()
	}
	v.refreshStatus()
}

// clearFilter drops an applied filter while the filter field is closed. It is a
// no-op when no filter is active, so esc is harmless on an unfiltered list.
func (v *PcapViewer) clearFilter() {
	if v.query == "" {
		return
	}
	v.query = ""
	v.search.SetValue("")
	v.applyView()
}

// cycleSort steps the sort preset by dir (+1 forward, -1 reverse), wrapping
// around, and re-applies the view.
func (v *PcapViewer) cycleSort(dir int) {
	n := len(pcapSortCycle)
	v.sortIdx = ((v.sortIdx+dir)%n + n) % n
	v.applyView()
}

// openMenu shows the views menu.
func (v *PcapViewer) openMenu() {
	v.menuOpen = true
	v.menu.Focus()
	v.refreshStatus()
}

// closeMenu hides the views menu.
func (v *PcapViewer) closeMenu() {
	v.menuOpen = false
	v.menu.Blur()
	v.refreshStatus()
}

// runMenuAction closes the menu and opens the chosen view.
func (v *PcapViewer) runMenuAction(action string) tea.Cmd {
	v.closeMenu()
	switch action {
	case "flows":
		v.showFlows()
	case "stats":
		v.showStats()
	}
	return nil
}

// showPackets returns to the packet-list mode, rebuilding the full (filtered and
// sorted) list in case it was left scoped to a followed conversation.
func (v *PcapViewer) showPackets() {
	v.mode = modePackets
	v.followMode = false
	v.followFromFlows = false
	v.applyView()
	v.relayout()
	v.refreshStatus()
}

// showFlows computes the capture's conversations and switches to the flows table.
func (v *PcapViewer) showFlows() {
	v.flows.SetFlows(pcap.Flows(v.packets))
	v.mode = modeFlows
	v.relayout()
	v.refreshStatus()
}

// showStats summarizes the capture and switches to the statistics panel.
func (v *PcapViewer) showStats() {
	v.stats.SetStats(pcap.Summarize(v.packets))
	v.mode = modeStats
	v.relayout()
	v.refreshStatus()
}

// followSelected scopes the packet list to the conversation of the packet
// highlighted in the list: every frame of that flow in capture order, so the
// frames leading up to the selection and those after it are shown together. A
// packet that is not part of any TCP or UDP conversation (such as ARP or ICMP) is
// still followed on its own, scoping the list to just that single frame so the
// action always does something. esc restores the full list.
func (v *PcapViewer) followSelected() {
	p, ok := v.table.Selected()
	if !ok {
		return
	}
	if flow, pos, ok := pcap.FlowAt(v.packets, p.Index); ok {
		v.enterFollow(flow.Indices, pos, p.Index, flowLabel(flow), false)
		return
	}
	v.enterFollow([]int{p.Index}, 0, p.Index, packetLabel(p), false)
}

// followFlow scopes the packet list to the conversation highlighted in the flows
// table, returning to that table on esc.
func (v *PcapViewer) followFlow() {
	f, ok := v.flows.Selected()
	if !ok || len(f.Indices) == 0 {
		return
	}
	v.enterFollow(f.Indices, 0, f.Indices[0], flowLabel(f), true)
}

// enterFollow switches to the packet list scoped to the given frames (by 1-based
// index), selecting the one at pos and labeling the view with label. returnIdx is
// the packet Index to re-select when leaving to the full list; fromFlows records
// whether esc should return to the flows table instead.
func (v *PcapViewer) enterFollow(indices []int, pos, returnIdx int, label string, fromFlows bool) {
	rows := make([]pcap.Packet, 0, len(indices))
	for _, idx := range indices {
		rows = append(rows, v.packets[idx-1])
	}
	v.mode = modePackets
	v.followMode = true
	v.followFromFlows = fromFlows
	v.followLabel = label
	v.followReturn = returnIdx
	if pos < 0 {
		pos = 0
	}
	v.curIdx = -1
	v.table.SetRows(rows)
	v.table.SetCursor(pos)
	v.focus.Focus(0) // start on the list pane
	v.relayout()
	v.syncPacket()
	v.refreshStatus()
}

// closeFollow leaves the follow view: back to the flows table when it was opened
// from there, otherwise back to the full (filtered and sorted) list with the
// followed frame re-selected.
func (v *PcapViewer) closeFollow() {
	v.followMode = false
	v.followLabel = ""
	if v.followFromFlows {
		v.followFromFlows = false
		v.mode = modeFlows
		v.relayout()
		v.refreshStatus()
		return
	}
	v.applyView()
	for row, p := range v.table.Rows() {
		if p.Index == v.followReturn {
			v.table.SetCursor(row)
			break
		}
	}
	v.syncPacket()
	v.refreshStatus()
}

// flowLabel names a conversation for the follow view's header: its endpoints and
// transport protocol.
func flowLabel(f pcap.Flow) string {
	return fmt.Sprintf("%s:%d \u2192 %s:%d (%s)", f.SrcIP, f.SrcPort, f.DstIP, f.DstPort, f.Protocol)
}

// packetLabel names a lone frame for the follow view's header when the frame is
// not part of any conversation: its endpoints and protocol, without ports.
func packetLabel(p pcap.Packet) string {
	return fmt.Sprintf("%s \u2192 %s (%s)", nonEmpty(p.Source(), "?"), nonEmpty(p.Dest(), "?"), p.Protocol())
}

// SetSize implements Screen, recording the viewport size and laying out the
// active view within it.
func (v *PcapViewer) SetSize(w, h int) {
	v.width, v.height = w, h
	v.relayout()
}

// relayout sizes the active view to fill the area below the one-line status bar.
// The packet-list mode divides that area between the list and the detail
// inspector (a small window or an empty capture gives it all to the list); the
// flows and stats modes each take the whole area.
func (v *PcapViewer) relayout() {
	w, h := v.width, v.height
	if w == 0 || h == 0 {
		return
	}
	bodyH := h - 1 // reserve the status line
	if bodyH < 1 {
		bodyH = 1
	}
	v.status.SetSize(w, 1)
	v.search.SetSize(w, 1)

	switch v.mode {
	case modeFlows:
		v.flows.SetSize(w, bodyH)
	case modeStats:
		v.stats.SetSize(w, bodyH)
	default:
		tableH, detailH := splitPcapHeights(bodyH, len(v.packets))
		v.hasDetail = detailH > 0
		v.table.SetSize(w, tableH)
		if v.hasDetail {
			v.detail.SetSize(w, detailH)
		} else {
			// With no detail pane to focus, keep the list focused so Tab has nothing
			// to strand focus on.
			v.focus.Focus(0)
		}
	}
}

// splitPcapHeights divides the viewer's content area (its height minus the status
// line) between the packet list and the detail inspector. With no packets, or too
// little room for a useful split, the whole area goes to the list and the detail
// is hidden.
func splitPcapHeights(avail, packets int) (tableH, detailH int) {
	const minForDetail = 8
	if avail < 1 {
		avail = 1
	}
	if packets == 0 || avail < minForDetail {
		return avail, 0
	}
	tableH = avail * 45 / 100
	if tableH < 3 {
		tableH = 3
	}
	return tableH, avail - tableH
}

// View implements Screen, rendering the active view above the one-line status bar
// (or the filter field while it is open) and compositing the views menu on top
// when it is open. The packet-list mode stacks the list over the detail
// inspector; the flows and stats modes fill the body.
func (v *PcapViewer) View() string {
	if v.width == 0 || v.height == 0 {
		return ""
	}
	bottom := v.status.View()
	if v.searching {
		bottom = v.search.View()
	}

	var body string
	switch v.mode {
	case modeFlows:
		body = v.flows.View()
	case modeStats:
		body = v.stats.View()
	default:
		switch {
		case len(v.table.Rows()) == 0:
			body = v.emptyBody()
		case v.hasDetail:
			body = layout.SplitVertical(v.table.View(), v.detail.View())
		default:
			body = v.table.View()
		}
	}

	base := layout.SplitVertical(body, bottom)
	if v.menuOpen {
		hint := v.theme.MutedText().Render("enter select \u00b7 esc close")
		box := v.theme.BorderStyle(true).Padding(1, 2).Render(v.menu.View() + "\n\n" + hint)
		return layout.Center(base, box)
	}
	return base
}

// emptyBody renders the centered placeholder shown in the packet-list mode when
// the list has no rows — the capture is empty, was truncated to nothing, or the
// active filter matched none. It fills the area above the status bar so the bar
// stays pinned to the bottom.
func (v *PcapViewer) emptyBody() string {
	h := v.height - 1
	if h < 1 {
		h = 1
	}
	return layout.Place(v.width, h, v.theme.MutedText().Render(v.emptyMessage()))
}

// emptyMessage is the placeholder text for an empty packet list. It distinguishes
// a filter that matched nothing from a capture with no packets, and folds in any
// capture notice (such as a truncated capture) so the reason is visible.
func (v *PcapViewer) emptyMessage() string {
	if len(v.packets) > 0 && v.query != "" {
		return fmt.Sprintf("No packets match %q.\nPress esc to clear the filter.", v.query)
	}
	if v.notice != "" {
		return "This capture has no packets to show (" + v.notice + ")."
	}
	return "This capture contains no packets."
}

// refreshStatus updates the status bar for the active mode: the cursor position
// over the row count, any active filter and sort, and key hints tailored to the
// mode and focused pane.
func (v *PcapViewer) refreshStatus() {
	switch v.mode {
	case modeFlows:
		n := len(v.flows.table.Rows())
		pos := 0
		if n > 0 {
			pos = v.flows.table.Cursor() + 1
		}
		v.status.SetLeft(fmt.Sprintf(" %d/%d ", pos, n))
		v.status.SetCenter("conversations")
		v.status.SetRight(" enter follow \u00b7 esc back \u00b7 ? help ")
	case modeStats:
		v.status.SetLeft(" statistics ")
		v.status.SetCenter("")
		v.status.SetRight(" \u2191/\u2193 scroll \u00b7 esc back \u00b7 ? help ")
	default:
		v.refreshPacketStatus()
	}
}

// refreshPacketStatus fills the status bar for the packet-list mode: the position
// over the shown count (with any capture caveat) on the left, the active filter
// and sort in the center, and pane-aware key hints on the right.
func (v *PcapViewer) refreshPacketStatus() {
	n := len(v.table.Rows())
	pos := 0
	if n > 0 {
		pos = v.table.Cursor() + 1
	}
	// The capture caveat rides on the left, beside the count: the center can be
	// clobbered by the wide key-hint segment, and the left segment stays clear.
	if v.notice != "" {
		v.status.SetLeft(fmt.Sprintf(" %d/%d \u00b7 %s ", pos, n, v.notice))
	} else {
		v.status.SetLeft(fmt.Sprintf(" %d/%d ", pos, n))
	}

	if v.followMode {
		// The header already names the followed conversation, so the center just
		// marks the mode.
		v.status.SetCenter("following")
	} else {
		var parts []string
		if spec := pcapSortCycle[v.sortIdx]; spec.key != pcap.SortNone {
			parts = append(parts, "sort:"+spec.label)
		}
		if v.query != "" {
			parts = append(parts, "filter:"+v.query)
		}
		v.status.SetCenter(strings.Join(parts, "  "))
	}

	switch {
	case v.detail.tree.Focused():
		v.status.SetRight(" layers \u00b7 \u2190/\u2192 collapse/expand \u00b7 \u2191/\u2193 move \u00b7 tab bytes \u00b7 ? help ")
	case v.detail.hex.Focused():
		v.status.SetRight(" bytes \u00b7 \u2190/\u2192 byte \u00b7 \u2191/\u2193 row \u00b7 tab list \u00b7 ? help ")
	case v.followMode:
		v.status.SetRight(" flow \u00b7 \u2191/\u2193 move \u00b7 tab layers/bytes \u00b7 esc back \u00b7 ? help ")
	default:
		v.status.SetRight(" / filter \u00b7 s sort \u00b7 enter follow \u00b7 e views \u00b7 tab panes \u00b7 ? help ")
	}
}

// relTime formats a packet's time relative to the capture start, in seconds with
// microsecond precision (e.g. "0.010000"), matching the headless packet list. It
// is duplicated from the CLI's formatter deliberately: the app layer must not
// depend on internal/cli, which in turn depends on this package.
func relTime(t, start time.Time) string {
	return fmt.Sprintf("%.6f", t.Sub(start).Seconds())
}

// protocolColor maps a decoded protocol name to a theme color so the PROTO column
// reads at a glance, mirroring how methodColor colors the HAR method column.
func protocolColor(th theme.Theme, proto string) lipgloss.Color {
	switch proto {
	case "HTTP":
		return th.Success
	case "DNS":
		return th.Secondary
	case "TLS":
		return th.Warning
	case "TCP":
		return th.Info
	case "UDP":
		return th.Primary
	case "ICMPv4", "ICMPv6":
		return th.Error
	case "ARP":
		return th.Muted
	default:
		return th.Fg
	}
}

var _ Screen = (*PcapViewer)(nil)
