package pcap

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcapgo"
)

const (
	samplePCAP   = "../../testdata/sample.pcap"
	samplePCAPNG = "../../testdata/sample.pcapng"
)

// loadSample parses the classic-pcap fixture, failing the test on any error.
func loadSample(t *testing.T) *Capture {
	t.Helper()
	c, err := ParseFile(samplePCAP)
	if err != nil {
		t.Fatalf("ParseFile(%s): %v", samplePCAP, err)
	}
	return c
}

func TestParseFilePacketCount(t *testing.T) {
	c := loadSample(t)
	if got := len(c.Packets); got != 16 {
		t.Fatalf("packets = %d, want 16", got)
	}
	if c.LinkType != layers.LinkTypeEthernet {
		t.Errorf("link type = %v, want Ethernet", c.LinkType)
	}
	for i, p := range c.Packets {
		if p.Index != i+1 {
			t.Errorf("packet %d has Index %d, want %d", i, p.Index, i+1)
		}
		if p.CapLen != p.OrigLen {
			t.Errorf("packet %d CapLen %d != OrigLen %d (fixture is untruncated)", p.Index, p.CapLen, p.OrigLen)
		}
		if len(p.Data) != p.CapLen {
			t.Errorf("packet %d has %d data bytes, want CapLen %d", p.Index, len(p.Data), p.CapLen)
		}
	}
}

// TestParsePcapngMatchesPcap verifies that the pcapng reader path decodes the
// same frames, protocols, and info as the classic-pcap path.
func TestParsePcapngMatchesPcap(t *testing.T) {
	classic := loadSample(t)
	ng, err := ParseFile(samplePCAPNG)
	if err != nil {
		t.Fatalf("ParseFile(%s): %v", samplePCAPNG, err)
	}
	if len(ng.Packets) != len(classic.Packets) {
		t.Fatalf("pcapng packets = %d, want %d", len(ng.Packets), len(classic.Packets))
	}
	for i := range classic.Packets {
		a, b := classic.Packets[i], ng.Packets[i]
		if a.Protocol() != b.Protocol() {
			t.Errorf("packet %d protocol: pcap %q, pcapng %q", i+1, a.Protocol(), b.Protocol())
		}
		if a.Info() != b.Info() {
			t.Errorf("packet %d info: pcap %q, pcapng %q", i+1, a.Info(), b.Info())
		}
		if !a.Timestamp.Equal(b.Timestamp) {
			t.Errorf("packet %d timestamp: pcap %v, pcapng %v", i+1, a.Timestamp, b.Timestamp)
		}
	}
}

func TestParseUnrecognizedFormat(t *testing.T) {
	if _, err := Parse(strings.NewReader("this is not a capture file")); err == nil {
		t.Fatal("expected an error for a non-capture reader, got nil")
	}
}

func TestParseShortReader(t *testing.T) {
	if _, err := Parse(strings.NewReader("ab")); err == nil {
		t.Fatal("expected an error for a truncated header, got nil")
	}
}

// TestPcapDoesNotImportHAR enforces the architectural rule that the two capture
// domains stay decoupled: internal/pcap must never depend on internal/har.
func TestPcapDoesNotImportHAR(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps",
		"github.com/bapatchirag/harharbinks/internal/pcap").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps: %v\n%s", err, out)
	}
	if strings.Contains(string(out), "harharbinks/internal/har") {
		t.Error("internal/pcap must not depend on internal/har")
	}
}

// TestParseTruncatedKeepsPackets verifies that a capture cut off mid-record is
// read up to the break: the frames before it are kept and the capture is flagged
// truncated, rather than the whole read failing.
func TestParseTruncatedKeepsPackets(t *testing.T) {
	raw, err := os.ReadFile(samplePCAP)
	if err != nil {
		t.Fatalf("read %s: %v", samplePCAP, err)
	}
	// Drop the final bytes so the last record's data is incomplete while its
	// header — and every earlier frame — stays intact.
	cut := raw[:len(raw)-8]
	c, err := Parse(bytes.NewReader(cut))
	if err != nil {
		t.Fatalf("Parse(truncated) returned error: %v", err)
	}
	if !c.Truncated {
		t.Error("a capture cut mid-record should set Capture.Truncated")
	}
	full := loadSample(t)
	if n := len(c.Packets); n == 0 || n >= len(full.Packets) {
		t.Errorf("truncated packets = %d, want between 1 and %d", n, len(full.Packets)-1)
	}
	for i, p := range c.Packets {
		if p.Index != i+1 {
			t.Errorf("kept packet %d has Index %d, want %d", i, p.Index, i+1)
		}
	}
}

// TestParseCleanCaptureNotTruncated confirms a complete capture is not flagged
// truncated.
func TestParseCleanCaptureNotTruncated(t *testing.T) {
	if loadSample(t).Truncated {
		t.Error("a complete capture must not be flagged truncated")
	}
}

// TestCaptureDecodable confirms the Ethernet sample reports a decodable link
// type and a human-readable link-type name.
func TestCaptureDecodable(t *testing.T) {
	c := loadSample(t)
	if !c.Decodable() {
		t.Error("the Ethernet sample capture should be decodable")
	}
	if c.LinkTypeName() == "" {
		t.Error("LinkTypeName should name the link type")
	}
}

// TestEmptyCaptureDecodable confirms an empty capture is treated as decodable —
// there is no undecodable frame to warn about.
func TestEmptyCaptureDecodable(t *testing.T) {
	if !(&Capture{}).Decodable() {
		t.Error("an empty capture should be decodable")
	}
}

// TestUnsupportedLinkTypeNotDecodable builds a tiny capture on a link type
// gopacket cannot decode (LINKTYPE_USER0) and verifies the frame is still read
// but the capture reports itself undecodable, so a caller can warn and fall back
// to a raw view.
func TestUnsupportedLinkTypeNotDecodable(t *testing.T) {
	const linkTypeUser0 = layers.LinkType(147)
	var buf bytes.Buffer
	w := pcapgo.NewWriter(&buf)
	if err := w.WriteFileHeader(65536, linkTypeUser0); err != nil {
		t.Fatalf("write file header: %v", err)
	}
	frame := []byte{0xde, 0xad, 0xbe, 0xef}
	ci := gopacket.CaptureInfo{Timestamp: time.Unix(0, 0), CaptureLength: len(frame), Length: len(frame)}
	if err := w.WritePacket(ci, frame); err != nil {
		t.Fatalf("write packet: %v", err)
	}
	c, err := Parse(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Parse(user-link) returned error: %v", err)
	}
	if len(c.Packets) != 1 {
		t.Fatalf("packets = %d, want 1", len(c.Packets))
	}
	if c.Decodable() {
		t.Error("a capture on an undecodable link type should report Decodable() == false")
	}
	if c.LinkType != linkTypeUser0 {
		t.Errorf("link type = %v, want %v", c.LinkType, linkTypeUser0)
	}
}
