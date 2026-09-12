package tui

import "charm.land/bubbles/v2/key"

// KeyMap centralizes the first screen's interaction contract.
type KeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Quit    key.Binding
	Enter   key.Binding
	Edit    key.Binding
	Reset   key.Binding
	Undo    key.Binding
	Preview key.Binding
	Save    key.Binding
	Help    key.Binding
	Escape  key.Binding
	Left    key.Binding
	Right   key.Binding
	Confirm key.Binding
	Reject  key.Binding
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
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Edit: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "edit"),
	),
	Reset: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "reset"),
	),
	Undo: key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("u", "revert"),
	),
	Preview: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "preview"),
	),
	Save: key.NewBinding(
		key.WithKeys("ctrl+s"),
		key.WithHelp("ctrl+s", "save"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "previous"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "next"),
	),
	Confirm: key.NewBinding(
		key.WithKeys("y", "enter"),
		key.WithHelp("y", "confirm"),
	),
	Reject: key.NewBinding(
		key.WithKeys("n", "esc"),
		key.WithHelp("n", "cancel"),
	),
}
