package app

import (
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/bapatchirag/harharbinks/internal/pcap"
	"github.com/bapatchirag/harharbinks/internal/tui/msg"
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
// capture, explains there are no packets, and reports a zero position.
func TestPcapViewerEmpty(t *testing.T) {
	v := NewPcapViewer(nil, "empty.pcap")
	v.SetSize(100, 23)
	out := v.View()
	if out == "" {
		t.Error("empty viewer should still render a frame")
	}
	if !strings.Contains(out, "no packets") {
		t.Errorf("empty capture should explain there are no packets; view:\n%s", out)
	}
	if _, ok := v.table.Selected(); ok {
		t.Error("empty capture should have no selection")
	}
}

// TestPcapViewerFilterEmptyMessage verifies that a filter matching no packets
// shows a distinct placeholder inviting the user to clear it, rather than the
// empty-capture message.
func TestPcapViewerFilterEmptyMessage(t *testing.T) {
	v := sizedPcapViewer(t)
	v.Update(msg.SearchMsg{Query: "proto:nonesuch"})
	if got := len(v.table.Rows()); got != 0 {
		t.Fatalf("filter should match nothing; got %d rows", got)
	}
	if out := v.View(); !strings.Contains(out, "No packets match") {
		t.Errorf("filtered-empty view should invite clearing the filter; view:\n%s", out)
	}
}

// TestPcapViewerNotice verifies a capture caveat set via SetNotice (for example a
// truncated capture) appears in the packet-list status bar.
func TestPcapViewerNotice(t *testing.T) {
	v := sizedPcapViewer(t)
	v.SetNotice("truncated capture")
	if out := v.View(); !strings.Contains(out, "truncated capture") {
		t.Errorf("notice should appear in the status bar; view:\n%s", out)
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

// TestPcapViewerHighlightClearsOnList verifies the layer→hex range highlight is
// shown only while the detail is focused: navigating the tree highlights bytes,
// the highlight stays while the hex pane is focused, and Tab-ing back to the list
// clears it rather than leaving it stuck.
func TestPcapViewerHighlightClearsOnList(t *testing.T) {
	v := sizedPcapViewer(t)
	v.handleKey(tea.KeyMsg{Type: tea.KeyTab}) // list -> layer tree
	v.handleKey(keyDown().(tea.KeyMsg))       // Frame -> first layer (highlights its bytes)
	if _, length := v.detail.hex.Highlight(); length == 0 {
		t.Fatal("navigating the layer tree should highlight the layer's bytes")
	}
	v.handleKey(tea.KeyMsg{Type: tea.KeyTab}) // tree -> hex: highlight stays
	if _, length := v.detail.hex.Highlight(); length == 0 {
		t.Error("the highlight should remain while the hex pane is focused")
	}
	v.handleKey(tea.KeyMsg{Type: tea.KeyTab}) // hex -> list: highlight clears
	if !v.table.Focused() {
		t.Fatal("focus should wrap back to the list")
	}
	if _, length := v.detail.hex.Highlight(); length != 0 {
		t.Errorf("the highlight should clear when the list is focused, got length %d", length)
	}
}

// TestPcapViewerFilter verifies "/" opens the display filter and a live query
// narrows the packet list to the matching protocol.
func TestPcapViewerFilter(t *testing.T) {
	v := sizedPcapViewer(t)
	v.Update(runeKey('/'))
	if !v.searching || !v.CapturesInput() {
		t.Fatal(`"/" should open the filter field and capture input`)
	}
	v.Update(msg.SearchMsg{Query: "tls"})
	rows := v.table.Rows()
	if len(rows) == 0 {
		t.Fatal("filtering by tls should keep the TLS packet(s)")
	}
	for _, p := range rows {
		if p.Protocol() != "TLS" {
			t.Errorf("filtered row protocol = %q, want TLS", p.Protocol())
		}
	}
}

// TestPcapViewerFilterClear verifies esc closes the filter field and restores the
// full list.
func TestPcapViewerFilterClear(t *testing.T) {
	v := sizedPcapViewer(t)
	v.Update(runeKey('/'))
	v.Update(msg.SearchMsg{Query: "tls"})
	if len(v.table.Rows()) == len(v.packets) {
		t.Fatal("filter should have narrowed the list")
	}
	v.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if v.searching {
		t.Error("esc should close the filter field")
	}
	if got := len(v.table.Rows()); got != len(v.packets) {
		t.Errorf("clearing the filter should restore all %d packets, got %d", len(v.packets), got)
	}
}

// TestPcapViewerSort steps the sort cycle to length-descending and checks the
// list is reordered accordingly.
func TestPcapViewerSort(t *testing.T) {
	v := sizedPcapViewer(t)
	v.Update(runeKey('s')) // none -> time
	v.Update(runeKey('s')) // time -> proto
	v.Update(runeKey('s')) // proto -> len (descending)
	if got := pcapSortCycle[v.sortIdx].label; got != "len\u2193" {
		t.Fatalf("after three s, sort = %q, want len\u2193", got)
	}
	rows := v.table.Rows()
	for i := 1; i < len(rows); i++ {
		if rows[i-1].OrigLen < rows[i].OrigLen {
			t.Errorf("rows not sorted by length descending at %d: %d < %d", i, rows[i-1].OrigLen, rows[i].OrigLen)
		}
	}
}

// TestPcapViewerViewsMenu verifies "e" opens the views menu and selecting
// Conversations switches to the flows table, which esc leaves.
func TestPcapViewerViewsMenu(t *testing.T) {
	v := sizedPcapViewer(t)
	v.Update(runeKey('e'))
	if !v.menuOpen || !v.CapturesInput() {
		t.Fatal(`"e" should open the views menu and capture input`)
	}
	v.Update(msg.MenuActionMsg{Action: "flows"})
	if v.mode != modeFlows {
		t.Fatalf("selecting Conversations should switch to flows mode, got %d", v.mode)
	}
	if got := len(v.flows.table.Rows()); got != 3 {
		t.Errorf("the sample capture has 3 conversations, got %d", got)
	}
	v.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if v.mode != modePackets {
		t.Error("esc should return from flows to the packet list")
	}
}

// TestPcapViewerStatsView verifies the statistics view renders the capture
// summary, protocol hierarchy, and top talkers.
func TestPcapViewerStatsView(t *testing.T) {
	v := sizedPcapViewer(t)
	v.Update(runeKey('e'))
	v.Update(msg.MenuActionMsg{Action: "stats"})
	if v.mode != modeStats {
		t.Fatalf("selecting statistics should switch to stats mode, got %d", v.mode)
	}
	view := v.View()
	for _, want := range []string{"Capture Summary", "Protocol Hierarchy", "Top Talkers"} {
		if !strings.Contains(view, want) {
			t.Errorf("stats view should contain %q; view:\n%s", want, view)
		}
	}
	v.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if v.mode != modePackets {
		t.Error("esc should return from stats to the packet list")
	}
}

// TestPcapViewerFollowFromList verifies enter on a packet scopes the list to its
// conversation — the whole path of frames, those leading up to the selected
// packet and those after it — and esc restores the full list with that frame
// re-selected.
func TestPcapViewerFollowFromList(t *testing.T) {
	v := sizedPcapViewer(t)
	v.table.SetCursor(7) // frame 8, the HTTP GET (in the port-80 conversation)
	v.syncPacket()
	v.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !v.followMode || v.mode != modePackets {
		t.Fatalf("enter should scope the list to the conversation; followMode=%v mode=%d", v.followMode, v.mode)
	}
	got := make([]int, 0, len(v.table.Rows()))
	for _, p := range v.table.Rows() {
		got = append(got, p.Index)
	}
	if want := []int{5, 6, 7, 8, 9, 10}; !slices.Equal(got, want) {
		t.Errorf("followed frames = %v, want the whole port-80 path %v", got, want)
	}
	if sel, _ := v.table.Selected(); sel.Index != 8 {
		t.Errorf("the followed frame should stay selected; got frame %d", sel.Index)
	}
	v.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if v.followMode || len(v.table.Rows()) != len(v.packets) {
		t.Errorf("esc should restore the full list; followMode=%v rows=%d", v.followMode, len(v.table.Rows()))
	}
	if sel, _ := v.table.Selected(); sel.Index != 8 {
		t.Errorf("esc should re-select the followed frame; got frame %d", sel.Index)
	}
}

// TestPcapViewerFollowFromFlows verifies enter on a conversation scopes the list
// to its frames and esc returns to the flows table rather than the full list.
func TestPcapViewerFollowFromFlows(t *testing.T) {
	v := sizedPcapViewer(t)
	v.Update(runeKey('e'))
	v.Update(msg.MenuActionMsg{Action: "flows"})
	v.flows.table.SetCursor(1) // the port-80 HTTP conversation (6 frames)
	v.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !v.followMode || v.mode != modePackets {
		t.Fatalf("enter on a flow should scope the list to it; followMode=%v mode=%d", v.followMode, v.mode)
	}
	if got := len(v.table.Rows()); got != 6 {
		t.Errorf("the port-80 conversation has 6 frames, got %d", got)
	}
	v.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if v.mode != modeFlows {
		t.Error("esc should return from the follow view to the flows table")
	}
}

// TestPcapViewerFollowSingleFrame verifies that following a packet not part of any
// conversation (ARP) still does something: it scopes the list to just that one
// frame, and esc restores the full list.
func TestPcapViewerFollowSingleFrame(t *testing.T) {
	v := sizedPcapViewer(t)
	v.table.SetCursor(0) // frame 1, ARP — not part of any flow
	v.syncPacket()
	v.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !v.followMode || v.mode != modePackets {
		t.Fatalf("following an unconnected frame should scope the list; followMode=%v mode=%d", v.followMode, v.mode)
	}
	if got := len(v.table.Rows()); got != 1 {
		t.Errorf("following a lone frame should show just that frame, got %d rows", got)
	}
	if sel, _ := v.table.Selected(); sel.Index != 1 {
		t.Errorf("the followed frame should be frame 1, got %d", sel.Index)
	}
	v.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if v.followMode || len(v.table.Rows()) != len(v.packets) {
		t.Errorf("esc should restore the full list; followMode=%v rows=%d", v.followMode, len(v.table.Rows()))
	}
}

// TestPcapViewerScopedFilter verifies the packet filter accepts HAR-style
// field:value terms combined conjunctively with each other and with free text.
func TestPcapViewerScopedFilter(t *testing.T) {
	v := sizedPcapViewer(t)
	v.Update(runeKey('/'))
	v.Update(msg.SearchMsg{Query: "proto:tls src:192.168.1.100"})
	if rows := v.table.Rows(); len(rows) != 1 || rows[0].Protocol() != "TLS" {
		t.Fatalf("proto:tls src:\u2026 should match the lone TLS ClientHello, got %d rows", len(rows))
	}
	// A scoped port term narrows to the port-80 conversation's six frames.
	v.Update(msg.SearchMsg{Query: "port:80"})
	if got := len(v.table.Rows()); got != 6 {
		t.Errorf("port:80 should match the 6 port-80 frames, got %d", got)
	}
}

// TestPcapFlowsGolden renders the conversations view over the sample capture and
// compares it to a checked-in golden (regenerate with -update).
func TestPcapFlowsGolden(t *testing.T) {
	v := sizedPcapViewer(t)
	v.SetSize(100, 24)
	v.showFlows()
	golden.RequireEqual(t, []byte(v.View()))
}

// TestPcapStatsGolden renders the capture-statistics view over the sample and
// compares it to a checked-in golden (regenerate with -update).
func TestPcapStatsGolden(t *testing.T) {
	v := sizedPcapViewer(t)
	v.SetSize(100, 24)
	v.showStats()
	golden.RequireEqual(t, []byte(v.View()))
}

// TestPcapFollowGolden renders the follow view of the sample's HTTP conversation
// — the packet list scoped to that flow's frames — and compares it to a
// checked-in golden (regenerate with -update).
func TestPcapFollowGolden(t *testing.T) {
	v := sizedPcapViewer(t)
	v.SetSize(100, 24)
	v.table.SetCursor(7) // frame 8, in the port-80 conversation
	v.syncPacket()
	v.followSelected()
	golden.RequireEqual(t, []byte(v.View()))
}
