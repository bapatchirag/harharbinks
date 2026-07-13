package har

import (
	"encoding/base64"
	"testing"
)

func TestFilterMethod(t *testing.T) {
	h := loadSample(t)
	if got := Filter(h.Log.Entries, FilterOptions{Method: "get"}); len(got) != 2 {
		t.Fatalf("GET entries = %d, want 2", len(got))
	}
}

func TestFilterStatusClassAndExact(t *testing.T) {
	h := loadSample(t)
	if got := Filter(h.Log.Entries, FilterOptions{Status: "2xx"}); len(got) != 3 {
		t.Errorf("2xx entries = %d, want 3", len(got))
	}
	if got := Filter(h.Log.Entries, FilterOptions{Status: "200"}); len(got) != 3 {
		t.Errorf("200 entries = %d, want 3", len(got))
	}
	if got := Filter(h.Log.Entries, FilterOptions{Status: "4xx"}); len(got) != 0 {
		t.Errorf("4xx entries = %d, want 0", len(got))
	}
}

func TestFilterTextCaseInsensitive(t *testing.T) {
	h := loadSample(t)
	got := Filter(h.Log.Entries, FilterOptions{Text: "LOGIN"})
	if len(got) != 1 {
		t.Fatalf("login entries = %d, want 1", len(got))
	}
	if got[0].Request.Method != "POST" {
		t.Errorf("method = %q, want POST", got[0].Request.Method)
	}
}

func TestSortSizeAscDesc(t *testing.T) {
	h := loadSample(t)
	entries := append([]Entry(nil), h.Log.Entries...) // sizes: 78, 33, 68

	Sort(entries, SortSize, false)
	if entries[0].Response.Content.Size != 33 || entries[2].Response.Content.Size != 78 {
		t.Errorf("asc = %d..%d, want 33..78",
			entries[0].Response.Content.Size, entries[2].Response.Content.Size)
	}

	Sort(entries, SortSize, true)
	if entries[0].Response.Content.Size != 78 {
		t.Errorf("desc first = %d, want 78", entries[0].Response.Content.Size)
	}
}

func TestSortNoneKeepsOrder(t *testing.T) {
	h := loadSample(t)
	entries := append([]Entry(nil), h.Log.Entries...)
	Sort(entries, SortNone, false)
	if entries[0].Request.Method != "GET" || entries[1].Request.Method != "POST" {
		t.Errorf("SortNone reordered entries")
	}
}

func TestParseSort(t *testing.T) {
	tests := []struct {
		in   string
		key  SortKey
		desc bool
		ok   bool
	}{
		{"", SortNone, false, true},
		{"size", SortSize, false, true},
		{"time:desc", SortTime, true, true},
		{"status:asc", SortStatus, false, true},
		{"bogus", SortNone, false, false},
		{"size:sideways", SortNone, false, false},
	}
	for _, tt := range tests {
		key, desc, err := ParseSort(tt.in)
		if tt.ok && err != nil {
			t.Errorf("ParseSort(%q) unexpected error: %v", tt.in, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("ParseSort(%q) expected error, got nil", tt.in)
		}
		if tt.ok && (key != tt.key || desc != tt.desc) {
			t.Errorf("ParseSort(%q) = (%q,%v), want (%q,%v)", tt.in, key, desc, tt.key, tt.desc)
		}
	}
}

func TestContainsMatchesAnyField(t *testing.T) {
	e := Entry{
		StartedDateTime: "2026-07-09T12:00:00Z",
		Connection:      "443",
		Request: Request{
			Method:   "POST",
			URL:      "https://api.example.com/login",
			Headers:  []NameValue{{Name: "Authorization", Value: "Bearer secrettoken"}},
			PostData: &PostData{MimeType: "application/json", Text: `{"user":"neo"}`},
		},
		Response: Response{
			Status: 201, StatusText: "Created",
			Content: Content{MimeType: "application/json", Text: `{"id":42,"role":"admin"}`},
		},
		ServerIPAddress: "93.184.216.34",
	}
	for _, term := range []string{
		"post", "LOGIN", "authorization", "bearer secrettoken", "neo",
		"201", "created", "admin", "role", "93.184.216.34", "443", "json", "2026-07-09",
	} {
		if !Contains(e, term) {
			t.Errorf("Contains(%q) = false, want true", term)
		}
	}
	if Contains(e, "notpresentanywhere") {
		t.Error(`Contains("notpresentanywhere") = true, want false`)
	}
	if !Contains(e, "") {
		t.Error("empty term should match everything")
	}
}

func TestContainsDecodesBody(t *testing.T) {
	e := Entry{Response: Response{Content: Content{
		Encoding: "base64",
		Text:     base64.StdEncoding.EncodeToString([]byte(`{"flag":"needle"}`)),
	}}}
	if !Contains(e, "needle") {
		t.Error("should match text inside a base64-decoded body")
	}
}

func TestContainsSkipsBinaryBody(t *testing.T) {
	e := Entry{Response: Response{Content: Content{
		MimeType: "image/png",
		Encoding: "base64",
		Text:     base64.StdEncoding.EncodeToString([]byte{0x89, 0x50, 0x4e, 0x47, 0x00, 0x01}),
	}}}
	if !Contains(e, "png") {
		t.Error("MIME type should remain searchable")
	}
	if Contains(e, "\x89PNG") {
		t.Error("binary body bytes should not be part of the search text")
	}
}

func TestQueryScopedFields(t *testing.T) {
	entries := []Entry{
		{
			Request:  Request{Method: "GET", URL: "https://api.example.com/users"},
			Response: Response{Status: 200, StatusText: "OK", Content: Content{MimeType: "application/json"}},
		},
		{
			Request:  Request{Method: "POST", URL: "https://api.example.com/login"},
			Response: Response{Status: 302, StatusText: "Found", Content: Content{MimeType: "text/html", Text: `{"postMessage":true}`}},
		},
		{
			Request:  Request{Method: "GET", URL: "https://cdn.example.com/app.js"},
			Response: Response{Status: 200, StatusText: "OK", Content: Content{MimeType: "application/javascript", Text: "window.postMessage(1)"}},
		},
	}
	count := func(raw string) int {
		q := ParseQuery(raw)
		n := 0
		for _, e := range entries {
			if q.Match(e) {
				n++
			}
		}
		return n
	}
	cases := []struct {
		raw  string
		want int
	}{
		{"method:post", 1},             // scoped to the method field, not bodies
		{"method:get", 2},              //
		{"post", 2},                    // free text: POST method + one postMessage body
		{"status:302", 1},              //
		{"host:cdn.example.com", 1},    //
		{"mime:json", 1},               // application/javascript does not contain "json"
		{"method:get url:cdn", 1},      // conjunctive: only the js file
		{"https://api.example.com", 2}, // unknown "https" field -> free text (colon-safe)
		{"", 3},                        // empty query matches everything
	}
	for _, c := range cases {
		if got := count(c.raw); got != c.want {
			t.Errorf("query %q matched %d entries, want %d", c.raw, got, c.want)
		}
	}
	if !ParseQuery("   ").Empty() {
		t.Error("blank query should be empty")
	}
	if ParseQuery("method:post").Empty() {
		t.Error("scoped query should not be empty")
	}
}
