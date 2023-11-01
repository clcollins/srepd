package tui

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Quit    key.Binding
	Back    key.Binding
	Refresh key.Binding
}

var defaultKeyMap = KeyMap{
	Up: key.NewBinding(
		key.WithKeys("k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("j", "down"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
	),
}
