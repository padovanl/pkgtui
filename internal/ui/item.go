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

// FilterValue is deliberately just the package name, not the summary too:
// bubbles/list's fuzzy filter matches any ordered subsequence, so pairing it
// with long free-text descriptions makes almost everything match something.
func (i item) FilterValue() string { return i.p.Name }

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

// heldSymbol marks a package pinned against upgrades (apt-mark hold).
// Diamond deliberately doesn't belong to the ●/▲/○ status-bullet family:
// held is orthogonal to status (a held package is still installed, or
// still upgradable-but-blocked), not a fourth status of its own.
const heldSymbol = "◆"

// legendEntry is one "symbol: meaning" pair shown in the legend and the
// help screen. Building both the row rendering and the legend from the
// same symbol/style values (statusBullet, securityMarkStyle, heldMarkStyle)
// keeps them from silently drifting apart.
type legendEntry struct {
	symbol string
	style  lipgloss.Style
	label  string
}

func legendEntries() []legendEntry {
	installed, installedStyle := statusBullet(pkg.StatusInstalled)
	upgradable, upgradableStyle := statusBullet(pkg.StatusUpgradable)
	available, availableStyle := statusBullet(pkg.StatusAvailable)
	return []legendEntry{
		{installed, installedStyle, "installed"},
		{upgradable, upgradableStyle, "upgrade available"},
		{upgradable, securityMarkStyle, "security update"},
		{available, availableStyle, "not installed"},
		{heldSymbol, heldMarkStyle, "held (upgrades blocked)"},
	}
}

// legendLine renders a one-line "symbol meaning" key, e.g.:
//
//	● installed   ▲ upgrade available   ▲ security update   ○ not installed   ◆ held (upgrades blocked)
func legendLine() string {
	entries := legendEntries()
	parts := make([]string, len(entries))
	for i, e := range entries {
		parts[i] = e.style.Render(e.symbol) + " " + dimStyle.Render(e.label)
	}
	return strings.Join(parts, dimStyle.Render("   "))
}

// humanizeBytes formats a byte count as a short human-readable size, e.g.
// 2214592 -> "2.1MB". Packages without a known size (b <= 0) render as "-".
func humanizeBytes(b int64) string {
	if b <= 0 {
		return "-"
	}
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// itemDelegate renders each row as:
//
//	● name          version          summary
//
// showSize swaps the version column for a human-readable installed size
// (used when the list is sorted by size); tagged marks multi-selected rows.
type itemDelegate struct {
	showSize bool
	tagged   map[string]bool
}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	it, ok := listItem.(item)
	if !ok {
		return
	}
	bullet, bulletStyle := statusBullet(it.p.Status)
	if it.p.Security && it.p.Status == pkg.StatusUpgradable {
		bulletStyle = securityMarkStyle
	}

	tagged := d.tagged[it.p.Name]

	version := it.p.Version
	if it.p.Status != pkg.StatusAvailable {
		version = it.p.Installed
	}
	if d.showSize {
		version = humanizeBytes(it.p.Size)
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

	held := ""
	if it.p.Held {
		held = " " + heldSymbol
	}

	selected := index == m.Index()

	var line string
	if selected {
		// The status bullet keeps its own color (green/yellow/red/muted)
		// even on the highlighted row, instead of blending into the row's
		// uniform badge-text color: it's the one signal that survives a
		// quick glance down a list where every other row is unselected.
		// Everything else is still one flat badge-text-on-highlight color,
		// same reasoning as before (per-segment ANSI resets would each cut
		// the shared background short) — just applied to fewer segments,
		// and with manual padding instead of a Style.Width() wrap so nothing
		// re-wraps already-colored text.
		mark := "  "
		if tagged {
			mark = "✓ "
		}
		prefix := mark + bullet
		prefixW := lipgloss.Width(prefix)
		rest := fmt.Sprintf(" %-*s %-*s %s%s", nameW, name, versionW, version, it.p.Summary, held)
		maxW := maxInt(m.Width()-2, 0)
		restMaxW := maxInt(maxW-prefixW, 0)
		if lipgloss.Width(rest) > restMaxW {
			rest = truncateANSI(rest, restMaxW)
		} else if pad := restMaxW - lipgloss.Width(rest); pad > 0 {
			rest += strings.Repeat(" ", pad)
		}
		bg := lipgloss.NewStyle().Background(colorHighlight)
		textStyle := bg.Foreground(colorBadgeText).Bold(true)
		line = textStyle.Render(mark) + bulletStyle.Background(colorHighlight).Bold(true).Render(bullet) + textStyle.Render(rest)
	} else {
		mark := "  "
		if tagged {
			mark = tagMarkStyle.Render("✓ ")
		}
		line = fmt.Sprintf("%s%s %-*s %-*s %s",
			mark,
			bulletStyle.Render(bullet),
			nameW, name,
			versionW, version,
			it.p.Summary,
		)
		if it.p.Held {
			line += heldMarkStyle.Render(held)
		}
		maxW := m.Width() - 2
		if maxW > 0 && lipgloss.Width(line) > maxW {
			line = truncateANSI(line, maxW)
		}
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
