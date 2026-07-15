// This file implements format detection for a bare file argument: it inspects a
// path and returns the screen that should inspect it, routing HTTP archives to
// the HAR viewer and packet captures to the PCAP viewer. Detection prefers the
// file extension and falls back to sniffing the leading magic bytes, so a
// capture with an unusual or missing extension still opens in the right viewer.
// It is the single seam where the app decides which domain a file belongs to;
// the HAR and PCAP domains themselves stay independent.
package app

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bapatchirag/harharbinks/internal/har"
	"github.com/bapatchirag/harharbinks/internal/pcap"
)

// Open loads the file at path and returns the screen that inspects it, choosing
// between the HAR viewer and the PCAP viewer by the file's detected format. The
// format is decided first by extension and, when that is unhelpful, by sniffing
// the file's leading bytes for a capture magic number; anything not recognized
// as a capture is parsed as a HAR document. A parse failure is returned to the
// caller so it can report the error and exit.
func Open(path string) (Screen, error) {
	if looksLikeCapture(path) {
		c, err := pcap.ParseFile(path)
		if err != nil {
			return nil, err
		}
		return NewPcapViewer(c.Packets, path), nil
	}
	h, err := har.ParseFile(path)
	if err != nil {
		return nil, err
	}
	return NewViewer(h.Log.Entries, path), nil
}

// looksLikeCapture reports whether path should be opened as a packet capture. It
// trusts a decisive extension (.pcap/.pcapng/.cap for captures, .har for HAR) and
// otherwise sniffs the first four bytes for a capture magic number. A file that
// cannot be opened is treated as HAR, so the subsequent parse surfaces a clearer
// error to the user rather than being masked here.
func looksLikeCapture(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pcap", ".pcapng", ".cap":
		return true
	case ".har":
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	var head [4]byte
	n, _ := io.ReadFull(f, head[:])
	return pcap.IsCapture(head[:n])
}
