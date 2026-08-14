package ui

import (
	"fmt"
	"sort"
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

// overlapEntry is a package name installed via both apt and snap.
type overlapEntry struct {
	name        string
	aptVersion  string
	snapVersion string
}

// staleSnap is an installed snap whose current revision hasn't been
// refreshed in at least staleThresholdDays.
type staleSnap struct {
	name        string
	version     string
	lastRefresh time.Time
}

// findDuplicates returns, sorted by name, every package installed via both
// apt and snap. Canonical's own substitution of apt packages with snap
// "transitional" packages (Firefox, Chromium...) is exactly the kind of
// overlap this surfaces -- something neither backend's own tooling has any
// way to see across the other, since each only ever looks at itself.
func findDuplicates(aptPkgs, snapPkgs []pkg.Package) []overlapEntry {
	snapByName := make(map[string]pkg.Package, len(snapPkgs))
	for _, p := range snapPkgs {
		snapByName[p.Name] = p
	}
	var out []overlapEntry
	for _, a := range aptPkgs {
		if s, ok := snapByName[a.Name]; ok {
			out = append(out, overlapEntry{name: a.Name, aptVersion: a.Installed, snapVersion: s.Installed})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// findStaleSnaps flags installed snaps whose current revision has sat
// untouched longer than threshold, oldest first. Returns nil (not an error)
// when staler is nil or its own calls fail -- staleness is a nice-to-know,
// not something worth surfacing an error banner over.
func findStaleSnaps(snapPkgs []pkg.Package, staler pkg.Staler, now time.Time, threshold time.Duration) []staleSnap {
	if staler == nil {
		return nil
	}
	revisions, err := staler.InstalledRevisions()
	if err != nil {
		return nil
	}
	var out []staleSnap
	for _, p := range snapPkgs {
		rev, ok := revisions[p.Name]
		if !ok {
			continue
		}
		t, err := staler.RefreshTime(p.Name, rev)
		if err != nil {
			continue
		}
		if now.Sub(t) >= threshold {
			out = append(out, staleSnap{name: p.Name, version: p.Installed, lastRefresh: t})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].lastRefresh.Before(out[j].lastRefresh) })
	return out
}

// overlapResultMsg carries the outcome of loadOverlapCmd back to the App.
// Not a backendMsg: this spans both panels at once, so it's handled by the
// root App directly instead of being routed to one Panel like everything
// else.
type overlapResultMsg struct {
	duplicates []overlapEntry
	stale      []staleSnap
	err        error
}

// loadOverlapCmd fetches both backends' installed lists and computes the
// duplicate/staleness view. aptMgr and snapMgr are passed explicitly rather
// than read from App fields so the returned closure has no shared state
// with the model it'll later be dispatched back into.
func loadOverlapCmd(aptMgr, snapMgr pkg.Manager) tea.Cmd {
	return func() tea.Msg {
		aptPkgs, aptErr := aptMgr.ListInstalled()
		snapPkgs, snapErr := snapMgr.ListInstalled()
		if aptErr != nil && snapErr != nil {
			return overlapResultMsg{err: aptErr}
		}
		var staler pkg.Staler
		if s, ok := snapMgr.(pkg.Staler); ok {
			staler = s
		}
		return overlapResultMsg{
			duplicates: findDuplicates(aptPkgs, snapPkgs),
			stale:      findStaleSnaps(snapPkgs, staler, time.Now(), staleThresholdDays*24*time.Hour),
		}
	}
}

// overlapScreen is a small app-wide overlay (like settingsScreen) showing
// packages installed via both backends and snaps that have sat untouched
// for a long time -- a view neither apt nor snap's own tooling can produce,
// since each only ever looks at itself.
type overlapScreen struct {
	loading    bool
	err        error
	duplicates []overlapEntry
	stale      []staleSnap
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
		titleStyle.Render(" apt + snap — overlap & staleness "),
		"",
		helpSectionStyle.Render(fmt.Sprintf("Installed via both apt and snap (%d)", len(s.duplicates))),
	}
	if len(s.duplicates) == 0 && !s.loading {
		rows = append(rows, dimStyle.Render("  none found"))
	}
	idx := 0
	for _, d := range s.duplicates {
		rows = append(rows, s.row(idx, fmt.Sprintf("  %-30s apt %-16s snap %s", d.name, d.aptVersion, d.snapVersion), width))
		idx++
	}

	rows = append(rows, "", helpSectionStyle.Render(fmt.Sprintf("Snaps not refreshed in %d+ days (%d)", staleThresholdDays, len(s.stale))))
	if len(s.stale) == 0 && !s.loading {
		rows = append(rows, dimStyle.Render("  none found"))
	}
	for _, st := range s.stale {
		days := int(time.Since(st.lastRefresh).Hours() / 24)
		rows = append(rows, s.row(idx, fmt.Sprintf("  %-30s %-16s last refreshed %d days ago", st.name, st.version, days), width))
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
