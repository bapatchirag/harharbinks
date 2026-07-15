package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/bapatchirag/harharbinks/internal/pcap"
)

// cmdPcap dispatches the `hhb pcap` subcommand group, which inspects packet
// captures headlessly (list, show, flows, stats). With no subcommand — or an
// unknown one — it prints the group usage; `hhb pcap help` prints it on request.
func cmdPcap(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		writePcapUsage(stderr)
		return 2
	}
	switch args[0] {
	case "ls":
		return cmdPcapLs(args[1:], stdout, stderr)
	case "show":
		return cmdPcapShow(args[1:], stdout, stderr)
	case "flows":
		return cmdPcapFlows(args[1:], stdout, stderr)
	case "stats":
		return cmdPcapStats(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		writePcapUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "hhb pcap: unknown subcommand %q\n\n", args[0])
		writePcapUsage(stderr)
		return 2
	}
}

// writePcapUsage prints help for the pcap command group: each subcommand with
// its syntax and purpose, the flags accepted by `ls`, the shared --json flag,
// and copy-pasteable examples.
func writePcapUsage(w io.Writer) {
	fmt.Fprint(w, `harharbinks — inspect a packet capture (.pcap / .pcapng), fully offline

Usage:
  hhb pcap <command> [flags] [file]

Commands:
  ls     [flags] [file]     List packets, one summary line each
  show   <index> [file]     Show one packet: layers + hex/ASCII dump
  flows  [file]             List bidirectional conversations (5-tuple)
  stats  [file]             Summarize totals, protocols, and top talkers

Flags for 'ls':
  --proto  <name>           Keep only one protocol (e.g. TCP, DNS, TLS)
  --filter <text>           Substring match over protocol, info, and addresses
  --sort   <key[:dir]>      Sort by time|len|proto|src|dst (dir is asc or desc)
  --limit  <n>              Show at most n packets

Every command also accepts --json to emit JSON instead of text. Each reads the
capture from the [file] argument, or from stdin when it is omitted
(e.g. hhb pcap ls < capture.pcap).

Examples:
  hhb pcap ls capture.pcap --proto TLS --sort len:desc
  hhb pcap show 14 capture.pcap
  hhb pcap flows capture.pcap
  hhb pcap stats capture.pcap --json
`)
}

// pcapListFlags are the query/output flags shared by `hhb pcap ls`.
type pcapListFlags struct {
	proto  string
	filter string
	sort   string
	limit  int
	json   bool
}

func (f *pcapListFlags) bind(fs *flag.FlagSet) {
	fs.StringVar(&f.proto, "proto", "", "filter by protocol (e.g. TCP, DNS, TLS)")
	fs.StringVar(&f.filter, "filter", "", "filter by substring across protocol, info, and addresses")
	fs.StringVar(&f.sort, "sort", "", "sort by time|len|proto|src|dst (append :asc or :desc)")
	fs.IntVar(&f.limit, "limit", 0, "show at most N packets (0 = all)")
	fs.BoolVar(&f.json, "json", false, "emit JSON instead of text")
}

// cmdPcapLs implements `hhb pcap ls [flags] [file]`.
func cmdPcapLs(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("hhb pcap ls", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var f pcapListFlags
	f.bind(fs)
	positionals, err := parseArgs(fs, args)
	if err != nil {
		return 2
	}
	file := ""
	if len(positionals) > 0 {
		file = positionals[0]
	}
	if err := ensureInput("capture", file, "hhb pcap ls file.pcap", "hhb pcap ls < file.pcap"); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	c, err := loadCapture(file)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	packets, err := selectPackets(c.Packets, f)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if f.json {
		if err := writePacketsJSON(stdout, packets); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	writePacketTable(stdout, packets, captureStart(c))
	return 0
}

// cmdPcapShow implements `hhb pcap show [--json] <index> [file]`.
func cmdPcapShow(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("hhb pcap show", flag.ContinueOnError)
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
	if err := ensureInput("capture", file, "hhb pcap show <index> file.pcap", "hhb pcap show <index> < file.pcap"); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	c, err := loadCapture(file)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if idx < 1 || idx > len(c.Packets) {
		fmt.Fprintf(stderr, "packet %d out of range (1..%d)\n", idx, len(c.Packets))
		return 1
	}
	p := c.Packets[idx-1]
	if *asJSON {
		if err := writePacketDetailJSON(stdout, p); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	writePacketDetail(stdout, p)
	return 0
}

// cmdPcapFlows implements `hhb pcap flows [--json] [file]`.
func cmdPcapFlows(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("hhb pcap flows", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit JSON instead of text")
	positionals, err := parseArgs(fs, args)
	if err != nil {
		return 2
	}
	file := ""
	if len(positionals) > 0 {
		file = positionals[0]
	}
	if err := ensureInput("capture", file, "hhb pcap flows file.pcap", "hhb pcap flows < file.pcap"); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	c, err := loadCapture(file)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	flows := pcap.Flows(c.Packets)
	if *asJSON {
		if err := writeFlowsJSON(stdout, flows); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	writeFlowTable(stdout, flows)
	return 0
}

// cmdPcapStats implements `hhb pcap stats [--json] [file]`.
func cmdPcapStats(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("hhb pcap stats", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit JSON instead of text")
	positionals, err := parseArgs(fs, args)
	if err != nil {
		return 2
	}
	file := ""
	if len(positionals) > 0 {
		file = positionals[0]
	}
	if err := ensureInput("capture", file, "hhb pcap stats file.pcap", "hhb pcap stats < file.pcap"); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	c, err := loadCapture(file)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	s := pcap.Summarize(c.Packets)
	if *asJSON {
		if err := writeStatsJSON(stdout, s); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	writeStats(stdout, s)
	return 0
}

// selectPackets filters, sorts, and limits packets per f, leaving each packet's
// original 1-based Index intact.
func selectPackets(packets []pcap.Packet, f pcapListFlags) ([]pcap.Packet, error) {
	key, desc, err := pcap.ParseSort(f.sort)
	if err != nil {
		return nil, err
	}
	out := pcap.Filter(packets, pcap.FilterOptions{Proto: f.proto, Text: f.filter})
	pcap.Sort(out, key, desc)
	if f.limit > 0 && len(out) > f.limit {
		out = out[:f.limit]
	}
	return out, nil
}

// loadCapture reads a capture from path, or from stdin when path is empty or "-".
func loadCapture(path string) (*pcap.Capture, error) {
	if path == "" || path == "-" {
		return pcap.Parse(os.Stdin)
	}
	return pcap.ParseFile(path)
}

// captureStart returns the timestamp of the capture's first packet, used as the
// zero point for the relative times shown in the packet list.
func captureStart(c *pcap.Capture) time.Time {
	if len(c.Packets) == 0 {
		return time.Time{}
	}
	return c.Packets[0].Timestamp
}
