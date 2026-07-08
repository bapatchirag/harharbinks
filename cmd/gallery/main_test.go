package gallery
package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

// TestGallerySmoke drives the gallery model headlessly: it sizes the terminal,
// cycles through every demo tab, triggers a toast and a modal, and quits. It
// asserts the model runs its full Update/View cycle without panicking.
func TestGallerySmoke(t *testing.T) {
	tm := teatest.NewTestModel(t, newModel(), teatest.WithInitialTermSize(100, 30))

	// Visit every demo tab (there are nine).
	for i := 0; i < 9; i++ {
		tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	}
	// Exercise a few interactive paths (harmless regardless of active demo).
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")}) // toast (on Toast demo)
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})                     // open modal (on Modal demo)
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})                       // dismiss

	if err := tm.Quit(); err != nil {
		t.Fatalf("quit: %v", err)
	}
	tm.WaitFinished(t)
}
