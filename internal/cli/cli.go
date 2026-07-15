// Package cli parses command-line arguments for harharbinks and dispatches to
// the requested mode. It handles --version and help, the headless subcommands
// (ls, show, curl), and launching the interactive TUI viewer for a bare HAR
// file argument.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/bapatchirag/harharbinks/internal/config"
	"github.com/bapatchirag/harharbinks/internal/update"
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
	// Keep a configuration file present and current on every launch, headless or
	// TUI. This is best-effort: a read-only or unavailable config location must not
	// stop any command from running.
	cfg, _ := config.Ensure()
	updateEnabled := update.Enabled(cfg)

	// Dispatch headless subcommands before top-level flag parsing so that their
	// own flags (e.g. --sort) are not intercepted here. Each headless command may
	// append a one-line "update available" hint (see withUpdateHint), while the
	// explicit update command manages its own network access.
	if len(args) > 0 {
		switch args[0] {
		case "ls":
			return withUpdateHint(cmdLs(args[1:], stdout, stderr), stderr, version, updateEnabled)
		case "show":
			return withUpdateHint(cmdShow(args[1:], stdout, stderr), stderr, version, updateEnabled)
		case "curl":
			return withUpdateHint(cmdCurl(args[1:], stdout, stderr), stderr, version, updateEnabled)
		case "pcap":
			return withUpdateHint(cmdPcap(args[1:], stdout, stderr), stderr, version, updateEnabled)
		case "update":
			return cmdUpdate(args[1:], stdout, stderr, version)
		}
	}

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

	// A bare non-flag argument is treated as a HAR file and opened in the
	// interactive viewer.
	if fs.NArg() > 0 {
		return launchViewer(fs.Arg(0), version, stderr)
	}

	// No file and no subcommand: open the interactive file browser so the user
	// can pick a capture. Help remains available via --help/-h.
	return launchBrowser(version, stderr)
}

// writeUsage prints the harharbinks help text. The synopsis lines show the real
// "hhb" command token so examples are copy-pasteable, while the prose refers to
// the product as harharbinks.
func writeUsage(w io.Writer) {
	fmt.Fprintf(w, `%s — an offline HAR & PCAP inspector

Usage:
  hhb                       Open the interactive file browser to pick a HAR file
  hhb [file.har]            Open a HAR file in the interactive viewer
  hhb <command> [args]      Run a headless command

HAR commands:
  ls     [file]             List HAR entries
  show   <index> [file]     Show details for a single entry
  curl   <index> [file]     Print an entry as a cURL command

PCAP commands (run as 'hhb pcap <command>'; see 'hhb pcap' for flags):
  ls     [file]             List packets in the capture
  show   <index> [file]     Show one packet (layer stack + hex)
  flows  [file]             List conversations (5-tuple flows)
  stats  [file]             Summarize protocols and top talkers

Other commands:
  update [--check]          Check for a newer release, and optionally install it

Headless commands read their input from the [file] argument, or from stdin when
it is omitted (e.g. hhb ls < file.har). One of the two is required.

harharbinks is offline by default. Update checks are opt-in: enable a daily
launch check by setting update_check in the config or the HHB_UPDATE_CHECK
environment variable. hhb update always checks on demand.

Flags:
  --version                 Print the %s version and exit
  -h, --help                Show this help text
`, ProductName, ProductName)
}
