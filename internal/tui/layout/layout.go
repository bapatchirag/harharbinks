// Package layout provides format-agnostic composition helpers: stacking panes,
// wrapping content in borders, and compositing one rendered view over another
// (used for modal overlays and toasts). Helpers operate on already-rendered
// strings so they stay independent of any component or domain type. Overlay math
// is ANSI-aware so styled content composites at the correct cell positions.
package layout

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// SplitVertical stacks top over bottom, left-aligned. It is the basis for the
// list-on-top / detail-on-bottom screen layout.
func SplitVertical(top, bottom string) string {
	return lipgloss.JoinVertical(lipgloss.Left, top, bottom)
}

// SplitHorizontal places left and right side by side, top-aligned.
func SplitHorizontal(left, right string) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

// Border wraps content in the given border style.
func Border(content string, style lipgloss.Style) string {
	return style.Render(content)
}

// Place centers content within a width x height area (blank padding around it).
func Place(width, height int, content string) string {
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

// Overlay composites the foreground fg onto the background bg starting at cell
// column x and row y. It is ANSI-aware: styled background cells outside the
// foreground footprint are preserved, and the foreground is assumed opaque
// (typical for modal boxes and toasts). Rows/columns of fg that fall outside bg
// are clipped.
func Overlay(bg, fg string, x, y int) string {
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")

	for i, fgLine := range fgLines {
		row := y + i
		if row >= len(bgLines) {
			break
		}
		bgLine := bgLines[row]
		bgWidth := ansi.StringWidth(bgLine)

		// Left slice of the background: cells [0, x).
		left := ansi.Truncate(bgLine, x, "")
		if pad := x - ansi.StringWidth(left); pad > 0 {
			left += strings.Repeat(" ", pad)
		}

		// Right slice of the background: cells [x+fgWidth, end).
		fgWidth := ansi.StringWidth(fgLine)
		var right string
		if cut := x + fgWidth; cut < bgWidth {
			right = ansi.TruncateLeft(bgLine, cut, "")
		}

		bgLines[row] = left + fgLine + right
	}
	return strings.Join(bgLines, "\n")
}

// Center composites fg centered over bg, preserving bg's dimensions.
func Center(bg, fg string) string {
	bgW, bgH := lipgloss.Width(bg), lipgloss.Height(bg)
	fgW, fgH := lipgloss.Width(fg), lipgloss.Height(fg)
	return Overlay(bg, fg, (bgW-fgW)/2, (bgH-fgH)/2)
}
