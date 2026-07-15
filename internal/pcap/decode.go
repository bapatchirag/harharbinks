package pcap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
)

// Layer is one decoded protocol layer: its name, a short human-readable summary,
// the byte range it occupies within the frame, and its individual fields. The
// byte range lets a packet-detail view highlight exactly the bytes a layer spans
// in a companion hex view, and the fields let it expand the layer into a tree —
// all without the view importing gopacket.
type Layer struct {
	Name    string  `json:"name"`
	Summary string  `json:"summary"`
	Offset  int     `json:"offset"`
	Length  int     `json:"length"`
	Fields  []Field `json:"fields,omitempty"`
}

// Field is one decoded name/value pair within a Layer, such as a TTL or a TCP
// port, together with the byte range it occupies in the frame so a detail view
// can highlight exactly those bytes. A field's value may span multiple lines (for
// example a pretty-printed HTTP body), which a tree view can split across rows.
type Field struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Offset int    `json:"offset"`
	Length int    `json:"length"`
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
// innermost (application), each with a short summary, its byte range within the
// frame, and its decoded fields. Byte offsets are the running sum of each layer's
// on-wire header length, so consecutive layers tile the frame and a detail view
// can map any layer back to the exact bytes it occupies.
func (p Packet) LayerStack() []Layer {
	var out []Layer
	offset := 0
	for _, l := range p.decoded.Layers() {
		length := len(l.LayerContents())
		out = append(out, Layer{
			Name:    l.LayerType().String(),
			Summary: layerSummary(l),
			Offset:  offset,
			Length:  length,
			Fields:  layerFields(l, offset),
		})
		offset += length
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

// tcpFlagNames returns the set control-flag names of a TCP segment in a stable
// order, so the info column and the field breakdown describe the same flags.
func tcpFlagNames(tcp *layers.TCP) []string {
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
	return flags
}

// tcpInfo renders a bare TCP segment with its flags, sequence, and payload length.
func tcpInfo(tcp *layers.TCP) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d → %d [%s] Seq=%d", uint16(tcp.SrcPort), uint16(tcp.DstPort),
		strings.Join(tcpFlagNames(tcp), ", "), tcp.Seq)
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

// layerFields returns a decoded layer's individual name/value fields, each with
// its absolute byte range in the frame, so a detail view can expand the layer
// into a tree and highlight the exact bytes of a selected field. base is the
// layer's own offset in the frame; the per-protocol extractors report offsets
// relative to the layer, and a field with no precise range known (length zero)
// falls back to spanning the whole layer.
func layerFields(l gopacket.Layer, base int) []Field {
	fields := rawLayerFields(l)
	layerLen := len(l.LayerContents())
	for i := range fields {
		if fields[i].Length <= 0 {
			fields[i].Offset = base
			fields[i].Length = layerLen
			continue
		}
		fields[i].Offset += base
	}
	return fields
}

// rawLayerFields breaks a decoded layer into its fields with offsets relative to
// the start of the layer. It mirrors layerSummary's protocol coverage; the
// application payload is parsed for HTTP and TLS, falling back to a byte count for
// opaque data.
func rawLayerFields(l gopacket.Layer) []Field {
	switch v := l.(type) {
	case *layers.Ethernet:
		return []Field{
			{"Destination MAC", v.DstMAC.String(), 0, 6},
			{"Source MAC", v.SrcMAC.String(), 6, 6},
			{"EtherType", v.EthernetType.String(), 12, 2},
		}
	case *layers.IPv4:
		return []Field{
			{"Version", fmt.Sprintf("%d", v.Version), 0, 1},
			{"Total Length", fmt.Sprintf("%d", v.Length), 2, 2},
			{"TTL", fmt.Sprintf("%d", v.TTL), 8, 1},
			{"Protocol", v.Protocol.String(), 9, 1},
			{"Source", v.SrcIP.String(), 12, 4},
			{"Destination", v.DstIP.String(), 16, 4},
		}
	case *layers.IPv6:
		return []Field{
			{"Next Header", v.NextHeader.String(), 6, 1},
			{"Hop Limit", fmt.Sprintf("%d", v.HopLimit), 7, 1},
			{"Source", v.SrcIP.String(), 8, 16},
			{"Destination", v.DstIP.String(), 24, 16},
		}
	case *layers.ARP:
		return arpFields(v)
	case *layers.ICMPv4:
		return []Field{
			{"Type", v.TypeCode.String(), 0, 2},
			{"Identifier", fmt.Sprintf("0x%04x", v.Id), 4, 2},
			{"Sequence", fmt.Sprintf("%d", v.Seq), 6, 2},
		}
	case *layers.TCP:
		return []Field{
			{"Source Port", fmt.Sprintf("%d", uint16(v.SrcPort)), 0, 2},
			{"Destination Port", fmt.Sprintf("%d", uint16(v.DstPort)), 2, 2},
			{"Sequence", fmt.Sprintf("%d", v.Seq), 4, 4},
			{"Acknowledgment", fmt.Sprintf("%d", v.Ack), 8, 4},
			{"Flags", joinFlags(tcpFlagNames(v)), 13, 1},
			{"Window", fmt.Sprintf("%d", v.Window), 14, 2},
		}
	case *layers.UDP:
		return []Field{
			{"Source Port", fmt.Sprintf("%d", uint16(v.SrcPort)), 0, 2},
			{"Destination Port", fmt.Sprintf("%d", uint16(v.DstPort)), 2, 2},
			{"Length", fmt.Sprintf("%d", v.Length), 4, 2},
		}
	case *layers.DNS:
		return dnsFields(v)
	default:
		if app, ok := l.(gopacket.ApplicationLayer); ok {
			return payloadFields(app.Payload())
		}
		return nil
	}
}

// arpFields breaks an ARP message into its operation and the sender/target
// hardware and protocol addresses, sizing each address field from the packet's
// own address-length fields so the byte ranges stay correct for any address size.
func arpFields(v *layers.ARP) []Field {
	hlen, plen := int(v.HwAddressSize), int(v.ProtAddressSize)
	sha := 8
	spa := sha + hlen
	tha := spa + plen
	tpa := tha + hlen
	return []Field{
		{"Operation", arpOperationName(v.Operation), 6, 2},
		{"Sender MAC", net.HardwareAddr(v.SourceHwAddress).String(), sha, hlen},
		{"Sender IP", net.IP(v.SourceProtAddress).String(), spa, plen},
		{"Target MAC", net.HardwareAddr(v.DstHwAddress).String(), tha, hlen},
		{"Target IP", net.IP(v.DstProtAddress).String(), tpa, plen},
	}
}

// joinFlags renders a set of flag names as a comma-separated list, or "none" when
// no flags are set, so the field always has a value.
func joinFlags(flags []string) string {
	if len(flags) == 0 {
		return "none"
	}
	return strings.Join(flags, ", ")
}

// arpOperationName names an ARP operation code (request or reply).
func arpOperationName(op uint16) string {
	switch op {
	case layers.ARPRequest:
		return "request"
	case layers.ARPReply:
		return "reply"
	default:
		return fmt.Sprintf("%d", op)
	}
}

// dnsFields breaks a DNS message into its transaction id, direction, and one
// field per question and answer record. The header fields carry precise byte
// ranges; the variable-length records fall back to the whole DNS layer.
func dnsFields(d *layers.DNS) []Field {
	fields := []Field{
		{"Transaction ID", fmt.Sprintf("0x%04x", d.ID), 0, 2},
		{"Message", dnsMessageType(d.QR), 2, 2},
	}
	for _, q := range d.Questions {
		fields = append(fields, Field{Name: "Query", Value: fmt.Sprintf("%s %s", q.Type, q.Name)})
	}
	for _, a := range d.Answers {
		fields = append(fields, Field{Name: "Answer", Value: dnsAnswerValue(a)})
	}
	return fields
}

// dnsMessageType labels a DNS message as a query or a response.
func dnsMessageType(qr bool) string {
	if qr {
		return "response"
	}
	return "query"
}

// dnsAnswerValue renders a DNS answer record, naming the common address and
// alias record types and falling back to the bare type for the rest.
func dnsAnswerValue(a layers.DNSResourceRecord) string {
	switch a.Type {
	case layers.DNSTypeA, layers.DNSTypeAAAA:
		if a.IP != nil {
			return fmt.Sprintf("%s %s %s", a.Name, a.Type, a.IP)
		}
	case layers.DNSTypeCNAME:
		return fmt.Sprintf("%s CNAME %s", a.Name, a.CNAME)
	}
	return fmt.Sprintf("%s %s", a.Name, a.Type)
}

// tlsFields decodes a TLS record header and, for a handshake, its message type; a
// ClientHello additionally contributes the negotiated version and SNI host name.
// Fixed-position fields carry byte ranges; the deeply nested SNI falls back to the
// whole layer. Without decryption only this cleartext handshake metadata is known.
func tlsFields(payload []byte) []Field {
	if len(payload) < 5 {
		return []Field{{Name: "TLS", Value: "record"}}
	}
	fields := []Field{
		{"Content Type", tlsContentTypeName(payload[0]), 0, 1},
		{"Version", tlsVersion(payload[1], payload[2]), 1, 2},
	}
	if payload[0] == 22 { // handshake
		hs := payload[5:]
		if len(hs) > 0 {
			fields = append(fields, Field{"Handshake Type", tlsHandshakeTypeName(hs[0]), 5, 1})
			if hs[0] == 1 { // ClientHello
				ver, sni := parseClientHello(hs)
				if ver != "" {
					fields = append(fields, Field{"Client Version", ver, 9, 2})
				}
				if sni != "" {
					fields = append(fields, Field{Name: "Server Name", Value: sni})
				}
			}
		}
	}
	return fields
}

// httpFields parses a cleartext HTTP request or response into its start line, its
// headers, and — when present — a formatted body, tracking each part's byte range
// within the payload so a detail view can highlight the exact line it points at.
func httpFields(payload []byte) []Field {
	headText, body := splitHTTPMessage(payload)
	lines := strings.Split(headText, "\r\n")

	var fields []Field
	pos := 0
	for i, line := range lines {
		switch {
		case i == 0 && line != "":
			name := "Request Line"
			if httpKind(payload) == "response" {
				name = "Status Line"
			}
			fields = append(fields, Field{name, line, pos, len(line)})
		case i > 0 && line != "":
			if c := strings.IndexByte(line, ':'); c >= 0 {
				fields = append(fields, Field{strings.TrimSpace(line[:c]), strings.TrimSpace(line[c+1:]), pos, len(line)})
			}
		}
		pos += len(line) + 2 // advance past the line and its CRLF
	}
	if len(body) > 0 {
		fields = append(fields, Field{"Body", formatHTTPBody(body), len(headText) + 4, len(body)})
	}
	return fields
}

// splitHTTPMessage splits a raw HTTP message into its header block and body at the
// blank line that separates them; a message without that separator is all headers.
func splitHTTPMessage(payload []byte) (head string, body []byte) {
	sep := []byte("\r\n\r\n")
	if i := bytes.Index(payload, sep); i >= 0 {
		return string(payload[:i]), payload[i+len(sep):]
	}
	return string(payload), nil
}

// formatHTTPBody pretty-prints a JSON body for readability and returns any other
// body unchanged, so a detail view can show structured payloads legibly without
// altering non-JSON content.
func formatHTTPBody(body []byte) string {
	trimmed := bytes.TrimSpace(body)
	if json.Valid(trimmed) {
		var buf bytes.Buffer
		if err := json.Indent(&buf, trimmed, "", "  "); err == nil {
			return buf.String()
		}
	}
	return string(body)
}

// payloadFields decodes an application-layer payload into fields, recognizing
// cleartext HTTP requests and responses and TLS records; anything else is
// summarized by its byte length so an opaque payload still shows a node.
func payloadFields(payload []byte) []Field {
	switch {
	case isTLSRecord(payload):
		return tlsFields(payload)
	case httpKind(payload) != "":
		return httpFields(payload)
	default:
		return []Field{{Name: "Payload", Value: fmt.Sprintf("%d bytes", len(payload))}}
	}
}

// tlsContentTypeName names a TLS record content type byte.
func tlsContentTypeName(t byte) string {
	switch t {
	case 20:
		return "change_cipher_spec"
	case 21:
		return "alert"
	case 22:
		return "handshake"
	case 23:
		return "application_data"
	default:
		return fmt.Sprintf("0x%02x", t)
	}
}

// tlsHandshakeTypeName names the TLS handshake message types this viewer surfaces.
func tlsHandshakeTypeName(t byte) string {
	switch t {
	case 1:
		return "Client Hello"
	case 2:
		return "Server Hello"
	default:
		return fmt.Sprintf("type %d", t)
	}
}
