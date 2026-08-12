package ui

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/padovanl/pkgtui/internal/pkg"
)

type viewMode int

const (
	viewInstalled viewMode = iota
	viewUpgradable
	viewSearch
)

func (m viewMode) label() string {
	switch m {
	case viewUpgradable:
		return "Upgradable"
	case viewSearch:
		return "Search"
	default:
		return "Installed"
	}
}

type screen int

const (
	screenList screen = iota
	screenDetail
	screenConfirm
)

// pendingAction holds a destructive/privileged action awaiting user
// confirmation via the y/N modal.
type pendingAction struct {
	label string
	argv  []string
}

// --- messages, each self-tagged with the backend it belongs to so the root
// App model can route it to the right Panel. ---

type backendMsg interface{ Backend() string }

type installedResultMsg struct {
	backend string
	pkgs    []pkg.Package
	err     error
}

func (m installedResultMsg) Backend() string { return m.backend }

type upgradableResultMsg struct {
	backend string
	pkgs    []pkg.Package
	err     error
}

func (m upgradableResultMsg) Backend() string { return m.backend }

type searchResultMsg struct {
	backend string
	pkgs    []pkg.Package
	err     error
}

func (m searchResultMsg) Backend() string { return m.backend }

type infoResultMsg struct {
	backend string
	text    string
	err     error
}

func (m infoResultMsg) Backend() string { return m.backend }

type actionDoneMsg struct {
	backend string
	err     error
}

func (m actionDoneMsg) Backend() string { return m.backend }

// Panel is the self-contained UI + state for a single package manager
// backend (apt or snap).
type Panel struct {
	mgr pkg.Manager

	list     list.Model
	search   textinput.Model
	viewport viewport.Model
	spinner  spinner.Model

	mode   viewMode
	screen screen

	loading       bool
	actionRunning bool
	err           error
	statusMsg     string
	lastQuery     string
	pending       *pendingAction

	width, height int
}

func NewPanel(mgr pkg.Manager) *Panel {
	l := list.New(nil, itemDelegate{}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)

	ti := textinput.New()
	ti.Placeholder = "search packages... (enter to confirm)"
	ti.CharLimit = 100
	ti.Prompt = "🔍 "

	vp := viewport.New(0, 0)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	return &Panel{
		mgr:      mgr,
		list:     l,
		search:   ti,
		viewport: vp,
		spinner:  sp,
		mode:     viewInstalled,
		screen:   screenList,
	}
}

func (p *Panel) Backend() string { return p.mgr.Name() }

// IsTyping reports whether the search box currently owns keyboard input, so
// the root App knows not to steal single-letter shortcuts (including "q").
func (p *Panel) IsTyping() bool { return p.search.Focused() }

func (p *Panel) Init() tea.Cmd {
	if !p.mgr.Available() {
		return nil
	}
	p.loading = true
	return tea.Batch(p.loadInstalledCmd(), p.spinner.Tick)
}

func (p *Panel) loadInstalledCmd() tea.Cmd {
	mgr := p.mgr
	return func() tea.Msg {
		pkgs, err := mgr.ListInstalled()
		return installedResultMsg{backend: mgr.Name(), pkgs: pkgs, err: err}
	}
}

func (p *Panel) loadUpgradableCmd() tea.Cmd {
	mgr := p.mgr
	return func() tea.Msg {
		pkgs, err := mgr.ListUpgradable()
		return upgradableResultMsg{backend: mgr.Name(), pkgs: pkgs, err: err}
	}
}

func (p *Panel) searchCmd(query string) tea.Cmd {
	mgr := p.mgr
	return func() tea.Msg {
		pkgs, err := mgr.Search(query)
		return searchResultMsg{backend: mgr.Name(), pkgs: pkgs, err: err}
	}
}

func (p *Panel) infoCmd(name string) tea.Cmd {
	mgr := p.mgr
	return func() tea.Msg {
		text, err := mgr.Info(name)
		return infoResultMsg{backend: mgr.Name(), text: text, err: err}
	}
}

func (p *Panel) setItems(pkgs []pkg.Package) {
	p.list.SetItems(itemsFrom(pkgs))
}

func (p *Panel) setSize(w, h int) {
	p.width, p.height = w, h
	headerH := 1
	searchH := 0
	if p.mode == viewSearch {
		searchH = 3
	}
	statusH := 1
	listH := h - headerH - searchH - statusH
	if listH < 3 {
		listH = 3
	}
	if w > 4 {
		p.list.SetSize(w-2, listH)
	} else {
		p.list.SetSize(w, listH)
	}
	p.viewport.Width = w - 4
	p.viewport.Height = h - 4
	p.search.Width = w - 8
}

func (p *Panel) Update(msg tea.Msg) (*Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case installedResultMsg:
		p.loading = false
		p.err = msg.err
		if msg.err == nil && p.mode == viewInstalled {
			p.setItems(msg.pkgs)
		}
		return p, nil
	case upgradableResultMsg:
		p.loading = false
		p.err = msg.err
		if msg.err == nil && p.mode == viewUpgradable {
			p.setItems(msg.pkgs)
		}
		return p, nil
	case searchResultMsg:
		p.loading = false
		p.err = msg.err
		if msg.err == nil && p.mode == viewSearch {
			p.setItems(msg.pkgs)
			p.statusMsg = fmt.Sprintf("%d results for %q", len(msg.pkgs), p.lastQuery)
		}
		return p, nil
	case infoResultMsg:
		p.loading = false
		if msg.err != nil {
			p.err = msg.err
			return p, nil
		}
		p.err = nil
		p.viewport.SetContent(msg.text)
		p.viewport.GotoTop()
		p.screen = screenDetail
		return p, nil
	case actionDoneMsg:
		p.actionRunning = false
		p.screen = screenList
		if msg.err != nil {
			p.statusMsg = errorStyle.Render("Error: " + msg.err.Error())
		} else {
			p.statusMsg = "Done."
		}
		p.loading = true
		return p, tea.Batch(p.refreshCmd(), p.spinner.Tick)
	case spinner.TickMsg:
		var cmd tea.Cmd
		p.spinner, cmd = p.spinner.Update(msg)
		return p, cmd
	case tea.KeyMsg:
		return p.handleKey(msg)
	}
	return p, nil
}

func (p *Panel) handleKey(msg tea.KeyMsg) (*Panel, tea.Cmd) {
	if p.screen == screenConfirm {
		switch {
		case key.Matches(msg, keys.Confirm):
			return p.executeConfirmedAction()
		case key.Matches(msg, keys.Cancel):
			p.pending = nil
			p.screen = screenList
		}
		return p, nil
	}

	if p.screen == screenDetail {
		switch {
		case key.Matches(msg, keys.Escape), key.Matches(msg, keys.Enter):
			p.screen = screenList
			return p, nil
		}
		var cmd tea.Cmd
		p.viewport, cmd = p.viewport.Update(msg)
		return p, cmd
	}

	if p.search.Focused() {
		switch msg.Type {
		case tea.KeyEnter:
			q := strings.TrimSpace(p.search.Value())
			p.search.Blur()
			if q == "" {
				return p, nil
			}
			p.lastQuery = q
			p.loading = true
			p.statusMsg = ""
			return p, tea.Batch(p.searchCmd(q), p.spinner.Tick)
		case tea.KeyEsc:
			p.search.Blur()
			return p, nil
		}
		var cmd tea.Cmd
		p.search, cmd = p.search.Update(msg)
		return p, cmd
	}

	switch {
	case key.Matches(msg, keys.Search):
		p.mode = viewSearch
		p.search.Focus()
		return p, textinput.Blink
	case key.Matches(msg, keys.Tab):
		p.cycleMode()
		cmd := p.loadForModeCmd()
		if p.mode == viewSearch {
			p.search.Focus()
			cmd = tea.Batch(cmd, textinput.Blink)
		}
		return p, cmd
	case key.Matches(msg, keys.Enter):
		return p.openDetail()
	case key.Matches(msg, keys.Install):
		return p.startInstall()
	case key.Matches(msg, keys.Remove):
		return p.startRemove()
	case key.Matches(msg, keys.Upgrade):
		return p.startUpgrade()
	case key.Matches(msg, keys.UpgradeAll):
		return p.startUpgradeAll()
	case key.Matches(msg, keys.Sync):
		return p.startSync()
	}

	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return p, cmd
}

func (p *Panel) cycleMode() {
	switch p.mode {
	case viewInstalled:
		p.mode = viewUpgradable
	case viewUpgradable:
		p.mode = viewSearch
	case viewSearch:
		p.mode = viewInstalled
	}
	p.statusMsg = ""
	p.setSize(p.width, p.height)
}

func (p *Panel) loadForModeCmd() tea.Cmd {
	switch p.mode {
	case viewInstalled:
		p.loading = true
		return tea.Batch(p.loadInstalledCmd(), p.spinner.Tick)
	case viewUpgradable:
		p.loading = true
		return tea.Batch(p.loadUpgradableCmd(), p.spinner.Tick)
	case viewSearch:
		p.list.SetItems(nil)
	}
	return nil
}

func (p *Panel) refreshCmd() tea.Cmd {
	switch p.mode {
	case viewUpgradable:
		return p.loadUpgradableCmd()
	case viewSearch:
		if p.lastQuery != "" {
			return p.searchCmd(p.lastQuery)
		}
		return p.loadInstalledCmd()
	default:
		return p.loadInstalledCmd()
	}
}

func (p *Panel) selected() (pkg.Package, bool) {
	it, ok := p.list.SelectedItem().(item)
	if !ok {
		return pkg.Package{}, false
	}
	return it.p, true
}

func (p *Panel) openDetail() (*Panel, tea.Cmd) {
	sel, ok := p.selected()
	if !ok {
		return p, nil
	}
	p.loading = true
	return p, tea.Batch(p.infoCmd(sel.Name), p.spinner.Tick)
}

func (p *Panel) startInstall() (*Panel, tea.Cmd) {
	sel, ok := p.selected()
	if !ok {
		return p, nil
	}
	if sel.Status != pkg.StatusAvailable {
		p.statusMsg = "Already installed: use 'u' to upgrade or 'd' to remove."
		return p, nil
	}
	argv := p.mgr.InstallCmd(sel.Name)
	p.pending = &pendingAction{label: fmt.Sprintf("Install %s?", sel.Name), argv: argv}
	p.screen = screenConfirm
	return p, nil
}

func (p *Panel) startRemove() (*Panel, tea.Cmd) {
	sel, ok := p.selected()
	if !ok {
		return p, nil
	}
	if sel.Status == pkg.StatusAvailable {
		p.statusMsg = "Not installed."
		return p, nil
	}
	argv := p.mgr.RemoveCmd(sel.Name)
	p.pending = &pendingAction{label: fmt.Sprintf("Remove %s?", sel.Name), argv: argv}
	p.screen = screenConfirm
	return p, nil
}

func (p *Panel) startUpgrade() (*Panel, tea.Cmd) {
	sel, ok := p.selected()
	if !ok {
		return p, nil
	}
	if sel.Status != pkg.StatusUpgradable {
		p.statusMsg = "No upgrade available for this package."
		return p, nil
	}
	argv := p.mgr.UpgradeCmd(sel.Name)
	p.pending = &pendingAction{label: fmt.Sprintf("Upgrade %s?", sel.Name), argv: argv}
	p.screen = screenConfirm
	return p, nil
}

func (p *Panel) startUpgradeAll() (*Panel, tea.Cmd) {
	argv := p.mgr.UpgradeCmd("")
	p.pending = &pendingAction{label: fmt.Sprintf("Upgrade ALL %s packages?", p.mgr.Name()), argv: argv}
	p.screen = screenConfirm
	return p, nil
}

func (p *Panel) startSync() (*Panel, tea.Cmd) {
	argv := p.mgr.UpdateCmd()
	if argv == nil {
		p.statusMsg = fmt.Sprintf("%s does not need an explicit sync.", p.mgr.Name())
		return p, nil
	}
	p.pending = &pendingAction{label: "Sync the package cache?", argv: argv}
	p.screen = screenConfirm
	return p, nil
}

func (p *Panel) executeConfirmedAction() (*Panel, tea.Cmd) {
	pending := p.pending
	p.pending = nil
	p.screen = screenList
	if pending == nil || len(pending.argv) == 0 {
		return p, nil
	}
	p.actionRunning = true
	mgr := p.mgr
	c := exec.Command(pending.argv[0], pending.argv[1:]...)
	return p, tea.ExecProcess(c, func(err error) tea.Msg {
		return actionDoneMsg{backend: mgr.Name(), err: err}
	})
}

func (p *Panel) renderHeader() string {
	count := len(p.list.Items())
	label := fmt.Sprintf(" %s — %s (%d) ", strings.ToUpper(p.mgr.Name()), p.mode.label(), count)
	return titleStyle.Render(label)
}

func (p *Panel) View() string {
	if !p.mgr.Available() {
		return dimStyle.Render(fmt.Sprintf("%s is not available on this system.", p.mgr.Name()))
	}

	if p.screen == screenDetail {
		return lipgloss.JoinVertical(lipgloss.Left,
			p.renderHeader(),
			detailBoxStyle.Width(maxInt(p.width-4, 10)).Render(p.viewport.View()),
			dimStyle.Render("esc/enter: back   ↑/↓: scroll"),
		)
	}

	var sections []string
	sections = append(sections, p.renderHeader())
	if p.mode == viewSearch {
		sections = append(sections, searchBoxStyle.Width(maxInt(p.width-4, 10)).Render(p.search.View()))
	}
	sections = append(sections, p.list.View())

	var status string
	switch {
	case p.loading:
		status = p.spinner.View() + " loading..."
	case p.err != nil:
		status = errorStyle.Render(p.err.Error())
	case p.statusMsg != "":
		status = dimStyle.Render(p.statusMsg)
	}
	if status != "" {
		sections = append(sections, status)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	if p.screen == screenConfirm && p.pending != nil {
		modal := modalStyle.Render(p.pending.label + "\n\n[y] confirm    [n] cancel")
		content = lipgloss.Place(p.width, p.height, lipgloss.Center, lipgloss.Center, modal)
	}

	return content
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

var _ backendMsg = installedResultMsg{}
