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
	Up:          key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "su")),
	Down:        key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "giù")),
	Tab:         key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "vista")),
	NextBackend: key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→", "apt/snap")),
	PrevBackend: key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←", "apt/snap")),
	Search:      key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "cerca")),
	Enter:       key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "dettagli")),
	Install:     key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "installa")),
	Remove:      key.NewBinding(key.WithKeys("d", "r"), key.WithHelp("d", "rimuovi")),
	Upgrade:     key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "aggiorna")),
	UpgradeAll:  key.NewBinding(key.WithKeys("U"), key.WithHelp("U", "aggiorna tutto")),
	Sync:        key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sync cache")),
	Escape:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "indietro")),
	Quit:        key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "esci")),
	Confirm:     key.NewBinding(key.WithKeys("y", "Y"), key.WithHelp("y", "conferma")),
	Cancel:      key.NewBinding(key.WithKeys("n", "N", "esc"), key.WithHelp("n", "annulla")),
}
