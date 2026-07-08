// Package theme centralizes the harharbinks color palette and derived lipgloss
// styles. A Theme is a plain, copyable value injected into components so that
// styling stays swappable and defined in exactly one place. It is format
// agnostic and shared by the HAR, PCAP, and audit screens alike.
package theme

import "github.com/charmbracelet/lipgloss"

// Theme is an injectable palette plus the small set of derived styles the
// component library needs. It is a value type: copy it freely and pass it into
// component constructors.
type Theme struct {
	Name string

	// Base palette.
	Primary   lipgloss.Color // brand / active accents
	Secondary lipgloss.Color // secondary accents
	Fg        lipgloss.Color // default foreground
	Muted     lipgloss.Color // de-emphasized foreground
	Bg        lipgloss.Color // default background
	Subtle    lipgloss.Color // subtle background (headers, bars)
	Border    lipgloss.Color // border lines

	// Semantic palette (severity, status).
	Success lipgloss.Color
	Warning lipgloss.Color
	Error   lipgloss.Color
	Info    lipgloss.Color
}

// Default returns the standard dark harharbinks theme.
func Default() Theme {
	return Theme{
		Name:      "harharbinks-dark",
		Primary:   lipgloss.Color("#7D56F4"),
		Secondary: lipgloss.Color("#43BF6D"),
		Fg:        lipgloss.Color("#DDDDDD"),
		Muted:     lipgloss.Color("#7A7A7A"),
		Bg:        lipgloss.Color("#1A1A1A"),
		Subtle:    lipgloss.Color("#2A2A2A"),
		Border:    lipgloss.Color("#4A4A4A"),
		Success:   lipgloss.Color("#43BF6D"),
		Warning:   lipgloss.Color("#D7AF00"),
		Error:     lipgloss.Color("#FF5F5F"),
		Info:      lipgloss.Color("#5FAFFF"),
	}
}

// Base is the default text style.
func (t Theme) Base() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Fg)
}

// Title styles a prominent heading.
func (t Theme) Title() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
}

// MutedText styles de-emphasized text.
func (t Theme) MutedText() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Muted)
}

// Selected styles the highlighted row/item in a list or table.
func (t Theme) Selected() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Fg).Background(t.Primary).Bold(true)
}

// Header styles a table header row or section header.
func (t Theme) Header() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Secondary).Background(t.Subtle).Bold(true)
}

// StatusBar styles a full-width status line.
func (t Theme) StatusBar() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Fg).Background(t.Subtle)
}

// BorderStyle styles a bordered panel; focused panels use the primary color.
func (t Theme) BorderStyle(focused bool) lipgloss.Style {
	c := t.Border
	if focused {
		c = t.Primary
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(c)
}
