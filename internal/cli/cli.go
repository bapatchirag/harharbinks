package cli
// Package cli parses command-line arguments for harharbinks and dispatches to
// the requested mode. Later milestones add the headless subcommands (ls, show,
// curl) and the interactive TUI; this skeleton handles --version and help so the
// hhb binary is runnable and testable from day one.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
)

// ProductName is the human-facing name shown in help and version output. The
// installed binary is "hhb", but all descriptions refer to "harharbinks".
const ProductName = "harharbinks"

// Run parses args and executes the requested command, returning a process exit
// code. The version string is injected by the caller at build time.
func Run(args []string, version string) int {
	return run(args, version, os.Stdout, os.Stderr)
}

// run is the testable core of Run with explicit output streams.
func run(args []string, version string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("hhb", flag.ContinueOnError)
	fs.SetOutput(stderr)
	// Usage is rendered explicitly below so we control which stream it targets.
	fs.Usage = func() {}

	showVersion := fs.Bool("version", false, "print the harharbinks version and exit")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			writeUsage(stdout)
			return 0
		}
		// flag has already reported the specific error on stderr.
		writeUsage(stderr)
		return 2
	}

	if *showVersion {
		fmt.Fprintf(stdout, "%s %s\n", ProductName, version)
		return 0
	}

	// P0 skeleton: subcommands and the TUI are not wired up yet.
	writeUsage(stdout)
	return 0
}

// writeUsage prints the harharbinks help text. The synopsis lines show the real
// "hhb" command token so examples are copy-pasteable, while the prose refers to
// the product as harharbinks.
func writeUsage(w io.Writer) {
	fmt.Fprintf(w, `%s — an offline HAR file viewer TUI

Usage:
  hhb [file.har]            Open a HAR file in the interactive viewer
  hhb <command> [args]      Run a headless command

Commands:
  ls    [file]              List HAR entries
  show  <index> [file]      Show details for a single entry
  curl  <index> [file]      Print an entry as a cURL command

Flags:
  --version                 Print the %s version and exit
  -h, --help                Show this help text
`, ProductName, ProductName)
}
