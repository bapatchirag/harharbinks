// Package pcap models packet captures (pcap and pcapng) and provides pure,
// UI-free helpers for parsing, decoding, querying, and summarizing them. Like
// internal/har it is independent of the TUI, CLI, and app layers and never
// imports internal/har, so the two capture domains stay fully decoupled.
//
// Captures are read with the pure-Go pcapgo reader — no libpcap, no cgo, and no
// live capture — which preserves harharbinks's single-static-binary,
// CGO_ENABLED=0 guarantee.
package pcap

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcapgo"
)

// Capture is a fully-read packet capture: every record decoded into a Packet,
// together with the link-layer type they were captured on.
type Capture struct {
	// Packets are the capture's frames in file order, each with a 1-based Index.
	Packets []Packet
	// LinkType is the link-layer type used to decode every packet (e.g. Ethernet).
	LinkType layers.LinkType
}

// Packet is a single captured frame plus its lazily-decoded protocol layers. The
// raw bytes are retained so callers can render a hex view or re-decode on demand.
type Packet struct {
	// Index is the frame's 1-based position within the capture.
	Index int
	// Timestamp is the time the frame was captured.
	Timestamp time.Time
	// CapLen is the number of bytes actually stored for the frame, which may be
	// smaller than OrigLen when the capture was snap-length truncated.
	CapLen int
	// OrigLen is the frame's original on-wire length.
	OrigLen int
	// LinkType is the link-layer type used to decode Data.
	LinkType layers.LinkType
	// Data is the raw frame bytes as captured.
	Data []byte

	// decoded is the gopacket view of Data, built once with lazy layer decoding so
	// callers that need only frame metadata never pay to walk every layer.
	decoded gopacket.Packet
}

// Capture magic numbers, read big-endian from the first four bytes: the classic
// pcap microsecond and nanosecond variants in both byte orders, and the pcapng
// Section Header Block type.
const (
	magicPCAPMicros    = 0xa1b2c3d4
	magicPCAPMicrosSwp = 0xd4c3b2a1
	magicPCAPNanos     = 0xa1b23c4d
	magicPCAPNanosSwp  = 0x4d3cb2a1
	magicPCAPNG        = 0x0a0d0d0a
)

// Parse reads an entire capture from r, auto-detecting the classic pcap and
// pcapng formats by their leading magic bytes and decoding every record.
func Parse(r io.Reader) (*Capture, error) {
	br := bufio.NewReader(r)
	magic, err := br.Peek(4)
	if err != nil {
		return nil, fmt.Errorf("read capture header: %w", err)
	}
	src, linkType, err := newSource(br, binary.BigEndian.Uint32(magic))
	if err != nil {
		return nil, err
	}

	c := &Capture{LinkType: linkType}
	for i := 0; ; i++ {
		data, ci, err := src.ReadPacketData()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read packet %d: %w", i+1, err)
		}
		// pcapgo may reuse its internal read buffer between calls, so copy the
		// frame bytes to give each Packet a stable, independent slice.
		frame := make([]byte, len(data))
		copy(frame, data)
		c.Packets = append(c.Packets, newPacket(i+1, ci, linkType, frame))
	}
	return c, nil
}

// ParseFile reads a capture from the file at path.
func ParseFile(path string) (*Capture, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(f)
}

// newSource builds the appropriate pcapgo reader for the detected magic number,
// returning it as a packet source together with its link-layer type.
func newSource(r io.Reader, magic uint32) (gopacket.PacketDataSource, layers.LinkType, error) {
	switch magic {
	case magicPCAPNG:
		ng, err := pcapgo.NewNgReader(r, pcapgo.DefaultNgReaderOptions)
		if err != nil {
			return nil, 0, fmt.Errorf("open pcapng: %w", err)
		}
		return ng, ng.LinkType(), nil
	case magicPCAPMicros, magicPCAPMicrosSwp, magicPCAPNanos, magicPCAPNanosSwp:
		rd, err := pcapgo.NewReader(r)
		if err != nil {
			return nil, 0, fmt.Errorf("open pcap: %w", err)
		}
		return rd, rd.LinkType(), nil
	default:
		return nil, 0, fmt.Errorf("unrecognized capture format (magic 0x%08x)", magic)
	}
}

// newPacket assembles a Packet from a capture record and primes its lazy decoder.
func newPacket(index int, ci gopacket.CaptureInfo, linkType layers.LinkType, data []byte) Packet {
	return Packet{
		Index:     index,
		Timestamp: ci.Timestamp,
		CapLen:    ci.CaptureLength,
		OrigLen:   ci.Length,
		LinkType:  linkType,
		Data:      data,
		decoded:   gopacket.NewPacket(data, linkType, gopacket.Lazy),
	}
}
