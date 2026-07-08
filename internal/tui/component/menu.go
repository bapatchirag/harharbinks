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

// MenuItem is one selectable command in a Menu. Key is an optional shortcut
// label shown to the user; Action is the opaque identifier emitted (via
// msg.MenuActionMsg) when the item is activated.
type MenuItem struct {
	Key    string
	Title  string
	Action string
}

// Menu is a compact, selectable command list — the basis for the in-app
// command menu / palette. It emits msg.MenuActionMsg when an item is activated.
type Menu struct {
	items   []MenuItem
	title   string
	cursor  int
	width   int
	height  int
	focused bool
	theme   theme.Theme
	keys    keymap.KeyMap
}

// NewMenu creates a Menu from the given items.
func NewMenu(items []MenuItem, th theme.Theme, km keymap.KeyMap) *Menu {
	return &Menu{items: items, theme: th, keys: km}
}

// SetItems replaces the menu's items, keeping the cursor in range.
func (mn *Menu) SetItems(items []MenuItem) {
	mn.items = items
	mn.cursor = clamp(mn.cursor, 0, max(0, len(items)-1))
}

// SetTitle sets an optional heading rendered above the items.
func (mn *Menu) SetTitle(s string) { mn.title = s }

// Cursor returns the index of the highlighted item.
func (mn *Menu) Cursor() int { return mn.cursor }

// Selected returns the highlighted item and true, or the zero value and false
// when the menu is empty.
func (mn *Menu) Selected() (MenuItem, bool) {
	if mn.cursor < 0 || mn.cursor >= len(mn.items) {
		return MenuItem{}, false
	}
	return mn.items[mn.cursor], true
}

// SetSize sets the menu's render dimensions in cells.
func (mn *Menu) SetSize(w, h int) { mn.width, mn.height = w, h }

// Focus gives the menu input focus.
func (mn *Menu) Focus() { mn.focused = true }

// Blur removes input focus.
func (mn *Menu) Blur() { mn.focused = false }

// Focused reports whether the menu has focus.
func (mn *Menu) Focused() bool { return mn.focused }

// Init implements tui.Component.
func (mn *Menu) Init() tea.Cmd { return nil }

// Update handles navigation keys while focused and emits msg.MenuActionMsg when
// an item is activated.
func (mn *Menu) Update(tmsg tea.Msg) tea.Cmd {
	if !mn.focused {
		return nil
	}
	k, ok := tmsg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	switch {
	case key.Matches(k, mn.keys.Up):
		if len(mn.items) > 0 {
			mn.cursor = (mn.cursor - 1 + len(mn.items)) % len(mn.items)
		}
	case key.Matches(k, mn.keys.Down):
		if len(mn.items) > 0 {
			mn.cursor = (mn.cursor + 1) % len(mn.items)
		}
	case key.Matches(k, mn.keys.Enter):
		if it, ok := mn.Selected(); ok {
			action := it.Action
			return func() tea.Msg { return msg.MenuActionMsg{Action: action} }
		}
	}
	return nil
}

// View renders the optional title plus the items, each as "key  title".
func (mn *Menu) View() string {
	width := mn.width
	if width <= 0 {
		width = 30
	}
	var b strings.Builder
	if mn.title != "" {
		b.WriteString(mn.theme.Title().Render(pad(mn.title, width)))
		b.WriteByte('\n')
	}
	for i, it := range mn.items {
		if i > 0 {
			b.WriteByte('\n')
		}
		label := it.Title
		if it.Key != "" {
			label = pad(it.Key, 6) + it.Title
		}
		mark := gutter
		if i == mn.cursor {
			mark = cursorGutter
		}
		line := pad(mark+label, width)
		if i == mn.cursor && mn.focused {
			b.WriteString(mn.theme.Selected().Render(line))
		} else {
			b.WriteString(mn.theme.Base().Render(line))
		}
	}
	return b.String()
}

var (
	_ tui.Component = (*Menu)(nil)
	_ tui.Sizeable  = (*Menu)(nil)
	_ tui.Focusable = (*Menu)(nil)
)
