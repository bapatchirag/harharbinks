// Package msg defines the decoupled Bubble Tea messages that components emit and
// screens consume. Routing communication through shared message types keeps
// components independent of one another and of the application layer: a
// component returns a command that yields one of these messages, and the
// enclosing screen decides what to do. These types carry no HAR, PCAP, or audit
// knowledge.
package msg

// Level classifies the severity of a transient notification (see the Toast
// component and ToastMsg).
type Level int

// Notification severities, ordered from least to most urgent.
const (
	Info Level = iota
	Success
	Warning
	Error
)

// String returns a short lowercase label for the level.
func (l Level) String() string {
	switch l {
	case Success:
		return "success"
	case Warning:
		return "warning"
	case Error:
		return "error"
	default:
		return "info"
	}
}

// SelectedMsg is emitted by list-like components (List, Table, Menu) when the
// user confirms the current item. Index is the item's position in the component's
// current (possibly filtered) view.
type SelectedMsg struct {
	Index int
}

// SearchMsg is emitted by the Search component whenever the query changes, so a
// screen can filter live.
type SearchMsg struct {
	Query string
}

// MenuActionMsg is emitted by the Menu component when the user activates an
// item; Action is the item's opaque action identifier.
type MenuActionMsg struct {
	Action string
}

// FileSelectedMsg is emitted by the FileBrowser when the user picks a file.
type FileSelectedMsg struct {
	Path string
}

// ToastMsg requests that a transient notification be shown.
type ToastMsg struct {
	Text  string
	Level Level
}

// DismissMsg requests that the active overlay (Modal or Toast) be dismissed.
type DismissMsg struct{}

// ErrorMsg carries an error to be surfaced to the user.
type ErrorMsg struct {
	Err error
}
