package ui

import "github.com/charmbracelet/lipgloss"

// Palette inspired by htop: dark background, green for "healthy"/installed,
// yellow for attention (upgradable), cyan for the active tab/accent, red for
// destructive actions.
var (
	colorBg        = lipgloss.Color("235")
	colorFg        = lipgloss.Color("252")
	colorAccent    = lipgloss.Color("42")  // green
	colorWarn      = lipgloss.Color("220") // yellow
	colorDanger    = lipgloss.Color("203") // red
	colorMuted     = lipgloss.Color("244")
	colorHighlight = lipgloss.Color("39") // cyan
)

var (
	headerBarStyle = lipgloss.NewStyle().
			Background(colorHighlight).
			Foreground(lipgloss.Color("235")).
			Bold(true).
			Padding(0, 1)

	tabActiveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("235")).
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
			Foreground(lipgloss.Color("235")).
			Background(colorHighlight).
			Bold(true).
			Padding(0, 1)

	statusInstalledStyle  = lipgloss.NewStyle().Foreground(colorAccent)
	statusUpgradableStyle = lipgloss.NewStyle().Foreground(colorWarn)
	statusAvailableStyle  = lipgloss.NewStyle().Foreground(colorMuted)

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(colorFg)
	dimStyle   = lipgloss.NewStyle().Foreground(colorMuted)

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

	helpBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorHighlight).
			Padding(1, 3)

	helpSectionStyle = lipgloss.NewStyle().Bold(true).Foreground(colorHighlight)

	spinnerStyle = lipgloss.NewStyle().Foreground(colorHighlight)

	errorStyle = lipgloss.NewStyle().Foreground(colorDanger).Bold(true)
)
