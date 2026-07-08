package har

import (
	"fmt"
	"strings"
)

// Curl renders an entry's request as a copy-pasteable cURL command. Headers are
// emitted with -H and any request body with --data. Values are single-quoted so
// the command is safe to paste into a POSIX shell.
func Curl(e Entry) string {
	var b strings.Builder
	b.WriteString("curl")
	if m := strings.ToUpper(e.Request.Method); m != "" && m != "GET" {
		fmt.Fprintf(&b, " -X %s", m)
	}
	fmt.Fprintf(&b, " %s", shellQuote(e.Request.URL))
	for _, h := range e.Request.Headers {
		fmt.Fprintf(&b, " \\\n  -H %s", shellQuote(h.Name+": "+h.Value))
	}
	if e.Request.PostData != nil && e.Request.PostData.Text != "" {
		fmt.Fprintf(&b, " \\\n  --data %s", shellQuote(e.Request.PostData.Text))
	}
	return b.String()
}

// shellQuote wraps s in single quotes, escaping any embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
