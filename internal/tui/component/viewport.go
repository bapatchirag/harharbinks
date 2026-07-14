package component

import (
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bapatchirag/harharbinks/internal/tui"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// Viewport is a focusable, scrollable text pane. It wraps the bubbles viewport
// so detail inspectors and body previews get consistent scrolling and styling.
type Viewport struct {
	vp      viewport.Model
	focused bool
	theme   theme.Theme
}

// NewViewport creates an empty viewport.
func NewViewport(th theme.Theme) *Viewport {
	return &Viewport{vp: viewport.New(0, 0), theme: th}
}

// SetContent replaces the scrollable content.
func (v *Viewport) SetContent(s string) { v.vp.SetContent(s) }

// SetSize sets the viewport's render dimensions in cells.
func (v *Viewport) SetSize(w, h int) {
	v.vp.Width = w
	v.vp.Height = h
}

// SetTheme swaps the viewport's palette at runtime.
func (v *Viewport) SetTheme(th theme.Theme) { v.theme = th }

// ScrollPercent reports the current scroll position in the range [0,1].
func (v *Viewport) ScrollPercent() float64 { return v.vp.ScrollPercent() }

// Focus gives the viewport input focus.
func (v *Viewport) Focus() { v.focused = true }

// Blur removes input focus.
func (v *Viewport) Blur() { v.focused = false }

// Focused reports whether the viewport has focus.
func (v *Viewport) Focused() bool { return v.focused }

// Init implements tui.Component.
func (v *Viewport) Init() tea.Cmd { return nil }

// Update forwards messages to the underlying viewport while focused, enabling
// scrolling.
func (v *Viewport) Update(tmsg tea.Msg) tea.Cmd {
	if !v.focused {
		return nil
	}
	var cmd tea.Cmd
	v.vp, cmd = v.vp.Update(tmsg)
	return cmd
}

// View renders the visible slice of content.
func (v *Viewport) View() string { return v.vp.View() }

var (
	_ tui.Component = (*Viewport)(nil)
	_ tui.Sizeable  = (*Viewport)(nil)
	_ tui.Focusable = (*Viewport)(nil)
	_ tui.Themeable = (*Viewport)(nil)
)
