package cli
package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunVersionPrintsProductAndVersion(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"--version"}, "v1.2.3", &out, &errOut); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	got := out.String()
	if !strings.Contains(got, ProductName) {
		t.Errorf("output %q does not contain product name %q", got, ProductName)
	}
	if !strings.Contains(got, "v1.2.3") {
		t.Errorf("output %q does not contain the version", got)
	}
}

func TestRunNoArgsShowsUsage(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run(nil, "dev", &out, &errOut); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Errorf("expected usage text, got %q", out.String())
	}
}

func TestRunHelpFlagShowsUsageOnStdout(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"--help"}, "dev", &out, &errOut); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), ProductName) {
		t.Errorf("expected product name in help output, got %q", out.String())
	}
}

func TestRunUnknownFlagReturnsError(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"--nope"}, "dev", &out, &errOut); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}
