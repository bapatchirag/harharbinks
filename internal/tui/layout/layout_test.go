package layout
package layout

import "testing"

func TestSplitVertical(t *testing.T) {
	if got := SplitVertical("a", "b"); got != "a\nb" {
		t.Errorf("SplitVertical = %q, want %q", got, "a\nb")
	}
}

func TestSplitHorizontal(t *testing.T) {
	if got := SplitHorizontal("a", "b"); got != "ab" {
		t.Errorf("SplitHorizontal = %q, want %q", got, "ab")
	}
}

func TestOverlay(t *testing.T) {
	bg := "......\n......\n......"
	got := Overlay(bg, "XX", 2, 1)
	want := "......\n..XX..\n......"
	if got != want {
		t.Errorf("Overlay = %q, want %q", got, want)
	}
}

func TestOverlayClipsExtraRows(t *testing.T) {
	bg := "....\n...."
	got := Overlay(bg, "AA\nBB\nCC", 1, 1)
	want := "....\n.AA."
	if got != want {
		t.Errorf("Overlay = %q, want %q", got, want)
	}
}

func TestCenter(t *testing.T) {
	bg := "-----\n-----\n-----"
	got := Center(bg, "X")
	want := "-----\n--X--\n-----"
	if got != want {
		t.Errorf("Center = %q, want %q", got, want)
	}
}
