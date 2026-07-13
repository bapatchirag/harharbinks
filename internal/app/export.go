// This file implements the HAR viewer's export menu: a small command palette
// opened with "e" that acts on the currently selected entry. It offers
// three actions — copy the request URL, copy the request as a cURL command, and
// save the response body to disk — each confirmed (or reported) via a toast.
// The menu itself is the generic component.Menu; this file is the app-layer glue
// that gives it HAR-specific actions while keeping the component reusable.
package app

import (
	"net/url"
	"os"
	"path"
	"path/filepath"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bapatchirag/harharbinks/internal/har"
	"github.com/bapatchirag/harharbinks/internal/tui/component"
	"github.com/bapatchirag/harharbinks/internal/tui/keymap"
	"github.com/bapatchirag/harharbinks/internal/tui/msg"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// Export menu action identifiers. The menu emits these opaque strings via
// msg.MenuActionMsg; the viewer maps them back to the operations below.
const (
	actionCopyURL  = "copy-url"
	actionCopyCurl = "copy-curl"
	actionSaveBody = "save-body"
)

// copyToClipboard writes text to the system clipboard. It is a package variable
// so tests can capture copies without touching (or requiring) a real clipboard.
var copyToClipboard = clipboard.WriteAll

// bodySaveDir is the directory the "save response body" action writes into. It
// defaults to the working directory and is a package variable so tests can
// redirect writes to a temporary directory.
var bodySaveDir = "."

// newExportMenu builds the viewer's export command menu.
func newExportMenu(th theme.Theme, km keymap.KeyMap) *component.Menu {
	mn := component.NewMenu([]component.MenuItem{
		{Key: "u", Title: "Copy URL", Action: actionCopyURL},
		{Key: "c", Title: "Copy as cURL", Action: actionCopyCurl},
		{Key: "s", Title: "Save response body", Action: actionSaveBody},
	}, th, km)
	mn.SetTitle("Export")
	mn.SetSize(26, 0)
	return mn
}

// openMenu shows the export menu and gives it focus so it handles navigation.
func (v *Viewer) openMenu() {
	v.menuOpen = true
	v.menu.Focus()
}

// closeMenu hides the export menu and returns control to the list/detail.
func (v *Viewer) closeMenu() {
	v.menuOpen = false
	v.menu.Blur()
}

// handleMenuKey routes a key while the export menu is open: esc closes it, and
// every other key drives the menu (navigation, and enter to activate an item,
// which emits a msg.MenuActionMsg the viewer then runs).
func (v *Viewer) handleMenuKey(k tea.KeyMsg) tea.Cmd {
	if key.Matches(k, v.keys.Back) {
		v.closeMenu()
		return nil
	}
	return v.menu.Update(k)
}

// runMenuAction performs an export action against the selected entry, closing
// the menu and returning a command that shows a confirmation (or error) toast.
func (v *Viewer) runMenuAction(action string) tea.Cmd {
	v.closeMenu()
	e, ok := v.table.Selected()
	if !ok {
		return v.toast.Show("No entry selected.", msg.Warning)
	}
	switch action {
	case actionCopyURL:
		if err := copyToClipboard(e.Request.URL); err != nil {
			return v.toast.Show("Copy failed: "+err.Error(), msg.Error)
		}
		return v.toast.Show("URL copied to clipboard.", msg.Success)
	case actionCopyCurl:
		if err := copyToClipboard(har.Curl(e)); err != nil {
			return v.toast.Show("Copy failed: "+err.Error(), msg.Error)
		}
		return v.toast.Show("cURL copied to clipboard.", msg.Success)
	case actionSaveBody:
		name, err := writeBody(bodySaveDir, e)
		if err != nil {
			return v.toast.Show("Save failed: "+err.Error(), msg.Error)
		}
		return v.toast.Show("Saved body to "+name, msg.Success)
	}
	return nil
}

// writeBody writes e's response body into dir under a file name derived from the
// request URL, returning the base name written. The body is base64-decoded first
// when the capture stored it encoded (see har.Content.Body).
func writeBody(dir string, e har.Entry) (string, error) {
	body, err := e.Response.Content.Body()
	if err != nil {
		return "", err
	}
	name := bodyFileName(e)
	if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
		return "", err
	}
	return name, nil
}

// bodyFileName chooses a file name for a saved response body from the request
// URL's final path segment, falling back to "response" when the URL has no
// usable name. Only that final segment is used (path.Base strips any directory
// components), so a crafted URL cannot direct the write outside dir.
func bodyFileName(e har.Entry) string {
	name := ""
	if u, err := url.Parse(e.Request.URL); err == nil {
		name = path.Base(u.Path)
	}
	switch name {
	case "", ".", "..", "/":
		name = "response"
	}
	return name
}
