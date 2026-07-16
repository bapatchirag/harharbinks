// This file implements the PCAP conversations view: a table of the capture's
// bidirectional TCP and UDP flows. It is the app-layer adapter that maps
// pcap.Flow values into the generic Table component through per-column render
// functions, so the component stays unaware of PCAP. Selecting a flow follows
// its reassembled stream (see pcapstream.go); the enclosing viewer opens and
// closes this view (see pcapviewer.go).
package app

import (
	"fmt"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bapatchirag/harharbinks/internal/pcap"
	"github.com/bapatchirag/harharbinks/internal/tui/component"
	"github.com/bapatchirag/harharbinks/internal/tui/keymap"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// pcapFlows is the conversations view: a Table of bidirectional TCP and UDP
// flows over the loaded capture. It owns only the table; the viewer drives it
// and reads the highlighted flow to follow its stream.
type pcapFlows struct {
	theme theme.Theme
	table *component.Table[pcap.Flow]
}

// newPcapFlows builds an empty conversations table styled with the given theme
// and key bindings.
func newPcapFlows(th theme.Theme, km keymap.KeyMap) *pcapFlows {
	f := &pcapFlows{theme: th}
	// The PROTO column reads f.theme (not the construction-time palette) so the
	// settings editor recolors it live along with the rest of the UI.
	f.table = component.NewTable([]component.Column[pcap.Flow]{
		{Title: "PROTO", Width: 6,
			Render: func(fl pcap.Flow) string { return fl.Protocol },
			Color:  func(fl pcap.Flow) lipgloss.Color { return protocolColor(f.theme, fl.Protocol) }},
		{Title: "SOURCE", Width: 22, Render: func(fl pcap.Flow) string { return fmt.Sprintf("%s:%d", fl.SrcIP, fl.SrcPort) }},
		{Title: "DEST", Width: 22, Render: func(fl pcap.Flow) string { return fmt.Sprintf("%s:%d", fl.DstIP, fl.DstPort) }},
		{Title: "PACKETS", Width: 8, Render: func(fl pcap.Flow) string { return strconv.Itoa(fl.Packets) }},
		{Title: "BYTES", Width: 9, Render: func(fl pcap.Flow) string { return humanSize(fl.Bytes) }},
		{Title: "DURATION", Width: 12, Flex: true, Render: func(fl pcap.Flow) string { return fl.Duration().String() }},
	}, th, km)
	f.table.Focus()
	return f
}

// SetFlows loads the conversations to show, resetting the selection to the top.
func (f *pcapFlows) SetFlows(flows []pcap.Flow) {
	f.table.SetRows(flows)
	f.table.SetCursor(0)
}

// Selected returns the highlighted flow, or false when the table is empty.
func (f *pcapFlows) Selected() (pcap.Flow, bool) { return f.table.Selected() }

// SetTheme swaps the view's palette at runtime.
func (f *pcapFlows) SetTheme(th theme.Theme) {
	f.theme = th
	f.table.SetTheme(th)
}

// SetSize sets the table's render area in cells.
func (f *pcapFlows) SetSize(w, h int) { f.table.SetSize(w, h) }

// Update forwards a key to the flows table (navigation).
func (f *pcapFlows) Update(k tea.Msg) tea.Cmd { return f.table.Update(k) }

// View renders the conversations table.
func (f *pcapFlows) View() string { return f.table.View() }
