package har

import (
	"strings"
	"testing"
)

const samplePath = "../../testdata/sample.har"

func loadSample(t *testing.T) *HAR {
	t.Helper()
	h, err := ParseFile(samplePath)
	if err != nil {
		t.Fatalf("ParseFile(%s): %v", samplePath, err)
	}
	return h
}

func TestParseFileEntries(t *testing.T) {
	h := loadSample(t)
	if got := len(h.Log.Entries); got != 3 {
		t.Fatalf("entries = %d, want 3", got)
	}
	e0 := h.Log.Entries[0]
	if e0.Request.Method != "GET" {
		t.Errorf("entry0 method = %q, want GET", e0.Request.Method)
	}
	if e0.Response.Status != 200 {
		t.Errorf("entry0 status = %d, want 200", e0.Response.Status)
	}
	if len(e0.Request.QueryString) != 2 {
		t.Errorf("entry0 query params = %d, want 2", len(e0.Request.QueryString))
	}
	if len(e0.Request.Cookies) != 1 {
		t.Errorf("entry0 request cookies = %d, want 1", len(e0.Request.Cookies))
	}
}

func TestParseInvalidJSON(t *testing.T) {
	if _, err := Parse(strings.NewReader("{ not valid")); err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestContentBodyBase64DecodesPNG(t *testing.T) {
	h := loadSample(t)
	c := h.Log.Entries[2].Response.Content // base64 PNG
	if c.Encoding != "base64" {
		t.Fatalf("entry2 encoding = %q, want base64", c.Encoding)
	}
	body, err := c.Body()
	if err != nil {
		t.Fatalf("Body: %v", err)
	}
	if len(body) < 8 || body[0] != 0x89 || string(body[1:4]) != "PNG" {
		t.Errorf("decoded bytes are not a PNG signature: % x", body[:min(8, len(body))])
	}
}

func TestContentBodyPlainText(t *testing.T) {
	body, err := Content{Text: "hello"}.Body()
	if err != nil {
		t.Fatalf("Body: %v", err)
	}
	if string(body) != "hello" {
		t.Errorf("body = %q, want %q", body, "hello")
	}
}

func TestContentBodyBadBase64(t *testing.T) {
	if _, err := (Content{Encoding: "base64", Text: "!!!not-base64!!!"}).Body(); err == nil {
		t.Fatal("expected error decoding invalid base64")
	}
}
