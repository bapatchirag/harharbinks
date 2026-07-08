package focus
// Package focus manages input focus across a set of Focusable components. A
// Manager keeps an ordered ring of components and guarantees that exactly one is
// focused at a time, cycling with Next and Prev (typically bound to Tab and
// Shift+Tab). It depends only on the tui contracts, not on any concrete
// component or domain type.
package focus

import "github.com/bapatchirag/harharbinks/internal/tui"

// Manager cycles focus over an ordered set of Focusable components.
type Manager struct {
	items []tui.Focusable
	idx   int
}

// New creates a Manager over the given components in order. The first component
// (if any) receives focus; the rest are blurred.
func New(items ...tui.Focusable) *Manager {
	m := &Manager{items: items}
	m.sync()
	return m
}

// Add appends a component to the ring. If it is the first component, it gains
// focus; otherwise it is blurred.
func (m *Manager) Add(f tui.Focusable) {
	m.items = append(m.items, f)
	m.sync()
}

// Len reports the number of managed components.
func (m *Manager) Len() int { return len(m.items) }

// Index reports the position of the currently focused component, or -1 if empty.
func (m *Manager) Index() int {
	if len(m.items) == 0 {
		return -1
	}
	return m.idx
}

// Focused returns the currently focused component, or nil if empty.
func (m *Manager) Focused() tui.Focusable {
	if len(m.items) == 0 {
		return nil
	}
	return m.items[m.idx]
}

// Next moves focus to the following component, wrapping around.
func (m *Manager) Next() {
	if len(m.items) == 0 {
		return
	}
	m.idx = (m.idx + 1) % len(m.items)
	m.sync()
}

// Prev moves focus to the previous component, wrapping around.
func (m *Manager) Prev() {
	if len(m.items) == 0 {
		return
	}
	m.idx = (m.idx - 1 + len(m.items)) % len(m.items)
	m.sync()
}

// Focus moves focus to the component at index i (ignored if out of range).
func (m *Manager) Focus(i int) {
	if i < 0 || i >= len(m.items) {
		return
	}
	m.idx = i
	m.sync()
}

// sync enforces the single-focus invariant.
func (m *Manager) sync() {
	for i, it := range m.items {
		if i == m.idx {
			it.Focus()
		} else {
			it.Blur()
		}
	}
}
