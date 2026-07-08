package har

import (
	"fmt"
	"sort"
	"strings"
)

// FilterOptions selects a subset of entries. A zero-valued field means "match
// anything" for that dimension.
type FilterOptions struct {
	// Text matches as a case-insensitive substring of the request URL.
	Text string
	// Method matches the request method, case-insensitively.
	Method string
	// Status matches either an exact code ("200") or a class ("2xx").
	Status string
}

// Match reports whether e satisfies every set filter dimension.
func (o FilterOptions) Match(e Entry) bool {
	if o.Text != "" && !strings.Contains(strings.ToLower(e.Request.URL), strings.ToLower(o.Text)) {
		return false
	}
	if o.Method != "" && !strings.EqualFold(e.Request.Method, o.Method) {
		return false
	}
	if o.Status != "" && !matchStatus(e.Response.Status, o.Status) {
		return false
	}
	return true
}

// matchStatus supports an exact code (e.g. "404") or a class (e.g. "4xx").
func matchStatus(status int, want string) bool {
	want = strings.ToLower(strings.TrimSpace(want))
	if strings.HasSuffix(want, "xx") && len(want) == 3 {
		if want[0] < '1' || want[0] > '5' {
			return false
		}
		return status/100 == int(want[0]-'0')
	}
	return fmt.Sprintf("%d", status) == want
}

// Filter returns the entries that satisfy o, preserving their original order.
func Filter(entries []Entry, o FilterOptions) []Entry {
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if o.Match(e) {
			out = append(out, e)
		}
	}
	return out
}

// SortKey identifies a sortable entry field.
type SortKey string

const (
	SortNone   SortKey = ""
	SortTime   SortKey = "time"
	SortSize   SortKey = "size"
	SortStatus SortKey = "status"
	SortURL    SortKey = "url"
	SortMethod SortKey = "method"
)

// Less returns a comparison function for key, or nil for SortNone/unknown keys.
func Less(key SortKey) func(a, b Entry) bool {
	switch key {
	case SortTime:
		return func(a, b Entry) bool { return a.Time < b.Time }
	case SortSize:
		return func(a, b Entry) bool { return a.Response.Content.Size < b.Response.Content.Size }
	case SortStatus:
		return func(a, b Entry) bool { return a.Response.Status < b.Response.Status }
	case SortURL:
		return func(a, b Entry) bool { return a.Request.URL < b.Request.URL }
	case SortMethod:
		return func(a, b Entry) bool { return a.Request.Method < b.Request.Method }
	default:
		return nil
	}
}

// Sort orders entries in place by key. When desc is true the order is reversed.
// SortNone leaves the slice untouched.
func Sort(entries []Entry, key SortKey, desc bool) {
	less := Less(key)
	if less == nil {
		return
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if desc {
			return less(entries[j], entries[i])
		}
		return less(entries[i], entries[j])
	})
}

// ParseSort parses a sort specifier of the form "key" or "key:asc"/"key:desc".
// An empty string yields SortNone.
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
	case SortTime, SortSize, SortStatus, SortURL, SortMethod:
		return key, desc, nil
	default:
		return SortNone, false, fmt.Errorf("unknown sort key %q (use time|size|status|url|method)", name)
	}
}
