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

// rowCount is theme + one row per rebindable action + the trailing
// "reset keybindings" row (see resetRow).
func (s *settingsScreen) rowCount() int { return 2 + len(rebindableKeys()) }

// resetRow is the cursor position of the "reset keybindings" row: right
// after the theme row (0) and every rebindable-key row (1..N).
func (s *settingsScreen) resetRow() int { return 1 + len(rebindableKeys()) }

// cycleTheme moves delta steps through ThemeNames (wrapping both ways) and
// applies the result immediately, so every visible screen — including this
// settings box itself — re-skins live as you browse, instead of only
// showing the change after leaving the settings screen.
func (s *settingsScreen) cycleTheme(delta int) {
	names := ThemeNames()
	idx := (themeIndex() + delta + len(names)) % len(names)
	ApplyTheme(names[idx])
	s.statusMsg = fmt.Sprintf("theme: %s (%d/%d)", CurrentTheme(), idx+1, len(names))
}

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
	case key.Matches(msg, keys.PrevBackend):
		if s.cursor == 0 {
			s.cycleTheme(-1)
			return true
		}
	case key.Matches(msg, keys.NextBackend):
		if s.cursor == 0 {
			s.cycleTheme(1)
			return true
		}
	case key.Matches(msg, keys.Enter):
		if s.cursor == 0 {
			s.cycleTheme(1)
			return true
		}
		if s.cursor == s.resetRow() {
			ResetKeybindings()
			s.statusMsg = "keybindings reset to defaults"
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
		s.row(0, fmt.Sprintf("Theme: %s", CurrentTheme()), fmt.Sprintf("%d/%d", themeIndex()+1, len(ThemeNames()))),
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
	rows = append(rows, "", s.row(s.resetRow(), "Reset keybindings to defaults", ""))
	hint := "↑/↓ move   enter select/rebind   esc close"
	switch s.cursor {
	case 0:
		hint = "↑/↓ move   ←/→ browse themes   esc close"
	case s.resetRow():
		hint = "↑/↓ move   enter reset all keybindings   esc close"
	}
	rows = append(rows, "", dimStyle.Render(s.statusMsg), "", dimStyle.Render(hint))

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
