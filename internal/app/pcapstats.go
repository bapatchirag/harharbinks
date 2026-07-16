// This file implements the PCAP capture-statistics view: overall totals, the
// protocol hierarchy, and the top talkers, rendered into a scrollable pane. It
// is the app-layer adapter that maps a pcap.Stats value into text for the generic
// Viewport component, mirroring the headless `hhb pcap stats` output. The
// enclosing viewer opens and closes this view (see pcapviewer.go).
package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bapatchirag/harharbinks/internal/pcap"
	"github.com/bapatchirag/harharbinks/internal/tui/component"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// statsTimeLayout formats the capture's absolute start and end timestamps.
const statsTimeLayout = "2006-01-02 15:04:05.000000 UTC"

// pcapStats is the capture-statistics view: a scrollable summary of totals, the
// per-protocol breakdown, and the busiest endpoints. It owns a Viewport and the
// stats it renders into it.
type pcapStats struct {
	theme theme.Theme
	body  *component.Viewport
	stats pcap.Stats
	width int
}

// newPcapStats builds an empty statistics view styled with the given theme. Its
// pane starts focused so it scrolls as soon as the view is shown.
func newPcapStats(th theme.Theme) *pcapStats {
	vp := component.NewViewport(th)
	vp.Focus()
	return &pcapStats{theme: th, body: vp}
}

// SetStats loads the statistics to show and renders them into the pane.
func (s *pcapStats) SetStats(st pcap.Stats) {
	s.stats = st
	s.render()
}

// SetTheme swaps the view's palette at runtime and re-renders so section
// headings pick up the new colors.
func (s *pcapStats) SetTheme(th theme.Theme) {
	s.theme = th
	s.body.SetTheme(th)
	s.render()
}

// SetSize sets the pane's render area in cells.
func (s *pcapStats) SetSize(w, h int) {
	s.width = w
	s.body.SetSize(w, h)
}

// Update forwards a key to the pane (scrolling).
func (s *pcapStats) Update(k tea.Msg) tea.Cmd { return s.body.Update(k) }

// View renders the statistics pane.
func (s *pcapStats) View() string { return s.body.View() }

// render lays out the summary, the protocol hierarchy, and the top talkers into
// the scrollable body: aligned key/value pairs and columns, section headings in
// the title accent so the three sections read as distinct blocks.
func (s *pcapStats) render() {
	title := s.theme.Title().Render
	var b strings.Builder

	b.WriteString(title("Capture Summary"))
	b.WriteByte('\n')
	fmt.Fprintf(&b, "  %s%s\n", padTo("Packets", 10), fmt.Sprintf("%d", s.stats.Packets))
	fmt.Fprintf(&b, "  %s%s\n", padTo("Bytes", 10), humanSize(s.stats.Bytes))
	fmt.Fprintf(&b, "  %s%s\n", padTo("Duration", 10), s.stats.Duration())
	if s.stats.Packets > 0 {
		fmt.Fprintf(&b, "  %s%s\n", padTo("Start", 10), s.stats.Start.UTC().Format(statsTimeLayout))
		fmt.Fprintf(&b, "  %s%s\n", padTo("End", 10), s.stats.End.UTC().Format(statsTimeLayout))
	}

	b.WriteByte('\n')
	b.WriteString(title("Protocol Hierarchy"))
	b.WriteByte('\n')
	b.WriteString("  " + padTo("PROTOCOL", 12) + padTo("PACKETS", 10) + "BYTES\n")
	for _, pc := range s.stats.Protocols {
		fmt.Fprintf(&b, "  %s%s%s\n", padTo(pc.Protocol, 12), padTo(fmt.Sprintf("%d", pc.Packets), 10), humanSize(pc.Bytes))
	}

	b.WriteByte('\n')
	b.WriteString(title("Top Talkers"))
	b.WriteByte('\n')
	b.WriteString("  " + padTo("ADDRESS", 20) + padTo("PACKETS", 10) + "BYTES\n")
	for _, tk := range s.stats.TopTalkers {
		fmt.Fprintf(&b, "  %s%s%s\n", padTo(tk.Address, 20), padTo(fmt.Sprintf("%d", tk.Packets), 10), humanSize(tk.Bytes))
	}

	s.body.SetContent(b.String())
}
