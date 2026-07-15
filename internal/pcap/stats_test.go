package pcap

import (
	"testing"
	"time"
)

func TestSummarize(t *testing.T) {
	c := loadSample(t)
	s := Summarize(c.Packets)

	if s.Packets != 16 {
		t.Errorf("packets = %d, want 16", s.Packets)
	}
	if s.Duration() != 150*time.Millisecond {
		t.Errorf("duration = %v, want 150ms", s.Duration())
	}

	// Protocols are ordered by descending packet count; TCP is the most frequent.
	if len(s.Protocols) != 6 {
		t.Fatalf("protocols = %d, want 6", len(s.Protocols))
	}
	if s.Protocols[0].Protocol != "TCP" || s.Protocols[0].Packets != 7 {
		t.Errorf("top protocol = %s/%d, want TCP/7", s.Protocols[0].Protocol, s.Protocols[0].Packets)
	}

	// Top talkers are ordered by descending bytes; the client touches every packet.
	if len(s.TopTalkers) == 0 || s.TopTalkers[0].Address != "192.168.1.100" {
		t.Fatalf("top talker = %+v, want 192.168.1.100 first", s.TopTalkers)
	}
	if s.TopTalkers[0].Packets != 16 {
		t.Errorf("top talker packets = %d, want 16", s.TopTalkers[0].Packets)
	}

	var protoTotal int
	for _, p := range s.Protocols {
		protoTotal += p.Packets
	}
	if protoTotal != s.Packets {
		t.Errorf("protocol packet counts sum to %d, want %d", protoTotal, s.Packets)
	}
}

func TestSummarizeEmpty(t *testing.T) {
	s := Summarize(nil)
	if s.Packets != 0 || s.Duration() != 0 {
		t.Errorf("empty summary = %d packets / %v, want 0 / 0", s.Packets, s.Duration())
	}
}
