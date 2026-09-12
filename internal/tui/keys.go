package tui

import "charm.land/bubbles/v2/key"

// KeyMap centralizes the first screen's interaction contract.
type KeyMap struct {
	Up   key.Binding
	Down key.Binding
	Quit key.Binding
}

var defaultKeyMap = KeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}
