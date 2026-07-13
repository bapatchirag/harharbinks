package component

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/muesli/termenv"

	"github.com/bapatchirag/harharbinks/internal/tui/keymap"
	"github.com/bapatchirag/harharbinks/internal/tui/msg"
	"github.com/bapatchirag/harharbinks/internal/tui/theme"
)

// TestMain forces a color-free profile so golden output is deterministic across
// environments (it captures layout and content, not ANSI color codes).
func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.Ascii)
	os.Exit(m.Run())
}

// --- builders --------------------------------------------------------------

func demoTable() *Table[string] {
	cols := []Column[string]{
		{Title: "NAME", Width: 8, Render: func(s string) string { return s }},
		{Title: "LEN", Width: 3, Render: func(s string) string { return fmt.Sprintf("%d", len(s)) }},
	}
	tbl := NewTable(cols, theme.Default(), keymap.Default())
	tbl.SetRows([]string{"alpha", "beta", "gamma", "delta", "epsilon"})
	return tbl
}

func demoList() *List[string] {
	l := NewList(func(s string) string { return s }, theme.Default(), keymap.Default())
	l.SetTitle("Items")
	l.SetItems([]string{"one", "two", "three", "four"})
	return l
}

func demoMenu() *Menu {
	mn := NewMenu([]MenuItem{
		{Key: "a", Title: "Alpha", Action: "a"},
		{Key: "b", Title: "Bravo", Action: "b"},
		{Key: "c", Title: "Charlie", Action: "c"},
	}, theme.Default(), keymap.Default())
	mn.SetTitle("Menu")
	return mn
}

// --- test helpers ----------------------------------------------------------

func keyDown() tea.Msg  { return tea.KeyMsg{Type: tea.KeyDown} }
func keyUp() tea.Msg    { return tea.KeyMsg{Type: tea.KeyUp} }
func keyEnter() tea.Msg { return tea.KeyMsg{Type: tea.KeyEnter} }

// single executes cmd and returns its single message (nil for a nil cmd).
func single(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

// collect executes cmd and flattens any tea.Batch into a slice of messages.
func collect(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	m := cmd()
	if batch, ok := m.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, collect(c)...)
		}
		return out
	}
	return []tea.Msg{m}
}

// --- golden tests ----------------------------------------------------------

func TestTableGolden(t *testing.T) {
	tbl := demoTable()
	tbl.SetSize(30, 6)
	tbl.Focus()
	golden.RequireEqual(t, []byte(tbl.View()))
}

// TestTableFlexColumn checks that a Flex column expands to share the table's
// leftover width, so a long value shows far more than its declared width.
func TestTableFlexColumn(t *testing.T) {
	long := "this-is-a-very-long-value-that-should-flex"
	cols := []Column[string]{
		{Title: "K", Width: 3, Render: func(string) string { return "k" }},
		{Title: "V", Width: 4, Flex: true, Render: func(s string) string { return s }},
	}
	tbl := NewTable(cols, theme.Default(), keymap.Default())
	tbl.SetRows([]string{long})
	tbl.SetSize(40, 3)

	// Flex width = 40 - gutter(2) - separator(1) - K(3) = 34, far past the
	// declared width of 4, so most of the value is visible.
	if out := tbl.View(); !strings.Contains(out, long[:30]) {
		t.Errorf("flex column should expand to show the long value; got:\n%s", out)
	}
}

// TestTableColorColumn exercises a per-cell Color on an unselected row: the text
// still renders correctly (color is stripped under the test's Ascii profile).
func TestTableColorColumn(t *testing.T) {
	cols := []Column[string]{
		{Title: "V", Width: 6,
			Render: func(s string) string { return s },
			Color:  func(string) lipgloss.Color { return lipgloss.Color("2") }},
	}
	tbl := NewTable(cols, theme.Default(), keymap.Default())
	tbl.SetRows([]string{"aa", "bb"})
	tbl.SetSize(20, 4)
	tbl.Focus() // cursor on row 0; row 1 is unselected so its Color path runs
	if out := tbl.View(); !strings.Contains(out, "aa") || !strings.Contains(out, "bb") {
		t.Errorf("colored column should render all rows; got:\n%s", out)
	}
}

func TestListGolden(t *testing.T) {
	l := demoList()
	l.SetSize(20, 6)
	l.Focus()
	golden.RequireEqual(t, []byte(l.View()))
}

func TestMenuGolden(t *testing.T) {
	mn := demoMenu()
	mn.SetSize(20, 6)
	mn.Focus()
	golden.RequireEqual(t, []byte(mn.View()))
}

func TestStatusBarGolden(t *testing.T) {
	s := NewStatusBar(theme.Default())
	s.SetSize(30, 1)
	s.SetLeft("GET 200")
	s.SetCenter("mid")
	s.SetRight("1.2KB")
	golden.RequireEqual(t, []byte(s.View()))
}

func TestModalGolden(t *testing.T) {
	md := NewModal(theme.Default(), keymap.Default())
	md.SetSize(50, 12)
	md.Show("Title", "Body line one\nBody line two")
	golden.RequireEqual(t, []byte(md.View()))
}

func TestViewportGolden(t *testing.T) {
	v := NewViewport(theme.Default())
	v.SetSize(24, 3)
	v.Focus()
	v.SetContent("line one\nline two\nline three\nline four\nline five")
	golden.RequireEqual(t, []byte(v.View()))
}

// --- behavior tests --------------------------------------------------------

func TestTableNavigationClamps(t *testing.T) {
	tbl := demoTable() // 5 rows
	tbl.SetSize(40, 4) // 3 visible rows
	tbl.Focus()

	tbl.Update(keyUp()) // already at top
	if tbl.Cursor() != 0 {
		t.Errorf("up at top: cursor = %d, want 0", tbl.Cursor())
	}
	tbl.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if tbl.Cursor() != 4 {
		t.Errorf("end: cursor = %d, want 4", tbl.Cursor())
	}
	tbl.Update(keyDown()) // already at bottom
	if tbl.Cursor() != 4 {
		t.Errorf("down at bottom: cursor = %d, want 4", tbl.Cursor())
	}
}

func TestTableSelectEmitsMessage(t *testing.T) {
	tbl := demoTable()
	tbl.SetSize(40, 6)
	tbl.Focus()
	tbl.Update(keyDown())
	got := single(tbl.Update(keyEnter()))
	sel, ok := got.(msg.SelectedMsg)
	if !ok || sel.Index != 1 {
		t.Errorf("got %#v, want msg.SelectedMsg{Index:1}", got)
	}
}

func TestUnfocusedTableIgnoresInput(t *testing.T) {
	tbl := demoTable()
	tbl.SetSize(40, 6) // not focused
	tbl.Update(keyDown())
	if tbl.Cursor() != 0 {
		t.Errorf("unfocused table moved cursor to %d", tbl.Cursor())
	}
}

func TestListSelectEmitsMessage(t *testing.T) {
	l := demoList()
	l.SetSize(20, 6)
	l.Focus()
	l.Update(keyDown())
	l.Update(keyDown())
	got := single(l.Update(keyEnter()))
	sel, ok := got.(msg.SelectedMsg)
	if !ok || sel.Index != 2 {
		t.Errorf("got %#v, want msg.SelectedMsg{Index:2}", got)
	}
}

func TestMenuWrapsAndEmitsAction(t *testing.T) {
	mn := demoMenu()
	mn.Focus()
	mn.Update(keyUp()) // wrap from 0 to last (index 2)
	if mn.Cursor() != 2 {
		t.Fatalf("menu cursor = %d, want 2 after wrap", mn.Cursor())
	}
	got := single(mn.Update(keyEnter()))
	act, ok := got.(msg.MenuActionMsg)
	if !ok || act.Action != "c" {
		t.Errorf("got %#v, want msg.MenuActionMsg{Action:\"c\"}", got)
	}
}

func TestMenuActivatesByShortcut(t *testing.T) {
	mn := demoMenu()
	mn.Focus()
	// Typing an item's shortcut Key ("b" for Bravo) activates it and moves the
	// cursor to it, without needing to navigate and press enter.
	got := single(mn.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}}))
	act, ok := got.(msg.MenuActionMsg)
	if !ok || act.Action != "b" {
		t.Errorf("got %#v, want msg.MenuActionMsg{Action:\"b\"}", got)
	}
	if mn.Cursor() != 1 {
		t.Errorf("shortcut should move the cursor to the item, cursor = %d, want 1", mn.Cursor())
	}
}

func TestSearchEmitsSearchMessage(t *testing.T) {
	s := NewSearch(theme.Default(), "placeholder")
	s.Focus()
	cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if s.Value() != "a" {
		t.Fatalf("value = %q, want %q", s.Value(), "a")
	}
	found := false
	for _, m := range collect(cmd) {
		if sm, ok := m.(msg.SearchMsg); ok && sm.Query == "a" {
			found = true
		}
	}
	if !found {
		t.Errorf("no msg.SearchMsg{Query:\"a\"} emitted")
	}
}

func TestToastShowAndAutoDismiss(t *testing.T) {
	to := NewToast(theme.Default())
	if to.Visible() {
		t.Fatal("new toast should be hidden")
	}
	if cmd := to.Show("hi", msg.Info); cmd == nil {
		t.Fatal("Show should return an auto-dismiss command")
	}
	if !to.Visible() {
		t.Fatal("toast should be visible after Show")
	}
	// A stale timeout (older sequence) must not hide the toast.
	to.Update(toastTimeoutMsg{seq: to.seq - 1})
	if !to.Visible() {
		t.Fatal("stale timeout should not dismiss")
	}
	// The current timeout dismisses it.
	to.Update(toastTimeoutMsg{seq: to.seq})
	if to.Visible() {
		t.Fatal("current timeout should dismiss")
	}
}

func TestModalDismiss(t *testing.T) {
	md := NewModal(theme.Default(), keymap.Default())
	md.Show("t", "b")
	if !md.Visible() {
		t.Fatal("modal should be visible after Show")
	}
	got := single(md.Update(tea.KeyMsg{Type: tea.KeyEsc}))
	if md.Visible() {
		t.Fatal("esc should dismiss the modal")
	}
	if _, ok := got.(msg.DismissMsg); !ok {
		t.Errorf("got %#v, want msg.DismissMsg", got)
	}
}

// --- teatest ---------------------------------------------------------------

// tableHarness adapts a Table into a standalone tea.Model for teatest.
type tableHarness struct{ tbl *Table[string] }

func (h tableHarness) Init() tea.Cmd                         { return h.tbl.Init() }
func (h tableHarness) Update(m tea.Msg) (tea.Model, tea.Cmd) { return h, h.tbl.Update(m) }
func (h tableHarness) View() string                          { return h.tbl.View() }

func TestTableTeatest(t *testing.T) {
	tbl := demoTable()
	tbl.SetSize(40, 6)
	tbl.Focus()

	tm := teatest.NewTestModel(t, tableHarness{tbl: tbl}, teatest.WithInitialTermSize(40, 8))
	tm.Send(keyDown())
	tm.Send(keyDown())
	if err := tm.Quit(); err != nil {
		t.Fatalf("quit: %v", err)
	}
	fm := tm.FinalModel(t).(tableHarness)
	if got := fm.tbl.Cursor(); got != 2 {
		t.Errorf("after two downs, cursor = %d, want 2", got)
	}
}

// --- reusability guard -----------------------------------------------------

// TestReusabilityGuard enforces the architectural rule that the format-agnostic
// TUI foundation never depends on the HAR, PCAP, or application layers.
func TestReusabilityGuard(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps",
		"github.com/bapatchirag/harharbinks/internal/tui/...").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps: %v\n%s", err, out)
	}
	forbidden := []string{
		"harharbinks/internal/har",
		"harharbinks/internal/pcap",
		"harharbinks/internal/app",
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		for _, f := range forbidden {
			if strings.Contains(line, f) {
				t.Errorf("foundation must not depend on %q (found dependency %q)", f, line)
			}
		}
	}
}
