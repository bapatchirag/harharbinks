# harharbinks

[![CI](https://github.com/bapatchirag/harharbinks/actions/workflows/ci.yml/badge.svg)](https://github.com/bapatchirag/harharbinks/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/bapatchirag/harharbinks.svg)](https://pkg.go.dev/github.com/bapatchirag/harharbinks)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**harharbinks** is an offline HAR (HTTP Archive) file viewer that runs entirely in
your terminal. Open a `.har` capture and browse its requests and responses —
headers, cookies, query strings, payloads, bodies, and a reconstructed raw view —
without a browser or any network access.

The command you run is **`hhb`** (short to type); everywhere else the project is
called harharbinks.

> **Status:** under active development. The command-line entry point and release
> pipeline are in place; the interactive TUI and headless subcommands are being
> built out milestone by milestone.

## Install

With the Go toolchain:

```sh
go install github.com/bapatchirag/harharbinks/cmd/hhb@latest
```

Or download a prebuilt binary for your platform from the
[Releases](https://github.com/bapatchirag/harharbinks/releases) page and put it on
your `PATH`.

## Usage

```text
hhb [file.har]            Open a HAR file in the interactive viewer
hhb <command> [args]      Run a headless command

Commands:
  ls    [file]            List HAR entries
  show  <index> [file]    Show details for a single entry
  curl  <index> [file]    Print an entry as a cURL command

Flags:
  --version               Print the harharbinks version and exit
  -h, --help              Show help
```

Check the installed version:

```sh
hhb --version
```

## Development

Requires [Go](https://go.dev/dl/) (latest stable).

```sh
make build          # compile ./bin/hhb
make test           # run tests with the race detector
make vet            # go vet
make fmt-check      # verify gofmt cleanliness
make lint           # golangci-lint (if installed)
make run            # build and run hhb
```

Releases are cut from SemVer tags (`vX.Y.Z`) via
[GoReleaser](https://goreleaser.com/) in GitHub Actions.

## License

[MIT](LICENSE) © 2026 Chirag Bapat
