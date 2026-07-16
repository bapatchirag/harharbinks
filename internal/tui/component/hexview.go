package component

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bapatchirag/harharbinks/internal/tui"
	"github.com/bapatchirag/harharbinks/internal/tui/keymap"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// hexBytesPerRow is the number of bytes shown on each HexView line, split into
// two eight-byte groups in the classic hexdump layout.
const hexBytesPerRow = 16

// HexView is a scrollable hexadecimal dump of an arbitrary byte slice. Each line
// shows the classic three columns — the byte offset, the hex values in two
// eight-byte groups, and a printable-ASCII gutter. It can highlight a contiguous
// byte range so a companion view (such as a protocol-layer Tree) can point at
// the exact bytes it describes, scrolling that range into view. It is format
// agnostic: it knows nothing beyond the raw bytes it is handed.
type HexView struct {
	data    []byte
	offset  int // index of the first visible row
	cursor  int // byte index of the cursor
	hlStart int // first byte of the highlighted range
	hlLen   int // length of the highlighted range in bytes; 0 means none
	width   int
	height  int
	focused bool
	theme   theme.Theme
	keys    keymap.KeyMap
}

// NewHexView creates an empty HexView with the given theme and key bindings.
func NewHexView(th theme.Theme, km keymap.KeyMap) *HexView {
	return &HexView{theme: th, keys: km}
}

// SetData replaces the bytes shown, resetting the cursor and scroll position and
// clearing any highlight.
func (h *HexView) SetData(b []byte) {
	h.data = b
	h.offset = 0
	h.cursor = 0
	h.hlStart, h.hlLen = 0, 0
}

// Data returns the bytes currently displayed.
func (h *HexView) Data() []byte { return h.data }

// Cursor returns the index of the byte under the cursor.
func (h *HexView) Cursor() int { return h.cursor }

// SetCursor moves the cursor to byte i, clamping it into range and scrolling it
// into view. It is a no-op on empty data.
func (h *HexView) SetCursor(i int) {
	if len(h.data) == 0 {
		h.cursor = 0
		return
	}
	h.cursor = clamp(i, 0, len(h.data)-1)
	h.scrollTo(h.cursor / hexBytesPerRow)
}

// moveCursor shifts the cursor by d bytes.
func (h *HexView) moveCursor(d int) {
	if len(h.data) == 0 {
		return
	}
	h.SetCursor(h.cursor + d)
}

// SetHighlight marks the half-open byte range [start, start+length) as selected
// and scrolls it into view. A length of zero or less, or a start past the end of
// the data, clears the highlight instead. The range is clamped to the data
// bounds, so callers may pass raw layer offsets without pre-checking them.
func (h *HexView) SetHighlight(start, length int) {
	if length <= 0 || start >= len(h.data) {
		h.hlStart, h.hlLen = 0, 0
		return
	}
	if start < 0 {
		start = 0
	}
	if start+length > len(h.data) {
		length = len(h.data) - start
	}
	h.hlStart, h.hlLen = start, length
	h.scrollTo(start / hexBytesPerRow)
}

// ClearHighlight removes any highlighted range.
func (h *HexView) ClearHighlight() { h.hlStart, h.hlLen = 0, 0 }

// Highlight returns the current highlighted range as a start byte and a length;
// a length of zero means no range is highlighted.
func (h *HexView) Highlight() (start, length int) { return h.hlStart, h.hlLen }

// SetSize sets the render dimensions in cells.
func (h *HexView) SetSize(w, ht int) {
	h.width, h.height = w, ht
	h.clampOffset()
}

// SetTheme swaps the palette at runtime.
func (h *HexView) SetTheme(th theme.Theme) { h.theme = th }

// Focus gives the view input focus.
func (h *HexView) Focus() { h.focused = true }

// Blur removes input focus.
func (h *HexView) Blur() { h.focused = false }

// Focused reports whether the view has focus.
func (h *HexView) Focused() bool { return h.focused }

// Init implements tui.Component.
func (h *HexView) Init() tea.Cmd { return nil }

// Update moves the byte cursor while focused; the view scrolls to keep the
// cursor visible. The left and right keys step one byte, up and down move a full
// row, and page keys move a page. The externally set highlight range is left
// untouched, so a companion view can mark a span independently of the cursor.
func (h *HexView) Update(tmsg tea.Msg) tea.Cmd {
	if !h.focused {
		return nil
	}
	k, ok := tmsg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	switch {
	case key.Matches(k, h.keys.Left):
		h.moveCursor(-1)
	case key.Matches(k, h.keys.Right):
		h.moveCursor(1)
	case key.Matches(k, h.keys.Up):
		h.moveCursor(-hexBytesPerRow)
	case key.Matches(k, h.keys.Down):
		h.moveCursor(hexBytesPerRow)
	case key.Matches(k, h.keys.PageUp):
		h.moveCursor(-h.page() * hexBytesPerRow)
	case key.Matches(k, h.keys.PageDown):
		h.moveCursor(h.page() * hexBytesPerRow)
	case key.Matches(k, h.keys.Home):
		h.SetCursor(0)
	case key.Matches(k, h.keys.End):
		h.SetCursor(len(h.data) - 1)
	}
	return nil
}

// View renders the visible window of hex rows.
func (h *HexView) View() string {
	if len(h.data) == 0 {
		return h.theme.MutedText().Render("(no data)")
	}
	vis := h.visible()
	total := h.rows()
	end := clamp(h.offset+vis, 0, total)
	var b strings.Builder
	for r := h.offset; r < end; r++ {
		if r > h.offset {
			b.WriteByte('\n')
		}
		b.WriteString(h.renderRow(r))
	}
	return b.String()
}

// renderRow renders one 16-byte line: the offset, the two hex groups, and the
// printable-ASCII gutter. Each byte cell is styled individually so a highlight
// range paints only its bytes; missing bytes in a final short row become blanks
// that keep the columns aligned.
func (h *HexView) renderRow(r int) string {
	base := r * hexBytesPerRow
	muted := h.theme.MutedText()

	var b strings.Builder
	b.WriteString(muted.Render(fmt.Sprintf("%08x", base)))
	b.WriteString("  ")

	// Hex columns, in two eight-byte groups.
	for i := 0; i < hexBytesPerRow; i++ {
		if i > 0 {
			b.WriteByte(' ')
			if i == hexBytesPerRow/2 {
				b.WriteByte(' ')
			}
		}
		idx := base + i
		if idx >= len(h.data) {
			b.WriteString("  ")
			continue
		}
		b.WriteString(h.cellStyle(idx).Render(fmt.Sprintf("%02x", h.data[idx])))
	}

	b.WriteString("  ")

	// Printable-ASCII gutter, delimited by pipes.
	b.WriteString(muted.Render("|"))
	for i := 0; i < hexBytesPerRow; i++ {
		idx := base + i
		if idx >= len(h.data) {
			b.WriteByte(' ')
			continue
		}
		b.WriteString(h.cellStyle(idx).Render(printable(h.data[idx])))
	}
	b.WriteString(muted.Render("|"))
	return b.String()
}

// cellStyle returns the style for the byte at idx. The cursor byte is painted
// only while the view is focused (a solid highlight), so moving to another pane
// leaves no byte lingering under the cursor's highlight. The externally set
// highlight range is next in precedence, then the plain base style.
func (h *HexView) cellStyle(idx int) lipgloss.Style {
	switch {
	case h.focused && idx == h.cursor:
		return h.theme.Selected()
	case h.hlLen > 0 && idx >= h.hlStart && idx < h.hlStart+h.hlLen:
		return h.rangeStyle()
	default:
		return h.theme.Base()
	}
}

// rangeStyle styles the bytes of the externally set highlight range so a
// companion view (such as a layer Tree) can mark a span of bytes distinctly from
// the user's cursor. It uses the theme's highlight style, which each palette
// tunes for visibility.
func (h *HexView) rangeStyle() lipgloss.Style {
	return h.theme.Highlight()
}

// rows is the number of 16-byte lines the data spans.
func (h *HexView) rows() int {
	return (len(h.data) + hexBytesPerRow - 1) / hexBytesPerRow
}

// visible is the number of rows that fit; with no height set, all rows show.
func (h *HexView) visible() int {
	if h.height <= 0 {
		return h.rows()
	}
	return h.height
}

// maxOffset is the largest first-row index that still fills the window.
func (h *HexView) maxOffset() int {
	return max(0, h.rows()-h.visible())
}

func (h *HexView) page() int {
	if p := h.visible(); p > 1 {
		return p
	}
	return 1
}

func (h *HexView) clampOffset() {
	h.offset = clamp(h.offset, 0, h.maxOffset())
}

// scrollTo adjusts the offset so that row is within the visible window.
func (h *HexView) scrollTo(row int) {
	vis := h.visible()
	if row < h.offset {
		h.offset = row
	} else if row >= h.offset+vis {
		h.offset = row - vis + 1
	}
	h.clampOffset()
}

// printable maps a byte to its single-character ASCII representation, using a
// dot for any value outside the printable range.
func printable(b byte) string {
	if b >= 0x20 && b < 0x7f {
		return string(b)
	}
	return "."
}

var (
	_ tui.Component = (*HexView)(nil)
	_ tui.Sizeable  = (*HexView)(nil)
	_ tui.Focusable = (*HexView)(nil)
	_ tui.Themeable = (*HexView)(nil)
)
