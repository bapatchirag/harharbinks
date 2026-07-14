// This file implements the in-app configuration editor: a centered overlay that
// edits the persisted user settings (internal/config) live. Category headings run
// along a horizontal bar at the top (cycled with tab/shift+tab, and scrolled
// horizontally when they overflow the overlay), and the active category's fields
// are listed below it (moved with up/down, scrolling vertically); an enum field's
// value is cycled with left/right. Changes apply and persist immediately. It is
// the app-layer bridge between the config document and an interactive editor, so
// config stays a plain data/persistence package with no UI. Today the only
// category is Appearance (the Theme palette), but the model is data-driven:
// adding a field or category is a descriptor change here, and the tab bar, field
// list, scrolling, and editing adapt automatically.
package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/bapatchirag/harharbinks/internal/config"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// settingKind enumerates how a field is edited. Only settingEnum exists today; it
// is the extension point for text, bool, or path fields added later.
type settingKind int

const (
	// settingEnum is a value chosen from a fixed, ordered list; left/right cycle it.
	settingEnum settingKind = iota
)

// settingField describes one editable configuration value within a category tab.
// Values are read and written through get/set closures so the editor never needs
// to know the Config layout, and display maps a stored value to its shown label.
type settingField struct {
	label   string
	help    string
	kind    settingKind
	options []string                     // settingEnum: selectable values, in order
	display func(string) string          // stored value -> shown label (nil = identity)
	get     func(config.Config) string   // read the field's current value
	set     func(*config.Config, string) // write a new value
}

// settingsTab groups related fields under one category label; it renders as one
// entry in the category sidebar.
type settingsTab struct {
	name   string
	fields []settingField
}

// settingsModel is the state of the open configuration editor: the categories,
// which category and field are active, independent scroll offsets for the
// category sidebar and the field pane, and the working configuration being
// edited. The working copy is seeded from the app's live config when the overlay
// opens and mirrored back on every change.
type settingsModel struct {
	tabs      []settingsTab
	activeTab int
	cursor    int
	scroll    int // field pane scroll offset
	tabScroll int // category sidebar scroll offset
	cfg       config.Config
}

// newSettings builds the editor seeded with cfg's current values.
func newSettings(cfg config.Config) *settingsModel {
	return &settingsModel{tabs: settingsTabs(), cfg: cfg}
}

// settingsTabs is the source of truth for the editor's categories and fields.
// Extend it to expose a new setting: add a field descriptor (or a new tab) and
// the overlay gains an editable row with no other changes.
func settingsTabs() []settingsTab {
	return []settingsTab{
		{
			name: "Appearance",
			fields: []settingField{
				{
					label:   "Theme",
					help:    "Color palette applied across the whole interface.",
					kind:    settingEnum,
					options: themeNames(),
					display: themeDisplayName,
					get:     func(c config.Config) string { return c.Theme },
					set:     func(c *config.Config, v string) { c.Theme = v },
				},
			},
		},
	}
}

// themeNames lists the built-in palette Names in selector order, used as the
// Theme field's enum options.
func themeNames() []string {
	ts := theme.Themes()
	names := make([]string, len(ts))
	for i, t := range ts {
		names[i] = t.Name
	}
	return names
}

// themeDisplayName maps a stored palette Name to its human label (e.g.
// "harharbinks-gruvbox" -> "Gruvbox"), falling back to the raw name.
func themeDisplayName(name string) string {
	if t, ok := theme.ByName(name); ok {
		return t.DisplayName()
	}
	return name
}

// activeFields returns the fields of the currently selected tab.
func (s *settingsModel) activeFields() []settingField {
	if s.activeTab < 0 || s.activeTab >= len(s.tabs) {
		return nil
	}
	return s.tabs[s.activeTab].fields
}

// moveCursor moves the field highlight by delta within the active tab, wrapping.
func (s *settingsModel) moveCursor(delta int) {
	n := len(s.activeFields())
	if n == 0 {
		return
	}
	s.cursor = (s.cursor%n + delta + n) % n
}

// switchTab changes the active category by delta (wrapping) and resets the field
// highlight and scroll to the top of the new tab.
func (s *settingsModel) switchTab(delta int) {
	n := len(s.tabs)
	if n == 0 {
		return
	}
	s.activeTab = (s.activeTab%n + delta + n) % n
	s.cursor = 0
	s.scroll = 0
}

// cycleValue advances the highlighted enum field by delta (wrapping) and reports
// whether a value actually changed, so the caller can apply and persist it.
func (s *settingsModel) cycleValue(delta int) bool {
	fields := s.activeFields()
	if s.cursor < 0 || s.cursor >= len(fields) {
		return false
	}
	f := fields[s.cursor]
	if f.kind != settingEnum || len(f.options) == 0 {
		return false
	}
	cur := f.get(s.cfg)
	idx := 0
	for i, opt := range f.options {
		if opt == cur {
			idx = i
			break
		}
	}
	next := f.options[(idx+delta+len(f.options))%len(f.options)]
	if next == cur {
		return false
	}
	f.set(&s.cfg, next)
	return true
}

// view renders the editor as a bordered, centered box styled with th, sized to
// fit within w x h. Category headings run along a scrollable bar at the top and
// the active category's fields are listed below it; the bar scrolls horizontally
// and the field list vertically, each keeping its selection visible, so many
// categories or fields stay navigable.
func (s *settingsModel) view(th theme.Theme, w, h int) string {
	heading := th.Title().Render(productName + " \u2014 settings")

	// Constrain the tab bar to the overlay's inner width so it scrolls instead of
	// widening the box past the terminal; the 8 accounts for the centering margin,
	// border, and padding.
	innerW := w - 8
	if innerW < 20 {
		innerW = 20
	}
	tabs := s.categoryTabs(th, innerW)

	// Field-list height budget: leave room for the heading, the tab bar, three
	// blank spacers, the focused field's help line, the hint, plus border+padding.
	fieldsH := len(s.activeFields())
	if avail := h - 12; avail >= 1 && fieldsH > avail {
		fieldsH = avail
	}
	if fieldsH < 1 {
		fieldsH = 1
	}
	fields := strings.Join(s.fieldsPane(th, fieldsH), "\n")

	help := ""
	if f := s.activeFields(); s.cursor >= 0 && s.cursor < len(f) {
		help = th.MutedText().Render(f[s.cursor].help)
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		heading, "", tabs, "", fields, "", help, "", th.MutedText().Render(s.hint()))
	return th.BorderStyle(true).Padding(1, 2).Render(content)
}

// categoryTabs renders the category headings as a horizontal bar, highlighting
// the active one. When the headings do not all fit within maxWidth the bar
// scrolls horizontally (via tabScroll): it shows the widest run of consecutive
// tabs that fits while keeping the active tab visible, with a "\u2039"/"\u203a"
// marker on whichever side has more.
func (s *settingsModel) categoryTabs(th theme.Theme, maxWidth int) string {
	n := len(s.tabs)
	if n == 0 {
		return ""
	}
	w := make([]int, n)
	total := 0
	for i, t := range s.tabs {
		w[i] = ansi.StringWidth(" " + t.name + " ")
		total += w[i]
		if i < n-1 {
			total++ // "\u2502" separator between adjacent tabs
		}
	}
	if total <= maxWidth {
		s.tabScroll = 0
		return s.renderTabs(th, 0, n)
	}

	// Overflowing: window the tabs, reserving space for a marker on each side, and
	// slide the window forward until the active tab falls inside it.
	if s.tabScroll > s.activeTab {
		s.tabScroll = s.activeTab
	}
	avail := maxWidth - 4 // two 2-cell overflow markers
	if avail < 1 {
		avail = 1
	}
	for {
		start := s.tabScroll
		end, used := start, 0
		for end < n {
			need := w[end]
			if end > start {
				need++ // separator
			}
			if used+need > avail && end > start {
				break
			}
			used += need
			end++
		}
		if s.activeTab < end {
			return s.renderTabs(th, start, end)
		}
		s.tabScroll++
	}
}

// renderTabs draws tabs [start,end) as a horizontal bar, highlighting the active
// tab and adding a "\u2039"/"\u203a" overflow marker when tabs exist beyond either
// edge of the window.
func (s *settingsModel) renderTabs(th theme.Theme, start, end int) string {
	var b strings.Builder
	if start > 0 {
		b.WriteString(th.MutedText().Render("\u2039 ")) // ‹
	}
	for i := start; i < end; i++ {
		label := " " + s.tabs[i].name + " "
		if i == s.activeTab {
			b.WriteString(th.TabActive().Render(label))
		} else {
			b.WriteString(th.TabInactive().Render(label))
		}
		if i < end-1 {
			b.WriteString(th.StatusBar().Render("\u2502"))
		}
	}
	if end < len(s.tabs) {
		b.WriteString(th.MutedText().Render(" \u203a")) // ›
	}
	return b.String()
}

// fieldsPane renders the active category's fields as editable rows, highlighting
// the focused one across the pane width, windowed to height so a long field list
// scrolls (via scroll) while keeping the focused field visible.
func (s *settingsModel) fieldsPane(th theme.Theme, height int) []string {
	fields := s.activeFields()
	labelW, valueW := 0, 0
	for _, f := range fields {
		if lw := ansi.StringWidth(f.label); lw > labelW {
			labelW = lw
		}
		for _, opt := range f.options {
			if vw := ansi.StringWidth(display(f, opt)); vw > valueW {
				valueW = vw
			}
		}
	}

	// Render each field to a raw (unstyled) row first so we can size the pane to
	// the widest row, then style the focused row across that full width.
	raws := make([]string, len(fields))
	rowW := 0
	for i, f := range fields {
		val := display(f, f.get(s.cfg))
		raws[i] = settingRow(f.label, i == s.cursor, labelW, valueW, val)
		if rw := ansi.StringWidth(raws[i]); rw > rowW {
			rowW = rw
		}
	}
	rows := make([]string, len(fields))
	for i, raw := range raws {
		padded := raw + strings.Repeat(" ", rowW-ansi.StringWidth(raw))
		if i == s.cursor {
			rows[i] = th.Selected().Render(padded)
		} else {
			rows[i] = th.Base().Render(padded)
		}
	}
	return windowRows(rows, s.cursor, height, &s.scroll)
}

// windowRows returns a height-line slice of rows that keeps the row at cursor
// visible, advancing *scroll as needed and padding the result to exactly height
// lines so a pane keeps a stable size regardless of how many rows it holds.
func windowRows(rows []string, cursor, height int, scroll *int) []string {
	if height < 1 {
		height = 1
	}
	if *scroll > cursor {
		*scroll = cursor
	}
	if cursor >= *scroll+height {
		*scroll = cursor - height + 1
	}
	if *scroll < 0 {
		*scroll = 0
	}
	out := make([]string, height)
	for i := 0; i < height; i++ {
		if idx := *scroll + i; idx >= 0 && idx < len(rows) {
			out[i] = rows[idx]
		}
	}
	return out
}

// hint is the one-line key legend at the bottom of the overlay. The section key
// is only advertised when more than one category exists.
func (s *settingsModel) hint() string {
	parts := []string{"\u2191/\u2193 field", "\u2190/\u2192 change"}
	if len(s.tabs) > 1 {
		parts = append(parts, "tab category")
	}
	parts = append(parts, "esc close")
	return strings.Join(parts, " \u00b7 ")
}

// settingRow builds the unstyled text of one field row: a highlight caret, the
// padded label, and the current value flanked by guillemets when the row is
// focused (a hint that left/right change it).
func settingRow(label string, focused bool, labelW, valueW int, val string) string {
	caret := " "
	left, right := "  ", "  "
	if focused {
		caret = "\u203a" // ›
		left, right = "\u2039 ", " \u203a"
	}
	return " " + caret + " " + padTo(label, labelW) + "   " + left + padTo(val, valueW) + right + " "
}

// display renders a field value to its shown label, applying the field's display
// mapping when present.
func display(f settingField, v string) string {
	if f.display != nil {
		return f.display(v)
	}
	return v
}
