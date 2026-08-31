package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/padovanl/pkgtui/internal/pkg"
)

// staleThresholdDays flags a snap whose currently installed revision hasn't
// actually changed in this long as "stale" -- long enough that "still on
// the version I installed" starts to look more like abandonment than
// intentional pinning. apt already has an equivalent signal built into this
// UI (the ▲ upgradable marker means a newer version exists); snap has
// nothing comparable: a snap can sit untouched indefinitely without ever
// showing as "behind" if nothing newer happens to exist on its tracked
// channel, or the machine simply never runs `snap refresh`.
const staleThresholdDays = 180

// backendVersion is one backend's take on a package: which manager it came
// from and what version that manager has installed.
type backendVersion struct {
	backend string
	version string
}

// overlapEntry is a package name installed through more than one backend,
// with every backend that has it.
type overlapEntry struct {
	name     string
	installs []backendVersion
}

// staleEntry is an installed package whose current revision hasn't been
// refreshed in at least staleThresholdDays.
type staleEntry struct {
	backend     string
	name        string
	version     string
	lastRefresh time.Time
}

// findDuplicates returns, sorted by name, every package installed through
// more than one backend, each entry listing them in the given backend
// order. Canonical's own substitution of apt packages with snap
// "transitional" packages (Firefox, Chromium...) is exactly the kind of
// overlap this surfaces -- and once flatpak and brew are in the picture too
// it's no longer a two-way question: the same app can just as easily be
// installed from apt and flatpak at once. No backend's own tooling can see
// any of this, since each only ever looks at itself.
func findDuplicates(byBackend map[string][]pkg.Package, order []string) []overlapEntry {
	installs := map[string][]backendVersion{}
	for _, backend := range order {
		for _, p := range byBackend[backend] {
			installs[p.Name] = append(installs[p.Name], backendVersion{backend: backend, version: p.Installed})
		}
	}
	var out []overlapEntry
	for name, versions := range installs {
		if len(versions) < 2 {
			continue
		}
		out = append(out, overlapEntry{name: name, installs: versions})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// findStale flags a backend's installed packages whose current revision has
// sat untouched longer than threshold, oldest first. Returns nil (not an
// error) when staler is nil or its own calls fail -- staleness is a
// nice-to-know, not something worth surfacing an error banner over.
func findStale(backend string, pkgs []pkg.Package, staler pkg.Staler, now time.Time, threshold time.Duration) []staleEntry {
	if staler == nil {
		return nil
	}
	revisions, err := staler.InstalledRevisions()
	if err != nil {
		return nil
	}
	var out []staleEntry
	for _, p := range pkgs {
		rev, ok := revisions[p.Name]
		if !ok {
			continue
		}
		t, err := staler.RefreshTime(p.Name, rev)
		if err != nil {
			continue
		}
		if now.Sub(t) >= threshold {
			out = append(out, staleEntry{backend: backend, name: p.Name, version: p.Installed, lastRefresh: t})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].lastRefresh.Before(out[j].lastRefresh) })
	return out
}

// overlapResultMsg carries the outcome of loadOverlapCmd back to the App.
// Not a backendMsg: this spans every panel at once, so it's handled by the
// root App directly instead of being routed to one Panel like everything
// else.
type overlapResultMsg struct {
	duplicates []overlapEntry
	stale      []staleEntry
	err        error
}

// loadOverlapCmd fetches every backend's installed list and computes the
// duplicate/staleness view. The managers are passed explicitly rather than
// read from App fields so the returned closure has no shared state with the
// model it'll later be dispatched back into.
func loadOverlapCmd(mgrs []pkg.Manager) tea.Cmd {
	return func() tea.Msg {
		byBackend := map[string][]pkg.Package{}
		order := make([]string, 0, len(mgrs))
		var stale []staleEntry
		var firstErr error
		failed := 0
		for _, mgr := range mgrs {
			name := mgr.Name()
			order = append(order, name)
			pkgs, err := mgr.ListInstalled()
			if err != nil {
				failed++
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			byBackend[name] = pkgs
			if staler, ok := mgr.(pkg.Staler); ok {
				stale = append(stale, findStale(name, pkgs, staler, time.Now(), staleThresholdDays*24*time.Hour)...)
			}
		}
		// One backend failing is expected (a manager whose tool is
		// installed but whose daemon isn't running, say); every one of them
		// failing means there's nothing to compare and the error is the
		// only thing worth showing.
		if failed == len(mgrs) && firstErr != nil {
			return overlapResultMsg{err: firstErr}
		}
		sort.Slice(stale, func(i, j int) bool { return stale[i].lastRefresh.Before(stale[j].lastRefresh) })
		return overlapResultMsg{duplicates: findDuplicates(byBackend, order), stale: stale}
	}
}

// overlapScreen is a small app-wide overlay (like settingsScreen) showing
// packages installed through more than one backend, plus packages that have
// sat untouched for a long time -- a view no single backend's own tooling
// can produce, since each only ever looks at itself.
type overlapScreen struct {
	loading    bool
	err        error
	duplicates []overlapEntry
	stale      []staleEntry
	cursor     int
}

func newOverlapScreen() *overlapScreen { return &overlapScreen{loading: true} }

func (s *overlapScreen) rowCount() int { return len(s.duplicates) + len(s.stale) }

func (s *overlapScreen) handleKey(msg tea.KeyMsg) {
	switch {
	case key.Matches(msg, keys.Up):
		if s.cursor > 0 {
			s.cursor--
		}
	case key.Matches(msg, keys.Down):
		if s.cursor < s.rowCount()-1 {
			s.cursor++
		}
	}
}

func (s *overlapScreen) row(idx int, text string, width int) string {
	maxW := maxInt(width-2, 0)
	if lipgloss.Width(text) > maxW {
		text = truncateANSI(text, maxW)
	}
	if idx == s.cursor {
		return lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(colorFg).Bold(true).Width(maxW).Render(text)
	}
	return text
}

func (s *overlapScreen) View(width, height int) string {
	rows := []string{
		titleStyle.Render(" Backend overlap & staleness "),
		"",
		helpSectionStyle.Render(fmt.Sprintf("Installed through more than one backend (%d)", len(s.duplicates))),
	}
	if len(s.duplicates) == 0 && !s.loading {
		rows = append(rows, dimStyle.Render("  none found"))
	}
	idx := 0
	for _, d := range s.duplicates {
		var installs []string
		for _, in := range d.installs {
			installs = append(installs, in.backend+" "+in.version)
		}
		rows = append(rows, s.row(idx, fmt.Sprintf("  %-30s %s", d.name, strings.Join(installs, "   ")), width))
		idx++
	}

	rows = append(rows, "", helpSectionStyle.Render(fmt.Sprintf("Not refreshed in %d+ days (%d)", staleThresholdDays, len(s.stale))))
	if len(s.stale) == 0 && !s.loading {
		rows = append(rows, dimStyle.Render("  none found"))
	}
	for _, st := range s.stale {
		days := int(time.Since(st.lastRefresh).Hours() / 24)
		rows = append(rows, s.row(idx, fmt.Sprintf("  %-10s %-30s %-16s last refreshed %d days ago", st.backend, st.name, st.version, days), width))
		idx++
	}

	var status string
	switch {
	case s.loading:
		status = "loading..."
	case s.err != nil:
		status = errorStyle.Render(s.err.Error())
	}
	if status != "" {
		rows = append(rows, "", dimStyle.Render(status))
	}
	rows = append(rows, "", dimStyle.Render("↑/↓ move   esc close"))

	body := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return body
}
