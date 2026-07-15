package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/bapatchirag/harharbinks/internal/pcap"
)

// detailTimeLayout formats an absolute packet timestamp in the show views.
const detailTimeLayout = "2006-01-02 15:04:05.000000 UTC"

// writePacketTable renders packets as an aligned list, with times relative to
// the capture's start.
func writePacketTable(w io.Writer, packets []pcap.Packet, start time.Time) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "#\tTIME\tSOURCE\tDESTINATION\tPROTO\tLENGTH\tINFO")
	for _, p := range packets {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%d\t%s\n",
			p.Index,
			relTime(p.Timestamp, start),
			nonEmpty(p.Source(), "-"),
			nonEmpty(p.Dest(), "-"),
			p.Protocol(),
			p.OrigLen,
			p.Info(),
		)
	}
	tw.Flush()
}

// writePacketDetail renders a single packet: its metadata, decoded layer stack,
// and a hex/ASCII dump of the raw frame.
func writePacketDetail(w io.Writer, p pcap.Packet) {
	fmt.Fprintf(w, "Packet %d\n", p.Index)
	fmt.Fprintf(w, "  Time:     %s\n", p.Timestamp.UTC().Format(detailTimeLayout))
	fmt.Fprintf(w, "  Source:   %s\n", nonEmpty(p.Source(), "-"))
	fmt.Fprintf(w, "  Dest:     %s\n", nonEmpty(p.Dest(), "-"))
	fmt.Fprintf(w, "  Protocol: %s\n", p.Protocol())
	fmt.Fprintf(w, "  Length:   %d bytes (captured %d)\n", p.OrigLen, p.CapLen)
	fmt.Fprintf(w, "  Info:     %s\n", p.Info())

	fmt.Fprintln(w, "\nLayers")
	for _, l := range p.LayerStack() {
		if l.Summary != "" {
			fmt.Fprintf(w, "  %s: %s\n", l.Name, l.Summary)
		} else {
			fmt.Fprintf(w, "  %s\n", l.Name)
		}
	}

	fmt.Fprintln(w, "\nHex")
	writeHexDump(w, p.Data)
}

// writeFlowTable renders conversations as an aligned table.
func writeFlowTable(w io.Writer, flows []pcap.Flow) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "PROTO\tSOURCE\tDESTINATION\tPACKETS\tBYTES\tDURATION")
	for _, f := range flows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\t%s\n",
			f.Protocol,
			fmt.Sprintf("%s:%d", f.SrcIP, f.SrcPort),
			fmt.Sprintf("%s:%d", f.DstIP, f.DstPort),
			f.Packets,
			humanSize(f.Bytes),
			f.Duration(),
		)
	}
	tw.Flush()
}

// writeStats renders a capture summary, protocol hierarchy, and top talkers.
func writeStats(w io.Writer, s pcap.Stats) {
	fmt.Fprintln(w, "Capture Summary")
	fmt.Fprintf(w, "  Packets:  %d\n", s.Packets)
	fmt.Fprintf(w, "  Bytes:    %s\n", humanSize(s.Bytes))
	fmt.Fprintf(w, "  Duration: %s\n", s.Duration())
	if s.Packets > 0 {
		fmt.Fprintf(w, "  Start:    %s\n", s.Start.UTC().Format(detailTimeLayout))
		fmt.Fprintf(w, "  End:      %s\n", s.End.UTC().Format(detailTimeLayout))
	}

	fmt.Fprintln(w, "\nProtocol Hierarchy")
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "  PROTOCOL\tPACKETS\tBYTES")
	for _, pc := range s.Protocols {
		fmt.Fprintf(tw, "  %s\t%d\t%s\n", pc.Protocol, pc.Packets, humanSize(pc.Bytes))
	}
	tw.Flush()

	fmt.Fprintln(w, "\nTop Talkers")
	tw = tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "  ADDRESS\tPACKETS\tBYTES")
	for _, t := range s.TopTalkers {
		fmt.Fprintf(tw, "  %s\t%d\t%s\n", t.Address, t.Packets, humanSize(t.Bytes))
	}
	tw.Flush()
}

// packetSummaryJSON is the compact JSON shape emitted by `hhb pcap ls --json`.
type packetSummaryJSON struct {
	Index    int    `json:"index"`
	Time     string `json:"time"`
	Source   string `json:"source"`
	Dest     string `json:"dest"`
	Protocol string `json:"protocol"`
	Length   int    `json:"length"`
	Info     string `json:"info"`
}

func writePacketsJSON(w io.Writer, packets []pcap.Packet) error {
	out := make([]packetSummaryJSON, len(packets))
	for i, p := range packets {
		out[i] = packetSummaryJSON{
			Index:    p.Index,
			Time:     p.Timestamp.UTC().Format(time.RFC3339Nano),
			Source:   p.Source(),
			Dest:     p.Dest(),
			Protocol: p.Protocol(),
			Length:   p.OrigLen,
			Info:     p.Info(),
		}
	}
	return encodeJSON(w, out)
}

// packetDetailJSON is the JSON shape emitted by `hhb pcap show --json`.
type packetDetailJSON struct {
	Index    int          `json:"index"`
	Time     string       `json:"time"`
	Source   string       `json:"source"`
	Dest     string       `json:"dest"`
	Protocol string       `json:"protocol"`
	Length   int          `json:"length"`
	CapLen   int          `json:"capLen"`
	Info     string       `json:"info"`
	Layers   []pcap.Layer `json:"layers"`
	Bytes    string       `json:"bytes"`
}

func writePacketDetailJSON(w io.Writer, p pcap.Packet) error {
	return encodeJSON(w, packetDetailJSON{
		Index:    p.Index,
		Time:     p.Timestamp.UTC().Format(time.RFC3339Nano),
		Source:   p.Source(),
		Dest:     p.Dest(),
		Protocol: p.Protocol(),
		Length:   p.OrigLen,
		CapLen:   p.CapLen,
		Info:     p.Info(),
		Layers:   p.LayerStack(),
		Bytes:    hex.EncodeToString(p.Data),
	})
}

// flowJSON is the JSON shape emitted by `hhb pcap flows --json`.
type flowJSON struct {
	Protocol string `json:"protocol"`
	Source   string `json:"source"`
	Dest     string `json:"dest"`
	Packets  int    `json:"packets"`
	Bytes    int    `json:"bytes"`
	Duration string `json:"duration"`
	Start    string `json:"start"`
	End      string `json:"end"`
}

func writeFlowsJSON(w io.Writer, flows []pcap.Flow) error {
	out := make([]flowJSON, len(flows))
	for i, f := range flows {
		out[i] = flowJSON{
			Protocol: f.Protocol,
			Source:   fmt.Sprintf("%s:%d", f.SrcIP, f.SrcPort),
			Dest:     fmt.Sprintf("%s:%d", f.DstIP, f.DstPort),
			Packets:  f.Packets,
			Bytes:    f.Bytes,
			Duration: f.Duration().String(),
			Start:    f.Start.UTC().Format(time.RFC3339Nano),
			End:      f.End.UTC().Format(time.RFC3339Nano),
		}
	}
	return encodeJSON(w, out)
}

// statsJSON is the JSON shape emitted by `hhb pcap stats --json`.
type statsJSON struct {
	Packets    int                  `json:"packets"`
	Bytes      int                  `json:"bytes"`
	Duration   string               `json:"duration"`
	Start      string               `json:"start"`
	End        string               `json:"end"`
	Protocols  []pcap.ProtocolCount `json:"protocols"`
	TopTalkers []pcap.Talker        `json:"topTalkers"`
}

func writeStatsJSON(w io.Writer, s pcap.Stats) error {
	out := statsJSON{
		Packets:    s.Packets,
		Bytes:      s.Bytes,
		Duration:   s.Duration().String(),
		Protocols:  s.Protocols,
		TopTalkers: s.TopTalkers,
	}
	if s.Packets > 0 {
		out.Start = s.Start.UTC().Format(time.RFC3339Nano)
		out.End = s.End.UTC().Format(time.RFC3339Nano)
	}
	return encodeJSON(w, out)
}

// encodeJSON writes v as indented JSON without HTML escaping, matching the HAR
// commands' output style.
func encodeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// relTime formats a packet's time relative to the capture start, in seconds with
// microsecond precision (e.g. "0.010000"), matching common packet-list displays.
func relTime(t, start time.Time) string {
	return fmt.Sprintf("%.6f", t.Sub(start).Seconds())
}

// writeHexDump writes a hex/ASCII dump of data, 16 bytes per row with an offset
// column, in the style of `hexdump -C`.
func writeHexDump(w io.Writer, data []byte) {
	for off := 0; off < len(data); off += 16 {
		end := min(off+16, len(data))
		chunk := data[off:end]
		var hexcol, ascii strings.Builder
		for i := 0; i < 16; i++ {
			if i == 8 {
				hexcol.WriteByte(' ')
			}
			if i < len(chunk) {
				fmt.Fprintf(&hexcol, "%02x ", chunk[i])
				ascii.WriteByte(printableByte(chunk[i]))
			} else {
				hexcol.WriteString("   ")
			}
		}
		fmt.Fprintf(w, "  %04x  %s|%s|\n", off, hexcol.String(), ascii.String())
	}
}

// printableByte returns b when it is a printable ASCII character, else a dot.
func printableByte(b byte) byte {
	if b >= 0x20 && b <= 0x7e {
		return b
	}
	return '.'
}
