// Package app composes the format-agnostic TUI foundation and the reusable
// component library into full-screen application views. It owns the root Bubble
// Tea model and a small Screen router: each Screen is one full-window view, and
// the App renders shared chrome (a title header), routes global keys, tracks the
// terminal size, and delegates everything else to the active Screen.
//
// The App is the composition/adapter layer described in the architecture: it is
// the only place that knows about both the HAR domain and the generic
// components, bridging one into the other so the components stay reusable.
package app

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bapatchirag/harharbinks/internal/tui/keymap"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// productName is the human-facing brand shown in the title header. The installed
// binary is "hhb", but the product is always referred to as "harharbinks".
const productName = "harharbinks"

// Screen is one full-window view managed by the App router. Screens share the
// component lifecycle (Init/Update/View) plus sizing, and expose a short Title
// for the shared header. The App gives each Screen the whole terminal area below
// the title bar.
type Screen interface {
	// Init returns an optional command to run when the screen becomes active.
	Init() tea.Cmd
	// Update handles a message and returns any command to schedule. Screens
	// mutate in place, mirroring the component contract.
	Update(tea.Msg) tea.Cmd
	// View renders the screen to exactly the height last set via SetSize.
	View() string
	// SetSize sets the screen's render area in cells.
	SetSize(width, height int)
	// Title is a short label shown in the app header (e.g. the file name).
	Title() string
}

// App is the root Bubble Tea model. It renders a title header, routes the global
// quit key, tracks the terminal size, and delegates all other messages to the
// active Screen.
type App struct {
	theme  theme.Theme
	keys   keymap.KeyMap
	screen Screen
	width  int
	height int
}

// New returns an App displaying the given initial screen.
func New(screen Screen) *App {
	return &App{
		theme:  theme.Default(),
		keys:   keymap.Default(),
		screen: screen,
	}
}

// Init implements tea.Model.
func (a *App) Init() tea.Cmd { return a.screen.Init() }

// Update implements tea.Model. It handles resizing and the global quit key, then
// forwards every other message to the active screen.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = m.Width, m.Height
		a.layout()
		return a, nil
	case tea.KeyMsg:
		if key.Matches(m, a.keys.Quit) {
			return a, tea.Quit
		}
	}
	return a, a.screen.Update(msg)
}

// View implements tea.Model, stacking the title header over the active screen.
func (a *App) View() string {
	if a.width == 0 || a.height == 0 {
		return "initializing…"
	}
	title := a.theme.Title().Render(fmt.Sprintf(" %s · %s ", productName, a.screen.Title()))
	header := lipgloss.NewStyle().Width(a.width).Render(title)
	return lipgloss.JoinVertical(lipgloss.Left, header, a.screen.View())
}

// layout hands the active screen the terminal area below the one-line header.
func (a *App) layout() {
	bodyH := a.height - 1
	if bodyH < 1 {
		bodyH = 1
	}
	a.screen.SetSize(a.width, bodyH)
}

// Run starts a Bubble Tea program showing the given screen full-screen. It is
// the production entry point used by the CLI; tests drive the model directly.
func Run(screen Screen) error {
	_, err := tea.NewProgram(New(screen), tea.WithAltScreen()).Run()
	return err
}

var _ tea.Model = (*App)(nil)
