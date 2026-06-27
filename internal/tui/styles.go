package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorBorder      = lipgloss.Color("240")
	colorActiveBorder = lipgloss.Color("69")
	colorSelected    = lipgloss.Color("69")
	colorTitle       = lipgloss.Color("255")
	colorDim         = lipgloss.Color("240")
	colorAdd         = lipgloss.Color("34")
	colorDanger      = lipgloss.Color("196")
	colorModified    = lipgloss.Color("214")

	stylePanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	stylePanelActive = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorActiveBorder).
				Padding(0, 1)

	styleTitle = lipgloss.NewStyle().
			Foreground(colorTitle).
			Bold(true)

	styleTitleActive = lipgloss.NewStyle().
				Foreground(colorActiveBorder).
				Bold(true)

	styleSelected = lipgloss.NewStyle().
			Foreground(colorSelected).
			Bold(true)

	styleDim = lipgloss.NewStyle().
			Foreground(colorDim)

	styleAdd = lipgloss.NewStyle().
			Foreground(colorAdd)

	styleDanger = lipgloss.NewStyle().
			Foreground(colorDanger).
			Bold(true)

	styleModified = lipgloss.NewStyle().
			Foreground(colorModified)

	styleHeader = lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("255")).
			Padding(0, 1)

	styleStatusBar = lipgloss.NewStyle().
			Foreground(colorDim)

	styleKey = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)

	styleKeyLabel = lipgloss.NewStyle().
			Foreground(colorDim)
)
