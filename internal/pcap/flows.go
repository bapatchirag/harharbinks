package pcap

import (
	"time"

	"github.com/gopacket/gopacket/layers"
)

// Flow is a bidirectional transport conversation identified by its 5-tuple
// (network addresses, transport ports, and transport protocol). Packets in
// either direction belong to the same flow; the source fields hold the endpoint
// that sent the flow's first packet.
type Flow struct {
	Protocol string    // "TCP" or "UDP"
	SrcIP    string    // initiator network address
	SrcPort  int       // initiator transport port
	DstIP    string    // responder network address
	DstPort  int       // responder transport port
	Packets  int       // packets seen in both directions
	Bytes    int       // summed original frame lengths
	Start    time.Time // timestamp of the first packet
	End      time.Time // timestamp of the last packet
	Indices  []int     // 1-based packet indices, in capture order
}

// Duration is the elapsed time between the flow's first and last packet.
func (f Flow) Duration() time.Duration { return f.End.Sub(f.Start) }

// Flows groups packets into bidirectional TCP and UDP conversations, preserving
// the order in which each flow first appears. Packets without a transport
// 5-tuple (such as ARP and ICMP) are skipped.
func Flows(packets []Packet) []Flow {
	var order []uint64
	byKey := map[uint64]*Flow{}
	for _, p := range packets {
		pkt := p.decoded
		nl := pkt.NetworkLayer()
		tl := pkt.TransportLayer()
		if nl == nil || tl == nil {
			continue
		}
		var proto string
		var srcPort, dstPort int
		switch t := tl.(type) {
		case *layers.TCP:
			proto, srcPort, dstPort = "TCP", int(t.SrcPort), int(t.DstPort)
		case *layers.UDP:
			proto, srcPort, dstPort = "UDP", int(t.SrcPort), int(t.DstPort)
		default:
			continue
		}
		netFlow := nl.NetworkFlow()
		// FastHash is symmetric, so A→B and B→A share a key; mixing in the
		// transport layer type keeps a TCP and a UDP conversation on identical
		// addresses and ports distinct.
		key := netFlow.FastHash() ^ tl.TransportFlow().FastHash() ^ uint64(tl.LayerType())
		f := byKey[key]
		if f == nil {
			f = &Flow{
				Protocol: proto,
				SrcIP:    netFlow.Src().String(),
				DstIP:    netFlow.Dst().String(),
				SrcPort:  srcPort,
				DstPort:  dstPort,
				Start:    p.Timestamp,
				End:      p.Timestamp,
			}
			byKey[key] = f
			order = append(order, key)
		}
		f.Packets++
		f.Bytes += p.OrigLen
		f.Indices = append(f.Indices, p.Index)
		if p.Timestamp.Before(f.Start) {
			f.Start = p.Timestamp
		}
		if p.Timestamp.After(f.End) {
			f.End = p.Timestamp
		}
	}
	out := make([]Flow, len(order))
	for i, k := range order {
		out[i] = *byKey[k]
	}
	return out
}
