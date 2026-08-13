package ui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up          key.Binding
	Down        key.Binding
	Tab         key.Binding
	NextBackend key.Binding
	PrevBackend key.Binding
	Search      key.Binding
	Filter      key.Binding
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
	Help        key.Binding
	ToggleTag   key.Binding
	SortSize    key.Binding
	Channel     key.Binding
	Hold        key.Binding
	Changelog   key.Binding
	PPA         key.Binding
	Settings    key.Binding
}

var keys = keyMap{
	Up:          key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:        key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Tab:         key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "view")),
	NextBackend: key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→", "apt/snap")),
	PrevBackend: key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←", "apt/snap")),
	Search:      key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	Filter:      key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter list")),
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
	Help:        key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	ToggleTag:   key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "tag")),
	SortSize:    key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "sort by size")),
	Channel:     key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "channel")),
	Hold:        key.NewBinding(key.WithKeys("H"), key.WithHelp("H", "hold/unhold")),
	Changelog:   key.NewBinding(key.WithKeys("C"), key.WithHelp("C", "changelog")),
	PPA:         key.NewBinding(key.WithKeys("P"), key.WithHelp("P", "PPAs")),
	Settings:    key.NewBinding(key.WithKeys(","), key.WithHelp(",", "settings")),
}

// bindingEntry is one row of the rebindable-keys registry used by both the
// settings screen and config load/save. action is the stable identifier
// persisted to disk; label is what the settings screen shows.
type bindingEntry struct {
	action string
	label  string
	ptr    *key.Binding
}

// rebindableKeys lists the action keys a user can remap from the settings
// screen. List-navigation (Up/Down) and the y/n/esc modal keys are
// deliberately excluded: remapping those has an outsized chance of leaving
// someone unable to navigate or dismiss a dialog at all.
func rebindableKeys() []bindingEntry {
	return []bindingEntry{
		{"next_backend", "Switch backend (next)", &keys.NextBackend},
		{"prev_backend", "Switch backend (prev)", &keys.PrevBackend},
		{"tab", "Switch view", &keys.Tab},
		{"search", "Search catalog", &keys.Search},
		{"filter", "Filter list", &keys.Filter},
		{"enter", "Package details", &keys.Enter},
		{"tag", "Tag for batch action", &keys.ToggleTag},
		{"install", "Install", &keys.Install},
		{"remove", "Remove", &keys.Remove},
		{"upgrade", "Upgrade", &keys.Upgrade},
		{"upgrade_all", "Upgrade all", &keys.UpgradeAll},
		{"sort_size", "Sort by size", &keys.SortSize},
		{"hold", "Hold/unhold", &keys.Hold},
		{"sync", "Sync cache", &keys.Sync},
		{"channel", "Cycle install channel", &keys.Channel},
		{"changelog", "View changelog", &keys.Changelog},
		{"ppa", "Manage PPAs", &keys.PPA},
		{"settings", "Open settings", &keys.Settings},
		{"help", "Help", &keys.Help},
		{"quit", "Quit", &keys.Quit},
	}
}

// ApplyKeybindingOverrides rebinds actions from a saved config, ignoring
// unknown action names (e.g. from a config written by a newer pkgtui).
func ApplyKeybindingOverrides(overrides map[string]string) {
	for _, e := range rebindableKeys() {
		if k, ok := overrides[e.action]; ok && k != "" {
			e.ptr.SetKeys(k)
		}
	}
}

// CurrentKeybindingOverrides snapshots every rebindable action's current
// primary key, for persisting to config.
func CurrentKeybindingOverrides() map[string]string {
	m := make(map[string]string)
	for _, e := range rebindableKeys() {
		if ks := e.ptr.Keys(); len(ks) > 0 {
			m[e.action] = ks[0]
		}
	}
	return m
}

// RebindKey points action at newKey, refusing if newKey is already used by
// a different rebindable action. Returns the label of the conflicting
// action on failure.
func RebindKey(action, newKey string) (conflict string, ok bool) {
	entries := rebindableKeys()
	var target *bindingEntry
	for i := range entries {
		if entries[i].action == action {
			target = &entries[i]
		}
	}
	if target == nil {
		return "", false
	}
	for _, e := range entries {
		if e.action == action {
			continue
		}
		for _, k := range e.ptr.Keys() {
			if k == newKey {
				return e.label, false
			}
		}
	}
	target.ptr.SetKeys(newKey)
	return "", true
}
