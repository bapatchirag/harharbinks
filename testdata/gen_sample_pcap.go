//go:build ignore

// Command gen_sample_pcap regenerates the deterministic capture fixtures used by
// the pcap domain and CLI tests. Run it from the module root:
//
//	go run ./testdata/gen_sample_pcap.go
//
// It writes testdata/sample.pcap (classic libpcap) and testdata/sample.pcapng
// (pcapng), each holding the same synthetic Ethernet frames: an ARP exchange, a
// DNS query/response, a TCP handshake carrying a cleartext HTTP request and
// response, a TLS ClientHello with an SNI extension, and an ICMP echo
// request/reply. Every timestamp is fixed so the fixtures — and the golden files
// derived from them — never change between runs.
package main

import (
	"bytes"
	"log"
	"net"
	"os"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcapgo"
)

var (
	clientMAC = net.HardwareAddr{0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa}
	gwMAC     = net.HardwareAddr{0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb}
	bcastMAC  = net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

	clientIP = net.IP{192, 168, 1, 100}
	dnsIP    = net.IP{192, 168, 1, 1}
	serverIP = net.IP{93, 184, 216, 34}

	baseTime = time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
)

func main() {
	frames := build()
	writePCAP("testdata/sample.pcap", frames)
	writePCAPNG("testdata/sample.pcapng", frames)
	log.Printf("wrote %d frames to testdata/sample.pcap and testdata/sample.pcapng", len(frames))
}

// build assembles every fixture frame in capture order.
func build() [][]byte {
	var frames [][]byte
	add := func(data []byte) { frames = append(frames, data) }

	// 1-2: ARP who-has / reply for the DNS resolver.
	add(ser(ethARP(clientMAC, bcastMAC), &layers.ARP{
		AddrType: layers.LinkTypeEthernet, Protocol: layers.EthernetTypeIPv4,
		HwAddressSize: 6, ProtAddressSize: 4, Operation: layers.ARPRequest,
		SourceHwAddress: clientMAC, SourceProtAddress: clientIP,
		DstHwAddress: net.HardwareAddr{0, 0, 0, 0, 0, 0}, DstProtAddress: dnsIP,
	}))
	add(ser(ethARP(gwMAC, clientMAC), &layers.ARP{
		AddrType: layers.LinkTypeEthernet, Protocol: layers.EthernetTypeIPv4,
		HwAddressSize: 6, ProtAddressSize: 4, Operation: layers.ARPReply,
		SourceHwAddress: gwMAC, SourceProtAddress: dnsIP,
		DstHwAddress: clientMAC, DstProtAddress: clientIP,
	}))

	// 3-4: DNS A query for example.com and its response.
	qip := ip4(clientIP, dnsIP, layers.IPProtocolUDP)
	qudp := &layers.UDP{SrcPort: 50000, DstPort: 53}
	mustChecksum(qudp, qip)
	add(ser(ethIP(clientMAC, gwMAC), qip, qudp, &layers.DNS{
		ID: 0x1a2b, RD: true,
		Questions: []layers.DNSQuestion{{Name: []byte("example.com"), Type: layers.DNSTypeA, Class: layers.DNSClassIN}},
	}))
	rip := ip4(dnsIP, clientIP, layers.IPProtocolUDP)
	rudp := &layers.UDP{SrcPort: 53, DstPort: 50000}
	mustChecksum(rudp, rip)
	add(ser(ethIP(gwMAC, clientMAC), rip, rudp, &layers.DNS{
		ID: 0x1a2b, QR: true, RD: true, RA: true,
		Questions: []layers.DNSQuestion{{Name: []byte("example.com"), Type: layers.DNSTypeA, Class: layers.DNSClassIN}},
		Answers:   []layers.DNSResourceRecord{{Name: []byte("example.com"), Type: layers.DNSTypeA, Class: layers.DNSClassIN, TTL: 300, IP: serverIP}},
	}))

	// 5-10: TCP handshake, HTTP GET / response, and a client FIN on port 80.
	req := []byte("GET /index.html HTTP/1.1\r\nHost: example.com\r\nUser-Agent: hhb-test\r\n\r\n")
	resp := []byte("HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nContent-Length: 13\r\n\r\nHello, world!")
	add(tcpFrame(clientMAC, gwMAC, clientIP, serverIP, 50001, 80, 0, 0, tcpFlags{syn: true}, nil))
	add(tcpFrame(gwMAC, clientMAC, serverIP, clientIP, 80, 50001, 0, 1, tcpFlags{syn: true, ack: true}, nil))
	add(tcpFrame(clientMAC, gwMAC, clientIP, serverIP, 50001, 80, 1, 1, tcpFlags{ack: true}, nil))
	add(tcpFrame(clientMAC, gwMAC, clientIP, serverIP, 50001, 80, 1, 1, tcpFlags{psh: true, ack: true}, req))
	add(tcpFrame(gwMAC, clientMAC, serverIP, clientIP, 80, 50001, 1, uint32(1+len(req)), tcpFlags{psh: true, ack: true}, resp))
	add(tcpFrame(clientMAC, gwMAC, clientIP, serverIP, 50001, 80, uint32(1+len(req)), uint32(1+len(resp)), tcpFlags{fin: true, ack: true}, nil))

	// 11-14: TCP handshake and a TLS ClientHello (SNI example.com) on port 443.
	add(tcpFrame(clientMAC, gwMAC, clientIP, serverIP, 50002, 443, 0, 0, tcpFlags{syn: true}, nil))
	add(tcpFrame(gwMAC, clientMAC, serverIP, clientIP, 443, 50002, 0, 1, tcpFlags{syn: true, ack: true}, nil))
	add(tcpFrame(clientMAC, gwMAC, clientIP, serverIP, 50002, 443, 1, 1, tcpFlags{ack: true}, nil))
	add(tcpFrame(clientMAC, gwMAC, clientIP, serverIP, 50002, 443, 1, 1, tcpFlags{psh: true, ack: true}, clientHello("example.com")))

	// 15-16: ICMP echo request and reply.
	echoReq := ip4(clientIP, serverIP, layers.IPProtocolICMPv4)
	add(ser(ethIP(clientMAC, gwMAC), echoReq,
		&layers.ICMPv4{TypeCode: layers.CreateICMPv4TypeCode(layers.ICMPv4TypeEchoRequest, 0), Id: 0x0001, Seq: 1},
		gopacket.Payload([]byte("abcdefghijklmnop"))))
	echoReply := ip4(serverIP, clientIP, layers.IPProtocolICMPv4)
	add(ser(ethIP(gwMAC, clientMAC), echoReply,
		&layers.ICMPv4{TypeCode: layers.CreateICMPv4TypeCode(layers.ICMPv4TypeEchoReply, 0), Id: 0x0001, Seq: 1},
		gopacket.Payload([]byte("abcdefghijklmnop"))))

	return frames
}

// tcpFlags selects which TCP control bits a synthesized segment carries.
type tcpFlags struct{ syn, ack, fin, psh bool }

// tcpFrame builds a complete Ethernet/IPv4/TCP frame, appending an optional
// application payload, and returns its serialized bytes.
func tcpFrame(srcMAC, dstMAC net.HardwareAddr, srcIP, dstIP net.IP, srcPort, dstPort layers.TCPPort, seq, ack uint32, f tcpFlags, payload []byte) []byte {
	ip := ip4(srcIP, dstIP, layers.IPProtocolTCP)
	tcp := &layers.TCP{
		SrcPort: srcPort, DstPort: dstPort, Seq: seq, Ack: ack, Window: 64240,
		SYN: f.syn, ACK: f.ack, FIN: f.fin, PSH: f.psh,
	}
	mustChecksum(tcp, ip)
	ls := []gopacket.SerializableLayer{ethIP(srcMAC, dstMAC), ip, tcp}
	if len(payload) > 0 {
		ls = append(ls, gopacket.Payload(payload))
	}
	return ser(ls...)
}

// clientHello crafts a minimal but well-formed TLS ClientHello record carrying a
// single SNI host_name extension.
func clientHello(sni string) []byte {
	name := []byte(sni)

	var serverName bytes.Buffer
	entryLen := 1 + 2 + len(name)
	serverName.Write(u16(entryLen)) // ServerNameList length
	serverName.WriteByte(0)         // name_type: host_name
	serverName.Write(u16(len(name)))
	serverName.Write(name)

	var ext bytes.Buffer
	ext.Write(u16(0x0000)) // extension_type: server_name
	ext.Write(u16(serverName.Len()))
	ext.Write(serverName.Bytes())

	var body bytes.Buffer
	body.Write([]byte{0x03, 0x03}) // client_version: TLS 1.2
	body.Write(make([]byte, 32))   // random (zeroed for determinism)
	body.WriteByte(0)              // session_id length
	body.Write([]byte{0x00, 0x02}) // cipher_suites length
	body.Write([]byte{0x13, 0x01}) // TLS_AES_128_GCM_SHA256
	body.Write([]byte{0x01, 0x00}) // compression_methods: null
	body.Write(u16(ext.Len()))     // extensions length
	body.Write(ext.Bytes())

	var hs bytes.Buffer
	hs.WriteByte(0x01) // handshake type: ClientHello
	hs.Write(u24(body.Len()))
	hs.Write(body.Bytes())

	var rec bytes.Buffer
	rec.WriteByte(0x16)           // content type: handshake
	rec.Write([]byte{0x03, 0x01}) // legacy record version: TLS 1.0
	rec.Write(u16(hs.Len()))
	rec.Write(hs.Bytes())
	return rec.Bytes()
}

// u16 and u24 encode an integer as a big-endian 2- or 3-byte slice.
func u16(n int) []byte { return []byte{byte(n >> 8), byte(n)} }
func u24(n int) []byte { return []byte{byte(n >> 16), byte(n >> 8), byte(n)} }

func ethIP(src, dst net.HardwareAddr) *layers.Ethernet {
	return &layers.Ethernet{SrcMAC: src, DstMAC: dst, EthernetType: layers.EthernetTypeIPv4}
}

func ethARP(src, dst net.HardwareAddr) *layers.Ethernet {
	return &layers.Ethernet{SrcMAC: src, DstMAC: dst, EthernetType: layers.EthernetTypeARP}
}

func ip4(src, dst net.IP, proto layers.IPProtocol) *layers.IPv4 {
	return &layers.IPv4{Version: 4, IHL: 5, TTL: 64, Protocol: proto, SrcIP: src, DstIP: dst}
}

// mustChecksum wires a transport layer to its network layer for checksum
// computation, failing loudly if the pairing is rejected.
func mustChecksum(tl interface {
	SetNetworkLayerForChecksum(gopacket.NetworkLayer) error
}, ip *layers.IPv4) {
	if err := tl.SetNetworkLayerForChecksum(ip); err != nil {
		log.Fatalf("set network layer for checksum: %v", err)
	}
}

// ser serializes the given layers into a single frame, fixing lengths and
// computing checksums.
func ser(ls ...gopacket.SerializableLayer) []byte {
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}
	if err := gopacket.SerializeLayers(buf, opts, ls...); err != nil {
		log.Fatalf("serialize: %v", err)
	}
	return buf.Bytes()
}

func writePCAP(path string, frames [][]byte) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	w := pcapgo.NewWriter(f)
	if err := w.WriteFileHeader(65536, layers.LinkTypeEthernet); err != nil {
		log.Fatal(err)
	}
	for i, data := range frames {
		if err := w.WritePacket(captureInfo(i, data), data); err != nil {
			log.Fatal(err)
		}
	}
}

func writePCAPNG(path string, frames [][]byte) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	w, err := pcapgo.NewNgWriter(f, layers.LinkTypeEthernet)
	if err != nil {
		log.Fatal(err)
	}
	for i, data := range frames {
		if err := w.WritePacket(captureInfo(i, data), data); err != nil {
			log.Fatal(err)
		}
	}
	if err := w.Flush(); err != nil {
		log.Fatal(err)
	}
}

// captureInfo builds the per-record metadata, spacing frames 10ms apart from the
// fixed base time so the capture has a deterministic, non-zero duration.
func captureInfo(i int, data []byte) gopacket.CaptureInfo {
	return gopacket.CaptureInfo{
		Timestamp:     baseTime.Add(time.Duration(i) * 10 * time.Millisecond),
		CaptureLength: len(data),
		Length:        len(data),
	}
}
