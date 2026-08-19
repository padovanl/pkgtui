package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/padovanl/pkgtui/internal/pkg"
)

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// sortBySizeDesc orders pkgs by installed size, largest first — the whole
// point of the metrics dashboard being a ranking, not just a listing.
func sortBySizeDesc(pkgs []pkg.Package) {
	sort.SliceStable(pkgs, func(i, j int) bool { return pkgs[i].Size > pkgs[j].Size })
}

// --- Metrics dashboard: installed packages ranked by disk usage, as a bar
// chart. Reuses pkg.Manager.ListInstalled (every backend already
// implements it, and both now populate Size — see snapFileSize for how
// snap does, since "snap list" itself has no size column at all).

func (p *Panel) openMetrics() (*Panel, tea.Cmd) {
	p.screen = screenMetrics
	p.metricsCursor = 0
	p.loading = true
	p.statusMsg = ""
	return p, tea.Batch(p.loadMetricsCmd(), p.spinner.Tick)
}

func (p *Panel) handleMetricsKey(msg tea.KeyMsg) (*Panel, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		p.screen = screenList
	case key.Matches(msg, keys.Up):
		if p.metricsCursor > 0 {
			p.metricsCursor--
		}
	case key.Matches(msg, keys.Down):
		if p.metricsCursor < len(p.metrics)-1 {
			p.metricsCursor++
		}
	}
	return p, nil
}

func (p *Panel) renderMetrics() string {
	var sections []string
	sections = append(sections, titleStyle.Render(fmt.Sprintf(" %s — Disk usage by package (%d) ", strings.ToUpper(p.mgr.Name()), len(p.metrics))))

	if len(p.metrics) == 0 && !p.loading {
		sections = append(sections, dimStyle.Render("No installed packages with a known size."))
	}

	var maxSize int64
	for _, pk := range p.metrics {
		if pk.Size > maxSize {
			maxSize = pk.Size
		}
	}

	const nameW = 26
	const sizeW = 10
	barW := maxInt(minInt(p.width-nameW-sizeW-6, 50), 10)

	var rows []string
	for i, pk := range p.metrics {
		name := pk.Name
		if len(name) > nameW {
			name = name[:nameW-1] + "…"
		}
		barLen := 0
		if maxSize > 0 {
			barLen = int(float64(barW) * float64(pk.Size) / float64(maxSize))
		}
		if barLen == 0 && pk.Size > 0 {
			barLen = 1 // a nonzero size always shows at least a sliver
		}
		selected := i == p.metricsCursor
		filled := strings.Repeat("█", barLen)
		empty := strings.Repeat("░", barW-barLen)

		var line string
		if selected {
			// Built from separately-styled, already-rendered segments
			// concatenated with "+", each carrying its own explicit
			// Background(...) — not one plain string wrapped in a single
			// outer Style.Render() afterwards, which would flatten the
			// bar's own accent color into the row's plain text color.
			// Same technique the status bullet on a selected row in the
			// main list uses, and for the same reason: without it, the one
			// row a user actually sees highlighted by default (the
			// largest package, since the cursor starts at 0 on a
			// size-descending list) would be the one row that looked like
			// theming wasn't applied to the chart at all.
			bg := lipgloss.NewStyle().Background(lipgloss.Color("237"))
			textStyle := bg.Foreground(colorFg).Bold(true)
			rest := fmt.Sprintf(" %s %*s", empty, sizeW, humanizeBytes(pk.Size))
			maxW := maxInt(p.width-2, 0)
			namePart := fmt.Sprintf("%-*s", nameW, name)
			if pad := maxW - lipgloss.Width(namePart+filled+rest); pad > 0 {
				rest += strings.Repeat(" ", pad)
			}
			line = textStyle.Render(namePart) + bg.Foreground(colorAccent).Bold(true).Render(filled) + textStyle.Render(rest)
		} else {
			bar := statusInstalledStyle.Render(filled) + dimStyle.Render(empty)
			line = fmt.Sprintf("%-*s %s %*s", nameW, name, bar, sizeW, humanizeBytes(pk.Size))
		}
		rows = append(rows, line)
	}
	listHeight := maxInt(p.height-4, 3)
	sections = append(sections, clampToWindow(rows, p.metricsCursor, listHeight)...)

	var status string
	switch {
	case p.loading:
		status = p.spinner.View() + " loading..."
	case p.err != nil:
		status = errorStyle.Render(p.err.Error())
	}
	if status != "" {
		sections = append(sections, status)
	}
	sections = append(sections, dimStyle.Render("sorted by installed size   esc: back"))

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// --- Upgrade conflicts (apt only): packages "apt-get upgrade" (not
// dist-upgrade) reports as kept back, distinct from an explicit hold.

func (p *Panel) openConflicts() (*Panel, tea.Cmd) {
	if p.conflictReporter == nil {
		p.statusMsg = fmt.Sprintf("Upgrade conflict detection isn't available for %s.", p.mgr.Name())
		return p, nil
	}
	p.screen = screenConflicts
	p.conflictsCursor = 0
	p.loading = true
	p.statusMsg = ""
	return p, tea.Batch(p.loadConflictsCmd(), p.spinner.Tick)
}

func (p *Panel) handleConflictsKey(msg tea.KeyMsg) (*Panel, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		p.screen = screenList
	case key.Matches(msg, keys.Up):
		if p.conflictsCursor > 0 {
			p.conflictsCursor--
		}
	case key.Matches(msg, keys.Down):
		if p.conflictsCursor < len(p.conflicts)-1 {
			p.conflictsCursor++
		}
	}
	return p, nil
}

func (p *Panel) renderConflicts() string {
	var sections []string
	sections = append(sections, titleStyle.Render(fmt.Sprintf(" %s — Upgrade conflicts (%d) ", strings.ToUpper(p.mgr.Name()), len(p.conflicts))))

	switch {
	case len(p.conflicts) == 0 && !p.loading:
		sections = append(sections, dimStyle.Render("Nothing is blocked — a plain upgrade would apply cleanly."))
	case len(p.conflicts) > 0:
		sections = append(sections, dimStyle.Render("These need a dependency change a plain upgrade won't make on its own. \"U\" (upgrade all) already resolves most of these via dist-upgrade."))
	}

	var rows []string
	for i, c := range p.conflicts {
		line := fmt.Sprintf("%-30s %s", c.Name, c.Reason)
		maxW := maxInt(p.width-2, 0)
		if lipgloss.Width(line) > maxW {
			line = truncateANSI(line, maxW)
		}
		if i == p.conflictsCursor {
			line = lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(colorFg).Bold(true).Width(maxW).Render(line)
		}
		rows = append(rows, line)
	}
	listHeight := maxInt(p.height-6, 3)
	sections = append(sections, clampToWindow(rows, p.conflictsCursor, listHeight)...)

	var status string
	switch {
	case p.loading:
		status = p.spinner.View() + " loading..."
	case p.err != nil:
		status = errorStyle.Render(p.err.Error())
	}
	if status != "" {
		sections = append(sections, status)
	}
	sections = append(sections, dimStyle.Render("esc: back"))

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// --- Action log: browses the shared sessionLog (both backends' history at
// once, appended to from dismissRunning).

func (p *Panel) openLog() (*Panel, tea.Cmd) {
	p.screen = screenLog
	p.logCursor = maxInt(len(sessionLog)-1, 0) // land on the most recent entry
	return p, nil
}

func (p *Panel) handleLogKey(msg tea.KeyMsg) (*Panel, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		p.screen = screenList
	case key.Matches(msg, keys.Up):
		if p.logCursor > 0 {
			p.logCursor--
		}
	case key.Matches(msg, keys.Down):
		if p.logCursor < len(sessionLog)-1 {
			p.logCursor++
		}
	}
	return p, nil
}

func (p *Panel) renderLog() string {
	var sections []string
	sections = append(sections, titleStyle.Render(fmt.Sprintf(" Action log (%d) ", len(sessionLog))))

	if len(sessionLog) == 0 {
		sections = append(sections, dimStyle.Render("Nothing has run yet this session."))
	}

	var rows []string
	for i, e := range sessionLog {
		mark := statusInstalledStyle.Render("✓")
		if !e.ok {
			mark = errorStyle.Render("✗")
		}
		line := fmt.Sprintf("%s %s  %-4s  %s", mark, e.when.Format("15:04:05"), strings.ToUpper(e.backend), e.summary)
		if !e.ok && e.detail != "" {
			line += dimStyle.Render("  — " + e.detail)
		}
		maxW := maxInt(p.width-2, 0)
		if lipgloss.Width(line) > maxW {
			line = truncateANSI(line, maxW)
		}
		if i == p.logCursor {
			line = lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(colorFg).Bold(true).Width(maxW).Render(line)
		}
		rows = append(rows, line)
	}
	listHeight := maxInt(p.height-3, 3)
	sections = append(sections, clampToWindow(rows, p.logCursor, listHeight)...)
	sections = append(sections, dimStyle.Render("esc: back"))

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}
