# Screenshots

## HAR viewer

### Request list & detail inspector

The main screen: a scrollable table of every entry on top, with a tabbed detail
inspector for the highlighted request below and a status bar of context-aware key
hints.

![The harharbinks main screen — a request list table above a tabbed detail inspector](images/main.png)

### Response body — pretty-printed JSON

The detail inspector groups a request into tabs (Overview, Headers, Cookies,
Payload, Response, Raw). JSON bodies are pretty-printed and syntax-highlighted;
base64 payloads are decoded and binary bodies are summarized.

![The Response tab showing a pretty-printed, syntax-highlighted JSON body](images/detail-response.png)

### Search & filter

Free-text search matches any field, and `field:value` terms (here `method:GET`)
scope the query; multiple terms are combined with AND. The list filters live as
you type.

![The filter prompt with a scoped method:GET query narrowing the request list](images/search.png)

### Follow session

From any entry, jump to every request and response that shares its connection
(falling back to same host and time proximity), so a login flow or an API
sequence reads as one story.

![The follow-session view showing the exchanges that share a connection](images/session.png)

### File browser

Pick a `.har`, `.pcap`, or `.pcapng` from within the app, with in-directory
filtering — no need to pass a path on the command line. Only HAR and capture
files are selectable, and each opens in the right viewer automatically.

![The in-app file browser listing a directory's files, with HAR and capture files selectable](images/browser.png)

### Export menu

Copy an entry's URL, copy it as a ready-to-run cURL command, or save its response
body to disk.

![The export menu with copy URL, copy as cURL, and save body actions](images/export.png)

### Settings — theme selector

Switch among the built-in palettes (Kanagawa, Gruvbox, Everforest, Zenburn) with
a live preview; your choice is persisted between runs.

![The settings editor with the theme selector open](images/settings.png)

### Help

Every screen documents its own key bindings in a help overlay, toggled with `?`.

![The help overlay listing the viewer's key bindings](images/help.png)

## PCAP viewer

### Packet list, layer tree & hex

Open a `.pcap`/`.pcapng` and every frame is decoded into a Wireshark-style list
on top, with a three-pane inspector below: a collapsible layer tree and a
hex/ASCII view. Selecting a layer or field highlights the exact bytes it spans;
`tab` cycles focus across the list, the tree, and the hex view.

![The PCAP viewer: a packet list above a layer tree and hex view, with a selected layer's bytes highlighted](images/pcap-main.png)

### Follow a conversation

From any packet, press `enter` to scope the list to its conversation — every
frame in that 5-tuple flow — the packet-capture analogue of follow-session.

![The PCAP viewer scoped to one conversation's frames](images/pcap-follow.png)

### Conversations (flows)

The views menu (`e`) opens a table of bidirectional conversations, each with its
packet, byte, and duration totals; `enter` follows one back into the packet list.

![The conversations view: a table of bidirectional 5-tuple flows](images/pcap-flows.png)

### Capture statistics

The statistics view summarizes the capture: totals, a protocol hierarchy, and the
top talkers by address.

![The capture-statistics view: totals, protocol hierarchy, and top talkers](images/pcap-stats.png)

## Headless CLI

For scripting and quick lookups, `hhb` also runs without the TUI. Each command
reads from a file argument or from stdin.

### `hhb ls` — list entries

```sh
hhb ls testdata/sample.har
```

![Output of hhb ls: a compact table of the capture's entries](images/cli-ls.png)

### `hhb show` — inspect one entry

```sh
hhb show 2 testdata/sample.har
```

![Output of hhb show: the request and response detail for a single entry](images/cli-show.png)

### `hhb curl` — reproduce as cURL

```sh
hhb curl 2 testdata/sample.har
```

![Output of hhb curl: an entry rendered as a runnable curl command](images/cli-curl.png)

### `hhb pcap ls` — list packets

```sh
hhb pcap ls testdata/sample.pcap
```

![Output of hhb pcap ls: a Wireshark-style table of the capture's packets](images/cli-pcap-ls.png)

### `hhb pcap show` — inspect one packet

```sh
hhb pcap show 8 testdata/sample.pcap
```

![Output of hhb pcap show: one packet's layer stack and hex/ASCII dump](images/cli-pcap-show.png)

### `hhb pcap flows` — list conversations

```sh
hhb pcap flows testdata/sample.pcap
```

![Output of hhb pcap flows: a table of bidirectional conversations](images/cli-pcap-flows.png)

### `hhb pcap stats` — summarize the capture

```sh
hhb pcap stats testdata/sample.pcap
```

![Output of hhb pcap stats: capture totals, protocol hierarchy, and top talkers](images/cli-pcap-stats.png)
