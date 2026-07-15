// This file implements the PCAP viewer screen: a scrollable, Wireshark-style
// packet list for a loaded capture. It is the app-layer adapter that maps
// pcap.Packet values into the generic Table component through per-column render
// functions, so the component itself stays unaware of PCAP. Row navigation drives
// the table; the richer per-packet detail (a layer tree synced to a hex view)
// arrives in a later phase, alongside flows, stats, and display filters.
package app

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bapatchirag/harharbinks/internal/pcap"
	"github.com/bapatchirag/harharbinks/internal/tui/component"
	"github.com/bapatchirag/harharbinks/internal/tui/keymap"
	"github.com/bapatchirag/harharbinks/internal/tui/layout"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// PcapViewer is the PCAP viewer screen. It composes a Table of packets with a
// status bar, adapting the PCAP domain into the generic components. For now it
// presents the packet list only; the layer-tree and hex detail panes, and the
// flows, stats, and filter/sort features, are added in later phases.
type PcapViewer struct {
	theme   theme.Theme
	keys    keymap.KeyMap
	table   *component.Table[pcap.Packet]
	status  *component.StatusBar
	packets []pcap.Packet
	start   time.Time
	title   string

	width  int
	height int
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
		status:  component.NewStatusBar(th),
		packets: packets,
		start:   start,
		title:   title,
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
	v.table.Focus()
	v.refreshStatus()
	return v
}

// Title implements Screen, naming the open capture in the app header.
func (v *PcapViewer) Title() string { return v.title }

// Help implements Screen, describing the viewer's key bindings for the overlay.
func (v *PcapViewer) Help() string {
	return strings.Join([]string{
		"Packet list",
		"  up/down, j/k       move selection",
		"  pgup/pgdn, b/f     page          g / G  top / bottom",
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
	v.status.SetTheme(th)
	v.refreshStatus()
}

// Init implements Screen.
func (v *PcapViewer) Init() tea.Cmd { return v.table.Init() }

// Update implements Screen, forwarding navigation to the packet list and
// refreshing the status line when a key may have moved the selection.
func (v *PcapViewer) Update(tmsg tea.Msg) tea.Cmd {
	cmd := v.table.Update(tmsg)
	if _, ok := tmsg.(tea.KeyMsg); ok {
		v.refreshStatus()
	}
	return cmd
}

// SetSize implements Screen, giving the packet list the area above a one-line
// status bar.
func (v *PcapViewer) SetSize(w, h int) {
	v.width, v.height = w, h
	tableH := h - 1 // reserve the status line
	if tableH < 1 {
		tableH = 1
	}
	v.table.SetSize(w, tableH)
	v.status.SetSize(w, 1)
}

// View implements Screen, stacking the packet list over the status bar.
func (v *PcapViewer) View() string {
	if v.width == 0 || v.height == 0 {
		return ""
	}
	return layout.SplitVertical(v.table.View(), v.status.View())
}

// refreshStatus updates the status bar with the cursor position over the packet
// count and the navigation hints.
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
	v.status.SetRight(" \u2191/\u2193 move \u00b7 pgup/pgdn page \u00b7 g/G top/bottom \u00b7 ? help ")
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
