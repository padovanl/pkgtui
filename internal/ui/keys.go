package ui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up          key.Binding
	Down        key.Binding
	Tab         key.Binding
	NextBackend key.Binding
	PrevBackend key.Binding
	Search      key.Binding
	Enter       key.Binding
	Install     key.Binding
	Remove      key.Binding
	Upgrade     key.Binding
	UpgradeAll  key.Binding
	Sync        key.Binding
	Escape      key.Binding
	Quit        key.Binding
	Confirm     key.Binding
	Cancel      key.Binding
}

var keys = keyMap{
	Up:          key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:        key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Tab:         key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "view")),
	NextBackend: key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→", "apt/snap")),
	PrevBackend: key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←", "apt/snap")),
	Search:      key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	Enter:       key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "details")),
	Install:     key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "install")),
	Remove:      key.NewBinding(key.WithKeys("d", "r"), key.WithHelp("d", "remove")),
	Upgrade:     key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "upgrade")),
	UpgradeAll:  key.NewBinding(key.WithKeys("U"), key.WithHelp("U", "upgrade all")),
	Sync:        key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sync cache")),
	Escape:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Quit:        key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Confirm:     key.NewBinding(key.WithKeys("y", "Y"), key.WithHelp("y", "confirm")),
	Cancel:      key.NewBinding(key.WithKeys("n", "N", "esc"), key.WithHelp("n", "cancel")),
}
