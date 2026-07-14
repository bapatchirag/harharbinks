// This file implements the HAR entry detail inspector: a tabbed view that maps a
// single har.Entry into grouped sections (Overview, Headers, Cookies, Payload,
// Response, Raw). It is the app-layer adapter that bridges the HAR domain into a
// scrollable, tabbed presentation; the generic components stay unaware of HAR.
// It mirrors the headless CLI's grouped detail output while adding tab
// navigation (left/right switch tabs, up/down scroll) and body formatting
// (base64 decode + JSON pretty-print).
package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/bapatchirag/harharbinks/internal/har"
	"github.com/bapatchirag/harharbinks/internal/tui/keymap"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// detailTab identifies one grouped section of the entry inspector.
type detailTab int

// The tabs, in display order.
const (
	tabOverview detailTab = iota
	tabHeaders
	tabCookies
	tabPayload
	tabResponse
	tabRaw
)

// tabNames are the tab labels shown in the tab bar, indexed by detailTab.
var tabNames = []string{"Overview", "Headers", "Cookies", "Payload", "Response", "Raw"}

// Detail is the tabbed inspector for a single HAR entry. It renders one grouped
// section at a time in a fixed-height pane, switches sections on arrow/tab keys,
// and scrolls long bodies. Content for the active tab is rendered to a cached
// slice of lines and windowed by a scroll offset, so the pane always occupies
// exactly its allotted height.
type Detail struct {
	theme    theme.Theme
	keys     keymap.KeyMap
	entry    har.Entry
	hasEntry bool
	active   detailTab
	scroll   int

	width   int
	height  int
	focused bool

	// content holds the rendered, width-wrapped lines of the active tab, rebuilt
	// when the entry, active tab, or width changes.
	content []string
}

// NewDetail returns an empty inspector styled with the given theme and keys.
func NewDetail(th theme.Theme, km keymap.KeyMap) *Detail {
	return &Detail{theme: th, keys: km}
}

// SetEntry shows e in the inspector, resetting the scroll to the top while
// keeping the active tab.
func (d *Detail) SetEntry(e har.Entry) {
	d.entry, d.hasEntry = e, true
	d.scroll = 0
	d.rebuild()
}

// Clear drops the current entry, showing the empty state.
func (d *Detail) Clear() {
	d.hasEntry = false
	d.scroll = 0
	d.content = nil
}

// Active reports the currently selected tab.
func (d *Detail) Active() detailTab { return d.active }

// Focus gives the inspector input focus, so its keys switch tabs and scroll.
func (d *Detail) Focus() { d.focused = true }

// Blur removes input focus.
func (d *Detail) Blur() { d.focused = false }

// Focused reports whether the inspector has focus.
func (d *Detail) Focused() bool { return d.focused }

// SetSize sets the inspector's render area; a width change re-wraps content.
func (d *Detail) SetSize(w, h int) {
	changed := w != d.width
	d.width, d.height = w, h
	if changed {
		d.rebuild()
	}
	d.clampScroll()
}

// SetTheme swaps the inspector's palette at runtime. The tab bar colors and the
// key/label styling are baked into the rendered content lines, so it re-renders
// the active tab to pick up the new palette.
func (d *Detail) SetTheme(th theme.Theme) {
	d.theme = th
	d.rebuild()
}

// HandleKey processes the inspector's keys while it holds focus: left/right
// switch tabs and the vertical navigation keys scroll the body. It reports
// whether the key was consumed. The enclosing screen only routes keys here when
// the inspector is focused.
func (d *Detail) HandleKey(k tea.KeyMsg) bool {
	switch {
	case key.Matches(k, d.keys.Left):
		d.PrevTab()
	case key.Matches(k, d.keys.Right):
		d.NextTab()
	case key.Matches(k, d.keys.Up):
		d.scrollBy(-1)
	case key.Matches(k, d.keys.Down):
		d.scrollBy(1)
	case key.Matches(k, d.keys.PageUp):
		d.scrollBy(-d.contentHeight())
	case key.Matches(k, d.keys.PageDown):
		d.scrollBy(d.contentHeight())
	case key.Matches(k, d.keys.Home):
		d.scroll = 0
	case key.Matches(k, d.keys.End):
		d.scroll = len(d.content)
		d.clampScroll()
	default:
		return false
	}
	return true
}

// NextTab moves to the next tab, wrapping around.
func (d *Detail) NextTab() { d.setTab((d.active + 1) % detailTab(len(tabNames))) }

// PrevTab moves to the previous tab, wrapping around.
func (d *Detail) PrevTab() {
	d.setTab((d.active + detailTab(len(tabNames)) - 1) % detailTab(len(tabNames)))
}

func (d *Detail) setTab(t detailTab) {
	if t == d.active {
		return
	}
	d.active = t
	d.scroll = 0
	d.rebuild()
}

// View renders the tab bar over the windowed content, fitted to the pane height.
func (d *Detail) View() string {
	if d.width == 0 || d.height == 0 {
		return ""
	}
	ch := d.contentHeight()
	if !d.hasEntry {
		empty := []string{d.theme.MutedText().Render("  No entries to display.")}
		return d.tabBar(d.width) + "\n" + fitLines(empty, d.width, ch)
	}
	window := d.content
	if d.scroll < len(window) {
		window = window[d.scroll:]
	} else {
		window = nil
	}
	return d.tabBar(d.width) + "\n" + fitLines(window, d.width, ch)
}

// tabBar renders the section labels as a full-width bar, highlighting the active
// tab (strongly when the inspector is focused). The bar background sets the
// detail section apart from the request list above it.
func (d *Detail) tabBar(width int) string {
	bar := d.theme.StatusBar()
	var b strings.Builder
	for i, name := range tabNames {
		label := " " + name + " "
		switch {
		case detailTab(i) == d.active && d.focused:
			b.WriteString(d.theme.Selected().Render(label))
		case detailTab(i) == d.active:
			b.WriteString(d.theme.TabActive().Render(label))
		default:
			b.WriteString(d.theme.TabInactive().Render(label))
		}
		if i < len(tabNames)-1 {
			b.WriteString(bar.Render("│"))
		}
	}
	out := b.String()
	switch w := ansi.StringWidth(out); {
	case w < width:
		out += bar.Render(strings.Repeat(" ", width-w))
	case w > width && width > 0:
		out = ansi.Truncate(out, width, "")
	}
	return out
}

// contentHeight is the number of body lines below the one-line tab bar.
func (d *Detail) contentHeight() int {
	if ch := d.height - 1; ch > 0 {
		return ch
	}
	return 1
}

func (d *Detail) scrollBy(n int) {
	d.scroll += n
	d.clampScroll()
}

func (d *Detail) clampScroll() {
	maxScroll := len(d.content) - d.contentHeight()
	if maxScroll < 0 {
		maxScroll = 0
	}
	if d.scroll > maxScroll {
		d.scroll = maxScroll
	}
	if d.scroll < 0 {
		d.scroll = 0
	}
}

// rebuild renders the active tab's content into d.content, wrapping each line to
// the pane width so long values are visible on continuation lines.
func (d *Detail) rebuild() {
	if !d.hasEntry {
		d.content = nil
		return
	}
	w := d.width
	if w <= 0 {
		w = 80
	}
	var lines []string
	switch d.active {
	case tabOverview:
		lines = d.overviewLines()
	case tabHeaders:
		lines = d.headersLines(w)
	case tabCookies:
		lines = d.cookiesLines(w)
	case tabPayload:
		lines = d.payloadLines(w)
	case tabResponse:
		lines = d.responseLines(w)
	case tabRaw:
		lines = d.rawLines(w)
	}
	d.content = wrapLines(lines, w)
	d.clampScroll()
}

// wrapLines wraps each line to width cells (ANSI-aware), so long values flow onto
// continuation lines instead of being truncated. Lines already within the width
// (including blanks) pass through unchanged.
func wrapLines(lines []string, width int) []string {
	if width <= 0 {
		return lines
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if ansi.StringWidth(line) <= width {
			out = append(out, line)
			continue
		}
		out = append(out, strings.Split(ansi.Hardwrap(line, width, false), "\n")...)
	}
	return out
}

// overviewLines summarizes the entry's request/response metadata.
func (d *Detail) overviewLines() []string {
	e := d.entry
	lines := []string{
		field(d.theme, "Method", methodText(d.theme, e.Request.Method)),
		field(d.theme, "URL", e.Request.URL),
		field(d.theme, "Status", fmt.Sprintf("%d %s", e.Response.Status, e.Response.StatusText)),
		field(d.theme, "HTTP", nonEmpty(e.Response.HTTPVersion, e.Request.HTTPVersion)),
	}
	if e.ServerIPAddress != "" {
		lines = append(lines, field(d.theme, "Server", e.ServerIPAddress))
	}
	if e.Connection != "" {
		lines = append(lines, field(d.theme, "Conn", e.Connection))
	}
	lines = append(lines,
		field(d.theme, "Time", humanMS(e.Time)),
		field(d.theme, "Size", humanSize(e.Response.Content.Size)),
		field(d.theme, "MIME", nonEmpty(e.Response.Content.MimeType, "-")),
	)
	if e.StartedDateTime != "" {
		lines = append(lines, field(d.theme, "Started", e.StartedDateTime))
	}
	return lines
}

// headersLines lists request then response headers in their recorded order.
func (d *Detail) headersLines(w int) []string {
	var lines []string
	lines = append(lines, subheader(d.theme, "Request Headers", w))
	lines = append(lines, pairLines(d.theme, d.entry.Request.Headers, ": ")...)
	lines = append(lines, "")
	lines = append(lines, subheader(d.theme, "Response Headers", w))
	lines = append(lines, pairLines(d.theme, d.entry.Response.Headers, ": ")...)
	return lines
}

// cookiesLines lists request then response cookies.
func (d *Detail) cookiesLines(w int) []string {
	var lines []string
	lines = append(lines, subheader(d.theme, "Request Cookies", w))
	lines = append(lines, cookieLines(d.theme, d.entry.Request.Cookies)...)
	lines = append(lines, "")
	lines = append(lines, subheader(d.theme, "Response Cookies", w))
	lines = append(lines, cookieLines(d.theme, d.entry.Response.Cookies)...)
	return lines
}

// payloadLines shows the query string and request body.
func (d *Detail) payloadLines(w int) []string {
	e := d.entry
	var lines []string
	lines = append(lines, subheader(d.theme, "Query String", w))
	lines = append(lines, pairLines(d.theme, e.Request.QueryString, " = ")...)
	lines = append(lines, "")

	pd := e.Request.PostData
	if pd == nil || (pd.Text == "" && len(pd.Params) == 0) {
		lines = append(lines, subheader(d.theme, "Request Body", w))
		lines = append(lines, "  (none)")
		return lines
	}
	lines = append(lines, subheader(d.theme, "Request Body ("+nonEmpty(pd.MimeType, "-")+")", w))
	if pd.Text != "" {
		lines = append(lines, bodyLines([]byte(pd.Text), pd.MimeType, "  ")...)
	} else {
		for _, p := range pd.Params {
			lines = append(lines, "  "+d.theme.Key().Render(p.Name)+" = "+p.Value)
		}
	}
	return lines
}

// responseLines shows the decoded, pretty-printed response body.
func (d *Detail) responseLines(w int) []string {
	c := d.entry.Response.Content
	lines := []string{subheader(d.theme, "Response Body ("+nonEmpty(c.MimeType, "-")+", "+humanSize(c.Size)+")", w)}
	body, err := c.Body()
	if err != nil {
		return append(lines, fmt.Sprintf("  <error decoding body: %v>", err))
	}
	return append(lines, bodyLines(body, c.MimeType, "  ")...)
}

// rawLines reconstructs the on-wire HTTP request and response text. HAR records
// no literal bytes, so header order/casing is a best-effort reconstruction.
func (d *Detail) rawLines(width int) []string {
	e := d.entry
	var lines []string

	target := requestTarget(e.Request.URL)
	lines = append(lines, fmt.Sprintf("%s %s %s", e.Request.Method, target, nonEmpty(e.Request.HTTPVersion, "HTTP/1.1")))
	if host := urlHost(e.Request.URL); host != "" && !hasHeader(e.Request.Headers, "host") {
		lines = append(lines, "Host: "+host)
	}
	for _, h := range e.Request.Headers {
		lines = append(lines, h.Name+": "+h.Value)
	}
	if pd := e.Request.PostData; pd != nil && pd.Text != "" {
		lines = append(lines, "")
		lines = append(lines, bodyLines([]byte(pd.Text), pd.MimeType, "")...)
	}

	rule := 40
	if width > 0 && width < rule {
		rule = width
	}
	lines = append(lines, "", d.theme.MutedText().Render(strings.Repeat("─", rule)), "")

	lines = append(lines, fmt.Sprintf("%s %d %s", nonEmpty(e.Response.HTTPVersion, "HTTP/1.1"), e.Response.Status, e.Response.StatusText))
	for _, h := range e.Response.Headers {
		lines = append(lines, h.Name+": "+h.Value)
	}
	if body, err := e.Response.Content.Body(); err == nil && len(body) > 0 {
		lines = append(lines, "")
		lines = append(lines, bodyLines(body, e.Response.Content.MimeType, "")...)
	}
	return lines
}

// subheader renders a full-width subsection header bar, mirroring the request
// list's column header so both sections read alike.
func subheader(th theme.Theme, title string, width int) string {
	return th.Header().Render(padTo(" "+title+" ", width))
}

// pairLines renders name/value pairs, or a muted "(none)" when empty.
func pairLines(th theme.Theme, pairs []har.NameValue, sep string) []string {
	if len(pairs) == 0 {
		return []string{"  (none)"}
	}
	var out []string
	for _, p := range pairs {
		out = append(out, kvLines(th, p.Name, sep, p.Value)...)
	}
	return out
}

// kvLines renders one name/value pair: the name is styled as a key, and a value
// that is a JSON object or array is pretty-printed across the following lines.
func kvLines(th theme.Theme, name, sep, value string) []string {
	key := th.Key().Render(name)
	if isJSONValue(value) {
		out := []string{"  " + key + strings.TrimRight(sep, " ")}
		for _, l := range strings.Split(prettyJSON(value), "\n") {
			out = append(out, "    "+l)
		}
		return out
	}
	return []string{"  " + key + sep + value}
}

// cookieLines renders cookies as "name = value" (name styled as a key) with
// trailing flags; a JSON value is pretty-printed.
func cookieLines(th theme.Theme, cookies []har.Cookie) []string {
	if len(cookies) == 0 {
		return []string{"  (none)"}
	}
	var out []string
	for _, c := range cookies {
		lines := kvLines(th, c.Name, " = ", c.Value)
		var flags []string
		if c.Secure {
			flags = append(flags, "Secure")
		}
		if c.HTTPOnly {
			flags = append(flags, "HttpOnly")
		}
		if len(flags) > 0 {
			lines[len(lines)-1] += "  [" + strings.Join(flags, ", ") + "]"
		}
		out = append(out, lines...)
	}
	return out
}

// bodyLines formats a body and splits it into prefixed display lines.
func bodyLines(body []byte, mime, prefix string) []string {
	return indentLines(formatBodyText(body, mime), prefix)
}

// formatBodyText decodes and pretty-prints a body: it summarizes binary data and
// indents JSON. It returns a possibly multi-line string.
func formatBodyText(body []byte, mime string) string {
	if len(body) == 0 {
		return "(empty)"
	}
	if !utf8.Valid(body) {
		return fmt.Sprintf("[binary data, %d bytes]", len(body))
	}
	text := string(body)
	if isJSONType(mime) || isJSONValue(text) {
		var buf bytes.Buffer
		if json.Indent(&buf, body, "", "  ") == nil {
			text = buf.String()
		}
	}
	return text
}

// indentLines splits s into lines, trimming a trailing newline and prefixing each.
func indentLines(s, prefix string) []string {
	s = strings.TrimRight(s, "\n")
	parts := strings.Split(s, "\n")
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = prefix + p
	}
	return out
}

// isJSONType reports whether a MIME type denotes JSON.
func isJSONType(mime string) bool {
	return strings.Contains(strings.ToLower(mime), "json")
}

// isJSONValue reports whether s is a JSON object or array, so it is worth
// pretty-printing across multiple lines (bare scalars are left inline).
func isJSONValue(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) == 0 || (s[0] != '{' && s[0] != '[') {
		return false
	}
	return json.Valid([]byte(s))
}

// prettyJSON returns s indented two spaces per level, or s unchanged when it is
// not valid JSON.
func prettyJSON(s string) string {
	var buf bytes.Buffer
	if json.Indent(&buf, []byte(s), "", "  ") != nil {
		return s
	}
	return buf.String()
}

// requestTarget derives the HTTP request target (path plus query) from a URL.
func requestTarget(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	target := u.Path
	if target == "" {
		target = "/"
	}
	if u.RawQuery != "" {
		target += "?" + u.RawQuery
	}
	return target
}

// urlHost returns the host component of a URL, or "" when it cannot be parsed.
func urlHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Host
}

// hasHeader reports whether headers contains a header named name (case-insensitive).
func hasHeader(headers []har.NameValue, name string) bool {
	for _, h := range headers {
		if strings.EqualFold(h.Name, name) {
			return true
		}
	}
	return false
}
