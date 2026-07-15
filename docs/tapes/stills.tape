# stills.tape — per-screen screenshots for the gallery in docs/screenshots.md.
#
# Regenerate with:  make screenshots   (or, from the repo root: vhs docs/tapes/stills.tape)
# Each `Screenshot` writes a full-resolution PNG into docs/images/. The `Output`
# GIF is only a throwaway required by vhs; the make target deletes it afterwards.
#
# The session is launched from inside testdata/ so the in-app file browser ("o")
# is rooted where sample.har lives. Screenshot/Output paths stay relative to the
# repo root (where vhs runs), independent of the shell's cd.

Output docs/images/.stills.gif

Set FontSize 16
Set Width 1024
Set Height 680
Set Padding 20

Hide
Type "cd testdata"
Enter
Type "../bin/hhb sample.har"
Enter
Sleep 1.5s
Show

# 1) Main viewer: request list on top, tabbed detail inspector below.
Sleep 700ms
Screenshot docs/images/main.png
Sleep 400ms

# 2) Search & filter: a scoped field:value query, live-filtering the list.
Type "/"
Type "method:GET"
Sleep 900ms
Screenshot docs/images/search.png
Sleep 400ms
Escape
Sleep 600ms

# 3) Follow session: every exchange sharing the selected entry's connection.
Up@150ms 3
Sleep 400ms
Enter
Sleep 900ms
Screenshot docs/images/session.png
Sleep 400ms
Escape
Sleep 600ms

# 4) Export menu: copy the URL, copy as cURL, or save the response body.
Up@150ms 3
Sleep 400ms
Type "e"
Sleep 900ms
Screenshot docs/images/export.png
Sleep 400ms
Escape
Sleep 600ms

# 5) Settings: the theme selector (the choice is persisted between runs).
Type "c"
Sleep 900ms
Screenshot docs/images/settings.png
Sleep 400ms
Escape
Sleep 600ms

# 6) Help overlay: the active screen's key bindings.
Type "?"
Sleep 900ms
Screenshot docs/images/help.png
Sleep 400ms
Escape
Sleep 600ms

# 7) Detail inspector: the Response tab with pretty-printed, highlighted JSON.
#    Focus the detail pane (settle) then step right to the Response tab. The
#    detail tabs wrap, so the count must be exact — the settle sleep after Tab
#    keeps the first Right from being swallowed by the focus change.
Up@150ms 3
Sleep 400ms
Tab
Sleep 500ms
Right@320ms 4
Sleep 900ms
Screenshot docs/images/detail-response.png
Sleep 400ms
Tab
Sleep 500ms

# 8) File browser (a full-window screen, captured last so its transition does
#    not swallow the next screen's opening key): pick a .har file from testdata/.
Type "o"
Sleep 1s
Screenshot docs/images/browser.png
Sleep 400ms
Escape
Sleep 500ms

# Quit.
Type "q"
Sleep 500ms
