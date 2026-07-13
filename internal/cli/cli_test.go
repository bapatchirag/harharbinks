package cli

import (
	"bytes"
	"io"
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

// TestRunNoArgsLaunchesBrowser verifies that a bare invocation (no file, no
// subcommand) opens the interactive file browser rather than printing usage.
func TestRunNoArgsLaunchesBrowser(t *testing.T) {
	orig := launchBrowser
	defer func() { launchBrowser = orig }()
	called := false
	launchBrowser = func(_ io.Writer) int { called = true; return 0 }

	var out, errOut bytes.Buffer
	if code := run(nil, "dev", &out, &errOut); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !called {
		t.Error("no-args run should launch the file browser")
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

// TestHeadlessNoFileAtTerminalErrors verifies that a headless command run with
// no file argument and an interactive stdin (nothing piped) reports a usage
// error instead of blocking on the terminal.
func TestHeadlessNoFileAtTerminalErrors(t *testing.T) {
	orig := stdinIsTerminal
	defer func() { stdinIsTerminal = orig }()
	stdinIsTerminal = func() bool { return true } // simulate an interactive terminal

	for _, args := range [][]string{{"ls"}, {"show", "1"}, {"curl", "1"}} {
		var out, errOut bytes.Buffer
		code := run(args, "dev", &out, &errOut)
		if code != 2 {
			t.Errorf("run(%v) exit = %d, want 2", args, code)
		}
		if !strings.Contains(errOut.String(), "no HAR input") {
			t.Errorf("run(%v) stderr = %q, want a no-input hint", args, errOut.String())
		}
		if out.Len() != 0 {
			t.Errorf("run(%v) unexpected stdout: %q", args, out.String())
		}
	}
}

// TestHeadlessFileArgSkipsTerminalGuard verifies the guard does not fire when a
// file argument is present, even at an interactive terminal.
func TestHeadlessFileArgSkipsTerminalGuard(t *testing.T) {
	orig := stdinIsTerminal
	defer func() { stdinIsTerminal = orig }()
	stdinIsTerminal = func() bool { return true }

	var out, errOut bytes.Buffer
	if code := run([]string{"ls", sampleHAR}, "dev", &out, &errOut); code != 0 {
		t.Fatalf("run ls with a file exit = %d, stderr=%q", code, errOut.String())
	}
	if out.Len() == 0 {
		t.Error("ls with a file should still produce output at a terminal")
	}
}
