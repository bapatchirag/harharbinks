package har

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
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

// writePairs appends each pair's non-empty name and value (newline-terminated).
func writePairs(b *strings.Builder, pairs []NameValue) {
	for _, p := range pairs {
		if p.Name != "" {
			b.WriteString(p.Name)
			b.WriteByte('\n')
		}
		if p.Value != "" {
			b.WriteString(p.Value)
			b.WriteByte('\n')
		}
	}
}

// writeCookies appends each cookie's non-empty name and value.
func writeCookies(b *strings.Builder, cookies []Cookie) {
	for _, c := range cookies {
		if c.Name != "" {
			b.WriteString(c.Name)
			b.WriteByte('\n')
		}
		if c.Value != "" {
			b.WriteString(c.Value)
			b.WriteByte('\n')
		}
	}
}

// headersText is an entry's combined request and response header text.
func headersText(e Entry) string {
	var b strings.Builder
	writePairs(&b, e.Request.Headers)
	writePairs(&b, e.Response.Headers)
	return b.String()
}

// queryText is an entry's request query-string parameters.
func queryText(e Entry) string {
	var b strings.Builder
	writePairs(&b, e.Request.QueryString)
	return b.String()
}

// cookieText is an entry's combined request and response cookie text.
func cookieText(e Entry) string {
	var b strings.Builder
	writeCookies(&b, e.Request.Cookies)
	writeCookies(&b, e.Response.Cookies)
	return b.String()
}

// bodyText is an entry's request and (decoded) response bodies. Base64 bodies
// are decoded; binary (non-UTF-8) response bodies are omitted so their raw bytes
// do not pollute the text.
func bodyText(e Entry) string {
	var b strings.Builder
	if pd := e.Request.PostData; pd != nil {
		if pd.MimeType != "" {
			b.WriteString(pd.MimeType)
			b.WriteByte('\n')
		}
		if pd.Text != "" {
			b.WriteString(pd.Text)
			b.WriteByte('\n')
		}
		writePairs(&b, pd.Params)
	}
	if body, err := e.Response.Content.Body(); err == nil && utf8.Valid(body) {
		b.Write(body)
		b.WriteByte('\n')
	}
	return b.String()
}

// statusText is an entry's response status code and reason phrase (e.g. "404 Not
// Found"), so a status filter can match either.
func statusText(e Entry) string {
	if e.Response.Status == 0 {
		return e.Response.StatusText
	}
	return strconv.Itoa(e.Response.Status) + " " + e.Response.StatusText
}

// scopedFields maps a filter field name to the entry text it searches. It is the
// single source of truth for both the field:value filter and the set of
// recognized field names.
var scopedFields = map[string]func(Entry) string{
	"method": func(e Entry) string { return e.Request.Method },
	"url":    func(e Entry) string { return e.Request.URL },
	"host":   func(e Entry) string { return requestHost(e.Request.URL) },
	"status": statusText,
	"mime":   func(e Entry) string { return e.Response.Content.MimeType },
	"type":   func(e Entry) string { return e.Response.Content.MimeType },
	"header": headersText,
	"cookie": cookieText,
	"query":  queryText,
	"body":   bodyText,
	"server": func(e Entry) string { return e.ServerIPAddress },
	"ip":     func(e Entry) string { return e.ServerIPAddress },
	"conn":   func(e Entry) string { return e.Connection },
}

// SearchText returns a single string containing an entry's human-readable,
// searchable fields — method, URL, HTTP versions, status, headers, query
// params, cookies, MIME types, server IP, connection id, timestamp, and request
// and response bodies — so a free-text query can match "anything" in an entry.
func SearchText(e Entry) string {
	var b strings.Builder
	add := func(s string) {
		if s != "" {
			b.WriteString(s)
			b.WriteByte('\n')
		}
	}
	add(e.Request.Method)
	add(e.Request.URL)
	add(e.Request.HTTPVersion)
	if e.Response.Status != 0 {
		add(strconv.Itoa(e.Response.Status))
	}
	add(e.Response.StatusText)
	add(e.Response.HTTPVersion)
	add(e.Response.RedirectURL)
	add(e.ServerIPAddress)
	add(e.Connection)
	add(e.StartedDateTime)
	b.WriteString(headersText(e))
	b.WriteString(queryText(e))
	b.WriteString(cookieText(e))
	add(e.Response.Content.MimeType)
	b.WriteString(bodyText(e))
	return b.String()
}

// Contains reports whether term appears (case-insensitively) anywhere in the
// entry's searchable text (see SearchText). An empty term matches everything.
func Contains(e Entry, term string) bool {
	if term == "" {
		return true
	}
	return strings.Contains(strings.ToLower(SearchText(e)), strings.ToLower(term))
}

// predicate is one term of a parsed filter: free text (field == "") matched
// against the whole entry, or a value scoped to a single field.
type predicate struct {
	field string
	value string // lowercased
}

// Query is a parsed interactive filter. An entry matches only when it satisfies
// every term (terms are conjunctive). Build one with ParseQuery.
type Query struct {
	preds []predicate
}

// ParseQuery parses a raw filter string into a Query. Whitespace separates
// terms. A term of the form field:value is scoped to that field when field is a
// recognized name (method, url, host, status, mime/type, header, cookie, query,
// body, server/ip, conn); otherwise the whole term is free text, so URLs and
// other colon-bearing text still search across every field. Matching is
// case-insensitive.
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
			continue // e.g. a bare "method:" while still being typed
		}
		q.preds = append(q.preds, predicate{field: field, value: strings.ToLower(value)})
	}
	return q
}

// Empty reports whether the query has no terms, so it matches every entry.
func (q Query) Empty() bool { return len(q.preds) == 0 }

// Match reports whether e satisfies every term of the query.
func (q Query) Match(e Entry) bool {
	return q.MatchText(e, strings.ToLower(SearchText(e)))
}

// MatchText is Match using a caller-supplied, already-lowercased SearchText for
// the entry (a "haystack"), so a live filter can precompute it once per entry
// instead of rebuilding it on every keystroke. Scoped terms are still evaluated
// against the entry's specific field.
func (q Query) MatchText(e Entry, haystackLower string) bool {
	for _, p := range q.preds {
		if p.field == "" {
			if !strings.Contains(haystackLower, p.value) {
				return false
			}
			continue
		}
		if !strings.Contains(strings.ToLower(scopedFields[p.field](e)), p.value) {
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
