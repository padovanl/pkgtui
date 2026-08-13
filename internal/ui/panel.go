package ui

import (
	"fmt"
	"os/exec"
	"sort"
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
	viewOrphaned
	viewSearch
)

func (m viewMode) label() string {
	switch m {
	case viewUpgradable:
		return "Upgradable"
	case viewOrphaned:
		return "Orphaned"
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
	screenHelp
)

// pendingAction holds a destructive/privileged action awaiting user
// confirmation via the y/N modal. channels is non-nil only for a snap
// install, letting the user cycle risk levels (stable/candidate/beta/edge)
// before confirming.
type pendingAction struct {
	label        string
	argv         []string
	channels     []string
	channelIdx   int
	channelBuild func(channel string) []string
}

func (a *pendingAction) currentChannel() string {
	if len(a.channels) == 0 {
		return ""
	}
	return a.channels[a.channelIdx%len(a.channels)]
}

func (a *pendingAction) cycleChannel() {
	if len(a.channels) == 0 {
		return
	}
	a.channelIdx = (a.channelIdx + 1) % len(a.channels)
	a.argv = a.channelBuild(a.currentChannel())
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

type orphanedResultMsg struct {
	backend string
	pkgs    []pkg.Package
	err     error
}

func (m orphanedResultMsg) Backend() string { return m.backend }

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
	mgr           pkg.Manager
	orphanLister  pkg.OrphanLister
	batchManager  pkg.BatchManager
	chanInstaller pkg.ChannelInstaller

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

	sortBySize bool
	tagged     map[string]bool

	upgradableCount    int // -1 = not yet known
	orphanedCount      int // -1 = not yet known
	startupBannerShown bool

	width, height int
	listTopOffset int // rows above the list body, for mouse row mapping
}

func NewPanel(mgr pkg.Manager) *Panel {
	l := list.New(nil, itemDelegate{}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	// Reuse our own "f" binding so the list's built-in fuzzy filter starts
	// on the same key we advertise, instead of its default "/" (which we
	// use for the apt-cache/snap catalog search instead).
	l.KeyMap.Filter = keys.Filter
	// bubbles/list's default NextPage/PrevPage bindings include "f"/"d"/"h"/
	// "l"/"b"/"u", which otherwise silently steal our filter/remove/backend-
	// switch/upgrade shortcuts before the list ever considers them for
	// anything else. We don't page through the list (it just scrolls), so
	// pare these down to the arrow/pgup/pgdn keys we don't use elsewhere.
	l.KeyMap.NextPage = key.NewBinding(key.WithKeys("pgdown"))
	l.KeyMap.PrevPage = key.NewBinding(key.WithKeys("pgup"))

	ti := textinput.New()
	ti.Placeholder = "search packages... (enter to confirm)"
	ti.CharLimit = 100
	ti.Prompt = "🔍 "

	vp := viewport.New(0, 0)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	p := &Panel{
		mgr:             mgr,
		list:            l,
		search:          ti,
		viewport:        vp,
		spinner:         sp,
		mode:            viewInstalled,
		screen:          screenList,
		upgradableCount: -1,
		orphanedCount:   -1,
	}
	if ol, ok := mgr.(pkg.OrphanLister); ok {
		p.orphanLister = ol
	}
	if bm, ok := mgr.(pkg.BatchManager); ok {
		p.batchManager = bm
	}
	if ci, ok := mgr.(pkg.ChannelInstaller); ok {
		p.chanInstaller = ci
	}
	return p
}

func (p *Panel) Backend() string { return p.mgr.Name() }

// IsTyping reports whether the search box or the list's local filter input
// currently owns keyboard input, so the root App knows not to steal
// single-letter shortcuts (including "q") meant to be typed as text.
func (p *Panel) IsTyping() bool {
	return p.search.Focused() || p.list.FilterState() == list.Filtering
}

func (p *Panel) Init() tea.Cmd {
	if !p.mgr.Available() {
		return nil
	}
	p.loading = true
	// The upgradable (and, if supported, orphaned) counts are fetched in the
	// background purely for the startup summary banner; they don't touch
	// p.loading unless that happens to already be the active view.
	cmds := []tea.Cmd{p.loadInstalledCmd(), p.loadUpgradableCmd(), p.spinner.Tick}
	if p.orphanLister != nil {
		cmds = append(cmds, p.loadOrphanedCmd())
	}
	return tea.Batch(cmds...)
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

func (p *Panel) loadOrphanedCmd() tea.Cmd {
	mgr := p.mgr
	ol := p.orphanLister
	return func() tea.Msg {
		pkgs, err := ol.ListOrphaned()
		return orphanedResultMsg{backend: mgr.Name(), pkgs: pkgs, err: err}
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

// sortPackages reorders pkgs in place by installed size (descending) when
// that sort is active; otherwise it leaves the backend's own ordering
// alone, since apt-cache/snap find already rank search results by
// relevance and re-sorting them alphabetically would just make search
// worse.
func (p *Panel) sortPackages(pkgs []pkg.Package) {
	if !p.sortBySize {
		return
	}
	sort.SliceStable(pkgs, func(i, j int) bool { return pkgs[i].Size > pkgs[j].Size })
}

func (p *Panel) setItems(pkgs []pkg.Package) {
	p.list.SetItems(itemsFrom(pkgs))
}

func (p *Panel) sortAndSetItems(pkgs []pkg.Package) {
	p.sortPackages(pkgs)
	p.setItems(pkgs)
}

func (p *Panel) refreshDelegate() {
	p.list.SetDelegate(itemDelegate{showSize: p.sortBySize, tagged: p.tagged})
}

func (p *Panel) setSize(w, h int) {
	p.width, p.height = w, h
	headerH := 1
	legendH := 1
	searchH := 0
	if p.mode == viewSearch {
		searchH = 3
	}
	statusH := 1
	p.listTopOffset = 1 + headerH + legendH + searchH // +1 for the app's own tab bar
	listH := max(h-headerH-legendH-searchH-statusH, 3)
	if w > 4 {
		p.list.SetSize(w-2, listH)
	} else {
		p.list.SetSize(w, listH)
	}
	p.viewport.Width = w - 4
	p.viewport.Height = h - 4
	p.search.Width = w - 8
}

// maybeShowStartupSummary sets the initial status line to a one-time
// "N upgradable · M orphaned" summary once both background counts (or just
// the upgradable one, for backends without ListOrphaned) have arrived, and
// nothing else has already claimed the status line.
func (p *Panel) maybeShowStartupSummary() {
	if p.startupBannerShown || p.upgradableCount < 0 {
		return
	}
	if p.orphanLister != nil && p.orphanedCount < 0 {
		return
	}
	p.startupBannerShown = true
	if p.statusMsg != "" {
		return
	}
	var parts []string
	if p.upgradableCount > 0 {
		parts = append(parts, fmt.Sprintf("%d upgradable", p.upgradableCount))
	}
	if p.orphanLister != nil && p.orphanedCount > 0 {
		parts = append(parts, fmt.Sprintf("%d orphaned", p.orphanedCount))
	}
	if len(parts) > 0 {
		p.statusMsg = strings.Join(parts, " · ")
	}
}

func (p *Panel) Update(msg tea.Msg) (*Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case installedResultMsg:
		if p.mode == viewInstalled {
			p.loading = false
			p.err = msg.err
			if msg.err == nil {
				p.sortAndSetItems(msg.pkgs)
			}
		}
		return p, nil
	case upgradableResultMsg:
		if msg.err == nil {
			p.upgradableCount = len(msg.pkgs)
		}
		if p.mode == viewUpgradable {
			p.loading = false
			p.err = msg.err
			if msg.err == nil {
				p.sortAndSetItems(msg.pkgs)
			}
		}
		p.maybeShowStartupSummary()
		return p, nil
	case orphanedResultMsg:
		if msg.err == nil {
			p.orphanedCount = len(msg.pkgs)
		}
		if p.mode == viewOrphaned {
			p.loading = false
			p.err = msg.err
			if msg.err == nil {
				p.sortAndSetItems(msg.pkgs)
			}
		}
		p.maybeShowStartupSummary()
		return p, nil
	case searchResultMsg:
		p.loading = false
		p.err = msg.err
		if msg.err == nil && p.mode == viewSearch {
			p.sortAndSetItems(msg.pkgs)
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
		p.tagged = nil
		p.refreshDelegate()
		if msg.err != nil {
			p.statusMsg = errorStyle.Render("Error: " + msg.err.Error())
		} else {
			p.statusMsg = "Done."
		}
		p.loading = true
		return p, tea.Batch(p.refreshCmd(), p.spinner.Tick)
	case spinner.TickMsg:
		if !p.loading {
			// Without this, the spinner reschedules itself forever: once
			// anything triggers a single load, it ticks for the rest of
			// the program's life even while idle.
			return p, nil
		}
		var cmd tea.Cmd
		p.spinner, cmd = p.spinner.Update(msg)
		return p, cmd
	case tea.MouseMsg:
		return p.handleMouse(msg)
	case tea.KeyMsg:
		return p.handleKey(msg)
	}

	// Anything we don't recognize ourselves (e.g. list.FilterMatchesMsg,
	// which bubbles/list sends itself asynchronously while computing the
	// fuzzy filter) still needs to reach the list, or its internal state
	// never finishes updating.
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return p, cmd
}

func (p *Panel) handleMouse(msg tea.MouseMsg) (*Panel, tea.Cmd) {
	if p.screen == screenDetail {
		var cmd tea.Cmd
		p.viewport, cmd = p.viewport.Update(msg)
		return p, cmd
	}
	if p.screen != screenList || msg.Action != tea.MouseActionPress {
		return p, nil
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		for range 3 {
			p.list.CursorUp()
		}
	case tea.MouseButtonWheelDown:
		for range 3 {
			p.list.CursorDown()
		}
	case tea.MouseButtonLeft:
		if row := msg.Y - p.listTopOffset; row >= 0 {
			p.list.Select(p.list.Paginator.Page*p.list.Paginator.PerPage + row)
		}
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
		case key.Matches(msg, keys.Channel):
			if p.pending != nil {
				p.pending.cycleChannel()
			}
		}
		return p, nil
	}

	if p.screen == screenHelp {
		switch {
		case key.Matches(msg, keys.Escape), key.Matches(msg, keys.Enter), key.Matches(msg, keys.Help):
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
		case tea.KeyTab:
			// Tab is the view-switch shortcut everywhere else; honor it here
			// too instead of letting the focused text input swallow it
			// (which used to make Tab appear "stuck" once you searched).
			return p.cycleView()
		}
		var cmd tea.Cmd
		p.search, cmd = p.search.Update(msg)
		return p, cmd
	}

	// While the list's own fuzzy filter is mid-input, let it own the
	// keyboard (including Enter to apply and Esc to cancel).
	if p.list.FilterState() == list.Filtering {
		var cmd tea.Cmd
		p.list, cmd = p.list.Update(msg)
		return p, cmd
	}

	switch {
	case key.Matches(msg, keys.Help):
		p.screen = screenHelp
		return p, nil
	case key.Matches(msg, keys.Search):
		p.mode = viewSearch
		p.search.Focus()
		return p, textinput.Blink
	case key.Matches(msg, keys.Filter):
		var cmd tea.Cmd
		p.list, cmd = p.list.Update(msg)
		return p, cmd
	case key.Matches(msg, keys.Tab):
		return p.cycleView()
	case key.Matches(msg, keys.Enter):
		return p.openDetail()
	case key.Matches(msg, keys.ToggleTag):
		return p.toggleTag()
	case key.Matches(msg, keys.SortSize):
		return p.toggleSortSize()
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

// availableModes lists the views this panel actually supports, in cycle
// order; Orphaned only appears for backends implementing OrphanLister.
func (p *Panel) availableModes() []viewMode {
	modes := []viewMode{viewInstalled, viewUpgradable}
	if p.orphanLister != nil {
		modes = append(modes, viewOrphaned)
	}
	return append(modes, viewSearch)
}

func (p *Panel) cycleMode() {
	modes := p.availableModes()
	idx := 0
	for i, m := range modes {
		if m == p.mode {
			idx = i
			break
		}
	}
	p.mode = modes[(idx+1)%len(modes)]
	p.statusMsg = ""
	p.setSize(p.width, p.height)
}

// cycleView advances to the next view, blurring the search box first so it
// doesn't swallow the Tab keypress that got us here, then re-focusing it if
// we landed on Search.
func (p *Panel) cycleView() (*Panel, tea.Cmd) {
	p.search.Blur()
	p.cycleMode()
	cmd := p.loadForModeCmd()
	if p.mode == viewSearch {
		p.search.Focus()
		cmd = tea.Batch(cmd, textinput.Blink)
	}
	return p, cmd
}

// toggleSortSize flips the size sort and re-fetches the current view's data
// rather than just re-sorting what's already on screen: turning the sort
// off needs to restore the backend's own ordering (alphabetical for
// installed/orphaned, relevance for search), which isn't recoverable from
// an already-resorted item list.
func (p *Panel) toggleSortSize() (*Panel, tea.Cmd) {
	p.sortBySize = !p.sortBySize
	p.refreshDelegate()
	p.loading = true
	return p, tea.Batch(p.refreshCmd(), p.spinner.Tick)
}

func (p *Panel) toggleTag() (*Panel, tea.Cmd) {
	sel, ok := p.selected()
	if !ok {
		return p, nil
	}
	if p.tagged == nil {
		p.tagged = map[string]bool{}
	}
	if p.tagged[sel.Name] {
		delete(p.tagged, sel.Name)
	} else {
		p.tagged[sel.Name] = true
	}
	p.refreshDelegate()
	p.list.CursorDown()
	return p, nil
}

func (p *Panel) taggedNames() []string {
	names := make([]string, 0, len(p.tagged))
	for n := range p.tagged {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func (p *Panel) loadForModeCmd() tea.Cmd {
	switch p.mode {
	case viewInstalled:
		p.loading = true
		return tea.Batch(p.loadInstalledCmd(), p.spinner.Tick)
	case viewUpgradable:
		p.loading = true
		return tea.Batch(p.loadUpgradableCmd(), p.spinner.Tick)
	case viewOrphaned:
		p.loading = true
		return tea.Batch(p.loadOrphanedCmd(), p.spinner.Tick)
	case viewSearch:
		p.list.SetItems(nil)
	}
	return nil
}

func (p *Panel) refreshCmd() tea.Cmd {
	switch p.mode {
	case viewUpgradable:
		return p.loadUpgradableCmd()
	case viewOrphaned:
		return p.loadOrphanedCmd()
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
	if len(p.tagged) > 0 {
		return p.startBatchInstall()
	}
	sel, ok := p.selected()
	if !ok {
		return p, nil
	}
	if sel.Status != pkg.StatusAvailable {
		p.statusMsg = "Already installed: use 'u' to upgrade or 'd' to remove."
		return p, nil
	}
	if p.chanInstaller != nil {
		channels := p.chanInstaller.Channels()
		name := sel.Name
		ci := p.chanInstaller
		p.pending = &pendingAction{
			label:        fmt.Sprintf("Install %s?", name),
			argv:         ci.InstallChannelCmd(name, channels[0]),
			channels:     channels,
			channelBuild: func(channel string) []string { return ci.InstallChannelCmd(name, channel) },
		}
		p.screen = screenConfirm
		return p, nil
	}
	argv := p.mgr.InstallCmd(sel.Name)
	p.pending = &pendingAction{label: fmt.Sprintf("Install %s?", sel.Name), argv: argv}
	p.screen = screenConfirm
	return p, nil
}

func (p *Panel) startRemove() (*Panel, tea.Cmd) {
	if len(p.tagged) > 0 {
		return p.startBatchRemove()
	}
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

func (p *Panel) startBatchInstall() (*Panel, tea.Cmd) {
	if p.batchManager == nil {
		p.statusMsg = "Batch install isn't supported for this backend."
		return p, nil
	}
	names := p.taggedNames()
	argv := p.batchManager.InstallManyCmd(names)
	p.pending = &pendingAction{label: fmt.Sprintf("Install %d selected packages?\n%s", len(names), strings.Join(names, ", ")), argv: argv}
	p.screen = screenConfirm
	return p, nil
}

func (p *Panel) startBatchRemove() (*Panel, tea.Cmd) {
	if p.batchManager == nil {
		p.statusMsg = "Batch remove isn't supported for this backend."
		return p, nil
	}
	names := p.taggedNames()
	argv := p.batchManager.RemoveManyCmd(names)
	p.pending = &pendingAction{label: fmt.Sprintf("Remove %d selected packages?\n%s", len(names), strings.Join(names, ", ")), argv: argv}
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

// SupportsSync reports whether the "s" sync-cache action does anything for
// this backend, so the root App can hide the hint when it wouldn't (snap).
func (p *Panel) SupportsSync() bool { return p.mgr.UpdateCmd() != nil }

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
	extra := ""
	if p.sortBySize {
		extra += " · by size"
	}
	if n := len(p.tagged); n > 0 {
		extra += fmt.Sprintf(" · %d tagged", n)
	}
	label := fmt.Sprintf(" %s — %s (%d)%s ", strings.ToUpper(p.mgr.Name()), p.mode.label(), count, extra)
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

	if p.screen == screenHelp {
		return p.renderHelp()
	}

	var sections []string
	sections = append(sections, p.renderHeader())
	sections = append(sections, legendLine())
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
		body := p.pending.label
		if len(p.pending.channels) > 0 {
			body += fmt.Sprintf("\n\nChannel: %s  [c] change", p.pending.currentChannel())
		}
		modal := modalStyle.Render(body + "\n\n[y] confirm    [n] cancel")
		content = lipgloss.Place(p.width, p.height, lipgloss.Center, lipgloss.Center, modal)
	}

	return content
}

func (p *Panel) renderHelp() string {
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(colorHighlight).Width(14)
	row := func(k, desc string) string {
		return keyStyle.Render(k) + dimStyle.Render(desc)
	}

	rows := []string{
		titleStyle.Render("pkgtui — help"),
		"",
		helpSectionStyle.Render("Navigation"),
		row("← / →", "switch backend (apt / snap)"),
		row("tab", "switch view (Installed / Upgradable"+p.orphanedTabLabel()+" / Search)"),
		row("↑/↓, j/k", "move selection"),
		row("/", "search the full apt/snap catalog, then enter to run it"),
		row("f", "filter the packages currently shown, as you type"),
		row("enter", "package details"),
		row("esc", "back / cancel filter"),
		"",
		helpSectionStyle.Render("Actions"),
		row("space", "tag/untag the selected package for a batch action"),
		row("i", "install selected/tagged package(s)"),
		row("d", "remove selected/tagged package(s)"),
		row("u", "upgrade selected package"),
		row("U", "upgrade ALL packages on this backend"),
		row("S", "sort the current view by installed size"),
		row("s", "sync package cache (apt only)"),
		row("y / n", "confirm / cancel a pending action"),
	}
	if p.chanInstaller != nil {
		rows = append(rows, row("c", "cycle install channel (while confirming a snap install)"))
	}
	rows = append(rows,
		"",
		helpSectionStyle.Render("Status symbols"),
		"  "+legendLine(),
		"",
		row("q", "quit"),
		row("?", "close this help"),
	)

	body := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return lipgloss.Place(p.width, p.height, lipgloss.Center, lipgloss.Center, helpBoxStyle.Render(body))
}

func (p *Panel) orphanedTabLabel() string {
	if p.orphanLister == nil {
		return ""
	}
	return " / Orphaned"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

var _ backendMsg = installedResultMsg{}
