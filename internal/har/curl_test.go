package har

import (
	"strings"
	"testing"
)

func TestCurlGetOmitsMethodAndIncludesHeaders(t *testing.T) {
	h := loadSample(t)
	got := Curl(h.Log.Entries[0])
	if strings.Contains(got, "-X") {
		t.Errorf("GET curl should not include -X: %q", got)
	}
	if !strings.Contains(got, "'https://api.example.com/users?page=2&limit=20'") {
		t.Errorf("curl missing quoted URL: %q", got)
	}
	if !strings.Contains(got, "-H 'Accept: application/json'") {
		t.Errorf("curl missing header: %q", got)
	}
}

func TestCurlPostIncludesMethodAndData(t *testing.T) {
	h := loadSample(t)
	got := Curl(h.Log.Entries[1])
	if !strings.Contains(got, "-X POST") {
		t.Errorf("POST curl missing -X POST: %q", got)
	}
	if !strings.Contains(got, `--data '{"username":"ada","password":"s3cr3t"}'`) {
		t.Errorf("curl missing --data body: %q", got)
	}
}

func TestCurlEscapesSingleQuotes(t *testing.T) {
	e := Entry{Request: Request{Method: "GET", URL: "https://x.test/a'b"}}
	got := Curl(e)
	if !strings.Contains(got, `'https://x.test/a'\''b'`) {
		t.Errorf("curl did not escape single quote: %q", got)
	}
}
