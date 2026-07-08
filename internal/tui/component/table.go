package component

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bapatchirag/harharbinks/internal/tui"
	"github.com/bapatchirag/harharbinks/internal/tui/keymap"
	"github.com/bapatchirag/harharbinks/internal/tui/msg"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// Column describes one column of a Table: a header title, a fixed display width
// in cells, and a function that renders a row value to that column's cell text.
type Column[T any] struct {
	Title  string
	Width  int
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
	b.WriteString(t.theme.Header().Render(pad(gutter+t.rowText(t.columnTitles), width)))

	vis := t.visible()
	end := clamp(t.offset+vis, 0, len(t.rows))
	for i := t.offset; i < end; i++ {
		b.WriteByte('\n')
		mark := gutter
		if i == t.cursor {
			mark = cursorGutter
		}
		line := pad(mark+t.rowText(func(c Column[T]) string { return c.Render(t.rows[i]) }), width)
		switch {
		case i == t.cursor && t.focused:
			b.WriteString(t.theme.Selected().Render(line))
		case i == t.cursor:
			b.WriteString(t.theme.Title().Render(line))
		default:
			b.WriteString(t.theme.Base().Render(line))
		}
	}
	for i := end - t.offset; i < vis; i++ {
		b.WriteByte('\n')
	}
	return b.String()
}

// columnTitles renders a column's header text (used with rowText).
func (t *Table[T]) columnTitles(c Column[T]) string { return c.Title }

// rowText assembles one row's cells, padding each to its column width and
// joining with a single space.
func (t *Table[T]) rowText(cell func(Column[T]) string) string {
	var sb strings.Builder
	for i, c := range t.columns {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(pad(cell(c), c.Width))
	}
	return sb.String()
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
)
