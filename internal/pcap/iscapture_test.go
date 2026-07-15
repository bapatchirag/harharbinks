package pcap

import "testing"

// TestIsCapture verifies the leading-magic sniff accepts every supported capture
// format in both byte orders and rejects non-captures and short inputs.
func TestIsCapture(t *testing.T) {
	cases := []struct {
		name   string
		header []byte
		want   bool
	}{
		{"pcap micros", []byte{0xa1, 0xb2, 0xc3, 0xd4}, true},
		{"pcap micros swapped", []byte{0xd4, 0xc3, 0xb2, 0xa1}, true},
		{"pcap nanos", []byte{0xa1, 0xb2, 0x3c, 0x4d}, true},
		{"pcap nanos swapped", []byte{0x4d, 0x3c, 0xb2, 0xa1}, true},
		{"pcapng", []byte{0x0a, 0x0d, 0x0d, 0x0a}, true},
		{"trailing bytes ignored", []byte{0xd4, 0xc3, 0xb2, 0xa1, 0x02, 0x00}, true},
		{"json (har)", []byte(`{"lo`), false},
		{"too short", []byte{0x0a, 0x0d}, false},
		{"empty", nil, false},
	}
	for _, c := range cases {
		if got := IsCapture(c.header); got != c.want {
			t.Errorf("IsCapture(%s) = %v, want %v", c.name, got, c.want)
		}
	}
}
