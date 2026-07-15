package tui

import "github.com/charmbracelet/bubbles/key"

func (k keymap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Back, k.Quit}
}

func (k keymap) FullHelp() [][]key.Binding {
	// Column layout:
	// Col 1: Navigation + Help
	// Col 2: Primary incident actions
	// Col 3: Settings & toggles, Quit at bottom
	// Col 4: Chord commands (dynamically generated)

	columns := [][]key.Binding{
		// Column 1: Help at top, navigation
		{k.Help, k.ViewDocs, k.Up, k.Down, k.Top, k.Bottom, k.Enter, k.Back},
		// Column 2: Primary incident actions
		{k.Ack, k.Note, k.Login, k.Open, k.SOP, k.UnAck, k.Silence, k.Merge, k.Tag, k.Input},
		// Column 3: Settings & toggles, Quit at bottom
		{k.Team, k.Refresh, k.AutoRefresh, k.AutoAck, k.Urgency, k.Watcher, k.ViewLog, k.Quit},
		// Column 4: Tab navigation (incident viewer)
		{k.TabNext, k.TabPrev},
	}

	// Column 4: Chord commands (generated from chordActions registry)
	chordBindings := chordHelpBindings()
	if len(chordBindings) > 0 {
		columns = append(columns, chordBindings)
	}

	return columns
}

type keymap struct {
	Up          key.Binding
	Down        key.Binding
	Top         key.Binding
	Bottom      key.Binding
	Back        key.Binding
	Enter       key.Binding
	Quit        key.Binding
	Help        key.Binding
	Team        key.Binding
	Refresh     key.Binding
	AutoRefresh key.Binding
	Note        key.Binding
	Silence     key.Binding
	Ack         key.Binding
	UnAck       key.Binding
	AutoAck     key.Binding
	Urgency     key.Binding
	Input       key.Binding
	Login       key.Binding
	Open        key.Binding
	SOP         key.Binding
	ViewLog     key.Binding
	Merge       key.Binding
	Watcher     key.Binding
	Tag         key.Binding
	TabNext     key.Binding
	TabPrev     key.Binding
	ViewDocs    key.Binding
}

type inputKeymap struct {
	Quit  key.Binding
	Back  key.Binding
	Enter key.Binding
}

func (k inputKeymap) ShortHelp() []key.Binding {
	return []key.Binding{k.Back, k.Enter, k.Quit}
}

func (k inputKeymap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Back, k.Enter},
		{k.Quit},
	}
}

// inputModeKeyMap contains only the keys that work in input mode
var inputModeKeyMap = inputKeymap{
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c", "ctrl+q"),
		key.WithHelp("ctrl+q/ctrl+c", "quit"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "ask Claude"),
	),
}

var defaultKeyMap = keymap{
	Up: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("↓/j", "down"),
	),
	Top: key.NewBinding(
		key.WithKeys("g"),
		key.WithHelp("g", "jump to top"),
	),
	Bottom: key.NewBinding(
		key.WithKeys("G"),
		key.WithHelp("G", "jump to bottom"),
	),
	Quit: key.NewBinding(
		key.WithKeys("ctrl+q", "ctrl+c"),
		key.WithHelp("ctrl+q/ctrl+c", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("h"),
		key.WithHelp("h", "help"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "view"),
	),
	Team: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "toggle team/individual"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	AutoRefresh: key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("ctrl+r", "toggle auto-refresh"),
	),
	Note: key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "add note"),
	),
	Silence: key.NewBinding(
		key.WithKeys("ctrl+s"),
		key.WithHelp("ctrl+s", "silence"),
	),
	Ack: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "acknowledge"),
	),
	UnAck: key.NewBinding(
		key.WithKeys("ctrl+e"),
		key.WithHelp("ctrl+e", "re-escalate"),
	),
	AutoAck: key.NewBinding(
		key.WithKeys("ctrl+a"),
		key.WithHelp("ctrl+a", "toggle auto-acknowledge"),
	),
	Urgency: key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("u", "toggle urgency filter"),
	),
	Input: key.NewBinding(
		key.WithKeys(":", "/"),
		key.WithHelp(":/", "command input"),
	),
	Login: key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "login to cluster"),
	),
	Open: key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", "open in browser"),
	),
	SOP: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "open SOP"),
	),
	ViewLog: key.NewBinding(
		key.WithKeys("ctrl+l"),
		key.WithHelp("ctrl+l", "view debug log"),
	),
	Merge: key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("m", "merge incident"),
	),
	Watcher: key.NewBinding(
		key.WithKeys("w"),
		key.WithHelp("w", "toggle watcher"),
	),
	Tag: key.NewBinding(
		key.WithKeys("ctrl+t"),
		key.WithHelp("ctrl+t", "add tags"),
	),
	TabNext: key.NewBinding(
		key.WithKeys("tab", "right"),
		key.WithHelp("tab/→", "next tab"),
	),
	TabPrev: key.NewBinding(
		key.WithKeys("shift+tab", "left"),
		key.WithHelp("shift+tab/←", "prev tab"),
	),
	ViewDocs: key.NewBinding(
		key.WithKeys("ctrl+h"),
		key.WithHelp("ctrl+h", "docs"),
	),
}

var errorViewKeyMap = keymap{
	Help: key.NewBinding(
		key.WithKeys("h"),
		key.WithHelp("h", "help"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	// Quit in error mode is handled by the global quit matcher in
	// keyMsgHandler, which listens for ctrl+q/ctrl+c — mirror those keys
	// here so the help text matches what actually quits
	Quit: key.NewBinding(
		key.WithKeys("ctrl+q", "ctrl+c"),
		key.WithHelp("ctrl+q/ctrl+c", "quit"),
	),
}
