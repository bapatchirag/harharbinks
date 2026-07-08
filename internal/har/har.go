// Package har models the HTTP Archive (HAR 1.2) format and provides pure,
// UI-free helpers for parsing, querying, and rendering HAR captures. It has no
// dependency on the TUI or CLI layers and is safe to reuse on its own.
package har

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// HAR is the root of a HAR document.
type HAR struct {
	Log Log `json:"log"`
}

// Log holds the capture metadata and the recorded entries.
type Log struct {
	Version string   `json:"version"`
	Creator Creator  `json:"creator"`
	Browser *Creator `json:"browser,omitempty"`
	Pages   []Page   `json:"pages,omitempty"`
	Entries []Entry  `json:"entries"`
}

// Creator identifies the tool (or browser) that produced the capture.
type Creator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Page describes a single navigated page referenced by entries.
type Page struct {
	ID              string         `json:"id"`
	Title           string         `json:"title"`
	StartedDateTime string         `json:"startedDateTime"`
	PageTimings     map[string]any `json:"pageTimings,omitempty"`
}

// Entry is a single request/response exchange.
type Entry struct {
	PageRef         string   `json:"pageref,omitempty"`
	StartedDateTime string   `json:"startedDateTime"`
	Time            float64  `json:"time"`
	Request         Request  `json:"request"`
	Response        Response `json:"response"`
	Timings         Timings  `json:"timings"`
	ServerIPAddress string   `json:"serverIPAddress,omitempty"`
	Connection      string   `json:"connection,omitempty"`
}

// Request is the outbound side of an entry.
type Request struct {
	Method      string      `json:"method"`
	URL         string      `json:"url"`
	HTTPVersion string      `json:"httpVersion"`
	Headers     []NameValue `json:"headers"`
	QueryString []NameValue `json:"queryString"`
	Cookies     []Cookie    `json:"cookies"`
	PostData    *PostData   `json:"postData,omitempty"`
	HeadersSize int         `json:"headersSize"`
	BodySize    int         `json:"bodySize"`
}

// Response is the inbound side of an entry.
type Response struct {
	Status      int         `json:"status"`
	StatusText  string      `json:"statusText"`
	HTTPVersion string      `json:"httpVersion"`
	Headers     []NameValue `json:"headers"`
	Cookies     []Cookie    `json:"cookies"`
	Content     Content     `json:"content"`
	RedirectURL string      `json:"redirectURL"`
	HeadersSize int         `json:"headersSize"`
	BodySize    int         `json:"bodySize"`
}

// NameValue is a generic name/value pair (headers, query params).
type NameValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Cookie is a request or response cookie.
type Cookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Path     string `json:"path,omitempty"`
	Domain   string `json:"domain,omitempty"`
	Expires  string `json:"expires,omitempty"`
	HTTPOnly bool   `json:"httpOnly,omitempty"`
	Secure   bool   `json:"secure,omitempty"`
}

// PostData is the request body.
type PostData struct {
	MimeType string      `json:"mimeType"`
	Text     string      `json:"text,omitempty"`
	Params   []NameValue `json:"params,omitempty"`
}

// Content is the response body.
type Content struct {
	Size     int    `json:"size"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text,omitempty"`
	Encoding string `json:"encoding,omitempty"`
}

// Timings breaks down the phases of an exchange, in milliseconds.
type Timings struct {
	Blocked float64 `json:"blocked"`
	DNS     float64 `json:"dns"`
	Connect float64 `json:"connect"`
	Send    float64 `json:"send"`
	Wait    float64 `json:"wait"`
	Receive float64 `json:"receive"`
	SSL     float64 `json:"ssl"`
}

// Parse decodes a HAR document from r.
func Parse(r io.Reader) (*HAR, error) {
	var h HAR
	if err := json.NewDecoder(r).Decode(&h); err != nil {
		return nil, fmt.Errorf("parse HAR: %w", err)
	}
	return &h, nil
}

// ParseFile decodes a HAR document from the file at path.
func ParseFile(path string) (*HAR, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(f)
}

// Body returns the response body bytes, decoding base64 when the content is
// base64-encoded (as indicated by Encoding).
func (c Content) Body() ([]byte, error) {
	if strings.EqualFold(c.Encoding, "base64") {
		b, err := base64.StdEncoding.DecodeString(c.Text)
		if err != nil {
			return nil, fmt.Errorf("decode base64 content: %w", err)
		}
		return b, nil
	}
	return []byte(c.Text), nil
}
