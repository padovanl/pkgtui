package ui

import (
	"math"
	"strconv"

	"github.com/charmbracelet/lipgloss"
)

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

// badgeTextBlack and badgeTextWhite are the only two colors ever used as
// text on a colored badge (active tab, header bar, key hints, warning
// banner, selected row). Both are base ANSI codes (0-15), not 256-color
// codes: a terminal that only supports 16 colors — TERM=xterm without
// "-256color", common in minimal Docker/SSH sessions — renders every
// 256-color code in a theme's own palette as its *nearest* approximation,
// which is exactly what made badge text hard to read before (a fixed
// black rendered fine against some approximated backgrounds and badly
// against others). The base 16 colors aren't approximated on any
// terminal, 16-color or not, so picking between just these two is safe
// everywhere, and contrastingBadgeText below picks whichever of the two
// actually contrasts better against each theme's real accent/highlight/
// danger color, instead of assuming black always works.
var (
	badgeTextBlack = lipgloss.Color("0")
	badgeTextWhite = lipgloss.Color("15")
)

// contrastingBadgeText returns whichever of badgeTextBlack/badgeTextWhite
// has the higher WCAG contrast ratio against bg (an xterm 256-color code
// string, as stored in palette), so badge text stays readable regardless
// of how light or dark a given theme's accent/highlight/danger color is.
func contrastingBadgeText(bg string) lipgloss.Color {
	bgLum := xterm256Luminance(bg)
	blackContrast := contrastRatio(bgLum, xterm256Luminance("0"))
	whiteContrast := contrastRatio(bgLum, xterm256Luminance("15"))
	if whiteContrast > blackContrast {
		return badgeTextWhite
	}
	return badgeTextBlack
}

// xterm256Luminance converts a 256-color xterm code to its relative
// luminance (WCAG definition), covering the 16 base colors, the 6x6x6
// color cube (16-231), and the grayscale ramp (232-255) — the three
// ranges every code in palette actually falls into.
func xterm256Luminance(code string) float64 {
	n, err := strconv.Atoi(code)
	if err != nil {
		return 0
	}
	var r, g, b float64
	switch {
	case n < 16:
		table := [16][3]float64{
			{0, 0, 0}, {205, 0, 0}, {0, 205, 0}, {205, 205, 0},
			{0, 0, 238}, {205, 0, 205}, {0, 205, 205}, {229, 229, 229},
			{127, 127, 127}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0},
			{92, 92, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
		}
		r, g, b = table[n][0], table[n][1], table[n][2]
	case n < 232:
		n -= 16
		level := func(x int) float64 {
			if x == 0 {
				return 0
			}
			return float64(55 + x*40)
		}
		r, g, b = level(n/36), level((n%36)/6), level(n%6)
	default:
		v := float64(8 + (n-232)*10)
		r, g, b = v, v, v
	}
	channel := func(c float64) float64 {
		c /= 255
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(r) + 0.7152*channel(g) + 0.0722*channel(b)
}

func contrastRatio(l1, l2 float64) float64 {
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

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

	// Text color for each badge background, picked fresh per theme by
	// contrastingBadgeText — see the comment on badgeTextBlack/White.
	colorBadgeTextOnHighlight lipgloss.Color
	colorBadgeTextOnAccent    lipgloss.Color
	colorBadgeTextOnDanger    lipgloss.Color
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

	colorBadgeTextOnHighlight = contrastingBadgeText(p.highlight)
	colorBadgeTextOnAccent = contrastingBadgeText(p.accent)
	colorBadgeTextOnDanger = contrastingBadgeText(p.danger)

	headerBarStyle = lipgloss.NewStyle().
		Background(colorHighlight).
		Foreground(colorBadgeTextOnHighlight).
		Bold(true).
		Padding(0, 1)

	tabActiveStyle = lipgloss.NewStyle().
		Foreground(colorBadgeTextOnAccent).
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
		Foreground(colorBadgeTextOnHighlight).
		Background(colorHighlight).
		Bold(true).
		Padding(0, 1)

	statusInstalledStyle = lipgloss.NewStyle().Foreground(colorAccent)
	statusUpgradableStyle = lipgloss.NewStyle().Foreground(colorWarn)
	statusAvailableStyle = lipgloss.NewStyle().Foreground(colorMuted)

	tagMarkStyle = lipgloss.NewStyle().Foreground(colorHighlight).Bold(true)
	securityMarkStyle = lipgloss.NewStyle().Foreground(colorDanger).Bold(true)
	heldMarkStyle = lipgloss.NewStyle().Foreground(colorHighlight).Bold(true)

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
		Foreground(colorBadgeTextOnDanger).
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
