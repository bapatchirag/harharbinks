package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"

	"github.com/bapatchirag/harharbinks/internal/har"
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
