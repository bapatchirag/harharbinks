package pcap

import (
	"fmt"
	"sort"
	"strings"
)

// FilterOptions selects a subset of packets. A zero-valued field matches anything.
type FilterOptions struct {
	// Proto matches the packet protocol name, case-insensitively (e.g. "tcp").
	Proto string
	// Text matches as a case-insensitive substring of the packet's protocol, info,
	// and source and destination addresses.
	Text string
}

// Match reports whether p satisfies every set filter dimension.
func (o FilterOptions) Match(p Packet) bool {
	if o.Proto != "" && !strings.EqualFold(p.Protocol(), o.Proto) {
		return false
	}
	if o.Text != "" && !strings.Contains(strings.ToLower(searchText(p)), strings.ToLower(o.Text)) {
		return false
	}
	return true
}

// searchText is the concatenated, free-text-searchable fields of a packet.
func searchText(p Packet) string {
	return strings.Join([]string{p.Protocol(), p.Info(), p.Source(), p.Dest()}, "\n")
}

// Filter returns the packets that satisfy o, preserving their original order.
func Filter(packets []Packet, o FilterOptions) []Packet {
	out := make([]Packet, 0, len(packets))
	for _, p := range packets {
		if o.Match(p) {
			out = append(out, p)
		}
	}
	return out
}

// SortKey identifies a sortable packet field.
type SortKey string

// The recognized packet sort keys.
const (
	SortNone  SortKey = ""
	SortTime  SortKey = "time"
	SortLen   SortKey = "len"
	SortProto SortKey = "proto"
	SortSrc   SortKey = "src"
	SortDst   SortKey = "dst"
)

// Less returns a comparison function for key, or nil for SortNone and unknown keys.
func Less(key SortKey) func(a, b Packet) bool {
	switch key {
	case SortTime:
		return func(a, b Packet) bool { return a.Timestamp.Before(b.Timestamp) }
	case SortLen:
		return func(a, b Packet) bool { return a.OrigLen < b.OrigLen }
	case SortProto:
		return func(a, b Packet) bool { return a.Protocol() < b.Protocol() }
	case SortSrc:
		return func(a, b Packet) bool { return a.Source() < b.Source() }
	case SortDst:
		return func(a, b Packet) bool { return a.Dest() < b.Dest() }
	default:
		return nil
	}
}

// Sort orders packets in place by key; desc reverses the order. SortNone is a
// no-op, leaving the slice untouched.
func Sort(packets []Packet, key SortKey, desc bool) {
	less := Less(key)
	if less == nil {
		return
	}
	sort.SliceStable(packets, func(i, j int) bool {
		if desc {
			return less(packets[j], packets[i])
		}
		return less(packets[i], packets[j])
	})
}

// ParseSort parses a sort specifier "key" or "key:asc"/"key:desc". An empty
// string yields SortNone.
func ParseSort(s string) (key SortKey, desc bool, err error) {
	if strings.TrimSpace(s) == "" {
		return SortNone, false, nil
	}
	name, dir, hasDir := strings.Cut(s, ":")
	key = SortKey(strings.ToLower(strings.TrimSpace(name)))
	if hasDir {
		switch strings.ToLower(strings.TrimSpace(dir)) {
		case "asc":
			desc = false
		case "desc":
			desc = true
		default:
			return SortNone, false, fmt.Errorf("invalid sort direction %q (use asc or desc)", dir)
		}
	}
	switch key {
	case SortTime, SortLen, SortProto, SortSrc, SortDst:
		return key, desc, nil
	default:
		return SortNone, false, fmt.Errorf("unknown sort key %q (use time|len|proto|src|dst)", name)
	}
}
