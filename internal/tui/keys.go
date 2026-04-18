package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap holds all key bindings for the TUI launcher.
type KeyMap struct {
	Quit   key.Binding
	Enter  key.Binding
	Delete key.Binding
	Save   key.Binding
	Help   key.Binding
	Filter key.Binding
	Escape key.Binding
}

// DefaultKeyMap returns the default key bindings for the TUI launcher.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("↵", "restore"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
		Save: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "save current"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "clear filter"),
		),
	}
}
