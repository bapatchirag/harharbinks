#!/usr/bin/env bash
# cli.sh — render the headless-CLI stills for docs/screenshots.md with
# charmbracelet/freeze. Run after `make build`; invoked by `make screenshots`.
#
#   docs/tapes/cli.sh
#
# Requires freeze (go install github.com/charmbracelet/freeze@latest). Output is
# piped so hhb prints its full, unwrapped headless output (no TTY width limit),
# and freeze renders it as a bordered terminal card.
set -euo pipefail

# Resolve the repository root so the script works from any working directory.
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

bin=./bin/hhb
har=testdata/sample.har
pcap=testdata/sample.pcap
out=docs/images
mkdir -p "$out"

# Shared freeze styling: a rounded, bordered terminal card with window controls.
frame=(--window --padding 20 --border.radius 8 --border.width 1 --border.color "#414868" --background "#1a1b26")

# HAR headless commands.
"$bin" ls     "$har" | freeze --language ansi -o "$out/cli-ls.png"   "${frame[@]}"
"$bin" show 2 "$har" | freeze --language ansi -o "$out/cli-show.png" "${frame[@]}"
"$bin" curl 2 "$har" | freeze --language bash -o "$out/cli-curl.png" "${frame[@]}"

# PCAP headless commands.
"$bin" pcap ls      "$pcap" | freeze --language ansi -o "$out/cli-pcap-ls.png"    "${frame[@]}"
"$bin" pcap show  8 "$pcap" | freeze --language ansi -o "$out/cli-pcap-show.png"  "${frame[@]}"
"$bin" pcap flows   "$pcap" | freeze --language ansi -o "$out/cli-pcap-flows.png" "${frame[@]}"
"$bin" pcap stats   "$pcap" | freeze --language ansi -o "$out/cli-pcap-stats.png" "${frame[@]}"

echo "Wrote $out/cli-ls.png, $out/cli-show.png, $out/cli-curl.png,"
echo "      $out/cli-pcap-ls.png, $out/cli-pcap-show.png, $out/cli-pcap-flows.png, $out/cli-pcap-stats.png"
