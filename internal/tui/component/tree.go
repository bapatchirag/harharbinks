package component

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bapatchirag/harharbinks/internal/tui"
	"github.com/bapatchirag/harharbinks/internal/tui/keymap"
	"github.com/bapatchirag/harharbinks/internal/tui/msg"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// Tree row markers: the disclosure glyphs for expanded and collapsed parents,
// the two-cell pad that keeps leaf labels aligned with their expandable
// siblings, and the per-level indentation unit.
const (
	treeExpanded  = "▾ "
	treeCollapsed = "▸ "
	treeLeafPad   = "  "
	treeIndent    = "  "
)

// TreeNode is a single node in a Tree. It carries an arbitrary value of type T
// and its ordered children; a node with children can be expanded or collapsed.
// The Tree renders each node's value to a label through an injected function, so
// the node stays unaware of what its value represents.
type TreeNode[T any] struct {
	Value    T
	Children []*TreeNode[T]
	Expanded bool
}

// Leaf constructs a childless TreeNode holding v.
func Leaf[T any](v T) *TreeNode[T] {
	return &TreeNode[T]{Value: v}
}

// Branch constructs a TreeNode holding v with the given children. It starts
// expanded so the subtree is visible by default.
func Branch[T any](v T, children ...*TreeNode[T]) *TreeNode[T] {
	return &TreeNode[T]{Value: v, Children: children, Expanded: true}
}

// treeRow is one node as it appears in the current flattened (visible) order,
// together with its depth for indentation.
type treeRow[T any] struct {
	node  *TreeNode[T]
	depth int
}

// Tree is a generic, scrollable, collapsible tree with single-node selection.
// Nodes hold any type T and render to a label via an injected function, so the
// tree stays agnostic of its contents. The arrow keys move and expand/collapse
// nodes; pressing enter toggles a parent and emits msg.SelectedMsg carrying the
// node's position in the current flattened order, so a companion view — such as
// a HexView — can react to the selection.
type Tree[T any] struct {
	roots   []*TreeNode[T]
	render  func(T) string
	flat    []treeRow[T]
	cursor  int
	offset  int
	width   int
	height  int
	focused bool
	theme   theme.Theme
	keys    keymap.KeyMap
}

// NewTree creates a Tree that renders each node's value with the given function.
func NewTree[T any](render func(T) string, th theme.Theme, km keymap.KeyMap) *Tree[T] {
	return &Tree[T]{render: render, theme: th, keys: km}
}

// SetRoots replaces the tree's top-level nodes, resetting the cursor and scroll.
func (t *Tree[T]) SetRoots(roots []*TreeNode[T]) {
	t.roots = roots
	t.cursor, t.offset = 0, 0
	t.rebuild()
}

// Roots returns the tree's top-level nodes.
func (t *Tree[T]) Roots() []*TreeNode[T] { return t.roots }

// Cursor returns the index of the highlighted node in the current flattened
// (visible) order.
func (t *Tree[T]) Cursor() int { return t.cursor }

// SetCursor moves the highlight to the flattened index i, clamping it into range
// and scrolling it into view.
func (t *Tree[T]) SetCursor(i int) {
	t.cursor = i
	t.clampCursor()
}

// Selected returns the highlighted node's value and true, or the zero value and
// false when the tree is empty.
func (t *Tree[T]) Selected() (T, bool) {
	if n, ok := t.SelectedNode(); ok {
		return n.Value, true
	}
	var zero T
	return zero, false
}

// SelectedNode returns the highlighted node and true, or nil and false when the
// tree is empty.
func (t *Tree[T]) SelectedNode() (*TreeNode[T], bool) {
	if t.cursor < 0 || t.cursor >= len(t.flat) {
		return nil, false
	}
	return t.flat[t.cursor].node, true
}

// SetSize sets the tree's render dimensions in cells.
func (t *Tree[T]) SetSize(w, h int) {
	t.width, t.height = w, h
	t.clampOffset()
}

// SetTheme swaps the tree's palette at runtime.
func (t *Tree[T]) SetTheme(th theme.Theme) { t.theme = th }

// Focus gives the tree input focus.
func (t *Tree[T]) Focus() { t.focused = true }

// Blur removes input focus.
func (t *Tree[T]) Blur() { t.focused = false }

// Focused reports whether the tree has focus.
func (t *Tree[T]) Focused() bool { return t.focused }

// Init implements tui.Component.
func (t *Tree[T]) Init() tea.Cmd { return nil }

// Update handles navigation, expansion, and selection keys while focused. The
// left and right keys collapse and expand the current node (falling back to
// moving to the parent or first child), and enter toggles a parent while
// emitting msg.SelectedMsg.
func (t *Tree[T]) Update(tmsg tea.Msg) tea.Cmd {
	if !t.focused {
		return nil
	}
	k, ok := tmsg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	switch {
	case key.Matches(k, t.keys.Up):
		t.move(-1)
	case key.Matches(k, t.keys.Down):
		t.move(1)
	case key.Matches(k, t.keys.PageUp):
		t.move(-t.page())
	case key.Matches(k, t.keys.PageDown):
		t.move(t.page())
	case key.Matches(k, t.keys.Home):
		t.cursor = 0
		t.clampOffset()
	case key.Matches(k, t.keys.End):
		t.cursor = len(t.flat) - 1
		t.clampCursor()
	case key.Matches(k, t.keys.Right):
		t.expand()
	case key.Matches(k, t.keys.Left):
		t.collapse()
	case key.Matches(k, t.keys.Enter):
		return t.toggleSelect()
	}
	return nil
}

// View renders the visible window of flattened nodes, each indented by depth and
// prefixed with a disclosure glyph for parents.
func (t *Tree[T]) View() string {
	width := t.width
	if width <= 0 {
		width = 40
	}
	if t.flat == nil {
		t.rebuild()
	}
	var b strings.Builder
	vis := t.visible()
	end := clamp(t.offset+vis, 0, len(t.flat))
	for i := t.offset; i < end; i++ {
		if i > t.offset {
			b.WriteByte('\n')
		}
		mark := gutter
		if i == t.cursor {
			mark = cursorGutter
		}
		line := pad(mark+t.rowText(t.flat[i]), width)
		switch {
		case i == t.cursor && t.focused:
			b.WriteString(t.theme.Selected().Render(line))
		case i == t.cursor:
			b.WriteString(t.theme.Title().Render(line))
		default:
			b.WriteString(t.theme.Base().Render(line))
		}
	}
	return b.String()
}

// rowText renders one node's line: its indentation, a disclosure glyph (or an
// aligning pad for leaves), and the rendered label.
func (t *Tree[T]) rowText(r treeRow[T]) string {
	marker := treeLeafPad
	if len(r.node.Children) > 0 {
		if r.node.Expanded {
			marker = treeExpanded
		} else {
			marker = treeCollapsed
		}
	}
	return strings.Repeat(treeIndent, r.depth) + marker + t.render(r.node.Value)
}

// rebuild recomputes the flattened view and re-clamps the cursor. It is called
// after any change that can alter which nodes are visible.
func (t *Tree[T]) rebuild() {
	t.flat = t.flatten()
	t.clampCursor()
}

// flatten walks the expanded subtree in display order, recording each visible
// node with its depth.
func (t *Tree[T]) flatten() []treeRow[T] {
	var out []treeRow[T]
	var walk func(nodes []*TreeNode[T], depth int)
	walk = func(nodes []*TreeNode[T], depth int) {
		for _, n := range nodes {
			out = append(out, treeRow[T]{node: n, depth: depth})
			if n.Expanded && len(n.Children) > 0 {
				walk(n.Children, depth+1)
			}
		}
	}
	walk(t.roots, 0)
	return out
}

// current returns the node under the cursor, or nil when the tree is empty.
func (t *Tree[T]) current() *TreeNode[T] {
	if t.cursor < 0 || t.cursor >= len(t.flat) {
		return nil
	}
	return t.flat[t.cursor].node
}

// expand opens a collapsed parent; on an already-open parent it descends to the
// first child. It does nothing on a leaf.
func (t *Tree[T]) expand() {
	n := t.current()
	if n == nil || len(n.Children) == 0 {
		return
	}
	if n.Expanded {
		t.move(1)
		return
	}
	n.Expanded = true
	t.rebuild()
}

// collapse closes an open parent; on a leaf or an already-closed parent it moves
// the cursor to the node's parent.
func (t *Tree[T]) collapse() {
	n := t.current()
	if n == nil {
		return
	}
	if len(n.Children) > 0 && n.Expanded {
		n.Expanded = false
		t.rebuild()
		return
	}
	t.moveToParent()
}

// toggleSelect flips a parent's expansion and emits msg.SelectedMsg for the node
// under the cursor. The node keeps its cursor index across the toggle.
func (t *Tree[T]) toggleSelect() tea.Cmd {
	n := t.current()
	if n == nil {
		return nil
	}
	if len(n.Children) > 0 {
		n.Expanded = !n.Expanded
		t.rebuild()
	}
	idx := t.cursor
	return func() tea.Msg { return msg.SelectedMsg{Index: idx} }
}

// moveToParent moves the cursor to the nearest preceding node at a shallower
// depth — the current node's parent.
func (t *Tree[T]) moveToParent() {
	if t.cursor <= 0 || t.cursor >= len(t.flat) {
		return
	}
	depth := t.flat[t.cursor].depth
	for i := t.cursor - 1; i >= 0; i-- {
		if t.flat[i].depth < depth {
			t.cursor = i
			t.clampOffset()
			return
		}
	}
}

func (t *Tree[T]) move(d int) {
	if len(t.flat) == 0 {
		return
	}
	t.cursor += d
	t.clampCursor()
}

func (t *Tree[T]) clampCursor() {
	if len(t.flat) == 0 {
		t.cursor, t.offset = 0, 0
		return
	}
	t.cursor = clamp(t.cursor, 0, len(t.flat)-1)
	t.clampOffset()
}

func (t *Tree[T]) clampOffset() {
	vis := t.visible()
	if vis <= 0 {
		return
	}
	if t.cursor < t.offset {
		t.offset = t.cursor
	}
	if t.cursor >= t.offset+vis {
		t.offset = t.cursor - vis + 1
	}
	t.offset = clamp(t.offset, 0, max(0, len(t.flat)-vis))
}

// visible is the number of rows that fit; with no height set, all rows show.
func (t *Tree[T]) visible() int {
	if t.height <= 0 {
		return len(t.flat)
	}
	return t.height
}

func (t *Tree[T]) page() int {
	if p := t.visible(); p > 1 {
		return p
	}
	return 1
}

var (
	_ tui.Component = (*Tree[int])(nil)
	_ tui.Sizeable  = (*Tree[int])(nil)
	_ tui.Focusable = (*Tree[int])(nil)
	_ tui.Themeable = (*Tree[int])(nil)
)
