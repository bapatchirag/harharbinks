// Command gallery is a standalone Bubble Tea program that exercises every
// reusable harharbinks TUI component in isolation. It is a living demo and a
// manual-testing surface for the foundation library shipped in milestone v0.2.0;
// it depends only on internal/tui and never on the HAR, PCAP, or audit domains.
//
// Run it with: go run ./cmd/gallery
//
// Switch demos with Tab / Shift+Tab, interact with the focused component using
// the arrow keys and Enter, and quit with q or Ctrl+C.
package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bapatchirag/harharbinks/internal/tui"
	"github.com/bapatchirag/harharbinks/internal/tui/component"
	"github.com/bapatchirag/harharbinks/internal/tui/keymap"
	"github.com/bapatchirag/harharbinks/internal/tui/layout"
	appmsg "github.com/bapatchirag/harharbinks/internal/tui/msg"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// row is a sample record for the Table demo.
type row struct {
	method string
	status int
	url    string
	size   int
}

// demo describes one entry in the gallery: a name, a body renderer, and
// optional focus/update/key hooks for its component.
type demo struct {
	name   string
	hint   string
	view   func() string
	focus  func()
	blur   func()
	update func(tea.Msg) tea.Cmd
	onKey  func(tea.KeyMsg) tea.Cmd
}

type model struct {
	theme  theme.Theme
	keys   keymap.KeyMap
	width  int
	height int
	active int
	demos  []demo

	// Components under demonstration.
	table  *component.Table[row]
	list   *component.List[string]
	view   *component.Viewport
	tree   *component.Tree[string]
	hex    *component.HexView
	search *component.Search
	menu   *component.Menu
	files  *component.FileBrowser

	// Always-present chrome and overlays.
	status *component.StatusBar
	toast  *component.Toast
	modal  *component.Modal

	sizeables []tui.Sizeable
	fruits    []string
}

func newModel() *model {
	th := theme.Default()
	km := keymap.Default()

	table := component.NewTable([]component.Column[row]{
		{Title: "METHOD", Width: 7, Render: func(r row) string { return r.method }},
		{Title: "STATUS", Width: 6, Render: func(r row) string { return fmt.Sprintf("%d", r.status) }},
		{Title: "URL", Width: 44, Render: func(r row) string { return r.url }},
		{Title: "SIZE", Width: 8, Render: func(r row) string { return fmt.Sprintf("%d", r.size) }},
	}, th, km)
	table.SetRows([]row{
		{"GET", 200, "https://example.com/", 1240},
		{"GET", 200, "https://example.com/app.js", 88213},
		{"POST", 302, "https://example.com/login", 512},
		{"GET", 404, "https://example.com/favicon.ico", 0},
		{"GET", 200, "https://api.example.com/v1/users", 4096},
		{"DELETE", 204, "https://api.example.com/v1/users/7", 0},
	})

	list := component.NewList(func(s string) string { return "• " + s }, th, km)
	list.SetTitle("Fruits")
	fruits := []string{"apple", "apricot", "banana", "blueberry", "cherry", "date", "elderberry", "fig", "grape", "kiwi", "lemon", "mango"}
	list.SetItems(fruits)

	vp := component.NewViewport(th)
	vp.SetContent(sampleProse)

	tree := component.NewTree(func(s string) string { return s }, th, km)
	tree.SetRoots([]*component.TreeNode[string]{
		component.Branch("Ethernet II",
			component.Leaf("Destination: ff:ff:ff:ff:ff:ff"),
			component.Leaf("Source: 00:1a:2b:3c:4d:5e"),
			component.Leaf("Type: IPv4 (0x0800)"),
		),
		component.Branch("Internet Protocol Version 4",
			component.Leaf("Source: 10.0.0.1"),
			component.Leaf("Destination: 93.184.216.34"),
			component.Branch("Flags: 0x02 (Don't Fragment)",
				component.Leaf("Reserved bit: Not set"),
				component.Leaf("Don't fragment: Set"),
			),
		),
		component.Branch("Transmission Control Protocol",
			component.Leaf("Source Port: 54321"),
			component.Leaf("Destination Port: 80"),
			component.Leaf("Flags: 0x018 (PSH, ACK)"),
		),
	})

	hex := component.NewHexView(th, km)
	hex.SetData([]byte("GET /index.html HTTP/1.1\r\nHost: example.com\r\nUser-Agent: harharbinks/1.0\r\nAccept: */*\r\n\r\n"))
	hex.SetHighlight(0, 3) // the request method, to show the highlight feature

	search := component.NewSearch(th, "type to filter fruits…")

	menu := component.NewMenu([]component.MenuItem{
		{Key: "o", Title: "Open file…", Action: "open"},
		{Key: "e", Title: "Export as cURL", Action: "export"},
		{Key: "r", Title: "Reload", Action: "reload"},
		{Key: "q", Title: "Quit", Action: "quit"},
	}, th, km)
	menu.SetTitle("Commands")

	files := component.NewFileBrowser(th, []string{".har", ".pcap", ".pcapng", ".go", ".md"})

	m := &model{
		theme:  th,
		keys:   km,
		table:  table,
		list:   list,
		view:   vp,
		tree:   tree,
		hex:    hex,
		search: search,
		menu:   menu,
		files:  files,
		status: component.NewStatusBar(th),
		toast:  component.NewToast(th),
		modal:  component.NewModal(th, km),
		fruits: fruits,
	}
	m.sizeables = []tui.Sizeable{table, list, vp, tree, hex, search, menu, files}
	m.demos = m.buildDemos()
	m.focusActive()
	return m
}

func (m *model) buildDemos() []demo {
	return []demo{
		{
			name: "Table", hint: "↑/↓ move · enter select",
			view:  func() string { return m.table.View() },
			focus: m.table.Focus, blur: m.table.Blur,
			update: m.table.Update,
		},
		{
			name: "List", hint: "↑/↓ move · enter select",
			view:  func() string { return m.list.View() },
			focus: m.list.Focus, blur: m.list.Blur,
			update: m.list.Update,
		},
		{
			name: "Viewport", hint: "↑/↓ · pgup/pgdn scroll",
			view:  func() string { return m.view.View() },
			focus: m.view.Focus, blur: m.view.Blur,
			update: m.view.Update,
		},
		{
			name: "Tree", hint: "↑/↓ move · →/← expand/collapse · enter toggle",
			view:  func() string { return m.tree.View() },
			focus: m.tree.Focus, blur: m.tree.Blur,
			update: m.tree.Update,
		},
		{
			name: "HexView", hint: "←/→/↑/↓ move byte cursor · pgup/pgdn page",
			view:  func() string { return m.hex.View() },
			focus: m.hex.Focus, blur: m.hex.Blur,
			update: m.hex.Update,
		},
		{
			name: "Search", hint: "type to filter live",
			view: func() string {
				q := strings.ToLower(m.search.Value())
				var matched []string
				for _, f := range m.fruits {
					if strings.Contains(f, q) {
						matched = append(matched, "• "+f)
					}
				}
				if len(matched) == 0 {
					matched = []string{m.theme.MutedText().Render("(no matches)")}
				}
				return m.search.View() + "\n\n" +
					m.theme.MutedText().Render("matches:") + "\n" +
					strings.Join(matched, "\n")
			},
			focus: m.search.Focus, blur: m.search.Blur,
			update: m.search.Update,
		},
		{
			name: "Menu", hint: "↑/↓ move · enter run",
			view:  func() string { return m.menu.View() },
			focus: m.menu.Focus, blur: m.menu.Blur,
			update: m.menu.Update,
		},
		{
			name: "StatusBar", hint: "a passive full-width bar",
			view: func() string {
				sample := component.NewStatusBar(m.theme)
				sample.SetSize(m.width, 1)
				sample.SetLeft(" GET 200 ")
				sample.SetCenter("example.com/app.js")
				sample.SetRight(" 88.2 KB ")
				return sample.View() + "\n\n" +
					m.theme.MutedText().Render("Left / center / right segments, rendered across the full width.")
			},
		},
		{
			name: "Modal", hint: "enter opens · esc closes",
			view: func() string {
				return m.theme.Base().Render("Press enter to open a modal dialog.\nIt composites over this view via layout.Center.")
			},
			onKey: func(k tea.KeyMsg) tea.Cmd {
				if key.Matches(k, m.keys.Enter) && !m.modal.Visible() {
					m.modal.Show("About harharbinks", "An offline HAR & PCAP viewer\nand passive security auditor.\n\nPress esc or enter to close.")
				}
				return nil
			},
		},
		{
			name: "Toast", hint: "i/s/w/e trigger a toast",
			view: func() string {
				return m.theme.Base().Render("Press a key to raise a toast:\n\n  i  info\n  s  success\n  w  warning\n  e  error\n\nToasts auto-dismiss after a few seconds.")
			},
			onKey: func(k tea.KeyMsg) tea.Cmd {
				switch k.String() {
				case "i":
					return m.toast.Show("Just so you know.", appmsg.Info)
				case "s":
					return m.toast.Show("Saved successfully.", appmsg.Success)
				case "w":
					return m.toast.Show("Heads up: large body.", appmsg.Warning)
				case "e":
					return m.toast.Show("Something went wrong.", appmsg.Error)
				}
				return nil
			},
		},
		{
			name: "FileBrowser", hint: "↑/↓ · enter open dir/file",
			view:  func() string { return m.files.View() },
			focus: m.files.Focus, blur: m.files.Blur,
			update: m.files.Update,
		},
	}
}

func (m *model) Init() tea.Cmd {
	return m.files.Init()
}

func (m *model) Update(tmsg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := tmsg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		// While a modal is up it owns all keys.
		if m.modal.Visible() {
			return m, m.modal.Update(tmsg)
		}
		if msg.String() == "q" && m.demos[m.active].name != "Search" {
			return m, tea.Quit
		}
		switch {
		case key.Matches(msg, m.keys.Tab):
			m.switchDemo(1)
			return m, nil
		case key.Matches(msg, m.keys.ShiftTab):
			m.switchDemo(-1)
			return m, nil
		}
		if h := m.demos[m.active].onKey; h != nil {
			cmds = append(cmds, h(msg))
		}

	case appmsg.SelectedMsg:
		cmds = append(cmds, m.toast.Show(fmt.Sprintf("%s: selected #%d", m.demos[m.active].name, msg.Index+1), appmsg.Info))

	case appmsg.MenuActionMsg:
		if msg.Action == "quit" {
			return m, tea.Quit
		}
		cmds = append(cmds, m.toast.Show("action: "+msg.Action, appmsg.Success))

	case appmsg.FileSelectedMsg:
		cmds = append(cmds, m.toast.Show("opened: "+msg.Path, appmsg.Success))
	}

	// Forward to the active component and always to the toast (for its timer).
	if up := m.demos[m.active].update; up != nil {
		cmds = append(cmds, up(tmsg))
	}
	cmds = append(cmds, m.toast.Update(tmsg))
	m.refreshStatus()
	return m, tea.Batch(cmds...)
}

func (m *model) View() string {
	if m.width == 0 {
		return "initializing…"
	}
	bodyH := m.height - 3
	if bodyH < 1 {
		bodyH = 1
	}
	title := m.theme.Title().Render(" harharbinks · component gallery ")
	body := lipgloss.NewStyle().Width(m.width).Height(bodyH).Render(m.demos[m.active].view())
	screen := lipgloss.JoinVertical(lipgloss.Left, title, m.renderTabs(), body, m.status.View())

	if m.toast.Visible() {
		tv := m.toast.View()
		screen = layout.Overlay(screen, tv, m.width-lipgloss.Width(tv)-1, 2)
	}
	if m.modal.Visible() {
		screen = layout.Center(screen, m.modal.View())
	}
	return screen
}

func (m *model) renderTabs() string {
	parts := make([]string, len(m.demos))
	for i, d := range m.demos {
		label := " " + d.name + " "
		if i == m.active {
			parts[i] = m.theme.Selected().Render(label)
		} else {
			parts[i] = m.theme.MutedText().Render(label)
		}
	}
	return lipgloss.NewStyle().Width(m.width).Render(lipgloss.JoinHorizontal(lipgloss.Top, parts...))
}

func (m *model) switchDemo(delta int) {
	m.demos[m.active].blurActive()
	m.active = (m.active + delta + len(m.demos)) % len(m.demos)
	m.focusActive()
}

func (m *model) focusActive() {
	if f := m.demos[m.active].focus; f != nil {
		f()
	}
	m.refreshStatus()
}

func (m *model) refreshStatus() {
	m.status.SetLeft(" " + m.demos[m.active].hint + " ")
	m.status.SetCenter("tab/shift+tab: switch demo")
	m.status.SetRight(" q: quit ")
}

func (m *model) resize() {
	bodyH := m.height - 3
	if bodyH < 1 {
		bodyH = 1
	}
	for _, s := range m.sizeables {
		s.SetSize(m.width, bodyH)
	}
	m.status.SetSize(m.width, 1)
	m.modal.SetSize(m.width, m.height)
}

// blurActive blurs a demo's component if it has a blur hook.
func (d demo) blurActive() {
	if d.blur != nil {
		d.blur()
	}
}

func main() {
	p := tea.NewProgram(newModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("gallery error:", err)
	}
}

const sampleProse = `harharbinks — offline HAR & PCAP viewer + security auditor

This viewport scrolls. Use the arrow keys, PageUp/PageDown, Home and End.

The component library is deliberately format-agnostic: none of these widgets
knows anything about HAR entries, PCAP packets, or audit findings. Screens in
the application layer adapt domain data into the generic components below.

Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor
incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis
nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.
Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu
fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in
culpa qui officia deserunt mollit anim id est laborum.

Widgets in this gallery: Table, List, Viewport, Search, Menu, StatusBar, Modal,
Toast, and FileBrowser. Each is independently testable and reused across the
HAR and PCAP screens as the project grows.`
