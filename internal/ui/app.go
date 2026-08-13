// Package ui implements the Bubble Tea application: two tabs (APT and
// SNAP), each backed by a Panel that can browse installed/upgradable
// packages, search, view details and run install/remove/upgrade actions.
package ui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/padovanl/pkgtui/internal/apt"
	"github.com/padovanl/pkgtui/internal/snap"
)

type App struct {
	panels []*Panel
	active int

	width, height int
}

func NewApp() *App {
	return &App{
		panels: []*Panel{
			NewPanel(apt.New()),
			NewPanel(snap.New()),
		},
	}
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
		switch {
		case key.Matches(msg, keys.Quit) && !a.activePanel().IsTyping():
			return a, tea.Quit
		case msg.String() == "ctrl+c":
			return a, tea.Quit
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

	case backendMsg:
		for i, p := range a.panels {
			if p.Backend() == msg.Backend() {
				updated, cmd := p.Update(msg)
				a.panels[i] = updated
				return a, cmd
			}
		}
		return a, nil
	}

	return a, nil
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

func (a *App) renderFooter() string {
	p := a.activePanel()
	hints := []key.Binding{keys.NextBackend, keys.Tab, keys.Search, keys.Enter}
	if p.screen == screenList && !p.search.Focused() {
		hints = append(hints, keys.Install, keys.Remove, keys.Upgrade, keys.UpgradeAll)
		if p.SupportsSync() {
			hints = append(hints, keys.Sync)
		}
	}
	hints = append(hints, keys.Help, keys.Quit)

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
	body := a.activePanel().View()
	return lipgloss.JoinVertical(lipgloss.Left,
		a.renderTabBar(),
		body,
		a.renderFooter(),
	)
}
