package ui

import "github.com/charmbracelet/lipgloss"

// palette is the set of base colors every derived style is built from, so a
// theme switch only has to replace these and rebuild the styles below.
type palette struct {
	bg        string
	fg        string
	accent    string // green: "healthy"/installed
	warn      string // yellow: attention/upgradable
	danger    string // red: destructive actions
	muted     string
	highlight string // cyan: active tab/accent
}

// Built-in themes. "default" mirrors htop's classic dark scheme. Every
// color here was picked by computing WCAG contrast ratios against a dark
// terminal background (and against pure black, used for text on the
// colored badges below) rather than by eye — see
// scripts/check-theme-contrast.py. muted is deliberately the same value
// in every theme: it's dim structural text, not part of a theme's
// identity, and needs to stay safely readable everywhere.
var palettes = map[string]palette{
	"default": {
		bg: "235", fg: "252", accent: "42", warn: "220",
		danger: "203", muted: "245", highlight: "39",
	},
	"dracula": {
		bg: "235", fg: "253", accent: "84", warn: "228",
		danger: "212", muted: "245", highlight: "141",
	},
	"nord": {
		bg: "235", fg: "251", accent: "108", warn: "222",
		danger: "167", muted: "245", highlight: "110",
	},
	"solarized": {
		bg: "235", fg: "244", accent: "70", warn: "136",
		danger: "196", muted: "245", highlight: "45",
	},
	"gruvbox": {
		bg: "235", fg: "223", accent: "142", warn: "214",
		danger: "167", muted: "245", highlight: "109",
	},
	"catppuccin": {
		bg: "235", fg: "189", accent: "151", warn: "223",
		danger: "210", muted: "245", highlight: "183",
	},
	"tokyo-night": {
		bg: "235", fg: "195", accent: "149", warn: "179",
		danger: "204", muted: "245", highlight: "111",
	},
	"monokai": {
		bg: "235", fg: "255", accent: "154", warn: "221",
		danger: "197", muted: "245", highlight: "80",
	},
	"darcula": {
		bg: "235", fg: "250", accent: "107", warn: "215",
		danger: "173", muted: "245", highlight: "74",
	},
	"vscode": {
		bg: "235", fg: "254", accent: "78", warn: "178",
		danger: "174", muted: "245", highlight: "33",
	},
	"ubuntu": {
		bg: "235", fg: "231", accent: "64", warn: "100",
		danger: "203", muted: "245", highlight: "166",
	},
}

// colorBadgeText is the foreground for text on a colored badge (active tab,
// header bar, key hints, warning banner). It's fixed at pure black rather
// than tied to the theme's own background: on some terminals a near-black
// 256-color code like colorBg doesn't render dark enough against a bright
// accent/highlight background to stay readable, while "0" (the base ANSI
// black every terminal supports) reliably does.
var colorBadgeText = lipgloss.Color("0")

// ThemeNames lists built-in themes in a stable order, for cycling in the
// settings screen.
func ThemeNames() []string {
	return []string{"default", "dracula", "nord", "solarized", "gruvbox", "catppuccin", "tokyo-night", "monokai", "darcula", "vscode", "ubuntu"}
}

var currentTheme = "default"

// CurrentTheme returns the active theme's name.
func CurrentTheme() string { return currentTheme }

// themeIndex returns CurrentTheme's position in ThemeNames, for progress
// display ("2/5") while browsing themes in the settings screen.
func themeIndex() int {
	for i, n := range ThemeNames() {
		if n == currentTheme {
			return i
		}
	}
	return 0
}

// ApplyTheme switches the active color palette and rebuilds every derived
// style, so already-open screens pick it up on their next render. Unknown
// names are ignored (returns false).
func ApplyTheme(name string) bool {
	p, ok := palettes[name]
	if !ok {
		return false
	}
	currentTheme = name
	applyPalette(p)
	return true
}

var (
	colorBg        lipgloss.Color
	colorFg        lipgloss.Color
	colorAccent    lipgloss.Color
	colorWarn      lipgloss.Color
	colorDanger    lipgloss.Color
	colorMuted     lipgloss.Color
	colorHighlight lipgloss.Color
)

var (
	headerBarStyle        lipgloss.Style
	tabActiveStyle        lipgloss.Style
	tabInactiveStyle      lipgloss.Style
	footerBarStyle        lipgloss.Style
	keyHintStyle          lipgloss.Style
	statusInstalledStyle  lipgloss.Style
	statusUpgradableStyle lipgloss.Style
	statusAvailableStyle  lipgloss.Style
	tagMarkStyle          lipgloss.Style
	securityMarkStyle     lipgloss.Style
	heldMarkStyle         lipgloss.Style
	selectedRowStyle      lipgloss.Style
	titleStyle            lipgloss.Style
	dimStyle              lipgloss.Style
	searchBoxStyle        lipgloss.Style
	detailBoxStyle        lipgloss.Style
	modalStyle            lipgloss.Style
	warnBannerStyle       lipgloss.Style
	helpBoxStyle          lipgloss.Style
	helpSectionStyle      lipgloss.Style
	spinnerStyle          lipgloss.Style
	errorStyle            lipgloss.Style
)

func applyPalette(p palette) {
	colorBg = lipgloss.Color(p.bg)
	colorFg = lipgloss.Color(p.fg)
	colorAccent = lipgloss.Color(p.accent)
	colorWarn = lipgloss.Color(p.warn)
	colorDanger = lipgloss.Color(p.danger)
	colorMuted = lipgloss.Color(p.muted)
	colorHighlight = lipgloss.Color(p.highlight)

	headerBarStyle = lipgloss.NewStyle().
		Background(colorHighlight).
		Foreground(colorBadgeText).
		Bold(true).
		Padding(0, 1)

	tabActiveStyle = lipgloss.NewStyle().
		Foreground(colorBadgeText).
		Background(colorAccent).
		Bold(true).
		Padding(0, 2)

	tabInactiveStyle = lipgloss.NewStyle().
		Foreground(colorMuted).
		Padding(0, 2)

	footerBarStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("236")).
		Foreground(colorFg).
		Padding(0, 1)

	keyHintStyle = lipgloss.NewStyle().
		Foreground(colorBadgeText).
		Background(colorHighlight).
		Bold(true).
		Padding(0, 1)

	statusInstalledStyle = lipgloss.NewStyle().Foreground(colorAccent)
	statusUpgradableStyle = lipgloss.NewStyle().Foreground(colorWarn)
	statusAvailableStyle = lipgloss.NewStyle().Foreground(colorMuted)

	tagMarkStyle = lipgloss.NewStyle().Foreground(colorHighlight).Bold(true)
	securityMarkStyle = lipgloss.NewStyle().Foreground(colorDanger).Bold(true)
	heldMarkStyle = lipgloss.NewStyle().Foreground(colorHighlight).Bold(true)

	// The selected row is rendered as one single styled span (see
	// itemDelegate.Render), not by wrapping an already-colored string: a
	// colored bullet/mark rendered on its own carries an embedded ANSI
	// reset, and wrapping that in an outer background only paints up to
	// that reset, leaving the rest of the row un-highlighted. Full black-
	// on-highlight (like the badges) keeps it visible on every theme.
	selectedRowStyle = lipgloss.NewStyle().
		Background(colorHighlight).
		Foreground(colorBadgeText).
		Bold(true)

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(colorFg)
	dimStyle = lipgloss.NewStyle().Foreground(colorMuted)

	searchBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorHighlight).
		Padding(0, 1)

	detailBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorAccent).
		Padding(1, 2)

	modalStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorDanger).
		Padding(1, 2).
		Bold(true)

	warnBannerStyle = lipgloss.NewStyle().
		Background(colorDanger).
		Foreground(colorBadgeText).
		Bold(true).
		Padding(0, 1)

	helpBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorHighlight).
		Padding(1, 3)

	helpSectionStyle = lipgloss.NewStyle().Bold(true).Foreground(colorHighlight)

	spinnerStyle = lipgloss.NewStyle().Foreground(colorHighlight)

	errorStyle = lipgloss.NewStyle().Foreground(colorDanger).Bold(true)
}

func init() {
	applyPalette(palettes[currentTheme])
}
