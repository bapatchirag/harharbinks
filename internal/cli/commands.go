package cli

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bapatchirag/harharbinks/internal/har"
	"github.com/bapatchirag/harharbinks/internal/update"
)

// row pairs an entry with its 1-based position in the original capture, so the
// index stays stable regardless of any filtering or sorting applied for display.
type row struct {
	Index int
	Entry har.Entry
}

// listFlags are the shared query/output flags used by the headless subcommands.
type listFlags struct {
	filter string
	sort   string
	method string
	status string
	json   bool
}

func (f *listFlags) bind(fs *flag.FlagSet) {
	fs.StringVar(&f.filter, "filter", "", "filter entries by URL substring")
	fs.StringVar(&f.sort, "sort", "", "sort by time|size|status|url|method (append :asc or :desc)")
	fs.StringVar(&f.method, "method", "", "filter by HTTP method (e.g. GET)")
	fs.StringVar(&f.status, "status", "", "filter by status code or class (e.g. 200 or 2xx)")
	fs.BoolVar(&f.json, "json", false, "emit JSON instead of text")
}

// cmdLs implements `hhb ls [flags] [file]`.
func cmdLs(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("hhb ls", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var f listFlags
	f.bind(fs)
	positionals, err := parseArgs(fs, args)
	if err != nil {
		return 2
	}
	file := ""
	if len(positionals) > 0 {
		file = positionals[0]
	}
	if err := ensureInput("HAR", file, "hhb ls file.har", "hhb ls < file.har"); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	h, err := loadHAR(file)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	rows, err := selectRows(h.Log.Entries, f)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if f.json {
		if err := writeRowsJSON(stdout, rows); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	writeTable(stdout, rows)
	return 0
}

// cmdShow implements `hhb show [--json] <index> [file]`.
func cmdShow(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("hhb show", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit JSON instead of text")
	positionals, err := parseArgs(fs, args)
	if err != nil {
		return 2
	}
	idx, file, err := indexAndFile(positionals)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if err := ensureInput("HAR", file, "hhb show <index> file.har", "hhb show <index> < file.har"); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	e, err := loadEntry(file, idx)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *asJSON {
		if err := writeEntryJSON(stdout, e); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	writeDetail(stdout, idx, e)
	return 0
}

// cmdCurl implements `hhb curl <index> [file]`.
func cmdCurl(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("hhb curl", flag.ContinueOnError)
	fs.SetOutput(stderr)
	positionals, err := parseArgs(fs, args)
	if err != nil {
		return 2
	}
	idx, file, err := indexAndFile(positionals)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if err := ensureInput("HAR", file, "hhb curl <index> file.har", "hhb curl <index> < file.har"); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	e, err := loadEntry(file, idx)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, har.Curl(e))
	return 0
}

// selectRows filters and sorts entries per f, preserving original 1-based indices.
func selectRows(entries []har.Entry, f listFlags) ([]row, error) {
	key, desc, err := har.ParseSort(f.sort)
	if err != nil {
		return nil, err
	}
	opts := har.FilterOptions{Text: f.filter, Method: f.method, Status: f.status}
	rows := make([]row, 0, len(entries))
	for i, e := range entries {
		if opts.Match(e) {
			rows = append(rows, row{Index: i + 1, Entry: e})
		}
	}
	if less := har.Less(key); less != nil {
		sort.SliceStable(rows, func(i, j int) bool {
			if desc {
				return less(rows[j].Entry, rows[i].Entry)
			}
			return less(rows[i].Entry, rows[j].Entry)
		})
	}
	return rows, nil
}

// indexAndFile parses a required 1-based index and an optional file path.
func indexAndFile(args []string) (index int, file string, err error) {
	if len(args) == 0 {
		return 0, "", fmt.Errorf("missing entry index; usage: hhb <command> <index> [file]")
	}
	index, err = strconv.Atoi(args[0])
	if err != nil {
		return 0, "", fmt.Errorf("invalid entry index %q", args[0])
	}
	if len(args) > 1 {
		file = args[1]
	}
	return index, file, nil
}

// loadEntry loads a HAR and returns its 1-based index'th entry.
func loadEntry(file string, index int) (har.Entry, error) {
	h, err := loadHAR(file)
	if err != nil {
		return har.Entry{}, err
	}
	if index < 1 || index > len(h.Log.Entries) {
		return har.Entry{}, fmt.Errorf("entry %d out of range (1..%d)", index, len(h.Log.Entries))
	}
	return h.Log.Entries[index-1], nil
}

// loadHAR reads a HAR document from path, or from stdin when path is empty or "-".
func loadHAR(path string) (*har.HAR, error) {
	if path == "" || path == "-" {
		return har.Parse(os.Stdin)
	}
	return har.ParseFile(path)
}

// cmdUpdate implements `hhb update [--check] [--yes]`. With --check it queries
// GitHub for the latest release and reports whether a newer one exists, changing
// nothing. Without it — and only for a real release build — it downloads the
// latest release, verifies its checksum, and replaces the running binary in place
// after confirmation. Every update action is explicit: nothing here runs unless
// the user invokes this command.
func cmdUpdate(args []string, stdout, stderr io.Writer, version string) int {
	fs := flag.NewFlagSet("hhb update", flag.ContinueOnError)
	fs.SetOutput(stderr)
	checkOnly := fs.Bool("check", false, "check for a newer release without installing it")
	assumeYes := fs.Bool("yes", false, "install without prompting for confirmation")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if *checkOnly {
		res, err := update.Check(ctx, version, true)
		if err != nil {
			fmt.Fprintf(stderr, "update check failed: %v\n", err)
			return 1
		}
		printUpdateStatus(stdout, version, res)
		return 0
	}

	// Self-installation replaces the running binary, so refuse development builds
	// up front — before any network access — to avoid clobbering a locally-built or
	// go-installed binary that has no matching release asset.
	if !update.IsReleaseBuild(version) {
		fmt.Fprintf(stderr, "hhb %s is a development build; refusing to self-update.\n", version)
		fmt.Fprintln(stderr, "Install the latest release with: go install github.com/bapatchirag/harharbinks/cmd/hhb@latest")
		return 1
	}

	res, err := update.Check(ctx, version, true)
	if err != nil {
		fmt.Fprintf(stderr, "update check failed: %v\n", err)
		return 1
	}
	if !res.Newer {
		fmt.Fprintf(stdout, "harharbinks %s is already up to date.\n", version)
		return 0
	}
	if !*assumeYes && !confirmUpdate(stdout, version, res.Latest) {
		fmt.Fprintln(stdout, "Update canceled.")
		return 1
	}

	applied, err := update.Apply(ctx, version)
	if err != nil {
		fmt.Fprintf(stderr, "update failed: %v\n", err)
		fmt.Fprintln(stderr, "Install the latest release manually with: go install github.com/bapatchirag/harharbinks/cmd/hhb@latest")
		return 1
	}
	fmt.Fprintf(stdout, "Updated harharbinks %s -> %s. Restart hhb to run the new version.\n", version, applied.Latest)
	return 0
}

// printUpdateStatus reports the outcome of a --check to stdout: a newer release,
// an up-to-date build, or a development build (which cannot be self-updated and
// is upgraded through the Go toolchain instead).
func printUpdateStatus(stdout io.Writer, version string, res update.Result) {
	switch {
	case res.Latest == "":
		fmt.Fprintln(stdout, "No published release was found.")
	case !update.IsReleaseBuild(version):
		fmt.Fprintf(stdout, "harharbinks %s is a development build; the latest release is %s.\n", version, res.Latest)
		fmt.Fprintln(stdout, "Install it with: go install github.com/bapatchirag/harharbinks/cmd/hhb@latest")
	case res.Newer:
		fmt.Fprintf(stdout, "A newer harharbinks is available: %s (you have %s).\n", res.Latest, version)
		fmt.Fprintln(stdout, "Install it with: hhb update")
	default:
		fmt.Fprintf(stdout, "harharbinks %s is the latest version.\n", version)
	}
}

// confirmUpdate asks the user to approve replacing version with latest, reading a
// yes/no answer from standard input. It returns false — declining the update —
// when standard input is not an interactive terminal, so a non-interactive
// invocation never blocks or updates without the explicit --yes flag.
func confirmUpdate(stdout io.Writer, version, latest string) bool {
	if !stdinIsTerminal() {
		fmt.Fprintf(stdout, "A newer version (%s) is available. Re-run with --yes to install it.\n", latest)
		return false
	}
	fmt.Fprintf(stdout, "Update harharbinks %s -> %s? [y/N] ", version, latest)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

// withUpdateHint returns code unchanged after optionally printing a one-line
// notice to stderr when opt-in update checks are enabled and the cache already
// knows of a newer release. It reads only the cache — never the network — and
// prints only to an interactive terminal, so scripts and pipelines see clean,
// unchanged output.
func withUpdateHint(code int, stderr io.Writer, version string, enabled bool) int {
	if !enabled || !writerIsTerminal(stderr) {
		return code
	}
	if res, ok := update.Cached(version); ok && res.Newer {
		fmt.Fprintf(stderr, "\nA new harharbinks (%s) is available — run `hhb update`.\n", res.Latest)
	}
	return code
}

// writerIsTerminal reports whether w is an interactive terminal, mirroring
// stdinIsTerminal for output streams. A non-file writer (such as a buffer in
// tests, or a pipe) is never a terminal, keeping redirected output clean.
func writerIsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// stdinIsTerminal reports whether standard input is an interactive terminal —
// that is, nothing was piped or redirected in. It is a package variable so tests
// can simulate either case. When true, a command with no file argument has no
// HAR to read and would otherwise block waiting on the keyboard.
var stdinIsTerminal = func() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// ensureInput validates that a headless command has something to read: either a
// file path was given, or something was piped/redirected into stdin. When
// neither holds (no file and stdin is an interactive terminal) it returns a usage
// error citing both correct forms, so the command reports bad usage instead of
// hanging. kind names the expected input (e.g. "HAR" or "capture").
func ensureInput(kind, file, fileExample, pipeExample string) error {
	if file != "" && file != "-" {
		return nil
	}
	if !stdinIsTerminal() {
		return nil
	}
	return fmt.Errorf("no %s input: pass a file (%s) or pipe one in (%s)", kind, fileExample, pipeExample)
}

// parseArgs parses fs while allowing flags and positional arguments to be
// interspersed; the stdlib flag package otherwise stops at the first positional.
// Positionals are returned in their original order.
func parseArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	var positionals []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		rest := fs.Args()
		if len(rest) == 0 {
			return positionals, nil
		}
		positionals = append(positionals, rest[0])
		args = rest[1:]
	}
}
