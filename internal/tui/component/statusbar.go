package component

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bapatchirag/harharbinks/internal/tui"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// StatusBar is a single, full-width line with left, center, and right segments.
// It is passive (it never handles input) and is typically pinned to the bottom
// of a screen to show context, hints, or counts.
type StatusBar struct {
	left   string
	center string
	right  string
	width  int
	theme  theme.Theme
}

// NewStatusBar creates an empty status bar.
func NewStatusBar(th theme.Theme) *StatusBar {
	return &StatusBar{theme: th}
}

// SetLeft sets the left-aligned segment.
func (s *StatusBar) SetLeft(v string) { s.left = v }

// SetCenter sets the centered segment.
func (s *StatusBar) SetCenter(v string) { s.center = v }

// SetRight sets the right-aligned segment.
func (s *StatusBar) SetRight(v string) { s.right = v }

// SetSize sets the bar width; the height argument is ignored (always one line).
func (s *StatusBar) SetSize(w, _ int) { s.width = w }

// Init implements tui.Component.
func (s *StatusBar) Init() tea.Cmd { return nil }

// Update implements tui.Component; the status bar consumes no input.
func (s *StatusBar) Update(tea.Msg) tea.Cmd { return nil }

// View renders the three segments across the full width.
func (s *StatusBar) View() string {
	w := s.width
	if w <= 0 {
		w = 80
	}
	buf := []rune(strings.Repeat(" ", w))
	place := func(str string, at int) {
		for i, r := range []rune(str) {
			if at+i >= 0 && at+i < w {
				buf[at+i] = r
			}
		}
	}
	place(s.left, 0)
	place(s.center, (w-len([]rune(s.center)))/2)
	place(s.right, w-len([]rune(s.right)))
	return s.theme.StatusBar().Render(string(buf))
}

var (
	_ tui.Component = (*StatusBar)(nil)
	_ tui.Sizeable  = (*StatusBar)(nil)
)
