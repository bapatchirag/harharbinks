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

// Default returns the active harharbinks theme. Swap the returned palette to
// experiment; the named palettes are defined just below and share the same
// derived styles.
func Default() Theme {
	return Kanagawa()
}

// Gruvbox returns a Gruvbox-inspired palette — warm, retro earth tones on a soft
// dark base.
func Gruvbox() Theme {
	return Theme{
		Name:      "harharbinks-gruvbox",
		Primary:   lipgloss.Color("#fe8019"), // orange
		Secondary: lipgloss.Color("#8ec07c"), // aqua
		Fg:        lipgloss.Color("#ebdbb2"), // fg1
		Muted:     lipgloss.Color("#928374"), // gray
		Bg:        lipgloss.Color("#282828"), // bg0
		Subtle:    lipgloss.Color("#3c3836"), // bg1 (headers, bars)
		Border:    lipgloss.Color("#665c54"), // bg3
		Success:   lipgloss.Color("#b8bb26"), // green
		Warning:   lipgloss.Color("#fabd2f"), // yellow
		Error:     lipgloss.Color("#fb4934"), // red
		Info:      lipgloss.Color("#83a598"), // blue
	}
}

// Everforest returns an Everforest-inspired palette — soft, green-leaning forest
// tones at an even lower contrast than Gruvbox.
func Everforest() Theme {
	return Theme{
		Name:      "harharbinks-everforest",
		Primary:   lipgloss.Color("#e69875"), // orange
		Secondary: lipgloss.Color("#83c092"), // aqua
		Fg:        lipgloss.Color("#d3c6aa"), // fg
		Muted:     lipgloss.Color("#859289"), // grey
		Bg:        lipgloss.Color("#2d353b"), // bg0
		Subtle:    lipgloss.Color("#3d484d"), // bg2 (headers, bars)
		Border:    lipgloss.Color("#475258"), // bg5
		Success:   lipgloss.Color("#a7c080"), // green
		Warning:   lipgloss.Color("#dbbc7f"), // yellow
		Error:     lipgloss.Color("#e67e80"), // red
		Info:      lipgloss.Color("#7fbbb3"), // blue
	}
}

// Kanagawa returns a Kanagawa-inspired palette — muted Hokusai wave tones on a
// deep indigo-black base.
func Kanagawa() Theme {
	return Theme{
		Name:      "harharbinks-kanagawa",
		Primary:   lipgloss.Color("#ffa066"), // surimi orange
		Secondary: lipgloss.Color("#7aa89f"), // wave aqua
		Fg:        lipgloss.Color("#dcd7ba"), // fuji white
		Muted:     lipgloss.Color("#727169"), // fuji gray
		Bg:        lipgloss.Color("#1f1f28"), // sumi ink
		Subtle:    lipgloss.Color("#363646"), // sumi ink 3 (headers, bars)
		Border:    lipgloss.Color("#54546d"), // sumi ink 4
		Success:   lipgloss.Color("#98bb6c"), // spring green
		Warning:   lipgloss.Color("#e6c384"), // carp yellow
		Error:     lipgloss.Color("#c34043"), // autumn red
		Info:      lipgloss.Color("#7e9cd8"), // crystal blue
	}
}

// Zenburn returns a Zenburn-inspired palette — low-contrast warm greys on a
// lighter base for a soft, easy-on-the-eyes look.
func Zenburn() Theme {
	return Theme{
		Name:      "harharbinks-zenburn",
		Primary:   lipgloss.Color("#dfaf8f"), // warm tan
		Secondary: lipgloss.Color("#8cd0d3"), // cyan
		Fg:        lipgloss.Color("#dcdccc"), // fg
		Muted:     lipgloss.Color("#989890"), // grey
		Bg:        lipgloss.Color("#3f3f3f"), // bg
		Subtle:    lipgloss.Color("#4f4f4f"), // lighter bg (headers, bars)
		Border:    lipgloss.Color("#6f6f6f"), // border
		Success:   lipgloss.Color("#8fb28f"), // green
		Warning:   lipgloss.Color("#f0dfaf"), // yellow
		Error:     lipgloss.Color("#cc9393"), // red
		Info:      lipgloss.Color("#94bff3"), // blue
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

// Selected styles the highlighted row/item in a list or table. It paints the
// dark base over the accent so the highlight stays legible on a light accent.
func (t Theme) Selected() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Bg).Background(t.Primary).Bold(true)
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

// Key styles the label half of a key/value pair (field names, header keys) as a
// bold accent so keys stand out from their values.
func (t Theme) Key() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Secondary).Bold(true)
}

// TabInactive styles an unselected tab label: a colored foreground on the subtle
// bar background, so every tab reads as colored rather than greyed out.
func (t Theme) TabInactive() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Secondary).Background(t.Subtle)
}

// TabActive styles the selected tab while its pane is unfocused: the primary
// accent, bold, on the subtle bar background.
func (t Theme) TabActive() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Primary).Background(t.Subtle).Bold(true)
}
