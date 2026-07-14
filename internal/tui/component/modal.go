package component

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bapatchirag/harharbinks/internal/tui"
	"github.com/bapatchirag/harharbinks/internal/tui/keymap"
	"github.com/bapatchirag/harharbinks/internal/tui/msg"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// Modal is a centered dialog box with a title and body. It renders only its box
// (callers composite it over a background with layout.Center or layout.Overlay).
// While visible it captures Enter/Esc to dismiss, emitting msg.DismissMsg.
type Modal struct {
	title   string
	body    string
	visible bool
	width   int
	height  int
	theme   theme.Theme
	keys    keymap.KeyMap
}

// NewModal creates a hidden modal.
func NewModal(th theme.Theme, km keymap.KeyMap) *Modal {
	return &Modal{theme: th, keys: km}
}

// Show displays the modal with the given title and body.
func (m *Modal) Show(title, body string) {
	m.title, m.body, m.visible = title, body, true
}

// Hide dismisses the modal.
func (m *Modal) Hide() { m.visible = false }

// Visible reports whether the modal is currently shown.
func (m *Modal) Visible() bool { return m.visible }

// SetSize records the screen dimensions used to bound the box width.
func (m *Modal) SetSize(w, h int) { m.width, m.height = w, h }

// SetTheme swaps the modal's palette at runtime.
func (m *Modal) SetTheme(th theme.Theme) { m.theme = th }

// Init implements tui.Component.
func (m *Modal) Init() tea.Cmd { return nil }

// Update dismisses the modal on Enter/Esc while visible, emitting msg.DismissMsg.
func (m *Modal) Update(tmsg tea.Msg) tea.Cmd {
	if !m.visible {
		return nil
	}
	if k, ok := tmsg.(tea.KeyMsg); ok && key.Matches(k, m.keys.Enter, m.keys.Back) {
		m.visible = false
		return func() tea.Msg { return msg.DismissMsg{} }
	}
	return nil
}

// View renders the modal box, or an empty string when hidden.
func (m *Modal) View() string {
	if !m.visible {
		return ""
	}
	boxWidth := 50
	if m.width > 0 && boxWidth > m.width-4 {
		boxWidth = m.width - 4
	}
	parts := []string{m.theme.Title().Render(m.title)}
	if m.body != "" {
		parts = append(parts, "", m.theme.Base().Render(m.body))
	}
	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return m.theme.BorderStyle(true).Padding(1, 2).Width(boxWidth).Render(content)
}

var (
	_ tui.Component = (*Modal)(nil)
	_ tui.Sizeable  = (*Modal)(nil)
	_ tui.Themeable = (*Modal)(nil)
)
