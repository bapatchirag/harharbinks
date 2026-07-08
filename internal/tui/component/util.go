// Package component holds the reusable, domain-agnostic TUI widgets shared by
// every harharbinks screen. Widgets are generic where it helps (Table[T],
// List[T]) so they can hold any row type, are styled via an injected theme, and
// communicate only through the decoupled messages in package msg. Nothing here
// imports the HAR, PCAP, audit, or application layers — a property enforced by
// the reusability guard test.
package component

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Selection gutters shown at the start of each row in list-like components, so
// the cursor is visible even when color is unavailable.
const (
	gutter       = "  "
	cursorGutter = "› "
)

// pad truncates s (with an ellipsis) or right-pads it with spaces so the result
// is exactly w display cells wide. Widths are measured ANSI-aware.
func pad(s string, w int) string {
	if w <= 0 {
		return ""
	}
	sw := ansi.StringWidth(s)
	if sw > w {
		return ansi.Truncate(s, w, "…")
	}
	return s + strings.Repeat(" ", w-sw)
}

// clamp constrains v to the inclusive range [lo, hi].
func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
