package har

import "testing"

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
