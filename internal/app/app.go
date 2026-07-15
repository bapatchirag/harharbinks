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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bapatchirag/harharbinks/internal/config"
	"github.com/bapatchirag/harharbinks/internal/tui/keymap"
	"github.com/bapatchirag/harharbinks/internal/tui/layout"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
	"github.com/bapatchirag/harharbinks/internal/update"
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
	// component the screen owns so the settings editor can recolor the whole UI
	// live.
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
// active Screen. It also owns a shared help overlay toggled with "?" and a
// settings editor toggled with "c".
type App struct {
	theme           theme.Theme
	keys            keymap.KeyMap
	screen          Screen
	cfg             config.Config
	persist         func(config.Config)
	settings        *settingsModel
	helpVisible     bool
	settingsVisible bool
	width           int
	height          int

	// Update-check state. version is the running build; updateEnabled gates the
	// opt-in launch check; updateNotice holds the newer version to advertise in the
	// header once a check has found one.
	version       string
	updateEnabled bool
	updateNotice  string
}

// Option customizes an App at construction. Options are applied by New in the
// order given, after the built-in defaults are set.
type Option func(*App)

// WithConfig seeds the app with a loaded configuration, adopting its persisted
// values (such as the theme palette) as the starting state so settings from a
// previous run take effect on the very first frame.
func WithConfig(c config.Config) Option {
	return func(a *App) {
		a.cfg = c
		if t, ok := theme.ByName(c.Theme); ok {
			a.theme = t
		}
	}
}

// WithConfigSaver registers a callback the app invokes whenever a configuration
// field changes in the settings editor, so the change is persisted immediately.
// It is optional: without a saver, edits apply for the session but do not
// survive it.
func WithConfigSaver(save func(config.Config)) Option {
	return func(a *App) { a.persist = save }
}

// WithUpdateCheck records the running version and whether opt-in update checks are
// enabled. When enabled for a release build, the app makes a single best-effort,
// day-cached request on launch and, if a newer release exists, shows a passive
// notice in the header. It never installs anything; the user updates explicitly
// with `hhb update`.
func WithUpdateCheck(version string, enabled bool) Option {
	return func(a *App) {
		a.version = version
		a.updateEnabled = enabled
	}
}

// New returns an App displaying the given initial screen. Options may override
// defaults such as the starting configuration and install a persistence hook.
func New(screen Screen, opts ...Option) *App {
	a := &App{
		cfg:    config.Default(),
		theme:  theme.Default(),
		keys:   keymap.Default(),
		screen: screen,
	}
	for _, opt := range opts {
		opt(a)
	}
	// Propagate the (possibly option-overridden) starting palette to the screen so
	// a restored theme colors the whole UI from the very first frame.
	a.screen.SetTheme(a.theme)
	return a
}

// Init implements tea.Model. It starts the active screen and, when opt-in update
// checks are enabled for a release build, kicks off a single best-effort check
// whose result may raise a passive header notice.
func (a *App) Init() tea.Cmd {
	cmds := []tea.Cmd{a.screen.Init()}
	if a.updateEnabled && update.IsReleaseBuild(a.version) {
		cmds = append(cmds, checkUpdateCmd(a.version))
	}
	return tea.Batch(cmds...)
}

// updateAvailableMsg is delivered when the launch update check finds a newer
// release; it carries the version to advertise in the header.
type updateAvailableMsg struct{ version string }

// checkUpdateCmd runs the opt-in update check off the UI path and yields an
// updateAvailableMsg only when a newer release is found. It is best-effort and
// silent: any error or an up-to-date result yields no message, and a short
// timeout keeps a slow or unreachable network from delaying anything.
func checkUpdateCmd(version string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		res, err := update.Check(ctx, version, false)
		if err != nil || !res.Newer {
			return nil
		}
		return updateAvailableMsg{version: res.Latest}
	}
}

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
	case updateAvailableMsg:
		// The launch check found a newer release; advertise it passively in the
		// header until the user quits. The app never installs it.
		a.updateNotice = m.version
		return a, nil
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
		// The settings editor overlay captures input while open: it navigates fields
		// and categories, edits values, and handles its own close keys.
		if a.settingsVisible {
			return a, a.handleSettingsKey(m)
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
		if key.Matches(m, a.keys.Config) {
			a.openSettings()
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
	base := lipgloss.JoinVertical(lipgloss.Left, a.header(), a.screen.View())
	switch {
	case a.helpVisible:
		return layout.Center(base, a.helpView())
	case a.settingsVisible:
		return layout.Center(base, a.settingsView())
	}
	return base
}

// header renders the one-line title bar: the product and active screen on the
// left and, when the launch update check found a newer release, a muted "update
// available" notice pinned to the right. The notice is dropped when the terminal
// is too narrow to fit both without overlap.
func (a *App) header() string {
	title := a.theme.Title().Render(fmt.Sprintf(" %s · %s ", productName, a.screen.Title()))
	if a.updateNotice != "" {
		notice := a.theme.MutedText().Render(fmt.Sprintf(" update %s available · run hhb update ", a.updateNotice))
		if gap := a.width - lipgloss.Width(title) - lipgloss.Width(notice); gap >= 1 {
			return lipgloss.NewStyle().Width(a.width).Render(title + strings.Repeat(" ", gap) + notice)
		}
	}
	return lipgloss.NewStyle().Width(a.width).Render(title)
}

// helpView renders the centered help overlay box: the active screen's key
// descriptions above a footer of the app-level keys (config, help, quit) that
// work on every screen. The box sizes itself to its content.
func (a *App) helpView() string {
	heading := a.theme.Title().Render(productName + " — keys")
	body := a.theme.Base().Render(a.screen.Help())
	footer := a.theme.MutedText().Render("c config · ? help · q quit")
	content := lipgloss.JoinVertical(lipgloss.Left, heading, "", body, "", footer)
	return a.theme.BorderStyle(true).Padding(1, 2).Render(content)
}

// applyTheme adopts palette t for the app chrome and propagates it to the active
// screen (and through it every component), recoloring the whole UI live.
func (a *App) applyTheme(t theme.Theme) {
	a.theme = t
	a.screen.SetTheme(t)
}

// openSettings shows the configuration editor overlay, seeded with the current
// settings so edits build on the live values.
func (a *App) openSettings() {
	a.settings = newSettings(a.cfg)
	a.settingsVisible = true
}

// handleSettingsKey routes a key while the settings editor is open: navigating
// fields (up/down) and categories (tab/shift+tab), cycling the highlighted
// field's value (left/right) — which applies and persists immediately — and
// closing the overlay (esc/c/q). ctrl+c still quits.
func (a *App) handleSettingsKey(m tea.KeyMsg) tea.Cmd {
	if m.Type == tea.KeyCtrlC {
		return tea.Quit
	}
	switch {
	case key.Matches(m, a.keys.Up):
		a.settings.moveCursor(-1)
	case key.Matches(m, a.keys.Down):
		a.settings.moveCursor(1)
	case key.Matches(m, a.keys.Left):
		if a.settings.cycleValue(-1) {
			a.applyConfig(a.settings.cfg)
		}
	case key.Matches(m, a.keys.Right):
		if a.settings.cycleValue(1) {
			a.applyConfig(a.settings.cfg)
		}
	case key.Matches(m, a.keys.Tab):
		a.settings.switchTab(1)
	case key.Matches(m, a.keys.ShiftTab):
		a.settings.switchTab(-1)
	case key.Matches(m, a.keys.Back, a.keys.Config, a.keys.Quit):
		a.settingsVisible = false
	}
	return nil
}

// applyConfig adopts the edited configuration c: it stores it, recolors the UI
// live when the theme changed, and persists the result so on-the-fly edits are
// saved immediately.
func (a *App) applyConfig(c config.Config) {
	prev := a.cfg
	a.cfg = c
	if c.Theme != prev.Theme {
		if t, ok := theme.ByName(c.Theme); ok {
			a.applyTheme(t)
		}
	}
	a.persistConfig()
}

// persistConfig writes the current configuration through the saver hook when one
// is installed. It is best-effort: persistence never blocks the UI.
func (a *App) persistConfig() {
	if a.persist != nil {
		a.persist(a.cfg)
	}
}

// settingsView renders the configuration editor overlay for the current frame.
func (a *App) settingsView() string {
	return a.settings.view(a.theme, a.width, a.height)
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
// the production entry point used by the CLI; tests drive the model directly. It
// restores the persisted configuration (theme palette and future settings) and
// persists any change made in the settings editor, so choices survive across
// launches. version is the running build, used only for the opt-in update check
// (see WithUpdateCheck); when checks are disabled it is otherwise unused.
func Run(screen Screen, version string) error {
	cfg := config.Load()
	opts := []Option{
		WithConfig(cfg),
		WithConfigSaver(func(c config.Config) { _ = config.Save(c) }),
		WithUpdateCheck(version, update.Enabled(cfg)),
	}
	_, err := tea.NewProgram(New(screen, opts...), tea.WithAltScreen()).Run()
	return err
}

var _ tea.Model = (*App)(nil)
