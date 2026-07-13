package har

import (
	"fmt"
	"net/url"
	"time"
)

// sessionGap is the maximum time between two consecutive connection-less
// exchanges to the same host for them to be treated as one session. A larger gap
// starts a new session, approximating a fresh keep-alive connection when the
// capture records no connection id.
const sessionGap = 30 * time.Second

// Session is a group of entries that belong together as one logical connection.
// Entries that record a connection id are grouped by it (scoped to their host,
// since a connection id is only meaningful per server); entries without one fall
// back to grouping by request host and time proximity. Sessions back the
// follow-session view, which shows every exchange sharing a connection.
type Session struct {
	// Key uniquely identifies the session within a single Sessions call. It is an
	// internal grouping token, not meant for display.
	Key string
	// Label is a human-friendly descriptor (host, and connection id when known).
	Label string
	// Entries are the member exchanges in their original capture order.
	Entries []Entry
	// Indices are the positions of Entries in the slice passed to Sessions, so a
	// caller can map a session member back to the source order.
	Indices []int
}

// Sessions groups entries into sessions, preserving the order in which each
// session first appears. Entries with a connection id are grouped by (host,
// connection id); the rest are grouped by host, split whenever the gap between
// consecutive same-host exchanges exceeds sessionGap.
func Sessions(entries []Entry) []Session {
	var order []string
	byKey := map[string]*Session{}

	// hostState tracks the current fallback bucket for a host so a large time gap
	// between consecutive connection-less exchanges starts a new session.
	type hostState struct {
		bucket  int
		last    time.Time
		hasLast bool
	}
	hosts := map[string]*hostState{}

	for i, e := range entries {
		host := requestHost(e.Request.URL)
		var key, label string
		if e.Connection != "" {
			key = "conn:" + host + "\x00" + e.Connection
			label = sessionLabel(host, e.Connection)
		} else {
			hs := hosts[host]
			if hs == nil {
				hs = &hostState{}
				hosts[host] = hs
			}
			if t, ok := parseHARTime(e.StartedDateTime); ok {
				if hs.hasLast && t.Sub(hs.last) > sessionGap {
					hs.bucket++
				}
				hs.last, hs.hasLast = t, true
			}
			key = fmt.Sprintf("host:%s#%d", host, hs.bucket)
			label = sessionLabel(host, "")
		}

		s := byKey[key]
		if s == nil {
			s = &Session{Key: key, Label: label}
			byKey[key] = s
			order = append(order, key)
		}
		s.Entries = append(s.Entries, e)
		s.Indices = append(s.Indices, i)
	}

	out := make([]Session, len(order))
	for i, k := range order {
		out[i] = *byKey[k]
	}
	return out
}

// SessionAt returns the session containing the entry at index i, together with
// that entry's position within the session's members. The bool is false when i
// is out of range.
func SessionAt(entries []Entry, i int) (Session, int, bool) {
	if i < 0 || i >= len(entries) {
		return Session{}, -1, false
	}
	for _, s := range Sessions(entries) {
		for pos, idx := range s.Indices {
			if idx == i {
				return s, pos, true
			}
		}
	}
	return Session{}, -1, false
}

// sessionLabel builds a display label from a host and optional connection id.
func sessionLabel(host, conn string) string {
	switch {
	case host == "" && conn == "":
		return "(unknown host)"
	case conn == "":
		return host
	case host == "":
		return "conn " + conn
	default:
		return host + " · conn " + conn
	}
}

// requestHost returns the host component of a request URL, or "" when it cannot
// be parsed.
func requestHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Host
}

// harTimeLayouts are the timestamp formats accepted for HAR startedDateTime,
// tried in order. HAR uses ISO 8601; browsers vary in fractional-second output.
var harTimeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.000Z07:00",
}

// parseHARTime parses a HAR startedDateTime, reporting whether it succeeded.
func parseHARTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range harTimeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
