package tui

import "github.com/charmbracelet/bubbles/key"

func (k keymap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Back, k.Quit}
}

func (k keymap) FullHelp() [][]key.Binding {
	// TODO: Return a pop-over window here instead
	return [][]key.Binding{
		// Each slice here is a column in the help window
		{k.Up, k.Down, k.Top, k.Bottom, k.Enter, k.Back},
		{k.Ack, k.Login, k.Open, k.Note},
		{k.UnAck, k.Silence},
		{k.Team, k.Refresh},
		{k.AutoRefresh, k.AutoAck, k.ToggleActionLog, k.Quit, k.Help},
	}
}

type keymap struct {
	Up              key.Binding
	Down            key.Binding
	Top             key.Binding
	Bottom          key.Binding
	Back            key.Binding
	Enter           key.Binding
	Quit            key.Binding
	Help            key.Binding
	Team            key.Binding
	Refresh         key.Binding
	AutoRefresh     key.Binding
	Note            key.Binding
	Silence         key.Binding
	Ack             key.Binding
	UnAck           key.Binding
	AutoAck         key.Binding
	ToggleActionLog key.Binding
	Input           key.Binding
	Login           key.Binding
	Open            key.Binding
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
		key.WithHelp("enter", "n/a"),
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
	ToggleActionLog: key.NewBinding(
		key.WithKeys("ctrl+l"),
		key.WithHelp("ctrl+l", "toggle action log"),
	),
	Input: key.NewBinding(
		key.WithKeys("i", ":"),
		key.WithHelp("i/:", "input"),
	),
	Login: key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "login to cluster"),
	),
	Open: key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", "open in browser"),
	),
}

var errorViewKeyMap = keymap{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q/ctrl+c", "quit"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
}
