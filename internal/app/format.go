// This file holds small, self-contained formatting helpers used by the HAR
// screens. They mirror the headless CLI's presentation (human-readable sizes and
// durations) but are duplicated here deliberately: the app layer must not depend
// on internal/cli, which in turn depends on this package.
package app

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// humanSize formats a byte count with a binary unit suffix (e.g. "88.2KB").
func humanSize(n int) string {
	if n < 1024 {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := int64(1024), 0
	for v := int64(n) / 1024; v >= 1024; v /= 1024 {
		div *= 1024
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// humanMS formats a duration in milliseconds, switching to seconds at >= 1s.
func humanMS(ms float64) string {
	if ms >= 1000 {
		return fmt.Sprintf("%.2fs", ms/1000)
	}
	return fmt.Sprintf("%dms", int(math.Round(ms)))
}

// nonEmpty returns s, or fallback when s is empty.
func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// shortType reduces a MIME type to its subtype (e.g. "application/json" -> "json"),
// matching the headless CLI's TYPE column. It returns "-" for an empty type.
func shortType(mime string) string {
	if mime == "" {
		return "-"
	}
	if i := strings.IndexByte(mime, ';'); i >= 0 {
		mime = mime[:i]
	}
	mime = strings.TrimSpace(mime)
	if i := strings.IndexByte(mime, '/'); i >= 0 {
		return mime[i+1:]
	}
	return mime
}

// padTo right-pads s with spaces to exactly w display cells (ANSI-aware). It
// never truncates; callers pass short, known-width strings.
func padTo(s string, w int) string {
	if pad := w - ansi.StringWidth(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}
