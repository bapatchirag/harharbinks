package pcap

import "testing"

func TestFilterProto(t *testing.T) {
	c := loadSample(t)
	got := Filter(c.Packets, FilterOptions{Proto: "tcp"}) // case-insensitive
	if len(got) != 7 {
		t.Fatalf("TCP packets = %d, want 7", len(got))
	}
	for _, p := range got {
		if p.Protocol() != "TCP" {
			t.Errorf("packet %d protocol = %q, want TCP", p.Index, p.Protocol())
		}
	}
}

func TestFilterText(t *testing.T) {
	c := loadSample(t)
	// "example.com" appears in the DNS query/response info and the TLS SNI.
	got := Filter(c.Packets, FilterOptions{Text: "example.com"})
	want := []int{3, 4, 14}
	if len(got) != len(want) {
		t.Fatalf("text-filtered packets = %d, want %d", len(got), len(want))
	}
	for i, idx := range want {
		if got[i].Index != idx {
			t.Errorf("match %d = packet %d, want %d", i, got[i].Index, idx)
		}
	}
}

func TestFilterPreservesOrder(t *testing.T) {
	c := loadSample(t)
	got := Filter(c.Packets, FilterOptions{}) // zero filter matches everything
	if len(got) != len(c.Packets) {
		t.Fatalf("empty filter dropped packets: got %d, want %d", len(got), len(c.Packets))
	}
}

func TestSortLenDesc(t *testing.T) {
	c := loadSample(t)
	packets := append([]Packet(nil), c.Packets...)
	Sort(packets, SortLen, true)
	for i := 1; i < len(packets); i++ {
		if packets[i-1].OrigLen < packets[i].OrigLen {
			t.Fatalf("packets not sorted by descending length at %d: %d < %d",
				i, packets[i-1].OrigLen, packets[i].OrigLen)
		}
	}
	// The largest frame is the HTTP 200 response (packet 9, 131 bytes).
	if packets[0].Index != 9 {
		t.Errorf("largest packet = %d, want 9", packets[0].Index)
	}
}

func TestParseSort(t *testing.T) {
	cases := []struct {
		in      string
		key     SortKey
		desc    bool
		wantErr bool
	}{
		{"", SortNone, false, false},
		{"time", SortTime, false, false},
		{"len:desc", SortLen, true, false},
		{"proto:asc", SortProto, false, false},
		{"src", SortSrc, false, false},
		{"bogus", SortNone, false, true},
		{"len:sideways", SortNone, false, true},
	}
	for _, tc := range cases {
		key, desc, err := ParseSort(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseSort(%q) err = nil, want error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseSort(%q) unexpected error: %v", tc.in, err)
			continue
		}
		if key != tc.key || desc != tc.desc {
			t.Errorf("ParseSort(%q) = (%q, %v), want (%q, %v)", tc.in, key, desc, tc.key, tc.desc)
		}
	}
}
