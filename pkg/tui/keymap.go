package tui

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Quit     key.Binding
	Help     key.Binding
	Back     key.Binding
	Refresh  key.Binding
	Enter    key.Binding
	Esc      key.Binding
	Team     key.Binding
	Silence  key.Binding
	Ack      key.Binding
	Escalate key.Binding
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit, k.Enter}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	// TODO: Return a pop-over window here instead
	return [][]key.Binding{
		{k.Up, k.Down, k.Quit, k.Help, k.Back, k.Refresh}, // First column
	}
}

var defaultKeyMap = KeyMap{
	Up: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp(upArrow+"/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp(downArrow+"/j", "down"),
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
	Refresh: key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "refresh"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "view details"),
	),
	Esc: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Team: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "toggle team/individual"),
	),
	Silence: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "silence"),
	),
	Ack: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "acknowledge"),
	),
	Escalate: key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("u", "un-acknowledge"),
	),
}
