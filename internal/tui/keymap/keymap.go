// Package keymap defines the shared key bindings used across harharbinks
// screens. Centralizing bindings keeps navigation consistent between the HAR,
// PCAP, and audit views and lets help text be generated from a single source.
// It implements the bubbles help.KeyMap interface so a help component can render
// these bindings directly.
package keymap

import "github.com/charmbracelet/bubbles/key"

// KeyMap is the set of bindings shared by list-like and detail components.
type KeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Home     key.Binding
	End      key.Binding
	Enter    key.Binding
	Back     key.Binding
	Tab      key.Binding
	ShiftTab key.Binding
	Search   key.Binding
	Sort     key.Binding
	SortRev  key.Binding
	Open     key.Binding
	Menu     key.Binding
	Theme    key.Binding
	Help     key.Binding
	Quit     key.Binding
}

// Default returns the standard harharbinks key bindings.
func Default() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "left"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "right"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "b"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "f"),
			key.WithHelp("pgdn", "page down"),
		),
		Home: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("home", "top"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("end", "bottom"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next pane"),
		),
		ShiftTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev pane"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		Sort: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "sort"),
		),
		SortRev: key.NewBinding(
			key.WithKeys("S"),
			key.WithHelp("S", "sort (reverse)"),
		),
		Open: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open file"),
		),
		Menu: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "export menu"),
		),
		Theme: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "theme"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

// ShortHelp implements bubbles help.KeyMap, returning the most important
// bindings for a single-line help view.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.Search, k.Quit}
}

// FullHelp implements bubbles help.KeyMap, returning all bindings grouped into
// columns for an expanded help view.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.PageUp, k.PageDown, k.Home, k.End},
		{k.Enter, k.Back, k.Tab, k.ShiftTab},
		{k.Search, k.Sort, k.SortRev, k.Menu, k.Theme, k.Help, k.Quit},
	}
}
