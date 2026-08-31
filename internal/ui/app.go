// Package ui implements the Bubble Tea application: one tab per package
// manager present on the system, each backed by a Panel that can browse
// installed/upgradable packages, search, view details and run
// install/remove/upgrade actions.
package ui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/padovanl/pkgtui/internal/apt"
	"github.com/padovanl/pkgtui/internal/config"
	"github.com/padovanl/pkgtui/internal/flatpak"
	"github.com/padovanl/pkgtui/internal/homebrew"
	"github.com/padovanl/pkgtui/internal/macports"
	"github.com/padovanl/pkgtui/internal/pkg"
	"github.com/padovanl/pkgtui/internal/snap"
)

type App struct {
	panels []*Panel
	active int

	settings *settingsScreen // nil unless the settings overlay is open
	overlap  *overlapScreen  // nil unless the cross-backend overlap overlay is open

	width, height int
}

// knownBackends lists every package manager pkgtui can drive, in tab order:
// the Linux system managers first, then the ones that sit on top of a
// system (flatpak) or come from outside it entirely (brew, macports).
func knownBackends() []pkg.Manager {
	return []pkg.Manager{apt.New(), snap.New(), flatpak.New(), homebrew.New(), macports.New()}
}

// filterAvailable keeps only the managers whose tool actually exists here,
// so a Fedora box doesn't carry an apt tab it can never fill and a Mac
// doesn't carry three of them. When nothing at all is available, every
// backend is kept instead: each panel then says so in its own words, which
// is more use than a window with no tabs in it — and on macOS, which is
// exactly where a fresh install finds no package manager it knows, the tabs
// double as the list of what to install to get started.
func filterAvailable(all []pkg.Manager) []pkg.Manager {
	var available []pkg.Manager
	for _, m := range all {
		if m.Available() {
			available = append(available, m)
		}
	}
	if len(available) == 0 {
		return all
	}
	return available
}

func activeBackends() []pkg.Manager { return filterAvailable(knownBackends()) }

func NewApp() *App {
	cfg, _ := config.Load()
	if cfg.Theme != "" {
		ApplyTheme(cfg.Theme)
	}
	if len(cfg.Keybindings) > 0 {
		ApplyKeybindingOverrides(cfg.Keybindings)
	}

	a := &App{}
	for _, mgr := range activeBackends() {
		p := NewPanel(mgr)
		if v, ok := cfg.LastView[mgr.Name()]; ok {
			p.SetInitialMode(v)
		}
		if mgr.Name() == cfg.LastBackend {
			a.active = len(a.panels)
		}
		a.panels = append(a.panels, p)
	}
	return a
}

// managers returns every panel's backend, for the app-wide views that span
// all of them at once rather than belonging to any single panel.
func (a *App) managers() []pkg.Manager {
	mgrs := make([]pkg.Manager, 0, len(a.panels))
	for _, p := range a.panels {
		mgrs = append(mgrs, p.mgr)
	}
	return mgrs
}

func (a *App) Init() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(a.panels))
	for _, p := range a.panels {
		if c := p.Init(); c != nil {
			cmds = append(cmds, c)
		}
	}
	return tea.Batch(cmds...)
}

func (a *App) activePanel() *Panel { return a.panels[a.active] }

// saveViewState persists which backend/view was active, for next launch.
func (a *App) saveViewState() {
	c, _ := config.Load()
	c.LastBackend = a.activePanel().Backend()
	if c.LastView == nil {
		c.LastView = map[string]string{}
	}
	for _, p := range a.panels {
		c.LastView[p.Backend()] = p.ModeName()
	}
	_ = c.Save()
}

func (a *App) quit() (tea.Model, tea.Cmd) {
	a.saveViewState()
	return a, tea.Quit
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		contentH := msg.Height - 3 // tab bar + footer
		if contentH < 5 {
			contentH = 5
		}
		for _, p := range a.panels {
			p.setSize(msg.Width, contentH)
		}
		return a, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+l" {
			// The conventional "redraw the screen" key across terminal
			// tools (vim, bash readline, htop, tmux...), and pkgtui's own
			// answer to any terminal-side rendering glitch: a stale
			// partial redraw a specific terminal emulator's own repaint
			// logic left behind, which — proven by driving the real
			// binary through a real pty and a real terminal emulator in
			// e2e/settings_test.go — isn't a case of pkgtui sending the
			// wrong bytes. tea.ClearScreen forces bubbletea to forget its
			// diff cache and fully repaint from scratch on the next
			// render, not just this key's immediate handler running.
			// Always available, even mid-input: ctrl+l isn't a character
			// any text field would want to actually consume.
			return a, tea.ClearScreen
		}
		if a.settings != nil {
			if changed := a.settings.handleKey(msg); changed {
				saveSettings()
				for _, p := range a.panels {
					p.ApplySettingsChange()
				}
			}
			if !a.settings.capturing && key.Matches(msg, keys.Escape) {
				a.settings = nil
			}
			return a, nil
		}
		if a.overlap != nil {
			if key.Matches(msg, keys.Escape) {
				a.overlap = nil
			} else {
				a.overlap.handleKey(msg)
			}
			return a, nil
		}
		switch {
		case key.Matches(msg, keys.Quit) && !a.activePanel().IsTyping():
			return a.quit()
		case msg.String() == "ctrl+c" && !a.activePanel().IsTyping():
			return a.quit()
		case key.Matches(msg, keys.Settings) && !a.activePanel().IsTyping():
			a.settings = newSettingsScreen()
			return a, nil
		case key.Matches(msg, keys.Overlap) && !a.activePanel().IsTyping():
			a.overlap = newOverlapScreen()
			return a, loadOverlapCmd(a.managers())
		case key.Matches(msg, keys.NextBackend) && !a.activePanel().IsTyping():
			a.active = (a.active + 1) % len(a.panels)
			return a, nil
		case key.Matches(msg, keys.PrevBackend) && !a.activePanel().IsTyping():
			a.active = (a.active - 1 + len(a.panels)) % len(a.panels)
			return a, nil
		}
		p, cmd := a.activePanel().Update(msg)
		a.panels[a.active] = p
		return a, cmd

	case overlapResultMsg:
		if a.overlap != nil {
			a.overlap.loading = false
			a.overlap.err = msg.err
			if msg.err == nil {
				a.overlap.duplicates = msg.duplicates
				a.overlap.stale = msg.stale
			}
		}
		return a, nil

	case backendMsg:
		for i, p := range a.panels {
			if p.Backend() == msg.Backend() {
				updated, cmd := p.Update(msg)
				a.panels[i] = updated
				return a, cmd
			}
		}
		return a, nil

	case tea.MouseMsg:
		if a.settings != nil || a.overlap != nil {
			return a, nil
		}
		if msg.Y == 0 && msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			if i, ok := a.tabAt(msg.X); ok {
				a.active = i
				return a, nil
			}
		}
	}

	// Anything untagged (e.g. list.FilterMatchesMsg, cursor blink ticks)
	// can only belong to whichever panel currently has keyboard focus,
	// since only that panel's list/inputs can be mid-interaction.
	p, cmd := a.activePanel().Update(msg)
	a.panels[a.active] = p
	return a, cmd
}

func (a *App) renderTabBar() string {
	var tabs string
	for i, p := range a.panels {
		label := " " + p.Backend() + " "
		if i == a.active {
			tabs += tabActiveStyle.Render(label)
		} else {
			tabs += tabInactiveStyle.Render(label)
		}
	}
	title := headerBarStyle.Render(" pkgtui ")
	gap := a.width - lipgloss.Width(title) - lipgloss.Width(tabs)
	if gap < 0 {
		gap = 0
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, title, lipgloss.NewStyle().Width(gap).Render(""), tabs)
}

// tabAt returns which panel's tab label a click at column x on the tab bar
// (row 0) landed on, mirroring renderTabBar's own layout so the click zones
// can't drift out of sync with what's drawn.
func (a *App) tabAt(x int) (int, bool) {
	widths := make([]int, len(a.panels))
	total := 0
	for i, p := range a.panels {
		label := " " + p.Backend() + " "
		var rendered string
		if i == a.active {
			rendered = tabActiveStyle.Render(label)
		} else {
			rendered = tabInactiveStyle.Render(label)
		}
		widths[i] = lipgloss.Width(rendered)
		total += widths[i]
	}
	start := a.width - total
	if x < start {
		return 0, false
	}
	pos := start
	for i, w := range widths {
		if x < pos+w {
			return i, true
		}
		pos += w
	}
	return 0, false
}

func (a *App) renderFooter() string {
	p := a.activePanel()
	hints := []key.Binding{keys.NextBackend, keys.Tab, keys.Search, keys.Filter, keys.Enter}
	if p.screen == screenList && !p.search.Focused() {
		hints = append(hints, keys.Install, keys.Remove, keys.Upgrade, keys.UpgradeAll)
		if p.SupportsSync() {
			hints = append(hints, keys.Sync)
		}
	}
	hints = append(hints, keys.Settings, keys.Help, keys.Quit)

	var s string
	for _, h := range hints {
		s += keyHintStyle.Render(h.Help().Key) + dimStyle.Render(" "+h.Help().Desc+"  ")
	}
	return footerBarStyle.Width(a.width).Render(s)
}

func (a *App) View() string {
	if a.width == 0 {
		return "starting..."
	}
	if a.settings != nil {
		return lipgloss.JoinVertical(lipgloss.Left,
			a.renderTabBar(),
			a.settings.View(a.width, a.height-3),
			footerBarStyle.Width(a.width).Render(dimStyle.Render("settings")),
		)
	}
	if a.overlap != nil {
		return lipgloss.JoinVertical(lipgloss.Left,
			a.renderTabBar(),
			a.overlap.View(a.width, a.height-3),
			footerBarStyle.Width(a.width).Render(dimStyle.Render("backend overlap")),
		)
	}
	body := a.activePanel().View()
	if a.activePanel().screen == screenRunning {
		// The global hint bar advertises keys (tab, search, install, q to
		// quit...) that don't apply here: every keystroke instead goes to
		// the running child process. renderRunning already carries its own
		// context-appropriate hint line, so showing it too would just be
		// redundant and, for "q quit", actively misleading.
		return lipgloss.JoinVertical(lipgloss.Left,
			a.renderTabBar(),
			body,
		)
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		a.renderTabBar(),
		body,
		a.renderFooter(),
	)
}
