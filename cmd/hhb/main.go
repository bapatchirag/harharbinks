// Command hhb is the harharbinks command-line entry point. The binary is named
// "hhb" for brevity, but all help and version output refers to the product by
// its full name, "harharbinks".
package main

import (
	"os"

	"github.com/bapatchirag/harharbinks/internal/cli"
)

// version is the harharbinks release version. It defaults to "dev" and is
// overridden at build time via -ldflags "-X main.version=<version>".
var version = "dev"

func main() {
	os.Exit(cli.Run(os.Args[1:], version))
}
