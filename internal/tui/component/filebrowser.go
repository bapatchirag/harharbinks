package component

import (
	"os"

	"github.com/charmbracelet/bubbles/filepicker"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bapatchirag/harharbinks/internal/tui"
	"github.com/bapatchirag/harharbinks/internal/tui/msg"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// FileBrowser is a focusable file picker for opening captures. It wraps the
// bubbles filepicker, optionally restricting selectable files by extension, and
// emits msg.FileSelectedMsg when the user chooses a file.
type FileBrowser struct {
	fp       filepicker.Model
	focused  bool
	selected string
	theme    theme.Theme
}

// NewFileBrowser creates a browser rooted at the current working directory. If
// allowed is non-empty, only files with those extensions (e.g. ".har", ".pcap")
// are selectable.
func NewFileBrowser(th theme.Theme, allowed []string) *FileBrowser {
	fp := filepicker.New()
	fp.AllowedTypes = allowed
	if cwd, err := os.Getwd(); err == nil {
		fp.CurrentDirectory = cwd
	}
	return &FileBrowser{fp: fp, theme: th}
}

// SetSize sets the number of file rows shown (the width is managed internally).
func (f *FileBrowser) SetSize(_, h int) { f.fp.Height = h }

// Selected returns the most recently chosen file path, if any.
func (f *FileBrowser) Selected() string { return f.selected }

// Focus gives the browser input focus.
func (f *FileBrowser) Focus() { f.focused = true }

// Blur removes input focus.
func (f *FileBrowser) Blur() { f.focused = false }

// Focused reports whether the browser has focus.
func (f *FileBrowser) Focused() bool { return f.focused }

// Init starts reading the initial directory.
func (f *FileBrowser) Init() tea.Cmd { return f.fp.Init() }

// Update advances the picker and emits msg.FileSelectedMsg once a file is chosen.
func (f *FileBrowser) Update(tmsg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	f.fp, cmd = f.fp.Update(tmsg)
	cmds := []tea.Cmd{cmd}
	if ok, path := f.fp.DidSelectFile(tmsg); ok {
		f.selected = path
		cmds = append(cmds, func() tea.Msg { return msg.FileSelectedMsg{Path: path} })
	}
	return tea.Batch(cmds...)
}

// View renders the file listing.
func (f *FileBrowser) View() string { return f.fp.View() }

var (
	_ tui.Component = (*FileBrowser)(nil)
	_ tui.Sizeable  = (*FileBrowser)(nil)
	_ tui.Focusable = (*FileBrowser)(nil)
)
