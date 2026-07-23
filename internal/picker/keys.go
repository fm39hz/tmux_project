package picker

import (
	"charm.land/bubbles/v2/key"
)

type uiKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Confirm key.Binding
	Quit    key.Binding
	Help    key.Binding
	Sticky  key.Binding
	Kill    key.Binding
	Freeze  key.Binding
	Edit    key.Binding
	Delete  key.Binding
}

var defaultKeyMap = uiKeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑/^p", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓/^n", "move down"),
	),
	Confirm: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "connect"),
	),
	Quit: key.NewBinding(
		key.WithKeys("esc", "ctrl+c"),
		key.WithHelp("esc/^c", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Sticky: key.NewBinding(
		key.WithKeys("ctrl+t"),
		key.WithHelp("^t", "sticky template"),
	),
	Kill: key.NewBinding(
		key.WithKeys("ctrl+x"),
		key.WithHelp("^x", "kill session"),
	),
	Freeze: key.NewBinding(
		key.WithKeys("ctrl+f"),
		key.WithHelp("^f", "freeze"),
	),
	Edit: key.NewBinding(
		key.WithKeys("ctrl+e"),
		key.WithHelp("^e", "edit preset"),
	),
	Delete: key.NewBinding(
		key.WithKeys("ctrl+d"),
		key.WithHelp("^d", "delete preset"),
	),
}

func (k uiKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Confirm, k.Quit, k.Help}
}

func (k uiKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Confirm, k.Quit},
		{k.Sticky, k.Kill, k.Freeze, k.Edit, k.Delete},
	}
}

type pickKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Confirm key.Binding
	Quit    key.Binding
}

var pickKeys = pickKeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑/^p", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓/^n", "move down"),
	),
	Confirm: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Quit: key.NewBinding(
		key.WithKeys("esc", "ctrl+c"),
		key.WithHelp("esc/^c", "quit"),
	),
}
