package cli

import (
	"fmt"
	"io"

	"github.com/bapatchirag/harharbinks/internal/app"
)

// launchViewer starts the interactive viewer for a bare file argument, opening
// it as a HAR archive or a packet capture according to its format. It is a
// package variable so tests can stub the TUI without a real terminal.
var launchViewer = runViewer

// launchBrowser starts the interactive file browser when hhb is run without a
// file. Like launchViewer it is a package variable so tests can stub it.
var launchBrowser = runBrowser

// runViewer loads file and opens it in the interactive viewer, choosing the HAR
// or PCAP screen by the file's detected format, and returns a process exit code.
// version is the running build, passed through to the app for the opt-in update
// check.
func runViewer(file, version string, stderr io.Writer) int {
	screen, err := app.Open(file)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := app.Run(screen, version); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// runBrowser opens the interactive file browser so the user can pick a capture,
// returning a process exit code. version is the running build, passed through to
// the app for the opt-in update check.
func runBrowser(version string, stderr io.Writer) int {
	if err := app.Run(app.NewBrowser(), version); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
