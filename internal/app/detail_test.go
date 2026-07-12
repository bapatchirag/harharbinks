package app

import (
	"encoding/base64"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bapatchirag/harharbinks/internal/har"
	"github.com/bapatchirag/harharbinks/internal/tui/keymap"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// richEntry is a fully-populated entry exercising every tab of the inspector.
func richEntry() har.Entry {
	return har.Entry{
		StartedDateTime: "2026-07-09T12:00:00Z",
		Time:            42,
		Request: har.Request{
			Method:      "POST",
			URL:         "https://api.example.com/login?next=/home",
			HTTPVersion: "HTTP/1.1",
			Headers: []har.NameValue{
				{Name: "Content-Type", Value: "application/json"},
				{Name: "Authorization", Value: "Bearer xyz"},
			},
			QueryString: []har.NameValue{{Name: "next", Value: "/home"}},
			Cookies:     []har.Cookie{{Name: "sid", Value: "abc123", Secure: true, HTTPOnly: true}},
			PostData: &har.PostData{
				MimeType: "application/json",
				Text:     `{"user":"neo","pw":"trinity"}`,
			},
		},
		Response: har.Response{
			Status:      200,
			StatusText:  "OK",
			HTTPVersion: "HTTP/1.1",
			Headers:     []har.NameValue{{Name: "Content-Type", Value: "application/json"}},
			Cookies:     []har.Cookie{{Name: "session", Value: "deadbeef"}},
			Content: har.Content{
				Size:     27,
				MimeType: "application/json",
				Text:     `{"ok":true,"token":"t0ken"}`,
			},
		},
		ServerIPAddress: "93.184.216.34",
		Connection:      "12345",
	}
}

// newDetail returns a sized inspector showing e, tall enough that content is not
// scrolled off (so tests assert on the full tab body).
func newDetail(e har.Entry) *Detail {
	d := NewDetail(theme.Default(), keymap.Default())
	d.SetSize(80, 30)
	d.SetEntry(e)
	return d
}

// viewTab selects tab t and returns the rendered inspector.
func viewTab(d *Detail, t detailTab) string {
	d.setTab(t)
	return d.View()
}

func TestDetailTabsContent(t *testing.T) {
	d := newDetail(richEntry())

	cases := []struct {
		tab  detailTab
		want []string
	}{
		{tabOverview, []string{"Method", "POST", "api.example.com/login", "200 OK", "93.184.216.34", "12345", "42ms"}},
		{tabHeaders, []string{"Request Headers", "Authorization: Bearer xyz", "Response Headers", "Content-Type: application/json"}},
		{tabCookies, []string{"Request Cookies", "sid = abc123", "Secure", "HttpOnly", "Response Cookies", "session = deadbeef"}},
		{tabPayload, []string{"Query String", "next = /home", "Request Body (application/json)", `"user": "neo"`}},
		{tabResponse, []string{"Response Body (application/json, 27B)", `"ok": true`, `"token": "t0ken"`}},
		{tabRaw, []string{"POST /login?next=/home HTTP/1.1", "Host: api.example.com", "Authorization: Bearer xyz", "HTTP/1.1 200 OK"}},
	}
	for _, c := range cases {
		out := viewTab(d, c.tab)
		for _, w := range c.want {
			if !strings.Contains(out, w) {
				t.Errorf("tab %s: output missing %q\n---\n%s", tabNames[c.tab], w, out)
			}
		}
	}
}

// TestDetailWrapsLongValue verifies a value longer than the pane wraps onto
// continuation lines rather than being truncated, so all of it stays visible.
func TestDetailWrapsLongValue(t *testing.T) {
	e := richEntry()
	e.Request.Headers = []har.NameValue{{Name: "X-Long", Value: strings.Repeat("z", 200)}}
	d := NewDetail(theme.Default(), keymap.Default())
	d.SetSize(40, 30)
	d.SetEntry(e)
	d.setTab(tabHeaders)

	if n := strings.Count(d.View(), "z"); n < 200 {
		t.Errorf("long header value should wrap with all 200 chars visible, got %d", n)
	}
}

func TestDetailTabWrap(t *testing.T) {
	d := newDetail(richEntry())
	if d.Active() != tabOverview {
		t.Fatalf("initial tab = %v, want Overview", d.Active())
	}
	d.PrevTab()
	if d.Active() != tabRaw {
		t.Errorf("PrevTab from Overview = %v, want Raw", d.Active())
	}
	d.NextTab()
	if d.Active() != tabOverview {
		t.Errorf("NextTab from Raw = %v, want Overview", d.Active())
	}
}

func TestDetailBase64Response(t *testing.T) {
	e := richEntry()
	e.Response.Content = har.Content{
		MimeType: "application/json",
		Encoding: "base64",
		Text:     base64.StdEncoding.EncodeToString([]byte(`{"a":1}`)),
	}
	d := newDetail(e)
	out := viewTab(d, tabResponse)
	if !strings.Contains(out, `"a": 1`) {
		t.Errorf("base64 response body not decoded/pretty-printed; got:\n%s", out)
	}
}

func TestDetailBinaryResponse(t *testing.T) {
	e := richEntry()
	e.Response.Content = har.Content{
		MimeType: "image/png",
		Encoding: "base64",
		Text:     base64.StdEncoding.EncodeToString([]byte{0x89, 0x50, 0x4e, 0x47, 0x00, 0x01}),
	}
	d := newDetail(e)
	out := viewTab(d, tabResponse)
	if !strings.Contains(out, "binary data") {
		t.Errorf("binary response should be summarized; got:\n%s", out)
	}
}

// TestDetailJSONHeaderValue verifies a header whose value is JSON is
// pretty-printed under the (styled) key.
func TestDetailJSONHeaderValue(t *testing.T) {
	e := richEntry()
	e.Request.Headers = append(e.Request.Headers, har.NameValue{Name: "X-Config", Value: `{"a":1,"b":2}`})
	d := newDetail(e)
	out := viewTab(d, tabHeaders)
	if !strings.Contains(out, `"a": 1`) || !strings.Contains(out, `"b": 2`) {
		t.Errorf("JSON header value should be pretty-printed; got:\n%s", out)
	}
}

// TestDetailJSONBodyAnyMIME verifies a JSON body is pretty-printed even when the
// MIME type does not advertise JSON.
func TestDetailJSONBodyAnyMIME(t *testing.T) {
	e := richEntry()
	e.Response.Content = har.Content{MimeType: "text/plain", Text: `{"k":1,"v":2}`}
	d := newDetail(e)
	out := viewTab(d, tabResponse)
	if !strings.Contains(out, `"k": 1`) {
		t.Errorf("JSON body with non-JSON MIME should still be pretty-printed; got:\n%s", out)
	}
}

func TestDetailScroll(t *testing.T) {
	e := richEntry()
	e.Response.Content = har.Content{
		MimeType: "text/plain",
		Text:     "FIRST\n" + strings.Repeat("mid\n", 40) + "LAST",
	}
	d := NewDetail(theme.Default(), keymap.Default())
	d.SetSize(40, 10) // small pane so the body cannot fit at once
	d.SetEntry(e)
	d.setTab(tabResponse)

	top := d.View()
	if !strings.Contains(top, "FIRST") || strings.Contains(top, "LAST") {
		t.Fatalf("unscrolled view should show FIRST and not LAST; got:\n%s", top)
	}
	d.scrollBy(1000) // clamp to bottom
	bottom := d.View()
	if !strings.Contains(bottom, "LAST") {
		t.Errorf("scrolled view should reveal LAST; got:\n%s", bottom)
	}
}

func TestDetailEmpty(t *testing.T) {
	d := NewDetail(theme.Default(), keymap.Default())
	d.SetSize(80, 10)
	if out := d.View(); !strings.Contains(out, "No entries") {
		t.Errorf("empty inspector should indicate no entries; got:\n%s", out)
	}
}

// TestMethodColor verifies HTTP methods map to distinct theme colors, are
// case-insensitive, and fall back to the default foreground when unknown.
func TestMethodColor(t *testing.T) {
	th := theme.Default()
	if methodColor(th, "GET") != th.Info {
		t.Error("GET should map to Info")
	}
	if methodColor(th, "post") != th.Success {
		t.Error("POST should map to Success (case-insensitive)")
	}
	if methodColor(th, "PUT") != th.Warning {
		t.Error("PUT should map to Warning")
	}
	if methodColor(th, "PATCH") != th.Secondary {
		t.Error("PATCH should map to Secondary")
	}
	if methodColor(th, "DELETE") != th.Error {
		t.Error("DELETE should map to Error")
	}
	if methodColor(th, "OPTIONS") != th.Muted {
		t.Error("OPTIONS should map to Muted")
	}
	if methodColor(th, "WEIRD") != th.Fg {
		t.Error("unknown method should fall back to Fg")
	}
}

func TestDetailHandleKey(t *testing.T) {
	d := newDetail(richEntry())

	consumed := []struct {
		name string
		key  tea.KeyMsg
	}{
		{"left", tea.KeyMsg{Type: tea.KeyLeft}},
		{"right", tea.KeyMsg{Type: tea.KeyRight}},
		{"up", tea.KeyMsg{Type: tea.KeyUp}},
		{"down", tea.KeyMsg{Type: tea.KeyDown}},
		{"pgup", tea.KeyMsg{Type: tea.KeyPgUp}},
		{"pgdown", tea.KeyMsg{Type: tea.KeyPgDown}},
		{"home", tea.KeyMsg{Type: tea.KeyHome}},
		{"end", tea.KeyMsg{Type: tea.KeyEnd}},
		{"k", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}},
		{"j", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}},
	}
	for _, c := range consumed {
		if !d.HandleKey(c.key) {
			t.Errorf("HandleKey(%s) = false, want consumed", c.name)
		}
	}

	passthrough := []struct {
		name string
		key  tea.KeyMsg
	}{
		{"tab", tea.KeyMsg{Type: tea.KeyTab}},
		{"shift+tab", tea.KeyMsg{Type: tea.KeyShiftTab}},
		{"enter", tea.KeyMsg{Type: tea.KeyEnter}},
		{"letter", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("z")}},
	}
	for _, c := range passthrough {
		if d.HandleKey(c.key) {
			t.Errorf("HandleKey(%s) = true, want not consumed", c.name)
		}
	}
}

func TestDetailTabSwitchResetsScroll(t *testing.T) {
	e := richEntry()
	e.Response.Content = har.Content{MimeType: "text/plain", Text: strings.Repeat("x\n", 60)}
	d := NewDetail(theme.Default(), keymap.Default())
	d.SetSize(40, 10)
	d.SetEntry(e)
	d.setTab(tabResponse)
	d.scrollBy(1000)
	if d.scroll == 0 {
		t.Fatal("precondition: expected a non-zero scroll after scrolling a long body")
	}
	d.setTab(tabOverview)
	if d.scroll != 0 {
		t.Errorf("switching tabs should reset scroll to 0, got %d", d.scroll)
	}
}
