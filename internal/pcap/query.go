package pcap

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gopacket/gopacket/layers"
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
	if o.Text != "" && !strings.Contains(strings.ToLower(SearchText(p)), strings.ToLower(o.Text)) {
		return false
	}
	return true
}

// SearchText returns a packet's human-readable, free-text-searchable fields
// joined into one string — its protocol, info line, and source and destination
// addresses — so a free-text query can match anything shown in the packet list.
func SearchText(p Packet) string {
	return strings.Join([]string{p.Protocol(), p.Info(), p.Source(), p.Dest()}, "\n")
}

// scopedFields maps a filter field name to the packet text it searches. It is the
// single source of truth for both the field:value filter and the set of
// recognized field names.
var scopedFields = map[string]func(Packet) string{
	"proto": func(p Packet) string { return p.Protocol() },
	"src":   func(p Packet) string { return p.Source() },
	"dst":   func(p Packet) string { return p.Dest() },
	"info":  func(p Packet) string { return p.Info() },
	"port":  portText,
}

// portText returns a packet's transport source and destination ports as text, or
// "" when it has no TCP or UDP layer, so a "port:" term can match either end.
func portText(p Packet) string {
	switch t := p.decoded.TransportLayer().(type) {
	case *layers.TCP:
		return fmt.Sprintf("%d %d", t.SrcPort, t.DstPort)
	case *layers.UDP:
		return fmt.Sprintf("%d %d", t.SrcPort, t.DstPort)
	default:
		return ""
	}
}

// predicate is one term of a parsed filter: free text (field == "") matched
// against the whole packet, or a value scoped to a single field.
type predicate struct {
	field string
	value string // lowercased
}

// Query is a parsed interactive filter. A packet matches only when it satisfies
// every term (terms are conjunctive). Build one with ParseQuery.
type Query struct {
	preds []predicate
}

// ParseQuery parses a raw filter string into a Query. Whitespace separates terms.
// A term of the form field:value is scoped to that field when field is a
// recognized name (proto, src, dst, port, info); otherwise the whole term is free
// text and matches across every searchable field, so addresses and other
// colon-bearing text still work. Matching is case-insensitive, and multiple terms
// are conjunctive.
func ParseQuery(raw string) Query {
	var q Query
	for _, tok := range strings.Fields(raw) {
		field, value := "", tok
		if i := strings.IndexByte(tok, ':'); i > 0 {
			if name := strings.ToLower(tok[:i]); isScopedField(name) {
				field, value = name, tok[i+1:]
			}
		}
		if value == "" {
			continue // e.g. a bare "proto:" while still being typed
		}
		q.preds = append(q.preds, predicate{field: field, value: strings.ToLower(value)})
	}
	return q
}

// Empty reports whether the query has no terms, so it matches every packet.
func (q Query) Empty() bool { return len(q.preds) == 0 }

// Match reports whether p satisfies every term of the query.
func (q Query) Match(p Packet) bool {
	return q.MatchText(p, strings.ToLower(SearchText(p)))
}

// MatchText is Match using a caller-supplied, already-lowercased SearchText for
// the packet (a "haystack"), so a live filter can precompute it once per packet
// instead of rebuilding it on every keystroke. Scoped terms are still evaluated
// against the packet's specific field.
func (q Query) MatchText(p Packet, haystackLower string) bool {
	for _, pred := range q.preds {
		if pred.field == "" {
			if !strings.Contains(haystackLower, pred.value) {
				return false
			}
			continue
		}
		if !strings.Contains(strings.ToLower(scopedFields[pred.field](p)), pred.value) {
			return false
		}
	}
	return true
}

// isScopedField reports whether name is a recognized scoped-filter field.
func isScopedField(name string) bool {
	_, ok := scopedFields[name]
	return ok
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
