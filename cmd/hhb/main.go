// Command hhb is the harharbinks command-line entry point. The binary is named
// "hhb" for brevity, but all help and version output refers to the product by
// its full name, "harharbinks".
package main

import (
	"os"
	"runtime/debug"

	"github.com/bapatchirag/harharbinks/internal/cli"
)

// version is the harharbinks release version. Release builds (goreleaser and
// `make build`) inject it via -ldflags "-X main.version=<version>". It defaults
// to "dev"; see resolveVersion for the `go install pkg@vX.Y.Z` fallback.
var version = "dev"

func main() {
	os.Exit(cli.Run(os.Args[1:], resolveVersion()))
}

// resolveVersion returns the ldflags-injected version when present. Otherwise
// (e.g. a plain `go install github.com/bapatchirag/harharbinks/cmd/hhb@vX.Y.Z`,
// which applies no ldflags) it falls back to the module version recorded in the
// binary's build info, and finally to the "dev" default.
func resolveVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return version
}
