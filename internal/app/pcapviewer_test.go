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

// sizedPcapViewer returns a PCAP viewer over the sample capture, sized tall
// enough that the whole packet list stays visible above the detail inspector.
func sizedPcapViewer(t *testing.T) *PcapViewer {
	v := NewPcapViewer(demoPackets(t), "sample.pcap")
	v.SetSize(100, 44)
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

// TestPcapViewerTabFocus verifies Tab cycles focus across the three panes — the
// packet list, the layer tree, and the hex view — and wraps back to the list.
func TestPcapViewerTabFocus(t *testing.T) {
	v := sizedPcapViewer(t)
	if !v.table.Focused() {
		t.Fatal("the list should start focused")
	}
	v.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	if !v.detail.tree.Focused() {
		t.Error("after one tab, the layer tree should be focused")
	}
	v.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	if !v.detail.hex.Focused() {
		t.Error("after two tabs, the hex view should be focused")
	}
	v.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	if !v.table.Focused() {
		t.Error("after three tabs, focus should wrap back to the list")
	}
}

// TestPcapViewerDetailSync verifies the detail inspector mirrors the list: it
// shows the first frame initially and re-syncs as the selection moves.
func TestPcapViewerDetailSync(t *testing.T) {
	v := sizedPcapViewer(t)
	if v.curIdx != 0 {
		t.Fatalf("initial curIdx = %d, want 0", v.curIdx)
	}
	if first, _ := v.detail.tree.Selected(); !strings.Contains(first.label, "Frame 1") {
		t.Errorf("detail should show frame 1; got %q", first.label)
	}

	v.handleKey(keyDown().(tea.KeyMsg))
	v.handleKey(keyDown().(tea.KeyMsg))
	if got := v.table.Cursor(); got != 2 {
		t.Fatalf("cursor = %d, want 2 (DNS frame)", got)
	}
	if frame, _ := v.detail.tree.Selected(); !strings.Contains(frame.label, "Frame 3") {
		t.Errorf("detail should show frame 3 after moving; got %q", frame.label)
	}
}

// TestPcapViewerLayerHighlight verifies that stepping onto a layer in the focused
// tree highlights exactly that layer's bytes in the hex view.
func TestPcapViewerLayerHighlight(t *testing.T) {
	v := sizedPcapViewer(t)
	v.handleKey(tea.KeyMsg{Type: tea.KeyTab}) // focus the layer tree
	v.handleKey(keyDown().(tea.KeyMsg))       // Frame -> first layer
	stack := v.packets[0].LayerStack()        // frame 1
	start, length := v.detail.hex.Highlight()
	if start != stack[0].Offset || length != stack[0].Length {
		t.Errorf("highlight = [%d,%d), want first layer [%d,%d)",
			start, start+length, stack[0].Offset, stack[0].Offset+stack[0].Length)
	}
}
