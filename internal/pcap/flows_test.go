package pcap

import (
	"testing"
	"time"
)

func TestFlows(t *testing.T) {
	c := loadSample(t)
	flows := Flows(c.Packets)
	if len(flows) != 3 {
		t.Fatalf("flows = %d, want 3", len(flows))
	}

	cases := []struct {
		proto            string
		srcIP            string
		srcPort, dstPort int
		dstIP            string
		packets          int
	}{
		{"UDP", "192.168.1.100", 50000, 53, "192.168.1.1", 2},
		{"TCP", "192.168.1.100", 50001, 80, "93.184.216.34", 6},
		{"TCP", "192.168.1.100", 50002, 443, "93.184.216.34", 4},
	}
	for i, want := range cases {
		f := flows[i]
		if f.Protocol != want.proto || f.SrcIP != want.srcIP || f.SrcPort != want.srcPort ||
			f.DstIP != want.dstIP || f.DstPort != want.dstPort {
			t.Errorf("flow %d = %s %s:%d->%s:%d, want %s %s:%d->%s:%d", i,
				f.Protocol, f.SrcIP, f.SrcPort, f.DstIP, f.DstPort,
				want.proto, want.srcIP, want.srcPort, want.dstIP, want.dstPort)
		}
		if f.Packets != want.packets {
			t.Errorf("flow %d packets = %d, want %d", i, f.Packets, want.packets)
		}
	}
}

// TestFlowsBidirectional verifies that request and response packets collapse into
// one flow rather than two directional ones.
func TestFlowsBidirectional(t *testing.T) {
	c := loadSample(t)
	http := Flows(c.Packets)[1] // the port-80 conversation
	// The HTTP flow spans packets 5..10, in both directions.
	want := []int{5, 6, 7, 8, 9, 10}
	if len(http.Indices) != len(want) {
		t.Fatalf("http flow indices = %v, want %v", http.Indices, want)
	}
	for i, idx := range want {
		if http.Indices[i] != idx {
			t.Errorf("http flow index %d = %d, want %d", i, http.Indices[i], idx)
		}
	}
}

func TestFlowDuration(t *testing.T) {
	c := loadSample(t)
	// The DNS flow is packets 3 (t=20ms) and 4 (t=30ms): a 10ms span.
	if got := Flows(c.Packets)[0].Duration(); got != 10*time.Millisecond {
		t.Errorf("dns flow duration = %v, want 10ms", got)
	}
}

// TestFlowAt returns the conversation a packet belongs to together with the
// packet's position within it, and reports frames outside any conversation.
func TestFlowAt(t *testing.T) {
	c := loadSample(t)
	// Frame 8 (the HTTP GET) is the 4th frame of the port-80 conversation
	// (frames 5, 6, 7, 8, …), so its position within the flow is 3.
	f, pos, ok := FlowAt(c.Packets, 8)
	if !ok {
		t.Fatal("frame 8 should belong to a flow")
	}
	if f.Protocol != "TCP" || f.DstPort != 80 {
		t.Errorf("frame 8 flow = %s :%d, want TCP :80", f.Protocol, f.DstPort)
	}
	if pos != 3 {
		t.Errorf("frame 8 position within its flow = %d, want 3", pos)
	}
	if want := []int{5, 6, 7, 8, 9, 10}; len(f.Indices) != len(want) {
		t.Errorf("flow frames = %v, want %v", f.Indices, want)
	}

	// ARP (frame 1) is not part of any TCP or UDP conversation.
	if _, _, ok := FlowAt(c.Packets, 1); ok {
		t.Error("ARP frame 1 should not belong to any flow")
	}
	// Indices outside the capture are rejected.
	if _, _, ok := FlowAt(c.Packets, 0); ok {
		t.Error("index 0 is out of range")
	}
	if _, _, ok := FlowAt(c.Packets, len(c.Packets)+1); ok {
		t.Error("index past the end is out of range")
	}
}
