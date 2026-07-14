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

// List is a generic, scrollable, single-select vertical list. Each item of type
// T is rendered to a single line by an injected render function, keeping the
// list agnostic of its contents. It emits msg.SelectedMsg when an item is
// activated.
type List[T any] struct {
	items   []T
	render  func(T) string
	title   string
	cursor  int
	offset  int
	width   int
	height  int
	focused bool
	theme   theme.Theme
	keys    keymap.KeyMap
}

// NewList creates a List that renders items with the given function.
func NewList[T any](render func(T) string, th theme.Theme, km keymap.KeyMap) *List[T] {
	return &List[T]{render: render, theme: th, keys: km}
}

// SetItems replaces the list's items, keeping the cursor in range.
func (l *List[T]) SetItems(items []T) {
	l.items = items
	l.clampCursor()
}

// Items returns the current items.
func (l *List[T]) Items() []T { return l.items }

// SetTitle sets an optional heading rendered above the items.
func (l *List[T]) SetTitle(s string) { l.title = s }

// Cursor returns the index of the highlighted item.
func (l *List[T]) Cursor() int { return l.cursor }

// Selected returns the highlighted item and true, or the zero value and false
// when the list is empty.
func (l *List[T]) Selected() (T, bool) {
	if l.cursor < 0 || l.cursor >= len(l.items) {
		var zero T
		return zero, false
	}
	return l.items[l.cursor], true
}

// SetSize sets the list's render dimensions in cells.
func (l *List[T]) SetSize(w, h int) {
	l.width, l.height = w, h
	l.clampOffset()
}

// SetTheme swaps the list's palette at runtime.
func (l *List[T]) SetTheme(th theme.Theme) { l.theme = th }

// Focus gives the list input focus.
func (l *List[T]) Focus() { l.focused = true }

// Blur removes input focus.
func (l *List[T]) Blur() { l.focused = false }

// Focused reports whether the list has focus.
func (l *List[T]) Focused() bool { return l.focused }

// Init implements tui.Component.
func (l *List[T]) Init() tea.Cmd { return nil }

// Update handles navigation keys while focused and emits msg.SelectedMsg when an
// item is activated.
func (l *List[T]) Update(tmsg tea.Msg) tea.Cmd {
	if !l.focused {
		return nil
	}
	k, ok := tmsg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	switch {
	case key.Matches(k, l.keys.Up):
		l.move(-1)
	case key.Matches(k, l.keys.Down):
		l.move(1)
	case key.Matches(k, l.keys.PageUp):
		l.move(-l.page())
	case key.Matches(k, l.keys.PageDown):
		l.move(l.page())
	case key.Matches(k, l.keys.Home):
		l.cursor = 0
		l.clampOffset()
	case key.Matches(k, l.keys.End):
		l.cursor = len(l.items) - 1
		l.clampCursor()
	case key.Matches(k, l.keys.Enter):
		if len(l.items) > 0 {
			idx := l.cursor
			return func() tea.Msg { return msg.SelectedMsg{Index: idx} }
		}
	}
	return nil
}

// View renders the optional title plus the visible window of items.
func (l *List[T]) View() string {
	width := l.width
	if width <= 0 {
		width = 40
	}
	var b strings.Builder
	if l.title != "" {
		b.WriteString(l.theme.Title().Render(pad(l.title, width)))
		b.WriteByte('\n')
	}
	vis := l.visible()
	end := clamp(l.offset+vis, 0, len(l.items))
	for i := l.offset; i < end; i++ {
		if i > l.offset {
			b.WriteByte('\n')
		}
		mark := gutter
		if i == l.cursor {
			mark = cursorGutter
		}
		line := pad(mark+l.render(l.items[i]), width)
		switch {
		case i == l.cursor && l.focused:
			b.WriteString(l.theme.Selected().Render(line))
		case i == l.cursor:
			b.WriteString(l.theme.Title().Render(line))
		default:
			b.WriteString(l.theme.Base().Render(line))
		}
	}
	return b.String()
}

func (l *List[T]) move(d int) {
	if len(l.items) == 0 {
		return
	}
	l.cursor += d
	l.clampCursor()
}

func (l *List[T]) clampCursor() {
	if len(l.items) == 0 {
		l.cursor, l.offset = 0, 0
		return
	}
	l.cursor = clamp(l.cursor, 0, len(l.items)-1)
	l.clampOffset()
}

func (l *List[T]) clampOffset() {
	vis := l.visible()
	if vis <= 0 {
		return
	}
	if l.cursor < l.offset {
		l.offset = l.cursor
	}
	if l.cursor >= l.offset+vis {
		l.offset = l.cursor - vis + 1
	}
	l.offset = clamp(l.offset, 0, max(0, len(l.items)-vis))
}

// visible is the number of items that fit, accounting for a title line.
func (l *List[T]) visible() int {
	h := l.height
	if l.title != "" {
		h--
	}
	if h <= 0 {
		return max(0, len(l.items))
	}
	return h
}

func (l *List[T]) page() int {
	if p := l.visible(); p > 1 {
		return p
	}
	return 1
}

var (
	_ tui.Component = (*List[int])(nil)
	_ tui.Sizeable  = (*List[int])(nil)
	_ tui.Focusable = (*List[int])(nil)
	_ tui.Themeable = (*List[int])(nil)
)
