// This file implements the PCAP packet-detail panes: a collapsible layer tree
// stacked over a hex view of the frame bytes. It is the app-layer adapter that
// maps a decoded pcap.Packet into the generic Tree and HexView components,
// keeping the tree selection synced to the highlighted byte range so selecting a
// layer (or one of its fields) lights up exactly the bytes it spans. The
// components stay unaware of PCAP; this file does the translation.
package app

import (
	"fmt"
	"strings"

	"github.com/bapatchirag/harharbinks/internal/pcap"
	"github.com/bapatchirag/harharbinks/internal/tui/component"
	"github.com/bapatchirag/harharbinks/internal/tui/keymap"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// layerNode is one row of the packet-detail tree — a frame, a protocol layer, or
// one of a layer's fields — carrying the label to render and the byte range it
// occupies in the frame so selecting it can highlight those bytes in the hex view.
type layerNode struct {
	label  string
	offset int
	length int
}

// pcapDetail is the packet inspector: a Tree of the selected packet's layer stack
// stacked over a HexView of its raw bytes. Moving through the tree highlights the
// selected node's byte range in the hex view. Both panes are focusable in their
// own right so the enclosing viewer can Tab between the list, the tree, and the
// bytes.
type pcapDetail struct {
	theme theme.Theme
	tree  *component.Tree[layerNode]
	hex   *component.HexView

	width int
	treeH int
	hexH  int
}

// newPcapDetail builds an empty packet inspector styled with the given theme and
// key bindings.
func newPcapDetail(th theme.Theme, km keymap.KeyMap) *pcapDetail {
	return &pcapDetail{
		theme: th,
		tree:  component.NewTree(func(n layerNode) string { return n.label }, th, km),
		hex:   component.NewHexView(th, km),
	}
}

// SetPacket shows packet p: it rebuilds the layer tree, loads the frame bytes into
// the hex view, and syncs the highlight to the initially selected node.
func (d *pcapDetail) SetPacket(p pcap.Packet) {
	d.tree.SetRoots(buildPacketTree(p))
	d.hex.SetData(p.Data)
	d.syncHighlight()
}

// Clear drops the current packet, emptying both panes.
func (d *pcapDetail) Clear() {
	d.tree.SetRoots(nil)
	d.hex.SetData(nil)
}

// syncHighlight points the hex view's highlight at the byte range of the tree's
// selected node, or clears it when the tree is empty. It is called after any tree
// navigation so the highlighted bytes always track the selection.
func (d *pcapDetail) syncHighlight() {
	if n, ok := d.tree.Selected(); ok {
		d.hex.SetHighlight(n.offset, n.length)
	} else {
		d.hex.ClearHighlight()
	}
}

// SetTheme swaps the inspector's palette at runtime, propagating it to both panes.
func (d *pcapDetail) SetTheme(th theme.Theme) {
	d.theme = th
	d.tree.SetTheme(th)
	d.hex.SetTheme(th)
}

// SetSize splits the inspector's area between the layer tree (top) and the hex
// view (bottom), reserving one row above each for its labeled header bar.
func (d *pcapDetail) SetSize(w, h int) {
	d.width = w
	contentH := h - 2 // one header bar above each of the two panes
	if contentH < 2 {
		contentH = 2
	}
	d.treeH = contentH / 2
	if d.treeH < 1 {
		d.treeH = 1
	}
	d.hexH = contentH - d.treeH
	if d.hexH < 1 {
		d.hexH = 1
	}
	d.tree.SetSize(w, d.treeH)
	d.hex.SetSize(w, d.hexH)
}

// View stacks the layer tree and the hex view, each under a labeled header bar so
// the two panes read as distinct sections, and fits each to its allotted height.
func (d *pcapDetail) View() string {
	tree := fitLines(strings.Split(d.tree.View(), "\n"), d.width, d.treeH)
	hex := fitLines(strings.Split(d.hex.View(), "\n"), d.width, d.hexH)
	return strings.Join([]string{
		d.paneHeader("Layers", d.tree.Focused()),
		tree,
		d.paneHeader("Bytes", d.hex.Focused()),
		hex,
	}, "\n")
}

// paneHeader renders a full-width title bar labeling a detail pane, highlighted
// when that pane holds focus, so the stacked panes stay clearly separated.
func (d *pcapDetail) paneHeader(title string, focused bool) string {
	style := d.theme.TabInactive()
	if focused {
		style = d.theme.Selected()
	}
	width := d.width
	if width < 1 {
		width = 1
	}
	return style.Width(width).Render(" " + title)
}

// buildPacketTree maps a decoded packet into the layer tree: a Frame root holding
// one collapsible node per protocol layer, each expandable to its decoded fields.
// The frame starts expanded to reveal the layer list; the layers start collapsed
// so the tree opens compact. Every node carries the byte range to highlight — a
// field inherits its layer's range, and the frame spans the whole packet.
func buildPacketTree(p pcap.Packet) []*component.TreeNode[layerNode] {
	stack := p.LayerStack()
	children := make([]*component.TreeNode[layerNode], 0, len(stack))
	for _, l := range stack {
		children = append(children, layerTreeNode(l))
	}
	frame := layerNode{
		label:  fmt.Sprintf("Frame %d: %d bytes on wire", p.Index, len(p.Data)),
		offset: 0,
		length: len(p.Data),
	}
	return []*component.TreeNode[layerNode]{component.Branch(frame, children...)}
}

// layerTreeNode builds one layer's node and its field leaves. A layer with fields
// becomes a collapsed branch (so the frame first shows a compact layer list); a
// layer without fields is a plain leaf. A field value that spans multiple lines
// becomes several leaves so the tree stays one row per line.
func layerTreeNode(l pcap.Layer) *component.TreeNode[layerNode] {
	node := layerNode{label: layerLabel(l), offset: l.Offset, length: l.Length}
	var fields []*component.TreeNode[layerNode]
	for _, f := range l.Fields {
		for _, line := range fieldLines(f) {
			fields = append(fields, component.Leaf(layerNode{label: line, offset: f.Offset, length: f.Length}))
		}
	}
	if len(fields) == 0 {
		return component.Leaf(node)
	}
	return &component.TreeNode[layerNode]{Value: node, Children: fields}
}

// layerLabel renders a layer's tree row: its name and, when present, its summary.
func layerLabel(l pcap.Layer) string {
	if l.Summary != "" {
		return l.Name + " — " + l.Summary
	}
	return l.Name
}

// fieldLines renders a field as one or more tree rows: the first row pairs the
// field name with the first line of its value, and any further value lines follow
// indented, so a multi-line value (such as a pretty-printed body) stays readable.
func fieldLines(f pcap.Field) []string {
	lines := strings.Split(f.Value, "\n")
	out := make([]string, 0, len(lines))
	out = append(out, f.Name+": "+lines[0])
	for _, cont := range lines[1:] {
		out = append(out, "  "+cont)
	}
	return out
}
