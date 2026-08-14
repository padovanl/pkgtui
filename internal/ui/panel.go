package ui

import (
	"fmt"
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
	screenChangelog
	screenPPA
	screenRunning
	screenDisk
	screenProvenance
	screenUnattended
	screenVersion
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

type changelogResultMsg struct {
	backend string
	text    string
	err     error
}

func (m changelogResultMsg) Backend() string { return m.backend }

type ppaListResultMsg struct {
	backend string
	ppas    []pkg.PPA
	err     error
}

func (m ppaListResultMsg) Backend() string { return m.backend }

type diskReportResultMsg struct {
	backend string
	items   []pkg.DiskItem
	err     error
}

func (m diskReportResultMsg) Backend() string { return m.backend }

type provenanceResultMsg struct {
	backend string
	name    string
	prov    pkg.Provenance
	err     error
}

func (m provenanceResultMsg) Backend() string { return m.backend }

type uaStatusResultMsg struct {
	backend string
	status  pkg.UnattendedUpgradesStatus
	err     error
}

func (m uaStatusResultMsg) Backend() string { return m.backend }

type versionsResultMsg struct {
	backend  string
	name     string
	versions []pkg.PackageVersion
	err      error
}

func (m versionsResultMsg) Backend() string { return m.backend }

// Panel is the self-contained UI + state for a single package manager
// backend (apt or snap).
type Panel struct {
	mgr                pkg.Manager
	orphanLister       pkg.OrphanLister
	batchManager       pkg.BatchManager
	chanInstaller      pkg.ChannelInstaller
	holder             pkg.Holder
	changelogger       pkg.Changelogger
	ppaManager         pkg.PPAManager
	diskAnalyzer       pkg.DiskAnalyzer
	provenanceProvider pkg.ProvenanceProvider
	uaReporter         pkg.UnattendedUpgradesReporter
	versionLister      pkg.VersionLister
	reverter           pkg.Reverter

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

	awaitingUpgradeAllConfirm bool
	returnScreen              screen // where finishing an action sends us back to
	running                   *runningProcess

	upgradableCount    int // -1 = not yet known
	orphanedCount      int // -1 = not yet known
	startupBannerShown bool

	// PPA screen (apt only).
	ppas      []pkg.PPA
	ppaCursor int
	ppaAdding bool
	ppaInput  textinput.Model

	// Disk cleanup screen.
	diskItems  []pkg.DiskItem
	diskCursor int

	// Provenance ("why is this installed") screen. provenanceStack holds
	// the breadcrumb of package names drilled through so far, so esc can
	// step back one level at a time instead of leaving straight to the list.
	provenanceName   string
	provenance       pkg.Provenance
	provenanceCursor int
	provenanceStack  []string

	// Unattended-upgrades dashboard (apt only).
	uaStatus pkg.UnattendedUpgradesStatus

	// Version picker screen (apt only; snap's "V" goes straight to a
	// revert confirmation instead, see openVersionAction).
	versionPkgName string
	versions       []pkg.PackageVersion
	versionCursor  int

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

	ppaInput := textinput.New()
	ppaInput.Placeholder = "ppa:user/name"
	ppaInput.CharLimit = 100
	ppaInput.Prompt = "➕ "

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
		ppaInput:        ppaInput,
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
	if h, ok := mgr.(pkg.Holder); ok {
		p.holder = h
	}
	if cl, ok := mgr.(pkg.Changelogger); ok {
		p.changelogger = cl
	}
	if pm, ok := mgr.(pkg.PPAManager); ok {
		p.ppaManager = pm
	}
	if da, ok := mgr.(pkg.DiskAnalyzer); ok {
		p.diskAnalyzer = da
	}
	if pp, ok := mgr.(pkg.ProvenanceProvider); ok {
		p.provenanceProvider = pp
	}
	if ua, ok := mgr.(pkg.UnattendedUpgradesReporter); ok {
		p.uaReporter = ua
	}
	if vl, ok := mgr.(pkg.VersionLister); ok {
		p.versionLister = vl
	}
	if rv, ok := mgr.(pkg.Reverter); ok {
		p.reverter = rv
	}
	return p
}

func (p *Panel) Backend() string { return p.mgr.Name() }

// IsTyping reports whether the search box or the list's local filter input
// currently owns keyboard input, so the root App knows not to steal
// single-letter shortcuts (including "q") meant to be typed as text.
func (p *Panel) IsTyping() bool {
	return p.search.Focused() || p.list.FilterState() == list.Filtering || p.screen == screenRunning
}

// ModeName and SetInitialMode round-trip the current view through config's
// last-view persistence. Search is deliberately never restored: starting
// on an empty query every launch would be more confusing than useful.
func (p *Panel) ModeName() string {
	switch p.mode {
	case viewUpgradable:
		return "upgradable"
	case viewOrphaned:
		return "orphaned"
	default:
		return "installed"
	}
}

func (p *Panel) SetInitialMode(name string) {
	switch name {
	case "upgradable":
		p.mode = viewUpgradable
	case "orphaned":
		if p.orphanLister != nil {
			p.mode = viewOrphaned
		}
	}
}

// ApplySettingsChange re-syncs the parts of list.Model that cached a key
// binding by value at construction time (KeyMap.Filter), so a runtime
// keybinding rebind in the settings screen actually takes effect.
func (p *Panel) ApplySettingsChange() {
	p.list.KeyMap.Filter = keys.Filter
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

func (p *Panel) changelogCmd(name string) tea.Cmd {
	mgr := p.mgr
	cl := p.changelogger
	return func() tea.Msg {
		text, err := cl.Changelog(name)
		return changelogResultMsg{backend: mgr.Name(), text: text, err: err}
	}
}

func (p *Panel) loadPPAsCmd() tea.Cmd {
	mgr := p.mgr
	pm := p.ppaManager
	return func() tea.Msg {
		ppas, err := pm.ListPPAs()
		return ppaListResultMsg{backend: mgr.Name(), ppas: ppas, err: err}
	}
}

func (p *Panel) loadDiskReportCmd() tea.Cmd {
	mgr := p.mgr
	da := p.diskAnalyzer
	return func() tea.Msg {
		items, err := da.DiskReport()
		return diskReportResultMsg{backend: mgr.Name(), items: items, err: err}
	}
}

func (p *Panel) loadProvenanceCmd(name string) tea.Cmd {
	mgr := p.mgr
	pp := p.provenanceProvider
	return func() tea.Msg {
		prov, err := pp.Provenance(name)
		return provenanceResultMsg{backend: mgr.Name(), name: name, prov: prov, err: err}
	}
}

func (p *Panel) loadUAStatusCmd() tea.Cmd {
	mgr := p.mgr
	ua := p.uaReporter
	return func() tea.Msg {
		status, err := ua.UnattendedUpgradesStatus()
		return uaStatusResultMsg{backend: mgr.Name(), status: status, err: err}
	}
}

func (p *Panel) loadVersionsCmd(name string) tea.Cmd {
	mgr := p.mgr
	vl := p.versionLister
	return func() tea.Msg {
		versions, err := vl.AvailableVersions(name)
		return versionsResultMsg{backend: mgr.Name(), name: name, versions: versions, err: err}
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
	// Every screen that wraps this viewport (detail, changelog, help) joins
	// it vertically with its own header line and a hint line on top of the
	// box's own border+padding (2+2 = 4 rows of chrome) — bubbles/viewport
	// always renders exactly Height lines, padding short content out to
	// fill it, so those two extra lines are never absorbed by "the content
	// was shorter than the box" the way they might look like they'd be.
	// Sizing this to h-4 (just the box chrome) reliably overflowed the
	// terminal by 2 rows on every use, caught live via the help screen
	// losing its own title off the top of a 100x34 terminal once its
	// content grew past a single screenful.
	p.viewport.Height = maxInt(h-6, 3)
	p.search.Width = w - 8
	if p.running != nil {
		p.running.resize(p.viewport.Width, p.viewport.Height)
	}
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
		if p.awaitingUpgradeAllConfirm {
			p.awaitingUpgradeAllConfirm = false
			p.loading = false
			if msg.err != nil {
				p.statusMsg = errorStyle.Render("Error: " + msg.err.Error())
			} else {
				p.openUpgradeAllConfirm(msg.pkgs)
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
	case changelogResultMsg:
		p.loading = false
		if msg.err != nil {
			p.err = msg.err
			return p, nil
		}
		p.err = nil
		p.viewport.SetContent(msg.text)
		p.viewport.GotoTop()
		p.screen = screenChangelog
		return p, nil
	case ppaListResultMsg:
		p.loading = false
		p.err = msg.err
		if msg.err == nil {
			p.ppas = msg.ppas
			if p.ppaCursor >= len(p.ppas) {
				p.ppaCursor = maxInt(len(p.ppas)-1, 0)
			}
		}
		return p, nil
	case diskReportResultMsg:
		p.loading = false
		p.err = msg.err
		if msg.err == nil {
			p.diskItems = msg.items
			if p.diskCursor >= len(p.diskItems) {
				p.diskCursor = maxInt(len(p.diskItems)-1, 0)
			}
		}
		return p, nil
	case provenanceResultMsg:
		p.loading = false
		if msg.err != nil {
			p.err = msg.err
			return p, nil
		}
		p.err = nil
		// A drill-down click can move p.provenanceName on before its own
		// request lands; discard a response that's no longer for the
		// package currently on screen instead of overwriting it with stale
		// data.
		if msg.name == p.provenanceName {
			p.provenance = msg.prov
		}
		return p, nil
	case uaStatusResultMsg:
		p.loading = false
		p.err = msg.err
		if msg.err == nil {
			p.uaStatus = msg.status
		}
		return p, nil
	case versionsResultMsg:
		p.loading = false
		p.err = msg.err
		if msg.err == nil && msg.name == p.versionPkgName {
			p.versions = msg.versions
		}
		return p, nil
	case ptyStartedMsg:
		if msg.err != nil {
			p.actionRunning = false
			p.screen = p.returnScreen
			p.statusMsg = errorStyle.Render("Error: " + msg.err.Error())
			return p, nil
		}
		p.running = msg.proc
		p.running.resize(p.viewport.Width, p.viewport.Height)
		p.viewport.SetContent("")
		return p, tea.Batch(readPTYCmd(p.running), waitPTYCmd(p.running))

	case ptyOutputMsg:
		if p.running == nil || msg.proc != p.running {
			return p, nil // stale read from a process we've already moved past
		}
		if len(msg.data) > 0 {
			p.running.buf.Write(msg.data)
			p.viewport.SetContent(p.running.buf.String())
			p.viewport.GotoBottom()
		}
		if msg.err != nil {
			// Read loop stops here (EOF once the child closes its pty side);
			// ptyExitMsg carries the actual success/failure and does the
			// screen transition once cmd.Wait() also returns.
			return p, nil
		}
		return p, readPTYCmd(p.running)

	case ptyExitMsg:
		if p.running == nil || msg.proc != p.running {
			return p, nil
		}
		// Leave the output on screen instead of immediately clearing it:
		// with tea.ExecProcess this used to hand the terminal straight
		// back, so any final lines (or an error) flashed by unread. Now
		// we just mark it finished; handleKey's screenRunning branch
		// dismisses it on the next keypress instead of forwarding one.
		p.running.exited = true
		p.running.exitErr = msg.err
		if msg.err != nil {
			p.running.buf.Write([]byte("\n--- Failed: " + msg.err.Error() + " ---\n"))
		} else {
			p.running.buf.Write([]byte("\n--- Done — press any key to continue ---\n"))
		}
		p.viewport.SetContent(p.running.buf.String())
		p.viewport.GotoBottom()
		return p, nil
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
	if p.screen == screenDetail || p.screen == screenChangelog {
		var cmd tea.Cmd
		p.viewport, cmd = p.viewport.Update(msg)
		return p, cmd
	}
	if p.screen == screenRunning {
		if p.running != nil && p.running.exited && msg.Action == tea.MouseActionPress {
			return p.dismissRunning()
		}
		return p, nil
	}
	if p.screen == screenPPA {
		if msg.Action != tea.MouseActionPress {
			return p, nil
		}
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if p.ppaCursor > 0 {
				p.ppaCursor--
			}
		case tea.MouseButtonWheelDown:
			if p.ppaCursor < len(p.ppas)-1 {
				p.ppaCursor++
			}
		}
		return p, nil
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
		// row >= p.list.Height() means the click landed below the list's
		// own rendered area entirely — the empty space under a short
		// list, or the footer hint bar further down still. Without this
		// check, that Y coordinate still maps to *some* row index
		// arithmetically, and on a long-enough list that index is a
		// genuinely valid item — so a click nowhere near the list would
		// silently jump the selection to an unrelated row somewhere in
		// it, which looked like clicking the footer "changed pages" for
		// no apparent reason.
		if row := msg.Y - p.listTopOffset; row >= 0 && row < p.list.Height() {
			// A click below the last real row (or anywhere on an empty
			// list) computes an index past the end of the items slice;
			// Select doesn't validate that itself, and the out-of-range
			// cursor later panics inside the list's own render (a slice
			// bounds crash in bubbles/list.Model.populatedView).
			if idx := p.list.Paginator.Page*p.list.Paginator.PerPage + row; idx < len(p.list.Items()) {
				p.list.Select(idx)
			}
		}
	}
	return p, nil
}

func (p *Panel) handleKey(msg tea.KeyMsg) (*Panel, tea.Cmd) {
	if p.screen == screenRunning {
		if p.running != nil && p.running.exited {
			// The command finished; wait for an explicit keypress instead
			// of auto-returning, so the final output (or an error) isn't
			// gone before it's been read.
			return p.dismissRunning()
		}
		// Full passthrough: sudo's password prompt, a debconf dialog, or
		// just apt asking "continue? [Y/n]" all need real keystrokes, not
		// our own shortcuts. Nothing here is treated as a pkgtui shortcut.
		if p.running != nil {
			if b := keyMsgToBytes(msg); b != nil {
				_, _ = p.running.ptmx.Write(b)
			}
		}
		return p, nil
	}

	if p.screen == screenConfirm {
		switch {
		case key.Matches(msg, keys.Confirm):
			return p.executeConfirmedAction()
		case key.Matches(msg, keys.Cancel):
			p.pending = nil
			p.screen = p.returnScreen
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
			return p, nil
		}
		var cmd tea.Cmd
		p.viewport, cmd = p.viewport.Update(msg)
		return p, cmd
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

	if p.screen == screenChangelog {
		switch {
		case key.Matches(msg, keys.Escape), key.Matches(msg, keys.Enter):
			p.screen = screenList
			return p, nil
		}
		var cmd tea.Cmd
		p.viewport, cmd = p.viewport.Update(msg)
		return p, cmd
	}

	if p.screen == screenPPA {
		return p.handlePPAKey(msg)
	}

	if p.screen == screenDisk {
		return p.handleDiskKey(msg)
	}

	if p.screen == screenProvenance {
		return p.handleProvenanceKey(msg)
	}

	if p.screen == screenUnattended {
		switch {
		case key.Matches(msg, keys.Escape), key.Matches(msg, keys.Enter):
			p.screen = screenList
		}
		return p, nil
	}

	if p.screen == screenVersion {
		return p.handleVersionKey(msg)
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
		p.viewport.SetContent(p.helpContent())
		p.viewport.GotoTop()
		p.screen = screenHelp
		return p, nil
	case key.Matches(msg, keys.Search):
		p.mode = viewSearch
		p.search.Focus()
		// The search box adds 3 rows; without recomputing the list's
		// height budget here, it stays sized for the previous mode and
		// overflows the terminal by those 3 rows, scrolling the header
		// (tab bar, title, legend) off the top of the screen.
		p.setSize(p.width, p.height)
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
	case key.Matches(msg, keys.Hold):
		return p.startHold()
	case key.Matches(msg, keys.Changelog):
		return p.openChangelog()
	case key.Matches(msg, keys.PPA):
		return p.openPPAScreen()
	case key.Matches(msg, keys.Disk):
		return p.openDiskScreen()
	case key.Matches(msg, keys.Provenance):
		return p.openProvenance()
	case key.Matches(msg, keys.Unattended):
		return p.openUnattended()
	case key.Matches(msg, keys.Version):
		return p.openVersionAction()
	}

	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return p, cmd
}

// handlePPAKey drives the PPA management screen: browsing the current list
// (up/down, "a" to add, "d"/"r" to remove, esc to leave) or, while adding,
// typing the new PPA name.
func (p *Panel) handlePPAKey(msg tea.KeyMsg) (*Panel, tea.Cmd) {
	if p.ppaAdding {
		switch msg.Type {
		case tea.KeyEnter:
			return p.startAddPPA()
		case tea.KeyEsc:
			p.ppaAdding = false
			p.ppaInput.Blur()
			p.ppaInput.SetValue("")
			return p, nil
		}
		var cmd tea.Cmd
		p.ppaInput, cmd = p.ppaInput.Update(msg)
		return p, cmd
	}

	switch {
	case key.Matches(msg, keys.Escape):
		p.screen = screenList
	case key.Matches(msg, keys.Up):
		if p.ppaCursor > 0 {
			p.ppaCursor--
		}
	case key.Matches(msg, keys.Down):
		if p.ppaCursor < len(p.ppas)-1 {
			p.ppaCursor++
		}
	case msg.String() == "a":
		p.ppaAdding = true
		p.ppaInput.Focus()
		return p, textinput.Blink
	case key.Matches(msg, keys.Remove):
		return p.startRemovePPA()
	}
	return p, nil
}

// handleDiskKey drives the disk cleanup screen: browsing findings (up/down)
// and purging the one under the cursor.
func (p *Panel) handleDiskKey(msg tea.KeyMsg) (*Panel, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		p.screen = screenList
	case key.Matches(msg, keys.Up):
		if p.diskCursor > 0 {
			p.diskCursor--
		}
	case key.Matches(msg, keys.Down):
		if p.diskCursor < len(p.diskItems)-1 {
			p.diskCursor++
		}
	case key.Matches(msg, keys.Remove):
		return p.startDiskPurge()
	}
	return p, nil
}

// startDiskPurge asks for confirmation before purging the disk-cleanup
// finding under the cursor. Deliberately one at a time, not a tagged batch
// like the main list's install/remove: each finding's Argv is already a
// complete standalone command (apt-get purge, dpkg --purge, snap remove
// --revision=N...), and those can't just be concatenated into one call the
// way plain package names can.
func (p *Panel) startDiskPurge() (*Panel, tea.Cmd) {
	if p.diskCursor < 0 || p.diskCursor >= len(p.diskItems) {
		return p, nil
	}
	target := p.diskItems[p.diskCursor]
	if len(target.Argv) == 0 {
		return p, nil
	}
	label := fmt.Sprintf("Purge %s?\n%s", target.Name, target.Reason)
	if target.Size > 0 {
		label += fmt.Sprintf(" (%s)", humanizeBytes(target.Size))
	}
	p.pending = &pendingAction{label: label, argv: target.Argv}
	p.returnScreen = screenDisk
	p.screen = screenConfirm
	return p, nil
}

// openDiskScreen opens the disk cleanup explorer (old kernels, leftover
// configs, disabled snap revisions — see pkg.DiskAnalyzer).
func (p *Panel) openDiskScreen() (*Panel, tea.Cmd) {
	if p.diskAnalyzer == nil {
		p.statusMsg = fmt.Sprintf("Disk cleanup isn't available for %s.", p.mgr.Name())
		return p, nil
	}
	p.screen = screenDisk
	p.diskCursor = 0
	p.loading = true
	p.statusMsg = ""
	return p, tea.Batch(p.loadDiskReportCmd(), p.spinner.Tick)
}

// handleProvenanceKey drives the "why is this installed" screen: browsing
// the selected package's reverse dependencies (up/down), drilling into one
// of them (enter), and stepping back out one level at a time (esc), rather
// than esc always leaving straight back to the package list.
func (p *Panel) handleProvenanceKey(msg tea.KeyMsg) (*Panel, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		if n := len(p.provenanceStack); n > 0 {
			prev := p.provenanceStack[n-1]
			p.provenanceStack = p.provenanceStack[:n-1]
			p.provenanceName = prev
			p.provenanceCursor = 0
			p.loading = true
			return p, tea.Batch(p.loadProvenanceCmd(prev), p.spinner.Tick)
		}
		p.screen = screenList
	case key.Matches(msg, keys.Up):
		if p.provenanceCursor > 0 {
			p.provenanceCursor--
		}
	case key.Matches(msg, keys.Down):
		if p.provenanceCursor < len(p.provenance.ReverseDeps)-1 {
			p.provenanceCursor++
		}
	case key.Matches(msg, keys.Enter):
		if p.provenanceCursor < 0 || p.provenanceCursor >= len(p.provenance.ReverseDeps) {
			return p, nil
		}
		next := p.provenance.ReverseDeps[p.provenanceCursor]
		p.provenanceStack = append(p.provenanceStack, p.provenanceName)
		p.provenanceName = next
		p.provenanceCursor = 0
		p.loading = true
		return p, tea.Batch(p.loadProvenanceCmd(next), p.spinner.Tick)
	}
	return p, nil
}

// openProvenance opens the "why is this installed" screen for the selected
// package.
func (p *Panel) openProvenance() (*Panel, tea.Cmd) {
	if p.provenanceProvider == nil {
		p.statusMsg = fmt.Sprintf("Provenance isn't available for %s.", p.mgr.Name())
		return p, nil
	}
	sel, ok := p.selected()
	if !ok {
		return p, nil
	}
	p.provenanceStack = nil
	p.provenanceName = sel.Name
	p.provenance = pkg.Provenance{}
	p.provenanceCursor = 0
	p.screen = screenProvenance
	p.loading = true
	return p, tea.Batch(p.loadProvenanceCmd(sel.Name), p.spinner.Tick)
}

// openUnattended opens the unattended-upgrades status dashboard.
func (p *Panel) openUnattended() (*Panel, tea.Cmd) {
	if p.uaReporter == nil {
		p.statusMsg = fmt.Sprintf("Unattended-upgrades status isn't available for %s.", p.mgr.Name())
		return p, nil
	}
	p.screen = screenUnattended
	p.loading = true
	p.statusMsg = ""
	return p, tea.Batch(p.loadUAStatusCmd(), p.spinner.Tick)
}

// handleVersionKey drives the version picker screen: browsing available
// versions (up/down) and confirming an install/downgrade to the one under
// the cursor (enter).
func (p *Panel) handleVersionKey(msg tea.KeyMsg) (*Panel, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		p.screen = screenList
	case key.Matches(msg, keys.Up):
		if p.versionCursor > 0 {
			p.versionCursor--
		}
	case key.Matches(msg, keys.Down):
		if p.versionCursor < len(p.versions)-1 {
			p.versionCursor++
		}
	case key.Matches(msg, keys.Enter):
		if p.versionCursor < 0 || p.versionCursor >= len(p.versions) {
			return p, nil
		}
		v := p.versions[p.versionCursor]
		label := fmt.Sprintf("Install %s version %s?", p.versionPkgName, v.Version)
		if v.Origin != "" {
			label += "\n" + v.Origin
		}
		p.pending = &pendingAction{label: label, argv: p.versionLister.InstallVersionCmd(p.versionPkgName, v.Version)}
		p.returnScreen = screenList
		p.screen = screenConfirm
	}
	return p, nil
}

// openVersionAction is "V": for apt, opens a picker over every version
// apt-cache madison knows about (installing a specific version, or
// downgrading, without hand-typing pkg=version); for snap, which has no
// comparable "exact version" concept, it goes straight to a revert
// confirmation instead — the two backends' actual capabilities here are
// different enough that presenting them identically would be misleading
// rather than convenient.
func (p *Panel) openVersionAction() (*Panel, tea.Cmd) {
	sel, ok := p.selected()
	if !ok {
		return p, nil
	}
	if p.versionLister != nil {
		p.versionPkgName = sel.Name
		p.versionCursor = 0
		p.versions = nil
		p.screen = screenVersion
		p.loading = true
		p.statusMsg = ""
		return p, tea.Batch(p.loadVersionsCmd(sel.Name), p.spinner.Tick)
	}
	if p.reverter != nil {
		if sel.Status == pkg.StatusAvailable {
			p.statusMsg = "Not installed."
			return p, nil
		}
		p.pending = &pendingAction{
			label: fmt.Sprintf("Revert %s to its previous revision?\nOnly works if a previous revision is still kept.", sel.Name),
			argv:  p.reverter.RevertCmd(sel.Name),
		}
		p.returnScreen = p.screen
		p.screen = screenConfirm
		return p, nil
	}
	p.statusMsg = fmt.Sprintf("Version selection isn't available for %s.", p.mgr.Name())
	return p, nil
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

func (p *Panel) openChangelog() (*Panel, tea.Cmd) {
	if p.changelogger == nil {
		p.statusMsg = fmt.Sprintf("Changelogs aren't available for %s.", p.mgr.Name())
		return p, nil
	}
	sel, ok := p.selected()
	if !ok {
		return p, nil
	}
	p.loading = true
	p.statusMsg = "fetching changelog (network)..."
	return p, tea.Batch(p.changelogCmd(sel.Name), p.spinner.Tick)
}

func (p *Panel) openPPAScreen() (*Panel, tea.Cmd) {
	if p.ppaManager == nil {
		p.statusMsg = fmt.Sprintf("PPA management isn't available for %s.", p.mgr.Name())
		return p, nil
	}
	p.screen = screenPPA
	p.loading = true
	p.statusMsg = ""
	return p, tea.Batch(p.loadPPAsCmd(), p.spinner.Tick)
}

func (p *Panel) startAddPPA() (*Panel, tea.Cmd) {
	name := strings.TrimSpace(p.ppaInput.Value())
	p.ppaInput.SetValue("")
	p.ppaInput.Blur()
	p.ppaAdding = false
	if name == "" {
		return p, nil
	}
	p.pending = &pendingAction{label: fmt.Sprintf("Add repository %s?\nThis runs add-apt-repository with root privileges.", name), argv: p.ppaManager.AddPPACmd(name)}
	p.returnScreen = p.screen
	p.screen = screenConfirm
	return p, nil
}

func (p *Panel) startRemovePPA() (*Panel, tea.Cmd) {
	if p.ppaCursor < 0 || p.ppaCursor >= len(p.ppas) {
		return p, nil
	}
	target := p.ppas[p.ppaCursor]
	p.pending = &pendingAction{label: fmt.Sprintf("Remove repository %s?", target.Name), argv: p.ppaManager.RemovePPACmd(target)}
	p.returnScreen = p.screen
	p.screen = screenConfirm
	return p, nil
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
		p.returnScreen = p.screen
		p.screen = screenConfirm
		return p, nil
	}
	argv := p.mgr.InstallCmd(sel.Name)
	p.pending = &pendingAction{label: fmt.Sprintf("Install %s?", sel.Name), argv: argv}
	p.returnScreen = p.screen
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
	p.returnScreen = p.screen
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
	p.returnScreen = p.screen
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
	p.returnScreen = p.screen
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
	p.returnScreen = p.screen
	p.screen = screenConfirm
	return p, nil
}

// startUpgradeAll fetches the current upgradable list before showing the
// confirmation, so the modal can say what's actually about to change
// instead of a blind "upgrade everything?".
func (p *Panel) startUpgradeAll() (*Panel, tea.Cmd) {
	p.awaitingUpgradeAllConfirm = true
	p.loading = true
	return p, tea.Batch(p.loadUpgradableCmd(), p.spinner.Tick)
}

func (p *Panel) openUpgradeAllConfirm(pkgs []pkg.Package) {
	if len(pkgs) == 0 {
		p.statusMsg = "Nothing to upgrade."
		return
	}
	names := make([]string, len(pkgs))
	security := 0
	for i, pk := range pkgs {
		names[i] = pk.Name
		if pk.Security {
			security++
		}
	}
	label := fmt.Sprintf("Upgrade %d %s packages?", len(pkgs), p.mgr.Name())
	if security > 0 {
		label += fmt.Sprintf(" (%d security)", security)
	}
	label += "\n" + strings.Join(names, ", ")
	argv := p.mgr.UpgradeCmd("")
	p.pending = &pendingAction{label: label, argv: argv}
	p.returnScreen = p.screen
	p.screen = screenConfirm
}

func (p *Panel) startHold() (*Panel, tea.Cmd) {
	if p.holder == nil {
		p.statusMsg = fmt.Sprintf("Holding isn't supported for %s.", p.mgr.Name())
		return p, nil
	}
	sel, ok := p.selected()
	if !ok {
		return p, nil
	}
	if sel.Status == pkg.StatusAvailable {
		p.statusMsg = "Not installed."
		return p, nil
	}
	if sel.Held {
		p.pending = &pendingAction{label: fmt.Sprintf("Unhold %s? (allow it to be upgraded again)", sel.Name), argv: p.holder.UnholdCmd(sel.Name)}
	} else {
		p.pending = &pendingAction{label: fmt.Sprintf("Hold %s? (block it from future upgrades)", sel.Name), argv: p.holder.HoldCmd(sel.Name)}
	}
	p.returnScreen = p.screen
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
	p.returnScreen = p.screen
	p.screen = screenConfirm
	return p, nil
}

// SupportsSync reports whether the "s" sync-cache action does anything for
// this backend, so the root App can hide the hint when it wouldn't (snap).
func (p *Panel) SupportsSync() bool { return p.mgr.UpdateCmd() != nil }

func (p *Panel) executeConfirmedAction() (*Panel, tea.Cmd) {
	pending := p.pending
	p.pending = nil
	if pending == nil || len(pending.argv) == 0 {
		p.screen = p.returnScreen
		return p, nil
	}
	p.actionRunning = true
	p.screen = screenRunning
	return p, startPTYCmd(p.mgr.Name(), pending.argv)
}

// dismissRunning leaves the live-output screen after the command has
// finished, refreshing whatever list/PPA view we came from.
func (p *Panel) dismissRunning() (*Panel, tea.Cmd) {
	exitErr := p.running.exitErr
	p.running.close()
	p.running = nil
	p.actionRunning = false
	p.tagged = nil
	p.refreshDelegate()
	if exitErr != nil {
		p.statusMsg = errorStyle.Render("Error: " + exitErr.Error())
	} else {
		p.statusMsg = "Done."
	}
	p.loading = true
	if p.returnScreen == screenPPA {
		p.screen = screenPPA
		return p, tea.Batch(p.loadPPAsCmd(), p.spinner.Tick)
	}
	if p.returnScreen == screenDisk {
		p.screen = screenDisk
		return p, tea.Batch(p.loadDiskReportCmd(), p.spinner.Tick)
	}
	p.screen = screenList
	return p, tea.Batch(p.refreshCmd(), p.spinner.Tick)
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

	if p.screen == screenChangelog {
		return lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render(fmt.Sprintf(" %s — Changelog ", strings.ToUpper(p.mgr.Name()))),
			detailBoxStyle.Width(maxInt(p.width-4, 10)).Render(p.viewport.View()),
			dimStyle.Render("esc/enter: back   ↑/↓: scroll"),
		)
	}

	if p.screen == screenRunning {
		return p.renderRunning()
	}

	if p.screen == screenHelp {
		return p.renderHelp()
	}

	if p.screen == screenPPA {
		return p.renderPPA()
	}

	if p.screen == screenDisk {
		return p.renderDisk()
	}

	if p.screen == screenProvenance {
		return p.renderProvenance()
	}

	if p.screen == screenUnattended {
		return p.renderUnattended()
	}

	if p.screen == screenVersion {
		return p.renderVersion()
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
	case p.list.FilterState() == list.Filtering:
		status = dimStyle.Render("esc: close without filtering   enter: apply filter")
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
		// "Upgrade all" joins every package name into a single unbroken
		// line — modalStyle has no width of its own, so without wrapping
		// it first, that line runs straight past the box border and off
		// the edge of the terminal instead of wrapping inside it. Only
		// wrap when the content actually needs it, though: forcing every
		// confirm through a fixed Width() would also pad a short one-line
		// "Install x?" out to that same width, turning today's small,
		// snug box into a wide mostly-empty one.
		if wrapWidth := min(76, maxInt(p.width-8, 20)); lipgloss.Width(body) > wrapWidth {
			body = lipgloss.NewStyle().Width(wrapWidth).Render(body)
		}
		modal := modalStyle.Render(body + "\n\n[y] confirm    [n] cancel")
		content = lipgloss.Place(p.width, p.height, lipgloss.Center, lipgloss.Center, modal)
	}

	return content
}

func (p *Panel) renderPPA() string {
	var sections []string
	sections = append(sections, titleStyle.Render(fmt.Sprintf(" %s — Third-party repositories (PPAs) (%d) ", strings.ToUpper(p.mgr.Name()), len(p.ppas))))
	sections = append(sections, warnBannerStyle.Width(maxInt(p.width, 10)).Render(
		"⚠ Careful: adding/removing repositories can break `apt update` or replace system packages. Know what you're adding."))

	if len(p.ppas) == 0 && !p.loading {
		sections = append(sections, dimStyle.Render("No third-party PPAs found in /etc/apt/sources.list.d."))
	}
	for i, ppa := range p.ppas {
		line := fmt.Sprintf("%s  %s", ppa.Name, dimStyle.Render(ppa.Description))
		if i == p.ppaCursor {
			line = lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(colorFg).Bold(true).Width(p.width - 2).Render(fmt.Sprintf("%s  %s", ppa.Name, ppa.Description))
		}
		sections = append(sections, line)
	}

	if p.ppaAdding {
		sections = append(sections, searchBoxStyle.Width(maxInt(p.width-4, 10)).Render(p.ppaInput.View()))
	}

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
	sections = append(sections, dimStyle.Render("a: add   d: remove selected   esc: back"))

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// diskRow renders one disk-cleanup finding, truncating (not wrapping) to
// stay within the terminal width — same approach itemDelegate.Render uses
// for the main list, for the same reason: a plain fmt.Sprintf with fixed
// column widths would run straight off the edge on a narrow terminal.
func (p *Panel) diskRow(it pkg.DiskItem, selected bool) string {
	line := fmt.Sprintf("%-40s %-42s %10s", it.Name, it.Reason, humanizeBytes(it.Size))
	maxW := maxInt(p.width-2, 0)
	if lipgloss.Width(line) > maxW {
		line = truncateANSI(line, maxW)
	}
	if selected {
		return lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(colorFg).Bold(true).Width(maxW).Render(line)
	}
	return line
}

func (p *Panel) renderDisk() string {
	var total int64
	for _, it := range p.diskItems {
		total += it.Size
	}
	var sections []string
	sections = append(sections, titleStyle.Render(fmt.Sprintf(" %s — Disk cleanup (%d finding(s), %s reclaimable) ",
		strings.ToUpper(p.mgr.Name()), len(p.diskItems), humanizeBytes(total))))

	if len(p.diskItems) == 0 && !p.loading {
		sections = append(sections, dimStyle.Render("Nothing to reclaim — no old kernels, leftover configs, or disabled revisions found."))
	}
	for i, it := range p.diskItems {
		sections = append(sections, p.diskRow(it, i == p.diskCursor))
	}

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
	sections = append(sections, dimStyle.Render("d/r: purge selected   esc: back"))

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (p *Panel) renderProvenance() string {
	var sections []string
	sections = append(sections, titleStyle.Render(fmt.Sprintf(" %s — Why is %s installed? ", strings.ToUpper(p.mgr.Name()), p.provenanceName)))

	reason := "pulled in as a dependency of something else, not asked for directly"
	if p.provenance.Manual {
		reason = "explicitly installed (apt-mark manual)"
	}
	sections = append(sections, dimStyle.Render(reason), "")

	switch {
	case p.loading:
		sections = append(sections, p.spinner.View()+" loading...")
	case len(p.provenance.ReverseDeps) == 0:
		sections = append(sections, dimStyle.Render("Nothing else currently depends on it."))
	default:
		sections = append(sections, helpSectionStyle.Render(fmt.Sprintf("Depended on by (%d) — enter to drill in:", len(p.provenance.ReverseDeps))))
		for i, name := range p.provenance.ReverseDeps {
			line := "  " + name
			maxW := maxInt(p.width-2, 0)
			if lipgloss.Width(line) > maxW {
				line = truncateANSI(line, maxW)
			}
			if i == p.provenanceCursor {
				line = lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(colorFg).Bold(true).Width(maxW).Render(line)
			}
			sections = append(sections, line)
		}
	}

	if p.err != nil {
		sections = append(sections, "", errorStyle.Render(p.err.Error()))
	}

	hint := "enter: drill in   esc: back"
	if len(p.provenanceStack) > 0 {
		hint = "enter: drill in   esc: back one level"
	}
	sections = append(sections, "", dimStyle.Render(hint))
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (p *Panel) renderUnattended() string {
	s := p.uaStatus
	var rows []string
	rows = append(rows, titleStyle.Render(" Unattended upgrades "), "")

	enabled := dimStyle.Render("disabled")
	if s.Enabled {
		enabled = statusInstalledStyle.Render("enabled")
	}
	rows = append(rows, "Automatic background upgrades: "+enabled, "")

	switch {
	case p.loading:
		rows = append(rows, p.spinner.View()+" loading...")
	case s.LastRunTime == "":
		rows = append(rows, dimStyle.Render("No completed run found in the local log."))
	default:
		rows = append(rows, "Last run: "+s.LastRunTime)
		if len(s.LastPackages) > 0 {
			rows = append(rows, dimStyle.Render("  upgraded: "+strings.Join(s.LastPackages, ", ")))
		} else {
			rows = append(rows, dimStyle.Render("  nothing needed upgrading"))
		}
	}
	rows = append(rows, "")
	if s.NextRunTime == "" {
		rows = append(rows, dimStyle.Render("Next run: unknown (apt-daily-upgrade.timer not found)"))
	} else {
		rows = append(rows, "Next run: "+s.NextRunTime)
	}

	if p.err != nil {
		rows = append(rows, "", errorStyle.Render(p.err.Error()))
	}
	rows = append(rows, "", dimStyle.Render("esc/enter: back"))

	body := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return lipgloss.Place(p.width, p.height, lipgloss.Center, lipgloss.Center, helpBoxStyle.Render(body))
}

func (p *Panel) renderVersion() string {
	var sections []string
	sections = append(sections, titleStyle.Render(fmt.Sprintf(" %s — Install a specific version of %s ", strings.ToUpper(p.mgr.Name()), p.versionPkgName)))

	if len(p.versions) == 0 && !p.loading && p.err == nil {
		sections = append(sections, dimStyle.Render("No other versions found."))
	}
	for i, v := range p.versions {
		marker := "  "
		if v.Current {
			marker = "* "
		}
		line := fmt.Sprintf("%s%-24s %s", marker, v.Version, v.Origin)
		maxW := maxInt(p.width-2, 0)
		if lipgloss.Width(line) > maxW {
			line = truncateANSI(line, maxW)
		}
		if i == p.versionCursor {
			line = lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(colorFg).Bold(true).Width(maxW).Render(line)
		}
		sections = append(sections, line)
	}

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
	sections = append(sections, dimStyle.Render("* = currently installed   enter: install this version   esc: back"))

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (p *Panel) renderRunning() string {
	cmdLine := ""
	hint := "live output — keystrokes go to the command (e.g. the sudo password); ctrl+c interrupts it"
	status := " — running "
	box := detailBoxStyle
	if p.running != nil {
		cmdLine = strings.Join(p.running.argv, " ")
		if p.running.exited {
			hint = "press any key to continue"
			if p.running.exitErr != nil {
				status = " — failed "
				box = box.BorderForeground(colorDanger)
			} else {
				status = " — done "
				box = box.BorderForeground(colorAccent)
			}
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render(fmt.Sprintf(" %s%s", strings.ToUpper(p.mgr.Name()), status)),
		dimStyle.Render(" $ "+cmdLine),
		box.Width(maxInt(p.width-4, 10)).Render(p.viewport.View()),
		dimStyle.Render(hint),
	)
}

// helpContent builds the help screen's body text. Rendered inside a
// viewport (see renderHelp) rather than sized to fit, since a fixed-height
// render already broke once from a single added row on a modest 100x34
// terminal (caught by e2e/navigation_test.go) — and this list only grows as
// backends gain more optional features.
func (p *Panel) helpContent() string {
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(colorHighlight).Width(14)
	row := func(k, desc string) string {
		return keyStyle.Render(k) + dimStyle.Render(desc)
	}

	rows := []string{
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
		row("U", "upgrade ALL packages (shows what will change first)"),
		row("S", "sort the current view by installed size"),
		row("s", "sync package cache (apt only)"),
		row("y / n", "confirm / cancel a pending action"),
	}
	if p.chanInstaller != nil {
		rows = append(rows, row("c", "cycle install channel (while confirming a snap install)"))
	}
	if p.holder != nil {
		rows = append(rows, row("H", "hold/unhold the selected package"))
	}
	if p.changelogger != nil {
		rows = append(rows, row("C", "view the selected package's changelog"))
	}
	if p.ppaManager != nil {
		rows = append(rows, row("P", "manage third-party repositories (PPAs)"))
	}
	if p.diskAnalyzer != nil {
		rows = append(rows, row("K", "disk cleanup: old kernels, leftover configs, disabled revisions"))
	}
	if p.provenanceProvider != nil {
		rows = append(rows, row("W", "why is the selected package installed"))
	}
	if p.uaReporter != nil {
		rows = append(rows, row("A", "unattended (automatic background) upgrades status"))
	}
	if p.versionLister != nil {
		rows = append(rows, row("V", "install a specific version of the selected package, or downgrade"))
	}
	if p.reverter != nil {
		rows = append(rows, row("V", "revert the selected package to its previous revision"))
	}
	rows = append(rows,
		"",
		helpSectionStyle.Render("Status symbols"),
		"  "+legendLine(),
		"",
		row("O", "apt+snap overlap: duplicate installs, stale snaps"),
		row(",", "settings (theme, keybindings)"),
		row("ctrl+l", "force a full screen redraw"),
		row("q", "quit"),
		row("?", "close this help"),
	)

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (p *Panel) renderHelp() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render(" pkgtui — help "),
		detailBoxStyle.Width(maxInt(p.width-4, 10)).Render(p.viewport.View()),
		dimStyle.Render("esc/enter/?: back   ↑/↓: scroll"),
	)
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
