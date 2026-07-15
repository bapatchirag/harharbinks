package app

import (
	"os"
	"path/filepath"
	"testing"
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
