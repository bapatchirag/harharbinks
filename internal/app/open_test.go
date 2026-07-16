package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcapgo"

	"github.com/bapatchirag/harharbinks/internal/pcap"
)

// TestOpenRoutesByFormat verifies Open returns the right screen for each sample
// capture: the HAR viewer for a HAR archive and the PCAP viewer for pcap and
// pcapng captures.
func TestOpenRoutesByFormat(t *testing.T) {
	h, err := Open("../../testdata/sample.har")
	if err != nil {
		t.Fatalf("Open(sample.har): %v", err)
	}
	if _, ok := h.(*Viewer); !ok {
		t.Errorf("Open(sample.har) = %T, want *Viewer", h)
	}

	for _, path := range []string{"../../testdata/sample.pcap", "../../testdata/sample.pcapng"} {
		s, err := Open(path)
		if err != nil {
			t.Fatalf("Open(%s): %v", path, err)
		}
		if _, ok := s.(*PcapViewer); !ok {
			t.Errorf("Open(%s) = %T, want *PcapViewer", path, s)
		}
	}
}

// TestLooksLikeCaptureByExtension verifies the extension alone decides the format
// before any file is opened, case-insensitively.
func TestLooksLikeCaptureByExtension(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"capture.pcap", true},
		{"capture.pcapng", true},
		{"capture.cap", true},
		{"trace.PCAP", true},
		{"archive.har", false},
	}
	for _, c := range cases {
		if got := looksLikeCapture(c.path); got != c.want {
			t.Errorf("looksLikeCapture(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

// TestLooksLikeCaptureSniffsUnknownExtension verifies that a file whose extension
// does not decide the format is classified by sniffing its leading bytes, and
// that an unreadable file falls through to HAR handling.
func TestLooksLikeCaptureSniffsUnknownExtension(t *testing.T) {
	dir := t.TempDir()

	capPath := filepath.Join(dir, "dump")
	if err := os.WriteFile(capPath, []byte{0xd4, 0xc3, 0xb2, 0xa1, 0x00, 0x00}, 0o600); err != nil {
		t.Fatal(err)
	}
	if !looksLikeCapture(capPath) {
		t.Error("a pcap-magic file with no capture extension should sniff as a capture")
	}

	harPath := filepath.Join(dir, "archive")
	if err := os.WriteFile(harPath, []byte(`{"log":{"entries":[]}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if looksLikeCapture(harPath) {
		t.Error("a JSON file with no capture extension should not sniff as a capture")
	}

	if looksLikeCapture(filepath.Join(dir, "does-not-exist")) {
		t.Error("a missing, extension-less file should default to HAR handling")
	}
}

// TestCaptureNotice verifies the viewer caveat string reflects a capture's state:
// empty for a clean capture, naming a truncation, and naming an undecodable link
// type.
func TestCaptureNotice(t *testing.T) {
	if got := captureNotice(&pcap.Capture{}); got != "" {
		t.Errorf("clean capture notice = %q, want empty", got)
	}
	if got := captureNotice(&pcap.Capture{Truncated: true}); !strings.Contains(got, "truncated") {
		t.Errorf("truncated notice = %q, want it to mention truncation", got)
	}
	note := captureNotice(unsupportedCapture(t))
	if !strings.Contains(note, "unsupported link type") {
		t.Errorf("unsupported-link notice = %q, want it to mention the link type", note)
	}
}

// unsupportedCapture builds a one-frame capture on LINKTYPE_USER0, a link type
// gopacket cannot decode, so Capture.Decodable reports false.
func unsupportedCapture(t *testing.T) *pcap.Capture {
	t.Helper()
	var buf bytes.Buffer
	w := pcapgo.NewWriter(&buf)
	if err := w.WriteFileHeader(65536, layers.LinkType(147)); err != nil {
		t.Fatalf("write file header: %v", err)
	}
	frame := []byte{0xde, 0xad, 0xbe, 0xef}
	ci := gopacket.CaptureInfo{Timestamp: time.Unix(0, 0), CaptureLength: len(frame), Length: len(frame)}
	if err := w.WritePacket(ci, frame); err != nil {
		t.Fatalf("write packet: %v", err)
	}
	c, err := pcap.Parse(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("parse crafted capture: %v", err)
	}
	return c
}
