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
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bapatchirag/harharbinks/internal/tui/keymap"
	"github.com/bapatchirag/harharbinks/internal/tui/layout"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// productName is the human-facing brand shown in the title header. The installed
// binary is "hhb", but the product is always referred to as "harharbinks".
const productName = "harharbinks"

// SwitchScreenMsg asks the App router to replace the active screen with Screen.
// A screen requests a transition by returning the SwitchTo command; the router
// sizes and initializes the new screen when it handles this message.
type SwitchScreenMsg struct{ Screen Screen }

// SwitchTo returns a command that asks the App to make s the active screen. It
// is how one screen hands off to another (e.g. the file browser opening a
// selected capture in the viewer).
func SwitchTo(s Screen) tea.Cmd {
	return func() tea.Msg { return SwitchScreenMsg{Screen: s} }
}

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
	// SetTheme swaps the screen's palette at runtime, propagating it to every
	// component the screen owns so the in-app theme selector can recolor the
	// whole UI live.
	SetTheme(theme.Theme)
	// Title is a short label shown in the app header (e.g. the file name).
	Title() string
	// Help returns a multi-line description of the screen's key bindings, shown
	// in the app's help overlay.
	Help() string
	// CapturesInput reports whether the screen is currently consuming all
	// keystrokes (for example a text field is open). While true, the router
	// suspends its global key bindings and forwards every key to the screen, so
	// characters like "q" and "?" reach the field instead of quitting or opening
	// help.
	CapturesInput() bool
}

// App is the root Bubble Tea model. It renders a title header, routes the global
// quit key, tracks the terminal size, and delegates all other messages to the
// active Screen. It also owns a shared help overlay toggled with "?" and a theme
// selector toggled with "t".
type App struct {
	theme        theme.Theme
	keys         keymap.KeyMap
	screen       Screen
	themes       []theme.Theme
	themePrev    theme.Theme
	helpVisible  bool
	themeVisible bool
	themeCursor  int
	width        int
	height       int
}

// New returns an App displaying the given initial screen.
func New(screen Screen) *App {
	return &App{
		theme:  theme.Default(),
		keys:   keymap.Default(),
		screen: screen,
		themes: theme.Themes(),
	}
}

// Init implements tea.Model.
func (a *App) Init() tea.Cmd { return a.screen.Init() }

// Update implements tea.Model. It handles resizing, the help overlay, and the
// global quit key, then forwards every other message to the active screen.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = m.Width, m.Height
		a.layout()
		return a, nil
	case SwitchScreenMsg:
		// A screen handed off to another (e.g. the browser opened a capture).
		// Adopt it, drop any help overlay, size it to the current terminal, and
		// run its Init so it can start any commands (directory reads, etc.).
		a.screen = m.Screen
		a.helpVisible = false
		if a.width > 0 && a.height > 0 {
			a.layout()
		}
		return a, a.screen.Init()
	case tea.KeyMsg:
		// While the help overlay is open it captures input: any of its dismiss
		// keys close it, ctrl+c still quits, and everything else is swallowed.
		if a.helpVisible {
			if m.Type == tea.KeyCtrlC {
				return a, tea.Quit
			}
			if key.Matches(m, a.keys.Help, a.keys.Back, a.keys.Enter, a.keys.Quit) {
				a.helpVisible = false
			}
			return a, nil
		}
		// The theme selector overlay likewise captures input while open: it handles
		// its own navigation, apply, and close keys.
		if a.themeVisible {
			return a, a.handleThemeKey(m)
		}
		// While the active screen captures input (e.g. its search field is open),
		// forward every key to it. Only the hard interrupt still quits, so keys like
		// "q" and "?" are typed into the field rather than triggering global actions.
		if a.screen.CapturesInput() {
			if m.Type == tea.KeyCtrlC {
				return a, tea.Quit
			}
			return a, a.screen.Update(msg)
		}
		if key.Matches(m, a.keys.Quit) {
			return a, tea.Quit
		}
		if key.Matches(m, a.keys.Help) {
			a.helpVisible = true
			return a, nil
		}
		if key.Matches(m, a.keys.Theme) {
			a.openThemeSelector()
			return a, nil
		}
	}
	return a, a.screen.Update(msg)
}

// View implements tea.Model, stacking the title header over the active screen and
// compositing the help overlay on top when it is visible.
func (a *App) View() string {
	if a.width == 0 || a.height == 0 {
		return "initializing…"
	}
	title := a.theme.Title().Render(fmt.Sprintf(" %s · %s ", productName, a.screen.Title()))
	header := lipgloss.NewStyle().Width(a.width).Render(title)
	base := lipgloss.JoinVertical(lipgloss.Left, header, a.screen.View())
	switch {
	case a.helpVisible:
		return layout.Center(base, a.helpView())
	case a.themeVisible:
		return layout.Center(base, a.themeView())
	}
	return base
}

// helpView renders the centered help overlay box: the active screen's key
// descriptions above a footer of the app-level keys (theme, help, quit) that
// work on every screen. The box sizes itself to its content.
func (a *App) helpView() string {
	heading := a.theme.Title().Render(productName + " — keys")
	body := a.theme.Base().Render(a.screen.Help())
	footer := a.theme.MutedText().Render("t theme · ? help · q quit")
	content := lipgloss.JoinVertical(lipgloss.Left, heading, "", body, "", footer)
	return a.theme.BorderStyle(true).Padding(1, 2).Render(content)
}

// openThemeSelector shows the theme selector overlay, starting the highlight on
// the palette currently in use and remembering it so a cancel can restore it.
func (a *App) openThemeSelector() {
	if len(a.themes) == 0 {
		return
	}
	a.themeVisible = true
	a.themePrev = a.theme
	a.themeCursor = 0
	for i, t := range a.themes {
		if t.Name == a.theme.Name {
			a.themeCursor = i
			break
		}
	}
}

// handleThemeKey routes a key while the theme selector is open. Moving the
// highlight applies that palette immediately so the whole UI previews live;
// enter keeps the previewed palette and closes; esc (or t, or q) cancels,
// restoring the palette that was active when the selector opened. ctrl+c quits.
func (a *App) handleThemeKey(m tea.KeyMsg) tea.Cmd {
	switch {
	case m.Type == tea.KeyCtrlC:
		return tea.Quit
	case key.Matches(m, a.keys.Up):
		a.themeCursor = (a.themeCursor - 1 + len(a.themes)) % len(a.themes)
		a.applyTheme(a.themes[a.themeCursor])
	case key.Matches(m, a.keys.Down):
		a.themeCursor = (a.themeCursor + 1) % len(a.themes)
		a.applyTheme(a.themes[a.themeCursor])
	case key.Matches(m, a.keys.Enter):
		a.themeVisible = false
	case key.Matches(m, a.keys.Back, a.keys.Theme, a.keys.Quit):
		a.applyTheme(a.themePrev)
		a.themeVisible = false
	}
	return nil
}

// applyTheme adopts palette t for the app chrome and propagates it to the active
// screen (and through it every component), recoloring the whole UI live.
func (a *App) applyTheme(t theme.Theme) {
	a.theme = t
	a.screen.SetTheme(t)
}

// themeView renders the centered theme-selector overlay: the built-in palettes
// as a highlightable list, the active one highlighted, above a key hint.
func (a *App) themeView() string {
	heading := a.theme.Title().Render(productName + " — theme")
	width := 0
	for _, t := range a.themes {
		if w := len([]rune(t.DisplayName())); w > width {
			width = w
		}
	}
	lines := make([]string, len(a.themes))
	for i, t := range a.themes {
		label := padTo(t.DisplayName(), width)
		if i == a.themeCursor {
			lines[i] = a.theme.Selected().Render(" \u203a " + label + " ")
		} else {
			lines[i] = a.theme.Base().Render("   " + label + " ")
		}
	}
	hint := a.theme.MutedText().Render("\u2191/\u2193 preview \u00b7 enter apply \u00b7 esc cancel")
	content := lipgloss.JoinVertical(lipgloss.Left, heading, "", strings.Join(lines, "\n"), "", hint)
	return a.theme.BorderStyle(true).Padding(1, 2).Render(content)
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
