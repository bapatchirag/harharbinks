package cli

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// update regenerates golden files: go test ./internal/cli -run TestGolden -update
var update = flag.Bool("update", false, "update golden files")

const sampleHAR = "../../testdata/sample.har"

// TestGolden exercises the headless subcommands against the sample capture and
// compares stdout to checked-in golden files.
func TestGolden(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"ls", []string{"ls", sampleHAR}},
		{"ls_sort_size_desc", []string{"ls", "--sort", "size:desc", sampleHAR}},
		{"ls_method_get", []string{"ls", "--method", "GET", sampleHAR}},
		{"ls_filter_login", []string{"ls", "--filter", "login", sampleHAR}},
		{"ls_flags_after_file", []string{"ls", sampleHAR, "--method", "GET", "--sort", "size:desc"}},
		{"ls_json", []string{"ls", "--json", sampleHAR}},
		{"show_1", []string{"show", "1", sampleHAR}},
		{"show_2", []string{"show", "2", sampleHAR}},
		{"show_3", []string{"show", "3", sampleHAR}},
		{"show_2_json", []string{"show", "--json", "2", sampleHAR}},
		{"curl_2", []string{"curl", "2", sampleHAR}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if code := run(tc.args, "vTEST", &out, &errOut); code != 0 {
				t.Fatalf("run(%v) exit=%d stderr=%q", tc.args, code, errOut.String())
			}
			golden := filepath.Join("testdata", "golden", tc.name+".golden")
			if *update {
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
