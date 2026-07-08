package keymap
package keymap

import "testing"

func TestDefaultHelp(t *testing.T) {
	k := Default()
	if len(k.ShortHelp()) == 0 {
		t.Error("ShortHelp should not be empty")
	}
	if len(k.FullHelp()) == 0 {
		t.Error("FullHelp should not be empty")
	}
	if !k.Quit.Enabled() {
		t.Error("Quit binding should be enabled")
	}
}
