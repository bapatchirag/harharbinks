package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
	"text/tabwriter"
	"unicode/utf8"

	"github.com/bapatchirag/harharbinks/internal/har"
)

// writeTable renders rows as an aligned summary table.
func writeTable(w io.Writer, rows []row) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "#\tMETHOD\tSTATUS\tTYPE\tSIZE\tTIME\tURL")
	for _, r := range rows {
		e := r.Entry
		fmt.Fprintf(tw, "%d\t%s\t%d\t%s\t%s\t%s\t%s\n",
			r.Index,
			e.Request.Method,
			e.Response.Status,
			shortType(e.Response.Content.MimeType),
			humanSize(e.Response.Content.Size),
			humanMS(e.Time),
			e.Request.URL,
		)
	}
	tw.Flush()
}

// writeDetail renders a single entry as grouped, human-readable sections.
func writeDetail(w io.Writer, index int, e har.Entry) {
	fmt.Fprintf(w, "Entry %d\n", index)
	fmt.Fprintf(w, "  Method:    %s\n", e.Request.Method)
	fmt.Fprintf(w, "  URL:       %s\n", e.Request.URL)
	fmt.Fprintf(w, "  Status:    %d %s\n", e.Response.Status, e.Response.StatusText)
	fmt.Fprintf(w, "  HTTP:      %s\n", e.Response.HTTPVersion)
	if e.ServerIPAddress != "" {
		fmt.Fprintf(w, "  Server IP: %s\n", e.ServerIPAddress)
	}
	fmt.Fprintf(w, "  Time:      %s\n", humanMS(e.Time))
	fmt.Fprintf(w, "  Size:      %s\n", humanSize(e.Response.Content.Size))

	writePairs(w, "Request Headers", ": ", e.Request.Headers)
	writePairs(w, "Query String", " = ", e.Request.QueryString)
	writeCookies(w, "Request Cookies", e.Request.Cookies)

	if pd := e.Request.PostData; pd != nil && (pd.Text != "" || len(pd.Params) > 0) {
		fmt.Fprintf(w, "\nRequest Body (%s)\n", nonEmpty(pd.MimeType, "-"))
		if pd.Text != "" {
			writeBody(w, []byte(pd.Text), pd.MimeType)
		} else {
			for _, p := range pd.Params {
				fmt.Fprintf(w, "  %s = %s\n", p.Name, p.Value)
			}
		}
	}

	writePairs(w, "Response Headers", ": ", e.Response.Headers)
	writeCookies(w, "Response Cookies", e.Response.Cookies)

	fmt.Fprintf(w, "\nResponse Body (%s, %s)\n",
		nonEmpty(e.Response.Content.MimeType, "-"), humanSize(e.Response.Content.Size))
	body, err := e.Response.Content.Body()
	if err != nil {
		fmt.Fprintf(w, "  <error decoding body: %v>\n", err)
		return
	}
	writeBody(w, body, e.Response.Content.MimeType)
}

func writePairs(w io.Writer, title, sep string, pairs []har.NameValue) {
	if len(pairs) == 0 {
		return
	}
	fmt.Fprintf(w, "\n%s\n", title)
	for _, p := range pairs {
		fmt.Fprintf(w, "  %s%s%s\n", p.Name, sep, p.Value)
	}
}

func writeCookies(w io.Writer, title string, cookies []har.Cookie) {
	if len(cookies) == 0 {
		return
	}
	fmt.Fprintf(w, "\n%s\n", title)
	for _, c := range cookies {
		fmt.Fprintf(w, "  %s = %s\n", c.Name, c.Value)
	}
}

// writeBody prints a body, pretty-printing JSON and summarizing binary content.
func writeBody(w io.Writer, body []byte, mimeType string) {
	if len(body) == 0 {
		fmt.Fprintln(w, "  (empty)")
		return
	}
	if !utf8.Valid(body) {
		fmt.Fprintf(w, "  [binary data, %d bytes]\n", len(body))
		return
	}
	text := string(body)
	if isJSON(mimeType) {
		var buf bytes.Buffer
		if json.Indent(&buf, body, "", "  ") == nil {
			text = buf.String()
		}
	}
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		fmt.Fprintf(w, "  %s\n", line)
	}
}

// entrySummary is the compact JSON shape emitted by `hhb ls --json`.
type entrySummary struct {
	Index    int     `json:"index"`
	Method   string  `json:"method"`
	Status   int     `json:"status"`
	MimeType string  `json:"mimeType"`
	Size     int     `json:"size"`
	Time     float64 `json:"time"`
	URL      string  `json:"url"`
}

func writeRowsJSON(w io.Writer, rows []row) error {
	out := make([]entrySummary, len(rows))
	for i, r := range rows {
		out[i] = entrySummary{
			Index:    r.Index,
			Method:   r.Entry.Request.Method,
			Status:   r.Entry.Response.Status,
			MimeType: r.Entry.Response.Content.MimeType,
			Size:     r.Entry.Response.Content.Size,
			Time:     r.Entry.Time,
			URL:      r.Entry.Request.URL,
		}
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func writeEntryJSON(w io.Writer, e har.Entry) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(e)
}

// shortType reduces a MIME type to its subtype (e.g. "application/json" -> "json").
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

func isJSON(mime string) bool {
	return strings.Contains(strings.ToLower(mime), "json")
}

// humanSize formats a byte count with a binary unit suffix.
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

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
