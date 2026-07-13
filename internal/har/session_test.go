package har

import "testing"

// ge builds a minimal GET entry to https://<host><path> with the given
// connection id and startedDateTime (either may be empty).
func ge(host, path, conn, started string) Entry {
	return Entry{
		StartedDateTime: started,
		Connection:      conn,
		Request:         Request{Method: "GET", URL: "https://" + host + path},
	}
}

// keys returns the member source indices of each session, for compact asserts.
func keys(sessions []Session) [][]int {
	out := make([][]int, len(sessions))
	for i, s := range sessions {
		out[i] = s.Indices
	}
	return out
}

func TestSessionsByConnection(t *testing.T) {
	entries := []Entry{
		ge("a.com", "/1", "1", ""),
		ge("a.com", "/2", "2", ""),
		ge("a.com", "/3", "1", ""), // same connection as entry 0
	}
	got := Sessions(entries)
	if len(got) != 2 {
		t.Fatalf("sessions = %d, want 2", len(got))
	}
	if got[0].Indices[0] != 0 || len(got[0].Indices) != 2 || got[0].Indices[1] != 2 {
		t.Errorf("first session indices = %v, want [0 2]", got[0].Indices)
	}
	if len(got[1].Indices) != 1 || got[1].Indices[0] != 1 {
		t.Errorf("second session indices = %v, want [1]", got[1].Indices)
	}
}

func TestSessionsFallbackByHost(t *testing.T) {
	entries := []Entry{
		ge("a.com", "/x", "", ""),
		ge("b.com", "/y", "", ""),
		ge("a.com", "/z", "", ""),
	}
	got := Sessions(entries)
	if len(got) != 2 {
		t.Fatalf("sessions = %d, want 2", len(got))
	}
	if got[0].Label != "a.com" {
		t.Errorf("first session label = %q, want a.com", got[0].Label)
	}
	if len(got[0].Indices) != 2 || got[0].Indices[1] != 2 {
		t.Errorf("a.com session indices = %v, want [0 2]", got[0].Indices)
	}
	if len(got[1].Indices) != 1 || got[1].Indices[0] != 1 {
		t.Errorf("b.com session indices = %v, want [1]", got[1].Indices)
	}
}

func TestSessionsTimeProximitySplit(t *testing.T) {
	entries := []Entry{
		ge("a.com", "/1", "", "2026-01-01T00:00:00Z"),
		ge("a.com", "/2", "", "2026-01-01T00:00:10Z"), // +10s, same session
		ge("a.com", "/3", "", "2026-01-01T00:01:00Z"), // +50s from /2, new session
	}
	got := keys(Sessions(entries))
	want := [][]int{{0, 1}, {2}}
	if len(got) != len(want) {
		t.Fatalf("sessions = %v, want %v", got, want)
	}
	for i := range want {
		if len(got[i]) != len(want[i]) {
			t.Fatalf("session %d = %v, want %v", i, got[i], want[i])
		}
		for j := range want[i] {
			if got[i][j] != want[i][j] {
				t.Fatalf("session %d = %v, want %v", i, got[i], want[i])
			}
		}
	}
}

// TestSessionsConnectionScopedByHost guards against connection-id reuse across
// different servers: the same id on two hosts must not be merged.
func TestSessionsConnectionScopedByHost(t *testing.T) {
	entries := []Entry{
		ge("a.com", "/x", "5", ""),
		ge("b.com", "/y", "5", ""),
	}
	if got := Sessions(entries); len(got) != 2 {
		t.Fatalf("sessions = %d, want 2 (distinct hosts)", len(got))
	}
}

func TestSessionAt(t *testing.T) {
	entries := []Entry{
		ge("a.com", "/x", "", ""),
		ge("b.com", "/y", "", ""),
		ge("a.com", "/z", "", ""),
	}
	s, pos, ok := SessionAt(entries, 2)
	if !ok {
		t.Fatal("SessionAt(2) not found")
	}
	if pos != 1 {
		t.Errorf("member position = %d, want 1", pos)
	}
	if len(s.Entries) != 2 || s.Entries[pos].Request.URL != "https://a.com/z" {
		t.Errorf("session member = %q, want https://a.com/z", s.Entries[pos].Request.URL)
	}
	if _, _, ok := SessionAt(entries, 99); ok {
		t.Error("SessionAt(99) should be out of range")
	}
}
