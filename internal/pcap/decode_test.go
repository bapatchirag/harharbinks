package pcap

import "testing"

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
