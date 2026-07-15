package pcap

import (
	"strings"
	"testing"
)

// packetAt returns the 1-based index'th packet of the sample capture.
func packetAt(t *testing.T, c *Capture, index int) Packet {
	t.Helper()
	if index < 1 || index > len(c.Packets) {
		t.Fatalf("index %d out of range (1..%d)", index, len(c.Packets))
	}
	return c.Packets[index-1]
}

func TestProtocol(t *testing.T) {
	c := loadSample(t)
	want := map[int]string{
		1: "ARP", 2: "ARP",
		3: "DNS", 4: "DNS",
		5: "TCP", 6: "TCP", 7: "TCP",
		8: "HTTP", 9: "HTTP",
		10: "TCP", 11: "TCP", 12: "TCP", 13: "TCP",
		14: "TLS",
		15: "ICMPv4", 16: "ICMPv4",
	}
	for idx, proto := range want {
		if got := packetAt(t, c, idx).Protocol(); got != proto {
			t.Errorf("packet %d protocol = %q, want %q", idx, got, proto)
		}
	}
}

func TestInfo(t *testing.T) {
	c := loadSample(t)
	cases := map[int]string{
		1:  "Who has 192.168.1.1? Tell 192.168.1.100",
		2:  "192.168.1.1 is at bb:bb:bb:bb:bb:bb",
		3:  "Standard query 0x1a2b A example.com",
		4:  "Standard query response 0x1a2b A example.com A 93.184.216.34",
		5:  "50001 → 80 [SYN] Seq=0 Win=64240 Len=0",
		8:  "GET /index.html HTTP/1.1",
		9:  "HTTP/1.1 200 OK",
		14: "TLS 1.2 Client Hello (SNI=example.com)",
		15: "Echo (ping) request id=0x0001 seq=1",
		16: "Echo (ping) reply id=0x0001 seq=1",
	}
	for idx, info := range cases {
		if got := packetAt(t, c, idx).Info(); got != info {
			t.Errorf("packet %d info = %q, want %q", idx, got, info)
		}
	}
}

func TestSourceDest(t *testing.T) {
	c := loadSample(t)
	cases := []struct {
		index    int
		src, dst string
	}{
		{1, "192.168.1.100", "192.168.1.1"}, // ARP uses protocol addresses
		{5, "192.168.1.100", "93.184.216.34"},
		{6, "93.184.216.34", "192.168.1.100"},
	}
	for _, tc := range cases {
		p := packetAt(t, c, tc.index)
		if got := p.Source(); got != tc.src {
			t.Errorf("packet %d source = %q, want %q", tc.index, got, tc.src)
		}
		if got := p.Dest(); got != tc.dst {
			t.Errorf("packet %d dest = %q, want %q", tc.index, got, tc.dst)
		}
	}
}

func TestLayerStack(t *testing.T) {
	c := loadSample(t)
	stack := packetAt(t, c, 3).LayerStack() // DNS query over UDP
	var names []string
	for _, l := range stack {
		names = append(names, l.Name)
	}
	want := []string{"Ethernet", "IPv4", "UDP", "DNS"}
	if len(names) != len(want) {
		t.Fatalf("layer names = %v, want %v", names, want)
	}
	for i, n := range want {
		if names[i] != n {
			t.Errorf("layer %d = %q, want %q", i, names[i], n)
		}
	}
	if stack[0].Summary == "" {
		t.Error("Ethernet layer summary should not be empty")
	}
	// Byte offsets tile the frame: Ethernet(14) + IPv4(20) + UDP(8) then DNS.
	wantOffsets := []int{0, 14, 34, 42}
	for i, off := range wantOffsets {
		if stack[i].Offset != off {
			t.Errorf("layer %d (%s) offset = %d, want %d", i, stack[i].Name, stack[i].Offset, off)
		}
	}
	// Layers expose decoded fields for the tree view.
	if len(stack[0].Fields) == 0 {
		t.Error("Ethernet layer should expose fields")
	}
	if len(stack[3].Fields) == 0 {
		t.Error("DNS layer should expose fields")
	}
}

// TestLayerStackContiguous checks that every packet's layers tile its frame with
// no gaps or overruns, so a detail view can map any layer to a valid byte range.
func TestLayerStackContiguous(t *testing.T) {
	c := loadSample(t)
	for _, p := range c.Packets {
		next := 0
		for i, l := range p.LayerStack() {
			if l.Offset != next {
				t.Errorf("packet %d layer %d (%s) offset = %d, want %d", p.Index, i, l.Name, l.Offset, next)
			}
			if l.Length < 0 || l.Offset+l.Length > len(p.Data) {
				t.Errorf("packet %d layer %d (%s) range [%d,%d) exceeds frame length %d",
					p.Index, i, l.Name, l.Offset, l.Offset+l.Length, len(p.Data))
			}
			next = l.Offset + l.Length
		}
	}
}

// fieldMap indexes a layer's fields by name for convenient assertions.
func fieldMap(fields []Field) map[string]Field {
	m := make(map[string]Field, len(fields))
	for _, f := range fields {
		m[f.Name] = f
	}
	return m
}

// TestLayerFieldsHTTP checks the application payload of a cleartext HTTP request
// decodes into its request line and headers, each pointing at its own bytes.
func TestLayerFieldsHTTP(t *testing.T) {
	c := loadSample(t)
	p := packetAt(t, c, 8) // HTTP GET request
	stack := p.LayerStack()
	app := stack[len(stack)-1]
	if app.Name != "Payload" {
		t.Fatalf("last layer = %q, want Payload", app.Name)
	}
	got := fieldMap(app.Fields)
	if got["Request Line"].Value != "GET /index.html HTTP/1.1" {
		t.Errorf("Request Line = %q, want the GET line", got["Request Line"].Value)
	}
	host := got["Host"]
	if host.Value != "example.com" {
		t.Errorf("Host field = %q, want example.com", host.Value)
	}
	// The Host field's byte range points at its own header line in the frame.
	if s := string(p.Data[host.Offset : host.Offset+host.Length]); s != "Host: example.com" {
		t.Errorf("Host field bytes = %q, want %q", s, "Host: example.com")
	}
}

// TestLayerFieldsTLS checks the application payload of a TLS ClientHello decodes
// into its handshake metadata, including the SNI server name.
func TestLayerFieldsTLS(t *testing.T) {
	c := loadSample(t)
	p := packetAt(t, c, 14) // TLS ClientHello
	stack := p.LayerStack()
	app := stack[len(stack)-1]
	got := fieldMap(app.Fields)
	if got["Content Type"].Value != "handshake" {
		t.Errorf("Content Type = %q, want handshake", got["Content Type"].Value)
	}
	if got["Handshake Type"].Value != "Client Hello" {
		t.Errorf("Handshake Type = %q, want Client Hello", got["Handshake Type"].Value)
	}
	if got["Server Name"].Value != "example.com" {
		t.Errorf("Server Name = %q, want example.com", got["Server Name"].Value)
	}
	// The Content Type field marks the single record-type byte (0x16 handshake).
	if ct := got["Content Type"]; ct.Length != 1 || p.Data[ct.Offset] != 0x16 {
		t.Errorf("Content Type byte at %d = 0x%02x (len %d), want 0x16 len 1", ct.Offset, p.Data[ct.Offset], ct.Length)
	}
}

// TestLayerFieldOffsets checks that fixed-layout fields point at exactly their
// own bytes within the frame.
func TestLayerFieldOffsets(t *testing.T) {
	c := loadSample(t)
	p := packetAt(t, c, 3) // DNS over UDP
	stack := p.LayerStack()

	// Ethernet Source MAC occupies bytes 6..11 (all 0xaa in the fixture).
	src := fieldMap(stack[0].Fields)["Source MAC"]
	if src.Offset != 6 || src.Length != 6 {
		t.Errorf("Source MAC range = [%d,%d), want [6,12)", src.Offset, src.Offset+src.Length)
	}
	for i := src.Offset; i < src.Offset+src.Length; i++ {
		if p.Data[i] != 0xaa {
			t.Errorf("Source MAC byte %d = 0x%02x, want 0xaa", i, p.Data[i])
			break
		}
	}

	// IPv4 TTL is a single byte holding 64.
	if ttl := fieldMap(stack[1].Fields)["TTL"]; ttl.Length != 1 || p.Data[ttl.Offset] != 64 {
		t.Errorf("TTL byte at %d = %d (len %d), want 64 len 1", ttl.Offset, p.Data[ttl.Offset], ttl.Length)
	}
}

// TestFormatHTTPBody checks that a JSON body is pretty-printed while any other
// body is returned unchanged.
func TestFormatHTTPBody(t *testing.T) {
	got := formatHTTPBody([]byte(`{"a":1,"b":[2,3]}`))
	if !strings.Contains(got, "\n") || !strings.Contains(got, `"a": 1`) {
		t.Errorf("JSON body not pretty-printed: %q", got)
	}
	if got := formatHTTPBody([]byte("Hello, world!")); got != "Hello, world!" {
		t.Errorf("plain body = %q, want unchanged", got)
	}
}

// TestParseClientHelloSNI checks the hand-rolled TLS ClientHello parser directly
// against a crafted handshake so its bounds handling is covered without the pcap.
func TestParseClientHelloSNI(t *testing.T) {
	if _, sni := parseClientHello(nil); sni != "" {
		t.Errorf("parseClientHello(nil) sni = %q, want empty", sni)
	}
	// A ServerHello (type 2) is not a ClientHello and must yield nothing.
	if ver, sni := parseClientHello([]byte{2, 0, 0, 0}); ver != "" || sni != "" {
		t.Errorf("parseClientHello(serverHello) = (%q, %q), want empty", ver, sni)
	}
}
