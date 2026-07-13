package component

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bapatchirag/harharbinks/internal/tui"
	"github.com/bapatchirag/harharbinks/internal/tui/keymap"
	"github.com/bapatchirag/harharbinks/internal/tui/msg"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// fileEntry is a single directory or file shown in the browser.
type fileEntry struct {
	name  string
	isDir bool
}

// FileBrowser is a focusable, filterable directory browser for opening captures.
// It lists the current directory (directories first, then files, both sorted
// case-insensitively), supports a live substring filter over the entry names via
// SetFilter, and emits msg.FileSelectedMsg when a file with an allowed extension
// is chosen. It wraps no external widget, so the visible set can be narrowed —
// the capability the screen's "/" search relies on (the bubbles filepicker keeps
// its file list private and cannot be filtered).
type FileBrowser struct {
	theme   theme.Theme
	keys    keymap.KeyMap
	allowed []string // selectable extensions (e.g. ".har"); empty means any file

	dir     string
	entries []fileEntry // everything in dir, sorted (unfiltered)
	shown   []fileEntry // entries matching the filter (the visible rows)
	filter  string

	cursor  int
	offset  int
	width   int
	height  int
	focused bool

	selected string
	readErr  string
}

// NewFileBrowser creates a browser rooted at the current working directory. If
// allowed is non-empty, only files with those extensions are selectable; others
// are shown dimmed. The initial directory is read immediately.
func NewFileBrowser(th theme.Theme, allowed []string) *FileBrowser {
	f := &FileBrowser{theme: th, keys: keymap.Default(), allowed: allowed}
	if cwd, err := os.Getwd(); err == nil {
		f.dir = cwd
	} else {
		f.dir = "."
	}
	f.readDir()
	return f
}

// SetFilter narrows the visible entries to those whose name contains q
// (case-insensitive); an empty q shows everything. The selection resets to the
// top so the first match is highlighted.
func (f *FileBrowser) SetFilter(q string) {
	if q == f.filter {
		return
	}
	f.filter = q
	f.cursor, f.offset = 0, 0
	f.applyFilter()
}

// Filter returns the active filter string ("" when unfiltered).
func (f *FileBrowser) Filter() string { return f.filter }

// SetSize sets the render area; only the height (row count) is used here.
func (f *FileBrowser) SetSize(w, h int) {
	f.width, f.height = w, h
	f.clampCursor()
}

// Selected returns the most recently chosen file path, if any.
func (f *FileBrowser) Selected() string { return f.selected }

// CurrentDir returns the directory currently listed. It changes as the user
// navigates into and out of directories.
func (f *FileBrowser) CurrentDir() string { return f.dir }

// Focus gives the browser input focus.
func (f *FileBrowser) Focus() { f.focused = true }

// Blur removes input focus.
func (f *FileBrowser) Blur() { f.focused = false }

// Focused reports whether the browser has focus.
func (f *FileBrowser) Focused() bool { return f.focused }

// Init implements tui.Component. The directory is read on construction, so no
// startup command is needed.
func (f *FileBrowser) Init() tea.Cmd { return nil }

// Update handles navigation keys: move the selection, page, jump to top/bottom,
// enter or leave directories, and open (select) a file. Selecting an allowed
// file emits msg.FileSelectedMsg. Non-key messages are ignored.
func (f *FileBrowser) Update(tmsg tea.Msg) tea.Cmd {
	k, ok := tmsg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	switch {
	case key.Matches(k, f.keys.Up):
		f.move(-1)
	case key.Matches(k, f.keys.Down):
		f.move(1)
	case key.Matches(k, f.keys.PageUp):
		f.move(-f.pageStep())
	case key.Matches(k, f.keys.PageDown):
		f.move(f.pageStep())
	case key.Matches(k, f.keys.Home):
		f.cursor = 0
		f.clampCursor()
	case key.Matches(k, f.keys.End):
		f.cursor = len(f.shown) - 1
		f.clampCursor()
	case key.Matches(k, f.keys.Left), k.Type == tea.KeyBackspace:
		f.goParent()
	case key.Matches(k, f.keys.Right), key.Matches(k, f.keys.Enter):
		return f.openCurrent()
	}
	return nil
}

// View renders the visible window of entries, or a muted message when the
// directory is empty, unreadable, or filtered to nothing.
func (f *FileBrowser) View() string {
	if f.readErr != "" {
		return f.theme.MutedText().Render("  " + f.readErr)
	}
	if len(f.shown) == 0 {
		if f.filter != "" {
			return f.theme.MutedText().Render(fmt.Sprintf("  no entries match %q", f.filter))
		}
		return f.theme.MutedText().Render("  (empty directory)")
	}
	vis := f.visibleRows()
	end := f.offset + vis
	if end > len(f.shown) {
		end = len(f.shown)
	}
	var b strings.Builder
	for i := f.offset; i < end; i++ {
		if i > f.offset {
			b.WriteByte('\n')
		}
		e := f.shown[i]
		mark := gutter
		if i == f.cursor {
			mark = cursorGutter
		}
		name := e.name
		if e.isDir {
			name += "/"
		}
		b.WriteString(f.entryStyle(e, i == f.cursor).Render(mark + name))
	}
	return b.String()
}

// entryStyle picks the row style: the selection accent for the cursor row, then
// directory / dimmed / normal coloring by type and selectability.
func (f *FileBrowser) entryStyle(e fileEntry, selected bool) lipgloss.Style {
	switch {
	case selected:
		return f.theme.Title()
	case e.isDir:
		return f.theme.Base().Foreground(f.theme.Secondary)
	case !f.selectable(e.name):
		return f.theme.MutedText()
	default:
		return f.theme.Base()
	}
}

// readDir loads f.dir into f.entries (directories first, then files, sorted
// case-insensitively), skipping dotfiles, then applies the current filter.
func (f *FileBrowser) readDir() {
	f.readErr = ""
	f.entries = nil
	des, err := os.ReadDir(f.dir)
	if err != nil {
		f.readErr = err.Error()
		f.shown = nil
		return
	}
	for _, de := range des {
		name := de.Name()
		if strings.HasPrefix(name, ".") {
			continue // hide dotfiles, matching typical file browsers
		}
		f.entries = append(f.entries, fileEntry{name: name, isDir: de.IsDir()})
	}
	sort.SliceStable(f.entries, func(i, j int) bool {
		a, b := f.entries[i], f.entries[j]
		if a.isDir != b.isDir {
			return a.isDir // directories before files
		}
		return strings.ToLower(a.name) < strings.ToLower(b.name)
	})
	f.applyFilter()
}

// applyFilter recomputes f.shown from f.entries and the current filter, then
// keeps the cursor in range.
func (f *FileBrowser) applyFilter() {
	if f.filter == "" {
		f.shown = f.entries
	} else {
		q := strings.ToLower(f.filter)
		shown := make([]fileEntry, 0, len(f.entries))
		for _, e := range f.entries {
			if strings.Contains(strings.ToLower(e.name), q) {
				shown = append(shown, e)
			}
		}
		f.shown = shown
	}
	f.clampCursor()
}

// openCurrent enters the highlighted directory, or selects the highlighted file
// when its extension is allowed (emitting msg.FileSelectedMsg).
func (f *FileBrowser) openCurrent() tea.Cmd {
	if len(f.shown) == 0 {
		return nil
	}
	e := f.shown[f.cursor]
	full := filepath.Join(f.dir, e.name)
	if e.isDir {
		f.enter(full)
		return nil
	}
	if !f.selectable(e.name) {
		return nil
	}
	f.selected = full
	return func() tea.Msg { return msg.FileSelectedMsg{Path: full} }
}

// goParent moves to the parent directory, unless already at the filesystem root.
func (f *FileBrowser) goParent() {
	if parent := filepath.Dir(f.dir); parent != f.dir {
		f.enter(parent)
	}
}

// enter switches to dir, resetting the selection and clearing the filter so it
// stays scoped to a single directory.
func (f *FileBrowser) enter(dir string) {
	f.dir = dir
	f.cursor, f.offset = 0, 0
	f.filter = ""
	f.readDir()
}

// selectable reports whether name has one of the allowed extensions (any file
// when no extensions were configured).
func (f *FileBrowser) selectable(name string) bool {
	if len(f.allowed) == 0 {
		return true
	}
	ext := strings.ToLower(filepath.Ext(name))
	for _, a := range f.allowed {
		if ext == strings.ToLower(a) {
			return true
		}
	}
	return false
}

// move shifts the selection by d rows, clamping into range.
func (f *FileBrowser) move(d int) {
	f.cursor += d
	f.clampCursor()
}

// clampCursor keeps the cursor in range and scrolls the window to keep it
// visible.
func (f *FileBrowser) clampCursor() {
	if len(f.shown) == 0 {
		f.cursor, f.offset = 0, 0
		return
	}
	if f.cursor < 0 {
		f.cursor = 0
	}
	if f.cursor > len(f.shown)-1 {
		f.cursor = len(f.shown) - 1
	}
	vis := f.visibleRows()
	if f.cursor < f.offset {
		f.offset = f.cursor
	}
	if f.cursor >= f.offset+vis {
		f.offset = f.cursor - vis + 1
	}
	if f.offset < 0 {
		f.offset = 0
	}
}

// visibleRows is the number of entry rows the current height can show.
func (f *FileBrowser) visibleRows() int {
	if f.height < 1 {
		return 1
	}
	return f.height
}

// pageStep is the row count a page key moves (a screen minus one for overlap).
func (f *FileBrowser) pageStep() int {
	if n := f.visibleRows() - 1; n > 1 {
		return n
	}
	return 1
}

var (
	_ tui.Component = (*FileBrowser)(nil)
	_ tui.Sizeable  = (*FileBrowser)(nil)
	_ tui.Focusable = (*FileBrowser)(nil)
)
