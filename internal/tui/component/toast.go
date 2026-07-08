package component

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bapatchirag/harharbinks/internal/tui"
	"github.com/bapatchirag/harharbinks/internal/tui/msg"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// DefaultToastDuration is how long a toast stays visible before auto-dismissing.
const DefaultToastDuration = 3 * time.Second

// toastTimeoutMsg is the internal auto-dismiss signal. It carries the sequence
// number of the toast that scheduled it so a stale timer cannot hide a newer
// toast.
type toastTimeoutMsg struct{ seq int }

// Toast is a small, transient, color-coded notification (info/success/warning/
// error). It auto-dismisses after a delay and can also be dismissed early via
// msg.DismissMsg.
type Toast struct {
	text     string
	level    msg.Level
	visible  bool
	seq      int
	duration time.Duration
	theme    theme.Theme
}

// NewToast creates a hidden toast using the default duration.
func NewToast(th theme.Theme) *Toast {
	return &Toast{theme: th, duration: DefaultToastDuration}
}

// Show displays text at the given level and returns a command that will
// auto-dismiss this toast after the configured duration.
func (t *Toast) Show(text string, level msg.Level) tea.Cmd {
	t.text, t.level, t.visible = text, level, true
	t.seq++
	seq := t.seq
	return tea.Tick(t.duration, func(time.Time) tea.Msg { return toastTimeoutMsg{seq: seq} })
}

// Hide dismisses the toast immediately.
func (t *Toast) Hide() { t.visible = false }

// Visible reports whether the toast is currently shown.
func (t *Toast) Visible() bool { return t.visible }

// Init implements tui.Component.
func (t *Toast) Init() tea.Cmd { return nil }

// Update handles show requests (msg.ToastMsg), explicit dismissals
// (msg.DismissMsg), and the internal auto-dismiss timeout.
func (t *Toast) Update(tmsg tea.Msg) tea.Cmd {
	switch m := tmsg.(type) {
	case toastTimeoutMsg:
		if m.seq == t.seq {
			t.visible = false
		}
	case msg.ToastMsg:
		return t.Show(m.Text, m.Level)
	case msg.DismissMsg:
		t.visible = false
	}
	return nil
}

// View renders the toast box, or an empty string when hidden.
func (t *Toast) View() string {
	if !t.visible {
		return ""
	}
	return t.style().Render(t.text)
}

func (t *Toast) style() lipgloss.Style {
	c := t.theme.Info
	switch t.level {
	case msg.Success:
		c = t.theme.Success
	case msg.Warning:
		c = t.theme.Warning
	case msg.Error:
		c = t.theme.Error
	}
	return lipgloss.NewStyle().Foreground(t.theme.Bg).Background(c).Bold(true).Padding(0, 1)
}

var _ tui.Component = (*Toast)(nil)
