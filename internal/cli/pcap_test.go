package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const samplePCAP = "../../testdata/sample.pcap"

// TestPcapGolden exercises the headless pcap subcommands against the sample
// capture and compares stdout to checked-in golden files. Regenerate them with:
// go test ./internal/cli -run TestPcapGolden -update
func TestPcapGolden(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"pcap_ls", []string{"pcap", "ls", samplePCAP}},
		{"pcap_ls_proto_tcp", []string{"pcap", "ls", "--proto", "TCP", samplePCAP}},
		{"pcap_ls_filter_example", []string{"pcap", "ls", "--filter", "example.com", samplePCAP}},
		{"pcap_ls_sort_len_desc", []string{"pcap", "ls", "--sort", "len:desc", samplePCAP}},
		{"pcap_ls_limit_5", []string{"pcap", "ls", "--limit", "5", samplePCAP}},
		{"pcap_ls_json", []string{"pcap", "ls", "--json", samplePCAP}},
		{"pcap_show_dns", []string{"pcap", "show", "3", samplePCAP}},
		{"pcap_show_http", []string{"pcap", "show", "8", samplePCAP}},
		{"pcap_show_tls", []string{"pcap", "show", "14", samplePCAP}},
		{"pcap_show_json", []string{"pcap", "show", "--json", "5", samplePCAP}},
		{"pcap_flows", []string{"pcap", "flows", samplePCAP}},
		{"pcap_flows_json", []string{"pcap", "flows", "--json", samplePCAP}},
		{"pcap_stats", []string{"pcap", "stats", samplePCAP}},
		{"pcap_stats_json", []string{"pcap", "stats", "--json", samplePCAP}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if code := run(tc.args, "vTEST", &out, &errOut); code != 0 {
				t.Fatalf("run(%v) exit=%d stderr=%q", tc.args, code, errOut.String())
			}
			golden := filepath.Join("testdata", "golden", tc.name+".golden")
			if *updateGolden {
				if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(golden, out.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("read golden (run with -update to create): %v", err)
			}
			if !bytes.Equal(out.Bytes(), want) {
				t.Errorf("output mismatch for %s\n--- got ---\n%s--- want ---\n%s", tc.name, out.String(), want)
			}
		})
	}
}

func TestPcapMissingSubcommand(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"pcap"}, "dev", &out, &errOut); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if out.Len() != 0 {
		t.Errorf("unexpected stdout: %q", out.String())
	}
}

func TestPcapUnknownSubcommand(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"pcap", "bogus", samplePCAP}, "dev", &out, &errOut); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

// TestPcapHelpOnStdout verifies that `hhb pcap help` prints the group usage —
// with each subcommand's syntax and the ls flags — to stdout and exits 0.
func TestPcapHelpOnStdout(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"pcap", "help"}, "dev", &out, &errOut); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	for _, want := range []string{"Usage:", "hhb pcap", "ls", "show", "flows", "stats", "--proto", "--json"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("pcap help output missing %q", want)
		}
	}
}

func TestPcapShowOutOfRange(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"pcap", "show", "999", samplePCAP}, "dev", &out, &errOut); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if out.Len() != 0 {
		t.Errorf("unexpected stdout: %q", out.String())
	}
}

// TestPcapNoInputAtTerminal verifies that a pcap command with no file and an
// interactive stdin reports a usage error instead of blocking on the terminal.
func TestPcapNoInputAtTerminal(t *testing.T) {
	orig := stdinIsTerminal
	defer func() { stdinIsTerminal = orig }()
	stdinIsTerminal = func() bool { return true }

	for _, args := range [][]string{{"pcap", "ls"}, {"pcap", "flows"}, {"pcap", "stats"}} {
		var out, errOut bytes.Buffer
		if code := run(args, "dev", &out, &errOut); code != 2 {
			t.Errorf("run(%v) exit = %d, want 2", args, code)
		}
		if out.Len() != 0 {
			t.Errorf("run(%v) unexpected stdout: %q", args, out.String())
		}
	}
}
