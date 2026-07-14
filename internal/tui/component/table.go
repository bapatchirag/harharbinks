package component

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/bapatchirag/harharbinks/internal/tui"
	"github.com/bapatchirag/harharbinks/internal/tui/keymap"
	"github.com/bapatchirag/harharbinks/internal/tui/msg"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// Column describes one column of a Table: a header title, a fixed display width
// in cells, and a function that renders a row value to that column's cell text.
type Column[T any] struct {
	Title string
	Width int
	// Flex, when true, makes the column expand to share the table's leftover
	// width instead of using a fixed Width. Width is then only a fallback used
	// before the table has been sized. Typically the last column flexes so long
	// values (like URLs) get the remaining space.
	Flex bool
	// Color, when set, returns the foreground color for this column's cell in a
	// given row (an empty color means no override). It applies on unselected
	// rows; the cursor row keeps a uniform selection highlight.
	Color  func(T) lipgloss.Color
	Render func(T) string
}

// Table is a generic, scrollable, single-select table. It holds rows of any
// type T and renders them through per-column Render functions, so it stays
// unaware of what it displays. It emits msg.SelectedMsg when a row is activated.
type Table[T any] struct {
	columns []Column[T]
	rows    []T
	cursor  int
	offset  int
	width   int
	height  int
	focused bool
	theme   theme.Theme
	keys    keymap.KeyMap
}

// NewTable creates a Table with the given columns, theme, and key bindings.
func NewTable[T any](cols []Column[T], th theme.Theme, km keymap.KeyMap) *Table[T] {
	return &Table[T]{columns: cols, theme: th, keys: km}
}

// SetRows replaces the table's rows, keeping the cursor in range.
func (t *Table[T]) SetRows(rows []T) {
	t.rows = rows
	t.clampCursor()
}

// Rows returns the current rows.
func (t *Table[T]) Rows() []T { return t.rows }

// Cursor returns the index of the highlighted row.
func (t *Table[T]) Cursor() int { return t.cursor }

// SetCursor moves the highlight to row i, clamping it into range and scrolling
// it into view. It is a no-op selection change on an empty table.
func (t *Table[T]) SetCursor(i int) {
	t.cursor = i
	t.clampCursor()
}

// Selected returns the highlighted row and true, or the zero value and false
// when the table is empty.
func (t *Table[T]) Selected() (T, bool) {
	if t.cursor < 0 || t.cursor >= len(t.rows) {
		var zero T
		return zero, false
	}
	return t.rows[t.cursor], true
}

// SetSize sets the table's render dimensions in cells.
func (t *Table[T]) SetSize(w, h int) {
	t.width, t.height = w, h
	t.clampOffset()
}

// SetTheme swaps the table's palette at runtime, so a runtime theme change can
// recolor it live.
func (t *Table[T]) SetTheme(th theme.Theme) { t.theme = th }

// Focus gives the table input focus.
func (t *Table[T]) Focus() { t.focused = true }

// Blur removes input focus.
func (t *Table[T]) Blur() { t.focused = false }

// Focused reports whether the table has focus.
func (t *Table[T]) Focused() bool { return t.focused }

// Init implements tui.Component.
func (t *Table[T]) Init() tea.Cmd { return nil }

// Update handles navigation keys while focused and emits msg.SelectedMsg when a
// row is activated.
func (t *Table[T]) Update(tmsg tea.Msg) tea.Cmd {
	if !t.focused {
		return nil
	}
	k, ok := tmsg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	switch {
	case key.Matches(k, t.keys.Up):
		t.move(-1)
	case key.Matches(k, t.keys.Down):
		t.move(1)
	case key.Matches(k, t.keys.PageUp):
		t.move(-t.page())
	case key.Matches(k, t.keys.PageDown):
		t.move(t.page())
	case key.Matches(k, t.keys.Home):
		t.cursor = 0
		t.clampOffset()
	case key.Matches(k, t.keys.End):
		t.cursor = len(t.rows) - 1
		t.clampCursor()
	case key.Matches(k, t.keys.Enter):
		if len(t.rows) > 0 {
			idx := t.cursor
			return func() tea.Msg { return msg.SelectedMsg{Index: idx} }
		}
	}
	return nil
}

// View renders the header plus the visible window of rows.
func (t *Table[T]) View() string {
	width := t.width
	if width <= 0 {
		width = 80
	}
	var b strings.Builder
	widths := t.effectiveWidths()
	b.WriteString(t.theme.Header().Render(pad(gutter+t.rowText(t.columnTitles, widths), width)))

	vis := t.visible()
	end := clamp(t.offset+vis, 0, len(t.rows))
	for i := t.offset; i < end; i++ {
		b.WriteByte('\n')
		b.WriteString(t.renderRow(i, widths, width))
	}
	for i := end - t.offset; i < vis; i++ {
		b.WriteByte('\n')
	}
	return b.String()
}

// renderRow renders one data row. Each cell (and the gutter, separators, and
// trailing pad) is rendered with the row's style so a per-column Color composes
// with the highlight background instead of terminating it with a reset. The
// cursor row keeps a uniform selection style, ignoring per-column colors.
func (t *Table[T]) renderRow(i int, widths []int, width int) string {
	selected := i == t.cursor
	var rowStyle lipgloss.Style
	switch {
	case selected && t.focused:
		rowStyle = t.theme.Selected()
	case selected:
		rowStyle = t.theme.Title()
	default:
		rowStyle = t.theme.Base()
	}

	mark := gutter
	if selected {
		mark = cursorGutter
	}

	var sb strings.Builder
	sb.WriteString(rowStyle.Render(mark))
	for j, c := range t.columns {
		if j > 0 {
			sb.WriteString(rowStyle.Render(" "))
		}
		cell := pad(c.Render(t.rows[i]), widths[j])
		style := rowStyle
		if !selected && c.Color != nil {
			if col := c.Color(t.rows[i]); col != "" {
				style = rowStyle.Foreground(col)
			}
		}
		sb.WriteString(style.Render(cell))
	}

	line := sb.String()
	switch gap := width - ansi.StringWidth(line); {
	case gap > 0:
		line += rowStyle.Render(strings.Repeat(" ", gap))
	case gap < 0:
		line = ansi.Truncate(line, width, "…")
	}
	return line
}

// columnTitles renders a column's header text (used with rowText).
func (t *Table[T]) columnTitles(c Column[T]) string { return c.Title }

// rowText assembles one row's cells, padding each to its effective width and
// joining with a single space.
func (t *Table[T]) rowText(cell func(Column[T]) string, widths []int) string {
	var sb strings.Builder
	for i, c := range t.columns {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(pad(cell(c), widths[i]))
	}
	return sb.String()
}

// effectiveWidths returns the render width of each column. Fixed columns keep
// their declared Width; columns marked Flex evenly share the table's remaining
// width after the gutter, the single-space separators, and the fixed columns.
// When the table has no width yet, or the leftover space is too tight, the
// declared widths are returned unchanged.
func (t *Table[T]) effectiveWidths() []int {
	widths := make([]int, len(t.columns))
	fixed, flex := 0, 0
	for i, c := range t.columns {
		widths[i] = c.Width
		if c.Flex {
			flex++
		} else {
			fixed += c.Width
		}
	}
	if flex == 0 || t.width <= 0 {
		return widths
	}
	overhead := ansi.StringWidth(gutter) + (len(t.columns) - 1)
	remaining := t.width - overhead - fixed
	if remaining < flex {
		return widths
	}
	each, extra := remaining/flex, remaining%flex
	for i, c := range t.columns {
		if c.Flex {
			widths[i] = each
			if extra > 0 {
				widths[i]++
				extra--
			}
		}
	}
	return widths
}

func (t *Table[T]) move(d int) {
	if len(t.rows) == 0 {
		return
	}
	t.cursor += d
	t.clampCursor()
}

func (t *Table[T]) clampCursor() {
	if len(t.rows) == 0 {
		t.cursor, t.offset = 0, 0
		return
	}
	t.cursor = clamp(t.cursor, 0, len(t.rows)-1)
	t.clampOffset()
}

func (t *Table[T]) clampOffset() {
	vis := t.visible()
	if vis <= 0 {
		return
	}
	if t.cursor < t.offset {
		t.offset = t.cursor
	}
	if t.cursor >= t.offset+vis {
		t.offset = t.cursor - vis + 1
	}
	t.offset = clamp(t.offset, 0, max(0, len(t.rows)-vis))
}

// visible is the number of data rows that fit below the header.
func (t *Table[T]) visible() int {
	if t.height <= 1 {
		return max(0, len(t.rows))
	}
	return t.height - 1
}

func (t *Table[T]) page() int {
	if p := t.visible(); p > 1 {
		return p
	}
	return 1
}

var (
	_ tui.Component = (*Table[int])(nil)
	_ tui.Sizeable  = (*Table[int])(nil)
	_ tui.Focusable = (*Table[int])(nil)
	_ tui.Themeable = (*Table[int])(nil)
)
