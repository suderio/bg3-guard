package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	ColorPrimary   = lipgloss.Color("#E5A93C") // Golden D20 / Honour gold
	ColorSecondary = lipgloss.Color("#44BBA4") // Cyan / teal
	ColorDark      = lipgloss.Color("#1B1B1E")
	ColorGray      = lipgloss.Color("#6E6E78")
	ColorLightGray = lipgloss.Color("#B0B0BB")
	ColorRed       = lipgloss.Color("#E76F51") // Warning / danger
	ColorGreen     = lipgloss.Color("#2A9D8F") // Success / active
	ColorPurple    = lipgloss.Color("#9D4EDD") // Honour Mode purple
	ColorBg        = lipgloss.Color("#121214")

	// Styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Background(ColorDark).
			Padding(0, 1).
			MarginRight(1)

	BadgeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(ColorPurple).
			Padding(0, 1)

	TabActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(ColorPrimary).
			Padding(0, 2)

	TabInactiveStyle = lipgloss.NewStyle().
				Foreground(ColorGray).
				Padding(0, 2)

	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorGray).
			Padding(1, 2)

	ActiveBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 2)

	WarningBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(ColorRed).
			Foreground(ColorRed).
			Padding(1, 2)

	SuccessBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorGreen).
			Foreground(ColorGreen).
			Padding(1, 2)

	CountdownStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSecondary)

	HeaderLabel = lipgloss.NewStyle().
			Foreground(ColorGray).
			Width(16)

	HeaderVal = lipgloss.NewStyle().
			Foreground(ColorLightGray)

	ListItemSelected = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary).
				Background(lipgloss.Color("#2D281E")).
				Padding(0, 1)

	ListItemNormal = lipgloss.NewStyle().
			Foreground(ColorLightGray).
			Padding(0, 1)

	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorGray).
			Italic(true)
)
