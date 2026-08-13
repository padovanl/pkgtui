package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/padovanl/pkgtui/internal/pkg"
)

// item wraps a pkg.Package so it satisfies list.Item.
type item struct {
	p pkg.Package
}

func (i item) FilterValue() string { return i.p.Name + " " + i.p.Summary }

func statusBullet(s pkg.Status) (string, lipgloss.Style) {
	switch s {
	case pkg.StatusInstalled:
		return "●", statusInstalledStyle
	case pkg.StatusUpgradable:
		return "▲", statusUpgradableStyle
	default:
		return "○", statusAvailableStyle
	}
}

// legendStatuses lists every status a row can show, in the order the legend
// should display them. statusBullet is the single source of truth for the
// symbol/color of each, so the legend can never drift out of sync with the
// actual list rendering.
var legendStatuses = []struct {
	status pkg.Status
	label  string
}{
	{pkg.StatusInstalled, "installed"},
	{pkg.StatusUpgradable, "upgrade available"},
	{pkg.StatusAvailable, "not installed"},
}

// legendLine renders a one-line "symbol meaning" key, e.g.:
//
//	● installed   ▲ upgrade available   ○ not installed
func legendLine() string {
	parts := make([]string, len(legendStatuses))
	for i, s := range legendStatuses {
		symbol, style := statusBullet(s.status)
		parts[i] = style.Render(symbol) + " " + dimStyle.Render(s.label)
	}
	return strings.Join(parts, dimStyle.Render("   "))
}

// itemDelegate renders each row as:
//
//	● name          version          summary
type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	it, ok := listItem.(item)
	if !ok {
		return
	}
	bullet, bulletStyle := statusBullet(it.p.Status)

	version := it.p.Version
	if it.p.Status != pkg.StatusAvailable {
		version = it.p.Installed
	}
	if version == "" {
		version = "-"
	}

	name := it.p.Name
	nameW := 28
	if len(name) > nameW {
		name = name[:nameW-1] + "…"
	}
	versionW := 16
	if len(version) > versionW {
		version = version[:versionW-1] + "…"
	}

	summary := it.p.Summary
	line := fmt.Sprintf("%s %-*s %-*s %s",
		bulletStyle.Render(bullet),
		nameW, name,
		versionW, version,
		summary,
	)

	maxW := m.Width() - 2
	if maxW > 0 && lipgloss.Width(line) > maxW {
		line = truncateANSI(line, maxW)
	}

	if index == m.Index() {
		line = lipgloss.NewStyle().
			Background(lipgloss.Color("237")).
			Foreground(colorFg).
			Bold(true).
			Width(m.Width() - 2).
			Render(line)
	}

	fmt.Fprint(w, line)
}

// truncateANSI trims a rendered (possibly styled) line to a display width.
func truncateANSI(s string, w int) string {
	if w <= 1 {
		return ""
	}
	stripped := lipgloss.NewStyle().Render(s)
	if lipgloss.Width(stripped) <= w {
		return s
	}
	// Fallback: plain truncation on the unstyled tail portion is good enough
	// here since long lines are just the free-text summary.
	runes := []rune(s)
	if len(runes) > w {
		return string(runes[:w-1]) + "…"
	}
	return s
}

// itemFor is a small constructor helper used by panel.go.
func itemFor(p pkg.Package) item { return item{p: p} }

// itemsFrom converts a package slice into list.Item values.
func itemsFrom(pkgs []pkg.Package) []list.Item {
	items := make([]list.Item, len(pkgs))
	for i, p := range pkgs {
		items[i] = itemFor(p)
	}
	return items
}
