package pcap

import (
	"slices"
	"testing"
)

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

// TestParseQuery checks free-text vs scoped term parsing, including that a
// colon-bearing term whose prefix is not a known field stays free text.
func TestParseQuery(t *testing.T) {
	cases := []struct {
		raw       string
		wantPreds []predicate
	}{
		{"", nil},
		{"tls", []predicate{{"", "tls"}}},
		{"proto:tcp", []predicate{{"proto", "tcp"}}},
		{"PROTO:TCP", []predicate{{"proto", "tcp"}}}, // field and value lowercased
		{"proto:tls src:1.2.3.4", []predicate{{"proto", "tls"}, {"src", "1.2.3.4"}}},
		{"time:12:30", []predicate{{"", "time:12:30"}}}, // unknown field -> free text
		{"proto:", nil}, // bare field while typing -> dropped
	}
	for _, tc := range cases {
		q := ParseQuery(tc.raw)
		if len(q.preds) != len(tc.wantPreds) {
			t.Errorf("ParseQuery(%q) preds = %v, want %v", tc.raw, q.preds, tc.wantPreds)
			continue
		}
		for i, p := range q.preds {
			if p != tc.wantPreds[i] {
				t.Errorf("ParseQuery(%q) pred %d = %v, want %v", tc.raw, i, p, tc.wantPreds[i])
			}
		}
	}
}

// TestQueryMatch exercises the parsed filter over the sample capture: free text,
// scoped terms, and conjunctive combinations of the two.
func TestQueryMatch(t *testing.T) {
	c := loadSample(t)
	match := func(raw string) []int {
		q := ParseQuery(raw)
		var out []int
		for _, p := range c.Packets {
			if q.Empty() || q.Match(p) {
				out = append(out, p.Index)
			}
		}
		return out
	}

	// Free text matches across fields (DNS info and the TLS SNI here).
	if got := match("example.com"); !slices.Equal(got, []int{3, 4, 14}) {
		t.Errorf(`match("example.com") = %v, want [3 4 14]`, got)
	}
	// A scoped protocol term.
	if got := match("proto:dns"); !slices.Equal(got, []int{3, 4}) {
		t.Errorf(`match("proto:dns") = %v, want [3 4]`, got)
	}
	// A scoped port term matches either endpoint's port.
	if got := match("port:443"); !slices.Equal(got, []int{11, 12, 13, 14}) {
		t.Errorf(`match("port:443") = %v, want [11 12 13 14]`, got)
	}
	// Conjunctive: two scoped terms.
	if got := match("proto:tls src:192.168.1.100"); !slices.Equal(got, []int{14}) {
		t.Errorf(`match("proto:tls src:...") = %v, want [14]`, got)
	}
	// Conjunctive: free text AND a scoped term.
	if got := match("example.com proto:tls"); !slices.Equal(got, []int{14}) {
		t.Errorf(`match("example.com proto:tls") = %v, want [14]`, got)
	}
	// An empty query matches everything.
	if got := match(""); len(got) != len(c.Packets) {
		t.Errorf("empty query matched %d packets, want %d", len(got), len(c.Packets))
	}
}
