package tui

import "github.com/charmbracelet/bubbles/key"

func (k keymap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

func (k keymap) FullHelp() [][]key.Binding {
	// TODO: Return a pop-over window here instead
	return [][]key.Binding{
		// Each slice here is a column in the help window
		{k.Up, k.Down, k.Enter, k.Back},
		{k.Team, k.Refresh, k.Ack, k.Silence},
		{k.Note, k.Login, k.Open},
		{k.Quit, k.Help},
	}
}

type keymap struct {
	Up      key.Binding
	Down    key.Binding
	Back    key.Binding
	Enter   key.Binding
	Quit    key.Binding
	Help    key.Binding
	Team    key.Binding
	Refresh key.Binding
	Note    key.Binding
	Silence key.Binding
	Ack     key.Binding
	Input   key.Binding
	Login   key.Binding
	Open    key.Binding
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
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q/ctrl+c", "quit"),
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
	Note: key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "add [n]ote"),
	),
	Silence: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "silence"),
	),
	Ack: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "acknowledge"),
	),
	Input: key.NewBinding(
		key.WithKeys("i"),
		key.WithHelp("i", "input"),
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
