// This file implements the PCAP viewer screen: a scrollable, Wireshark-style
// packet list for a loaded capture over a packet-detail inspector. It is the
// app-layer adapter that maps pcap.Packet values into the generic Table component
// through per-column render functions, so the component itself stays unaware of
// PCAP. The list, the detail's layer tree, and its hex view form a three-pane
// focus ring cycled with Tab; moving through the list drives the inspector (see
// pcapdetail.go), and moving through the layer tree highlights the matching
// bytes. Flows, stats, and display filters arrive in a later phase.
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
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// PcapViewer is the PCAP viewer screen. It composes a Table of packets over a
// packet-detail inspector (a layer tree synced to a hex view) with a status bar,
// adapting the PCAP domain into the generic components. Tab cycles focus across
// the list, the layer tree, and the hex view. The flows, stats, and filter/sort
// features are added in later phases.
type PcapViewer struct {
	theme   theme.Theme
	keys    keymap.KeyMap
	table   *component.Table[pcap.Packet]
	detail  *pcapDetail
	status  *component.StatusBar
	focus   *focus.Manager
	packets []pcap.Packet
	start   time.Time
	title   string
	curIdx  int // table row currently mirrored in the detail, or -1 when none

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
	v.table.SetRows(packets)
	// The list, layer tree, and hex view form a three-pane focus ring cycled with
	// Tab; the list starts focused.
	v.focus = focus.New(v.table, v.detail.tree, v.detail.hex)
	v.syncPacket()
	v.refreshStatus()
	return v
}

// Title implements Screen, naming the open capture in the app header.
func (v *PcapViewer) Title() string { return v.title }

// Help implements Screen, describing the viewer's key bindings for the overlay.
func (v *PcapViewer) Help() string {
	return strings.Join([]string{
		"Panes",
		"  tab / shift+tab    switch focused pane (list / layers / bytes)",
		"",
		"Packet list",
		"  up/down, j/k       move selection",
		"  pgup/pgdn, b/f     page          g / G  top / bottom",
		"",
		"Layer tree",
		"  up/down, j/k       move          left/right  collapse / expand",
		"  enter              toggle a layer's fields",
		"",
		"Hex view",
		"  left/right         previous / next byte",
		"  up/down            previous / next row     pgup/pgdn  page",
	}, "\n")
}

// CapturesInput implements Screen. The packet list has no input-capturing mode
// yet (no filter field or menu), so the app's global keys stay active.
func (v *PcapViewer) CapturesInput() bool { return false }

// SetTheme implements Screen, swapping the viewer's palette at runtime and
// propagating it to every component so the settings editor recolors the whole
// screen live.
func (v *PcapViewer) SetTheme(th theme.Theme) {
	v.theme = th
	v.table.SetTheme(th)
	v.detail.SetTheme(th)
	v.status.SetTheme(th)
	v.refreshStatus()
}

// Init implements Screen.
func (v *PcapViewer) Init() tea.Cmd { return v.table.Init() }

// Update implements Screen. It routes keys to the active mode: Tab cycles the
// focused pane, and every other key goes to whichever of the list, layer tree, or
// hex view currently holds focus. Moving the list re-syncs the detail; moving the
// tree re-syncs the highlighted bytes.
func (v *PcapViewer) Update(tmsg tea.Msg) tea.Cmd {
	k, ok := tmsg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	return v.handleKey(k)
}

// handleKey dispatches a key to the focused pane. Tab and Shift+Tab cycle focus
// across the panes (a no-op while the detail is hidden). The list forwards to the
// table and mirrors the new selection into the detail; the layer tree forwards to
// the tree and re-points the hex highlight; the hex view moves its byte cursor.
func (v *PcapViewer) handleKey(k tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(k, v.keys.Tab):
		if v.hasDetail {
			v.focus.Next()
		}
		v.refreshStatus()
		return nil
	case key.Matches(k, v.keys.ShiftTab):
		if v.hasDetail {
			v.focus.Prev()
		}
		v.refreshStatus()
		return nil
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

// syncPacket mirrors the table's selected packet into the detail inspector,
// rebuilding its layer tree and hex view only when the selection actually moves.
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
}

// SetSize implements Screen, dividing the area below the one-line status bar
// between the packet list and the detail inspector; a small window or an empty
// capture gives the whole area to the list.
func (v *PcapViewer) SetSize(w, h int) {
	v.width, v.height = w, h
	tableH, detailH := splitPcapHeights(h-1, len(v.packets))
	v.hasDetail = detailH > 0
	v.table.SetSize(w, tableH)
	if v.hasDetail {
		v.detail.SetSize(w, detailH)
	} else {
		// With no detail pane to focus, keep the list focused so Tab has nothing to
		// strand focus on.
		v.focus.Focus(0)
	}
	v.status.SetSize(w, 1)
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

// View implements Screen, stacking the packet list over the detail inspector over
// the status bar; with the detail hidden the list fills the area above the bar.
func (v *PcapViewer) View() string {
	if v.width == 0 || v.height == 0 {
		return ""
	}
	if !v.hasDetail {
		return layout.SplitVertical(v.table.View(), v.status.View())
	}
	return layout.SplitVertical(
		v.table.View(),
		layout.SplitVertical(v.detail.View(), v.status.View()),
	)
}

// refreshStatus updates the status bar with the cursor position over the packet
// count and key hints tailored to the focused pane.
func (v *PcapViewer) refreshStatus() {
	n := len(v.table.Rows())
	pos := 0
	if n > 0 {
		pos = v.table.Cursor() + 1
	}
	v.status.SetLeft(fmt.Sprintf(" %d/%d ", pos, n))
	// The header already names the capture, so the center stays empty to give the
	// right-hand key hints room.
	v.status.SetCenter("")
	switch {
	case v.detail.tree.Focused():
		v.status.SetRight(" layers \u00b7 \u2190/\u2192 collapse/expand \u00b7 \u2191/\u2193 move \u00b7 tab bytes \u00b7 ? help ")
	case v.detail.hex.Focused():
		v.status.SetRight(" bytes \u00b7 \u2190/\u2192 byte \u00b7 \u2191/\u2193 row \u00b7 tab list \u00b7 ? help ")
	case v.hasDetail:
		v.status.SetRight(" list \u00b7 \u2191/\u2193 move \u00b7 tab layers/bytes \u00b7 ? help ")
	default:
		v.status.SetRight(" \u2191/\u2193 move \u00b7 pgup/pgdn page \u00b7 g/G top/bottom \u00b7 ? help ")
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
