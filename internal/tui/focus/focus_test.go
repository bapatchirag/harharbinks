package focus

import (
	"testing"

	"github.com/bapatchirag/harharbinks/internal/tui"
)

// fake is a minimal Focusable for testing the manager.
type fake struct{ focused bool }

func (f *fake) Focus()        { f.focused = true }
func (f *fake) Blur()         { f.focused = false }
func (f *fake) Focused() bool { return f.focused }

var _ tui.Focusable = (*fake)(nil)

// focusedIndex returns the single focused index, or -2 if the single-focus
// invariant is violated, or -1 if none are focused.
func focusedIndex(items ...*fake) int {
	idx := -1
	for i, f := range items {
		if f.focused {
			if idx != -1 {
				return -2
			}
			idx = i
		}
	}
	return idx
}

func TestManagerFocusesFirst(t *testing.T) {
	a, b, c := &fake{}, &fake{}, &fake{}
	New(a, b, c)
	if focusedIndex(a, b, c) != 0 {
		t.Fatalf("first component should be focused; got index %d", focusedIndex(a, b, c))
	}
}

func TestManagerNextPrevWrap(t *testing.T) {
	a, b, c := &fake{}, &fake{}, &fake{}
	m := New(a, b, c)

	m.Next()
	if focusedIndex(a, b, c) != 1 || m.Index() != 1 {
		t.Fatalf("after Next: focused=%d index=%d, want 1", focusedIndex(a, b, c), m.Index())
	}
	m.Next()
	m.Next() // wrap back to 0
	if focusedIndex(a, b, c) != 0 {
		t.Fatalf("Next should wrap to 0; got %d", focusedIndex(a, b, c))
	}
	m.Prev() // wrap to last
	if focusedIndex(a, b, c) != 2 {
		t.Fatalf("Prev should wrap to 2; got %d", focusedIndex(a, b, c))
	}
}

func TestManagerFocusIndex(t *testing.T) {
	a, b, c := &fake{}, &fake{}, &fake{}
	m := New(a, b, c)
	m.Focus(2)
	if focusedIndex(a, b, c) != 2 {
		t.Fatalf("Focus(2) failed; got %d", focusedIndex(a, b, c))
	}
	m.Focus(99) // out of range: ignored
	if focusedIndex(a, b, c) != 2 {
		t.Fatalf("out-of-range Focus changed focus; got %d", focusedIndex(a, b, c))
	}
}

func TestManagerEmpty(t *testing.T) {
	m := New()
	if m.Focused() != nil {
		t.Fatal("empty manager should have nil focused")
	}
	if m.Index() != -1 {
		t.Fatalf("empty manager index = %d, want -1", m.Index())
	}
	m.Next() // must not panic
	m.Prev()
}

func TestManagerAdd(t *testing.T) {
	m := New()
	a := &fake{}
	m.Add(a)
	if !a.focused {
		t.Fatal("first added component should be focused")
	}
	b := &fake{}
	m.Add(b)
	if b.focused {
		t.Fatal("second added component should not steal focus")
	}
	if m.Len() != 2 {
		t.Fatalf("Len = %d, want 2", m.Len())
	}
}
