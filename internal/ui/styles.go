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

// Built-in themes. "default" mirrors htop's classic dark scheme.
var palettes = map[string]palette{
	"default": {
		bg: "235", fg: "252", accent: "42", warn: "220",
		danger: "203", muted: "244", highlight: "39",
	},
	"dracula": {
		bg: "235", fg: "253", accent: "84", warn: "228",
		danger: "212", muted: "61", highlight: "141",
	},
	"nord": {
		bg: "235", fg: "251", accent: "108", warn: "222",
		danger: "167", muted: "245", highlight: "110",
	},
	"solarized": {
		bg: "235", fg: "244", accent: "64", warn: "136",
		danger: "160", muted: "241", highlight: "37",
	},
	"gruvbox": {
		bg: "235", fg: "223", accent: "142", warn: "214",
		danger: "167", muted: "245", highlight: "109",
	},
}

// ThemeNames lists built-in themes in a stable order, for cycling in the
// settings screen.
func ThemeNames() []string {
	return []string{"default", "dracula", "nord", "solarized", "gruvbox"}
}

var currentTheme = "default"

// CurrentTheme returns the active theme's name.
func CurrentTheme() string { return currentTheme }

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
		Foreground(colorBg).
		Bold(true).
		Padding(0, 1)

	tabActiveStyle = lipgloss.NewStyle().
		Foreground(colorBg).
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
		Foreground(colorBg).
		Background(colorHighlight).
		Bold(true).
		Padding(0, 1)

	statusInstalledStyle = lipgloss.NewStyle().Foreground(colorAccent)
	statusUpgradableStyle = lipgloss.NewStyle().Foreground(colorWarn)
	statusAvailableStyle = lipgloss.NewStyle().Foreground(colorMuted)

	tagMarkStyle = lipgloss.NewStyle().Foreground(colorHighlight).Bold(true)
	securityMarkStyle = lipgloss.NewStyle().Foreground(colorDanger).Bold(true)
	heldMarkStyle = lipgloss.NewStyle().Foreground(colorMuted).Bold(true)

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
		Foreground(colorBg).
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
