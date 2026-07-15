package pcap

import (
	"sort"
	"time"
)

// ProtocolCount is the packet and byte total observed for one protocol.
type ProtocolCount struct {
	Protocol string `json:"protocol"`
	Packets  int    `json:"packets"`
	Bytes    int    `json:"bytes"`
}

// Talker is a single endpoint address ranked by the traffic it participated in.
type Talker struct {
	Address string `json:"address"`
	Packets int    `json:"packets"`
	Bytes   int    `json:"bytes"`
}

// Stats summarizes a capture: overall totals, the time span, the per-protocol
// breakdown, and the busiest endpoints.
type Stats struct {
	Packets    int
	Bytes      int
	Start      time.Time
	End        time.Time
	Protocols  []ProtocolCount // descending by packets, then protocol name
	TopTalkers []Talker        // descending by bytes, then address
}

// Duration is the elapsed time between the capture's first and last packet.
func (s Stats) Duration() time.Duration {
	if s.Packets == 0 {
		return 0
	}
	return s.End.Sub(s.Start)
}

// Summarize computes capture statistics over packets.
func Summarize(packets []Packet) Stats {
	var s Stats
	protoAt := map[string]int{}
	talkerAt := map[string]int{}
	for i, p := range packets {
		s.Packets++
		s.Bytes += p.OrigLen
		if i == 0 || p.Timestamp.Before(s.Start) {
			s.Start = p.Timestamp
		}
		if i == 0 || p.Timestamp.After(s.End) {
			s.End = p.Timestamp
		}

		proto := p.Protocol()
		if j, ok := protoAt[proto]; ok {
			s.Protocols[j].Packets++
			s.Protocols[j].Bytes += p.OrigLen
		} else {
			protoAt[proto] = len(s.Protocols)
			s.Protocols = append(s.Protocols, ProtocolCount{Protocol: proto, Packets: 1, Bytes: p.OrigLen})
		}

		for _, addr := range packetEndpoints(p) {
			if j, ok := talkerAt[addr]; ok {
				s.TopTalkers[j].Packets++
				s.TopTalkers[j].Bytes += p.OrigLen
			} else {
				talkerAt[addr] = len(s.TopTalkers)
				s.TopTalkers = append(s.TopTalkers, Talker{Address: addr, Packets: 1, Bytes: p.OrigLen})
			}
		}
	}

	sort.SliceStable(s.Protocols, func(i, j int) bool {
		if s.Protocols[i].Packets != s.Protocols[j].Packets {
			return s.Protocols[i].Packets > s.Protocols[j].Packets
		}
		return s.Protocols[i].Protocol < s.Protocols[j].Protocol
	})
	sort.SliceStable(s.TopTalkers, func(i, j int) bool {
		if s.TopTalkers[i].Bytes != s.TopTalkers[j].Bytes {
			return s.TopTalkers[i].Bytes > s.TopTalkers[j].Bytes
		}
		return s.TopTalkers[i].Address < s.TopTalkers[j].Address
	})
	return s
}

// packetEndpoints returns the distinct source and destination addresses of a
// packet for top-talker accounting.
func packetEndpoints(p Packet) []string {
	src, dst := p.Source(), p.Dest()
	switch {
	case src == "" && dst == "":
		return nil
	case src == dst:
		return []string{src}
	case src == "":
		return []string{dst}
	case dst == "":
		return []string{src}
	default:
		return []string{src, dst}
	}
}
