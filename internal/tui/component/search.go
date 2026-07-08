package component

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bapatchirag/harharbinks/internal/tui"
	"github.com/bapatchirag/harharbinks/internal/tui/msg"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// Search is a single-line text input for filtering. It wraps the bubbles
// textinput and emits msg.SearchMsg whenever its value changes, so screens can
// filter live as the user types.
type Search struct {
	input textinput.Model
	last  string
	theme theme.Theme
}

// NewSearch creates a search input with the given placeholder text.
func NewSearch(th theme.Theme, placeholder string) *Search {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Prompt = "/ "
	return &Search{input: ti, theme: th}
}

// Value returns the current query.
func (s *Search) Value() string { return s.input.Value() }

// SetValue replaces the current query without emitting a change message.
func (s *Search) SetValue(v string) {
	s.input.SetValue(v)
	s.last = v
}

// SetSize sets the input's text-field width (the height is ignored).
func (s *Search) SetSize(w, _ int) { s.input.Width = w }

// Focus gives the input focus and starts accepting keystrokes.
func (s *Search) Focus() { s.input.Focus() }

// Blur removes input focus.
func (s *Search) Blur() { s.input.Blur() }

// Focused reports whether the input has focus.
func (s *Search) Focused() bool { return s.input.Focused() }

// Init implements tui.Component.
func (s *Search) Init() tea.Cmd { return nil }

// Update forwards keystrokes to the input while focused and emits msg.SearchMsg
// when the query changes.
func (s *Search) Update(tmsg tea.Msg) tea.Cmd {
	if !s.input.Focused() {
		return nil
	}
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(tmsg)
	cmds := []tea.Cmd{cmd}
	if v := s.input.Value(); v != s.last {
		s.last = v
		cmds = append(cmds, func() tea.Msg { return msg.SearchMsg{Query: v} })
	}
	return tea.Batch(cmds...)
}

// View renders the input line.
func (s *Search) View() string { return s.input.View() }

var (
	_ tui.Component = (*Search)(nil)
	_ tui.Sizeable  = (*Search)(nil)
	_ tui.Focusable = (*Search)(nil)
)
