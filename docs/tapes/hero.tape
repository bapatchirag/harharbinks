# hero.tape — the harharbinks feature reel shown at the top of the README.
#
# Regenerate with:  make screenshots   (or, from the repo root: vhs docs/tapes/hero.tape)
# Requires the binary at ./bin/hhb (the `screenshots` make target builds it first)
# and vhs (https://github.com/charmbracelet/vhs). The output GIF is optimized by
# gifsicle in the make target.
#
# The reel runs in two acts — the HAR viewer, then the PCAP viewer. Between them
# the HAR viewer is quit and the capture opened while hidden, so the GIF cuts
# straight from one to the other with no shell prompt in view.

Output docs/images/hero.gif

Set FontSize 16
Set Width 1024
Set Height 680
Set Padding 20
Set TypingSpeed 55ms
Set Framerate 24

# --- Act 1: the HAR viewer ---

# Launch the viewer on the bundled sample HAR (the shell command is hidden).
Hide
Type "./bin/hhb testdata/sample.har"
Enter
Sleep 1.5s
Show

# Browse the request list.
Sleep 600ms
Down@450ms 2
Sleep 600ms

# Inspect a response: focus the detail pane and step to the Response (JSON) tab.
# The settle sleep after Tab keeps the first Right from being swallowed by the
# focus change (the detail tabs wrap, so the count must land exactly on Response).
Up@220ms 2
Sleep 400ms
Tab
Sleep 450ms
Right@300ms 4
Sleep 1.3s
Tab
Sleep 400ms

# Filter with a scoped field:value query.
Type "/"
Type "method:POST"
Sleep 1.3s
Escape
Sleep 700ms

# --- Act 2: the PCAP viewer ---

# Quit the HAR viewer and relaunch on the bundled capture, both hidden, so the
# reel cuts straight into the PCAP viewer with no shell prompt in view.
Hide
Type "q"
Sleep 700ms
Type "./bin/hhb testdata/sample.pcap"
Enter
Sleep 1.5s
Show

# Browse the packet list to an HTTP request (frame 8).
Sleep 600ms
Down@300ms 7
Sleep 700ms

# Inspect the packet: focus the layer tree and expand a layer so the bytes it
# spans light up in the hex view, then step through its fields.
Tab
Sleep 500ms
Down@260ms 2
Sleep 300ms
Right
Sleep 900ms
Down@360ms 2
Sleep 1.1s

# Follow the packet's conversation: return focus to the list, then scope it.
Tab
Sleep 200ms
Tab
Sleep 400ms
Enter
Sleep 1.3s
Escape
Sleep 600ms

# Open the conversations (flows) view from the views menu (mnemonic 'f').
Type "e"
Sleep 700ms
Type "f"
Sleep 1.3s
Escape
Sleep 600ms

# Open the capture-statistics view (mnemonic 's').
Type "e"
Sleep 700ms
Type "s"
Sleep 1.6s

# Quit (hidden, so the reel ends on the statistics view, not a shell prompt).
Hide
Escape
Type "q"
