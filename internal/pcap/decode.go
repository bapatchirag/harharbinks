package pcap

import (
	"fmt"
	"net"
	"strings"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
)

// Layer is one decoded protocol layer's name and a short human-readable summary.
// It lets packet-detail views present the layer stack without importing gopacket.
type Layer struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
}

// Protocol returns the short name of the packet's most specific decoded protocol
// (e.g. "HTTP", "DNS", "TLS", "TCP", "UDP", "ICMPv4", "ARP"), suitable for the
// protocol column of a packet list.
func (p Packet) Protocol() string {
	pkt := p.decoded
	if pkt.Layer(layers.LayerTypeDNS) != nil {
		return "DNS"
	}
	if tcp, ok := tcpLayer(pkt); ok {
		switch {
		case isTLSRecord(tcp.Payload):
			return "TLS"
		case httpKind(tcp.Payload) != "":
			return "HTTP"
		default:
			return "TCP"
		}
	}
	if pkt.Layer(layers.LayerTypeUDP) != nil {
		return "UDP"
	}
	if pkt.Layer(layers.LayerTypeICMPv4) != nil {
		return "ICMPv4"
	}
	if pkt.Layer(layers.LayerTypeICMPv6) != nil {
		return "ICMPv6"
	}
	if pkt.Layer(layers.LayerTypeARP) != nil {
		return "ARP"
	}
	if nl := pkt.NetworkLayer(); nl != nil {
		return nl.LayerType().String()
	}
	if ll := pkt.LinkLayer(); ll != nil {
		return ll.LayerType().String()
	}
	return "Unknown"
}

// Info returns a one-line, Wireshark-style summary of the packet's most specific
// protocol, suitable for the info column of a packet list.
func (p Packet) Info() string {
	pkt := p.decoded
	if dns, ok := pkt.Layer(layers.LayerTypeDNS).(*layers.DNS); ok {
		return dnsInfo(dns)
	}
	if arp, ok := pkt.Layer(layers.LayerTypeARP).(*layers.ARP); ok {
		return arpInfo(arp)
	}
	if icmp, ok := pkt.Layer(layers.LayerTypeICMPv4).(*layers.ICMPv4); ok {
		return icmpInfo(icmp)
	}
	if tcp, ok := tcpLayer(pkt); ok {
		switch {
		case isTLSRecord(tcp.Payload):
			return tlsInfo(tcp.Payload)
		case httpKind(tcp.Payload) != "":
			return httpFirstLine(tcp.Payload)
		default:
			return tcpInfo(tcp)
		}
	}
	if udp, ok := pkt.Layer(layers.LayerTypeUDP).(*layers.UDP); ok {
		return udpInfo(udp)
	}
	return p.Protocol()
}

// Source returns the packet's source address for a list view: the network-layer
// address when present, the ARP sender's IP for ARP, otherwise the link address.
func (p Packet) Source() string { return endpoint(p.decoded, true) }

// Dest returns the packet's destination address, mirroring Source.
func (p Packet) Dest() string { return endpoint(p.decoded, false) }

// LayerStack returns the packet's decoded layers from outermost (link) to
// innermost (application), each with a short summary.
func (p Packet) LayerStack() []Layer {
	var out []Layer
	for _, l := range p.decoded.Layers() {
		out = append(out, Layer{Name: l.LayerType().String(), Summary: layerSummary(l)})
	}
	return out
}

// tcpLayer returns the packet's TCP layer, if it has one.
func tcpLayer(pkt gopacket.Packet) (*layers.TCP, bool) {
	if l, ok := pkt.Layer(layers.LayerTypeTCP).(*layers.TCP); ok {
		return l, true
	}
	return nil, false
}

// endpoint returns a packet's source (src true) or destination address string.
func endpoint(pkt gopacket.Packet, src bool) string {
	if nl := pkt.NetworkLayer(); nl != nil {
		f := nl.NetworkFlow()
		if src {
			return f.Src().String()
		}
		return f.Dst().String()
	}
	if arp, ok := pkt.Layer(layers.LayerTypeARP).(*layers.ARP); ok {
		if src {
			return net.IP(arp.SourceProtAddress).String()
		}
		return net.IP(arp.DstProtAddress).String()
	}
	if ll := pkt.LinkLayer(); ll != nil {
		f := ll.LinkFlow()
		if src {
			return f.Src().String()
		}
		return f.Dst().String()
	}
	return ""
}

// dnsInfo renders a DNS query or response like Wireshark's info column.
func dnsInfo(dns *layers.DNS) string {
	var b strings.Builder
	b.WriteString("Standard query")
	if dns.QR {
		b.WriteString(" response")
	}
	fmt.Fprintf(&b, " 0x%04x", dns.ID)
	for _, q := range dns.Questions {
		fmt.Fprintf(&b, " %s %s", q.Type, q.Name)
	}
	if dns.QR {
		for _, a := range dns.Answers {
			switch a.Type {
			case layers.DNSTypeA, layers.DNSTypeAAAA:
				if a.IP != nil {
					fmt.Fprintf(&b, " %s %s", a.Type, a.IP)
				}
			case layers.DNSTypeCNAME:
				fmt.Fprintf(&b, " CNAME %s", a.CNAME)
			default:
				fmt.Fprintf(&b, " %s", a.Type)
			}
		}
	}
	return b.String()
}

// arpInfo renders an ARP request or reply.
func arpInfo(arp *layers.ARP) string {
	switch arp.Operation {
	case layers.ARPRequest:
		return fmt.Sprintf("Who has %s? Tell %s",
			net.IP(arp.DstProtAddress), net.IP(arp.SourceProtAddress))
	case layers.ARPReply:
		return fmt.Sprintf("%s is at %s",
			net.IP(arp.SourceProtAddress), net.HardwareAddr(arp.SourceHwAddress))
	default:
		return fmt.Sprintf("ARP operation %d", arp.Operation)
	}
}

// icmpInfo renders an ICMPv4 message, naming the common echo request/reply types.
func icmpInfo(icmp *layers.ICMPv4) string {
	switch icmp.TypeCode.Type() {
	case layers.ICMPv4TypeEchoRequest:
		return fmt.Sprintf("Echo (ping) request id=0x%04x seq=%d", icmp.Id, icmp.Seq)
	case layers.ICMPv4TypeEchoReply:
		return fmt.Sprintf("Echo (ping) reply id=0x%04x seq=%d", icmp.Id, icmp.Seq)
	default:
		return icmp.TypeCode.String()
	}
}

// tcpInfo renders a bare TCP segment with its flags, sequence, and payload length.
func tcpInfo(tcp *layers.TCP) string {
	var flags []string
	for _, f := range []struct {
		set  bool
		name string
	}{
		{tcp.SYN, "SYN"}, {tcp.ACK, "ACK"}, {tcp.FIN, "FIN"},
		{tcp.RST, "RST"}, {tcp.PSH, "PSH"}, {tcp.URG, "URG"},
	} {
		if f.set {
			flags = append(flags, f.name)
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d → %d [%s] Seq=%d", uint16(tcp.SrcPort), uint16(tcp.DstPort),
		strings.Join(flags, ", "), tcp.Seq)
	if tcp.ACK {
		fmt.Fprintf(&b, " Ack=%d", tcp.Ack)
	}
	fmt.Fprintf(&b, " Win=%d Len=%d", tcp.Window, len(tcp.Payload))
	return b.String()
}

// udpInfo renders a bare UDP datagram's ports and payload length.
func udpInfo(udp *layers.UDP) string {
	return fmt.Sprintf("%d → %d Len=%d", uint16(udp.SrcPort), uint16(udp.DstPort), len(udp.Payload))
}

// httpMethods are the request-line prefixes used to recognize a cleartext HTTP
// request at the start of a TCP payload.
var httpMethods = []string{"GET ", "POST ", "PUT ", "DELETE ", "HEAD ", "OPTIONS ", "PATCH ", "TRACE ", "CONNECT "}

// httpKind reports whether a TCP payload begins with a cleartext HTTP request
// ("request") or response ("response"), or "" when it is neither.
func httpKind(payload []byte) string {
	line := httpFirstLine(payload)
	if strings.HasPrefix(line, "HTTP/") {
		return "response"
	}
	for _, m := range httpMethods {
		if strings.HasPrefix(line, m) {
			return "request"
		}
	}
	return ""
}

// httpFirstLine returns the first CRLF-delimited line of a payload, trimmed of
// its trailing carriage return and capped so a binary payload cannot produce an
// unbounded string.
func httpFirstLine(payload []byte) string {
	const max = 256
	end := len(payload)
	if end > max {
		end = max
	}
	s := string(payload[:end])
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimRight(s, "\r")
}

// isTLSRecord reports whether payload begins with what looks like a TLS record: a
// known content type followed by a 0x03 major version byte.
func isTLSRecord(payload []byte) bool {
	if len(payload) < 5 {
		return false
	}
	switch payload[0] {
	case 20, 21, 22, 23: // change_cipher_spec, alert, handshake, application_data
		return payload[1] == 0x03
	default:
		return false
	}
}

// tlsInfo renders a one-line summary of a TLS record, decoding the handshake type
// and — for a ClientHello — the negotiated version and SNI server name.
func tlsInfo(payload []byte) string {
	recVer := tlsVersion(payload[1], payload[2])
	switch payload[0] {
	case 22: // handshake
		hs := payload[5:]
		if len(hs) == 0 {
			return "TLS " + recVer + " Handshake"
		}
		switch hs[0] {
		case 1: // ClientHello
			ver, sni := parseClientHello(hs)
			if ver == "" {
				ver = recVer
			}
			if sni != "" {
				return fmt.Sprintf("TLS %s Client Hello (SNI=%s)", ver, sni)
			}
			return fmt.Sprintf("TLS %s Client Hello", ver)
		case 2: // ServerHello
			return fmt.Sprintf("TLS %s Server Hello", recVer)
		default:
			return fmt.Sprintf("TLS %s Handshake", recVer)
		}
	case 21:
		return "TLS " + recVer + " Alert"
	case 20:
		return "TLS " + recVer + " Change Cipher Spec"
	case 23:
		return "TLS " + recVer + " Application Data"
	default:
		return "TLS " + recVer
	}
}

// tlsVersion maps a TLS record or handshake version byte pair to a short label.
func tlsVersion(major, minor byte) string {
	if major != 3 {
		return fmt.Sprintf("0x%02x%02x", major, minor)
	}
	switch minor {
	case 0:
		return "SSL 3.0"
	case 1:
		return "1.0"
	case 2:
		return "1.1"
	case 3:
		return "1.2"
	case 4:
		return "1.3"
	default:
		return fmt.Sprintf("0x03%02x", minor)
	}
}

// parseClientHello extracts the legacy client_version and the SNI host name from
// a TLS handshake that begins with a ClientHello. hs is the handshake bytes
// following the 5-byte TLS record header. It returns empty strings when the
// message is not a well-formed ClientHello.
func parseClientHello(hs []byte) (version, sni string) {
	// Handshake header: type(1) + length(3); type 1 is ClientHello.
	if len(hs) < 4 || hs[0] != 1 {
		return "", ""
	}
	b := hs[4:]
	// client_version(2) + random(32).
	if len(b) < 34 {
		return "", ""
	}
	version = tlsVersion(b[0], b[1])
	off := 34
	// session_id: length(1) + id.
	if off >= len(b) {
		return version, ""
	}
	off += 1 + int(b[off])
	// cipher_suites: length(2) + suites.
	if off+2 > len(b) {
		return version, ""
	}
	off += 2 + (int(b[off])<<8 | int(b[off+1]))
	// compression_methods: length(1) + methods.
	if off+1 > len(b) {
		return version, ""
	}
	off += 1 + int(b[off])
	// extensions: length(2) + extensions.
	if off+2 > len(b) {
		return version, ""
	}
	extEnd := off + 2 + (int(b[off])<<8 | int(b[off+1]))
	off += 2
	if extEnd > len(b) {
		extEnd = len(b)
	}
	for off+4 <= extEnd {
		extType := int(b[off])<<8 | int(b[off+1])
		extLen := int(b[off+2])<<8 | int(b[off+3])
		off += 4
		if off+extLen > extEnd {
			break
		}
		if extType == 0x0000 { // server_name
			return version, parseSNIExtension(b[off : off+extLen])
		}
		off += extLen
	}
	return version, ""
}

// parseSNIExtension returns the first host_name in a TLS server_name extension.
func parseSNIExtension(ext []byte) string {
	// server_name_list: length(2), then entries of type(1) + length(2) + name.
	if len(ext) < 2 {
		return ""
	}
	list := ext[2:]
	if n := int(ext[0])<<8 | int(ext[1]); n < len(list) {
		list = list[:n]
	}
	for len(list) >= 3 {
		nameType := list[0]
		nameLen := int(list[1])<<8 | int(list[2])
		list = list[3:]
		if nameLen > len(list) {
			break
		}
		if nameType == 0 { // host_name
			return string(list[:nameLen])
		}
		list = list[nameLen:]
	}
	return ""
}

// layerSummary returns a short human-readable description for a decoded layer,
// reusing the same renderers as the packet info column where they apply.
func layerSummary(l gopacket.Layer) string {
	switch v := l.(type) {
	case *layers.Ethernet:
		return fmt.Sprintf("%s → %s (%s)", v.SrcMAC, v.DstMAC, v.EthernetType)
	case *layers.IPv4:
		return fmt.Sprintf("%s → %s ttl=%d proto=%s", v.SrcIP, v.DstIP, v.TTL, v.Protocol)
	case *layers.IPv6:
		return fmt.Sprintf("%s → %s next=%s", v.SrcIP, v.DstIP, v.NextHeader)
	case *layers.ARP:
		return arpInfo(v)
	case *layers.ICMPv4:
		return icmpInfo(v)
	case *layers.TCP:
		return tcpInfo(v)
	case *layers.UDP:
		return udpInfo(v)
	case *layers.DNS:
		return dnsInfo(v)
	default:
		if app, ok := l.(gopacket.ApplicationLayer); ok {
			return fmt.Sprintf("%d bytes payload", len(app.Payload()))
		}
		return ""
	}
}
