// Package tui defines the format-agnostic foundation shared by every
// harharbinks screen: the core component contracts. It is deliberately tiny and
// depends only on Bubble Tea so that the theme, keymap, layout, focus, and
// component packages can all build on a common vocabulary without importing one
// another. It knows nothing about HAR, PCAP, or audit data.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// Component is the minimal contract every reusable widget satisfies. It mirrors
// the Bubble Tea model lifecycle, but Update mutates the receiver in place and
// returns only a command, so concrete (and generic) component types stay usable
// without type assertions after each update.
type Component interface {
	// Init returns an optional command to run when the component starts.
	Init() tea.Cmd
	// Update handles a message, mutating the component, and returns any command
	// to schedule. Components communicate outward by returning commands that
	// emit decoupled messages (see package msg) rather than calling app logic.
	Update(tea.Msg) tea.Cmd
	// View renders the component to a string.
	View() string
}

// Sizeable is implemented by components whose dimensions are controlled by their
// parent layout. Sizes are in terminal cells.
type Sizeable interface {
	SetSize(width, height int)
}

// Focusable is implemented by components that can hold input focus. The focus
// package cycles a set of Focusable components, keeping exactly one focused.
type Focusable interface {
	Focus()
	Blur()
	Focused() bool
}

// Themeable is implemented by components whose palette can be swapped at runtime.
// The settings editor propagates a new theme.Theme to every component so the
// whole UI recolors live, mirroring how Sizeable lets the layout resize them.
type Themeable interface {
	SetTheme(theme.Theme)
}
