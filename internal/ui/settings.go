package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/padovanl/pkgtui/internal/config"
)

// settingsScreen is a small app-wide overlay (not per-backend) for cycling
// the color theme and rebinding action keys, both applied immediately and
// persisted to config.
type settingsScreen struct {
	cursor    int // 0 = theme row; 1..N index into rebindableKeys()
	capturing bool
	statusMsg string
}

func newSettingsScreen() *settingsScreen { return &settingsScreen{} }

// displayKey renders a raw key string for the settings list; " " alone is
// invisible next to a label, so spell it out like the rest of the UI does.
func displayKey(k string) string {
	if k == " " {
		return "space"
	}
	return k
}

func (s *settingsScreen) rowCount() int { return 1 + len(rebindableKeys()) }

// handleKey processes one keypress. changed reports whether persisted state
// (theme or a keybinding) actually changed, so the caller knows to save.
func (s *settingsScreen) handleKey(msg tea.KeyMsg) (changed bool) {
	if s.capturing {
		s.capturing = false
		if key.Matches(msg, keys.Escape) {
			s.statusMsg = "cancelled"
			return false
		}
		entry := rebindableKeys()[s.cursor-1]
		newKey := msg.String()
		if conflict, ok := RebindKey(entry.action, newKey); !ok {
			s.statusMsg = fmt.Sprintf("%q is already used by %q", newKey, conflict)
			return false
		}
		s.statusMsg = fmt.Sprintf("%s -> %s", entry.label, displayKey(newKey))
		return true
	}

	switch {
	case key.Matches(msg, keys.Up):
		if s.cursor > 0 {
			s.cursor--
		}
	case key.Matches(msg, keys.Down):
		if s.cursor < s.rowCount()-1 {
			s.cursor++
		}
	case key.Matches(msg, keys.Enter):
		if s.cursor == 0 {
			names := ThemeNames()
			idx := 0
			for i, n := range names {
				if n == CurrentTheme() {
					idx = i
				}
			}
			ApplyTheme(names[(idx+1)%len(names)])
			s.statusMsg = "theme: " + CurrentTheme()
			return true
		}
		s.capturing = true
		s.statusMsg = "press a key to rebind (esc to cancel)..."
	}
	return false
}

func (s *settingsScreen) View(width, height int) string {
	rows := []string{
		titleStyle.Render("pkgtui — settings"),
		"",
		s.row(0, fmt.Sprintf("Theme: %s", CurrentTheme()), "enter to cycle"),
		"",
		helpSectionStyle.Render("Keybindings (enter to rebind)"),
	}
	for i, e := range rebindableKeys() {
		k := "-"
		if ks := e.ptr.Keys(); len(ks) > 0 {
			k = displayKey(ks[0])
		}
		rows = append(rows, s.row(i+1, e.label, k))
	}
	rows = append(rows, "", dimStyle.Render(s.statusMsg), "", dimStyle.Render("↑/↓ move   enter select/rebind   esc close"))

	body := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, helpBoxStyle.Render(body))
}

func (s *settingsScreen) row(idx int, label, value string) string {
	labelStyle := lipgloss.NewStyle().Width(30)
	line := labelStyle.Render(label) + dimStyle.Render(value)
	if idx == s.cursor {
		line = lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(colorFg).Bold(true).Render(labelStyle.Render(label) + value)
	}
	return line
}

// saveSettings persists the current theme and keybindings to config,
// leaving last-view fields untouched (callers that also track those pass
// them in separately via saveViewState).
func saveSettings() {
	c, _ := config.Load()
	c.Theme = CurrentTheme()
	c.Keybindings = CurrentKeybindingOverrides()
	_ = c.Save()
}
