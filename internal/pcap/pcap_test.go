package pcap

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/gopacket/gopacket/layers"
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
