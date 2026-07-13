package cli

import (
	"fmt"
	"io"

	"github.com/bapatchirag/harharbinks/internal/app"
)

// launchViewer starts the interactive HAR viewer for a bare file argument. It is
// a package variable so tests can stub the TUI without a real terminal.
var launchViewer = runViewer

// launchBrowser starts the interactive file browser when hhb is run without a
// file. Like launchViewer it is a package variable so tests can stub it.
var launchBrowser = runBrowser

// runViewer loads the HAR at file and opens it in the interactive viewer,
// returning a process exit code.
func runViewer(file string, stderr io.Writer) int {
	h, err := loadHAR(file)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := app.Run(app.NewViewer(h.Log.Entries, file)); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// runBrowser opens the interactive file browser so the user can pick a capture,
// returning a process exit code.
func runBrowser(stderr io.Writer) int {
	if err := app.Run(app.NewBrowser()); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
