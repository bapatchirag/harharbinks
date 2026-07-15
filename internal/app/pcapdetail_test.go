package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bapatchirag/harharbinks/internal/pcap"
	"github.com/bapatchirag/harharbinks/internal/tui/keymap"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// keyRight is a right-arrow key event for the detail tests (keyDown lives in
// app_test.go).
func keyRight() tea.Msg { return tea.KeyMsg{Type: tea.KeyRight} }

// httpPacket returns the sample capture's HTTP GET request (frame 8), whose
// Ethernet/IPv4/TCP/HTTP stack gives the detail tests the richest layer tree.
func httpPacket(t *testing.T) pcap.Packet {
	t.Helper()
	return demoPackets(t)[7] // 1-based frame 8
}

// TestBuildPacketTree checks the layer tree mirrors the packet's layer stack: a
// single expanded Frame root over one collapsed node per layer, each carrying the
// layer's byte range, with field leaves that inherit their layer's range.
func TestBuildPacketTree(t *testing.T) {
	p := httpPacket(t)
	roots := buildPacketTree(p)
	if len(roots) != 1 {
		t.Fatalf("roots = %d, want 1 (Frame)", len(roots))
	}
	frame := roots[0]
	if !frame.Expanded {
		t.Error("Frame node should start expanded")
	}
	if frame.Value.length != len(p.Data) {
		t.Errorf("Frame range length = %d, want %d", frame.Value.length, len(p.Data))
	}

	stack := p.LayerStack()
	if len(frame.Children) != len(stack) {
		t.Fatalf("Frame children = %d, want %d layers", len(frame.Children), len(stack))
	}
	for i, child := range frame.Children {
		if child.Value.offset != stack[i].Offset || child.Value.length != stack[i].Length {
			t.Errorf("layer %d node range = [%d,%d), want [%d,%d)", i,
				child.Value.offset, child.Value.length, stack[i].Offset, stack[i].Length)
		}
	}

	// The IPv4 layer is a collapsed branch whose field leaves each fall within the
	// layer's byte range.
	ip := frame.Children[1]
	if ip.Expanded {
		t.Error("layer nodes should start collapsed")
	}
	if len(ip.Children) == 0 {
		t.Fatal("IPv4 layer should expose field leaves")
	}
	start, end := stack[1].Offset, stack[1].Offset+stack[1].Length
	for _, leaf := range ip.Children {
		if leaf.Value.offset < start || leaf.Value.offset >= end {
			t.Errorf("IPv4 field leaf offset %d outside layer range [%d,%d)", leaf.Value.offset, start, end)
		}
	}
}

// TestPcapDetailSyncHighlight checks the hex highlight tracks the tree selection:
// the whole frame at first, then a single layer's range once selected.
func TestPcapDetailSyncHighlight(t *testing.T) {
	d := newPcapDetail(theme.Default(), keymap.Default())
	p := httpPacket(t)
	d.SetPacket(p)
	d.SetSize(80, 12)

	if start, length := d.hex.Highlight(); start != 0 || length != len(p.Data) {
		t.Errorf("initial highlight = [%d,%d), want whole frame [0,%d)", start, start+length, len(p.Data))
	}

	// Step from the Frame onto the first layer; the highlight narrows to it.
	d.tree.Focus()
	d.tree.Update(keyDown())
	d.syncHighlight()
	stack := p.LayerStack()
	if start, length := d.hex.Highlight(); start != stack[0].Offset || length != stack[0].Length {
		t.Errorf("highlight after one down = [%d,%d), want Ethernet [%d,%d)",
			start, start+length, stack[0].Offset, stack[0].Offset+stack[0].Length)
	}
}

// TestPcapDetailViewShowsFrame is a smoke test that the inspector renders the
// selected packet's frame header.
func TestPcapDetailViewShowsFrame(t *testing.T) {
	d := newPcapDetail(theme.Default(), keymap.Default())
	d.SetPacket(httpPacket(t))
	d.SetSize(80, 12)
	if view := d.View(); !strings.Contains(view, "Frame 8") {
		t.Errorf("detail view should name the frame; got:\n%s", view)
	}
}

// TestPcapDetailFieldHighlight verifies that selecting a field (not just a layer)
// narrows the hex highlight to that field's own bytes.
func TestPcapDetailFieldHighlight(t *testing.T) {
	d := newPcapDetail(theme.Default(), keymap.Default())
	d.SetPacket(httpPacket(t))
	d.SetSize(80, 16)
	d.tree.Focus()
	d.tree.Update(keyDown())  // Frame -> Ethernet layer
	d.tree.Update(keyRight()) // expand the Ethernet layer's fields
	d.tree.Update(keyDown())  // step onto the first field (a MAC address)
	d.syncHighlight()
	if _, length := d.hex.Highlight(); length != 6 {
		t.Errorf("field highlight length = %d, want 6 (a MAC address)", length)
	}
}
