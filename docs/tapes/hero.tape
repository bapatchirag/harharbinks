# hero.tape — the harharbinks feature reel shown at the top of the README.
#
# Regenerate with:  make screenshots   (or, from the repo root: vhs docs/tapes/hero.tape)
# Requires the binary at ./bin/hhb (the `screenshots` make target builds it first)
# and vhs (https://github.com/charmbracelet/vhs). The output GIF is optimized by
# gifsicle in the make target.

Output docs/images/hero.gif

Set FontSize 16
Set Width 1024
Set Height 680
Set Padding 20
Set TypingSpeed 55ms
Set Framerate 24

# Launch the viewer on the bundled sample capture (the shell command is hidden).
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
Sleep 1.2s
Escape
Sleep 600ms

# Open the export menu (copy URL, copy as cURL, save the body).
Type "e"
Sleep 1.2s
Escape
Sleep 500ms

# Switch the color theme live from the settings editor.
Type "c"
Sleep 600ms
Right@520ms 2
Sleep 900ms
Escape
Sleep 700ms

# Quit.
Type "q"
Sleep 600ms
