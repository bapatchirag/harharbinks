package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/bapatchirag/harharbinks/internal/pcap"
)

// demoPackets loads the shared sample capture as a stable fixture for the PCAP
// viewer tests. The sample is generated deterministically, so its packets and
// their relative times are fixed frame to frame.
func demoPackets(t *testing.T) []pcap.Packet {
	t.Helper()
	c, err := pcap.ParseFile("../../testdata/sample.pcap")
	if err != nil {
		t.Fatalf("load sample capture: %v", err)
	}
	return c.Packets
}

// sizedPcapViewer returns a PCAP viewer over the sample capture, sized like the
// golden frame.
func sizedPcapViewer(t *testing.T) *PcapViewer {
	v := NewPcapViewer(demoPackets(t), "sample.pcap")
	v.SetSize(100, 23)
	return v
}

// TestPcapViewerGolden renders the full app frame over the sample capture after a
// couple of moves and compares it to a checked-in golden (regenerate with
// -update).
func TestPcapViewerGolden(t *testing.T) {
	var m tea.Model = New(NewPcapViewer(demoPackets(t), "sample.pcap"))
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m, _ = m.Update(keyDown())
	m, _ = m.Update(keyDown())
	golden.RequireEqual(t, []byte(m.View()))
}

// TestPcapViewerTeatest drives the app through key events and asserts the
// highlighted packet advanced accordingly.
func TestPcapViewerTeatest(t *testing.T) {
	a := New(NewPcapViewer(demoPackets(t), "sample.pcap"))
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(100, 24))
	tm.Send(keyDown())
	tm.Send(keyDown())
	tm.Send(keyDown())
	if err := tm.Quit(); err != nil {
		t.Fatalf("quit: %v", err)
	}
	v := tm.FinalModel(t).(*App).screen.(*PcapViewer)
	if got := v.table.Cursor(); got != 3 {
		t.Errorf("after three downs, cursor = %d, want 3", got)
	}
}

// TestPcapViewerColumns verifies the packet list decodes and renders the sample
// capture's protocols, which span ARP, DNS, TCP, HTTP, and TLS frames.
func TestPcapViewerColumns(t *testing.T) {
	v := sizedPcapViewer(t)
	view := v.View()
	for _, proto := range []string{"ARP", "DNS", "TCP", "HTTP", "TLS"} {
		if !strings.Contains(view, proto) {
			t.Errorf("packet list should show a %s packet; view:\n%s", proto, view)
		}
	}
}

// TestPcapViewerEmpty verifies the viewer renders without panicking over an empty
// capture and reports a zero position.
func TestPcapViewerEmpty(t *testing.T) {
	v := NewPcapViewer(nil, "empty.pcap")
	v.SetSize(100, 23)
	if got := v.View(); got == "" {
		t.Error("empty viewer should still render a frame")
	}
	if _, ok := v.table.Selected(); ok {
		t.Error("empty capture should have no selection")
	}
}
