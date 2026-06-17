package tui

import "github.com/charmbracelet/lipgloss"

const (
	sidebarWidth = 34
	contentWidth = 56
)

var (
	colorOrange = lipgloss.Color("#FF8C00")
	colorWhite  = lipgloss.Color("#EFEFEF")
	colorDim    = lipgloss.Color("#666666")
	colorGreen  = lipgloss.Color("#4CAF50")
	colorRed    = lipgloss.Color("#F44336")
	colorCyan   = lipgloss.Color("#00BCD4")

	appTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorOrange)

	sectionLabelStyle = lipgloss.NewStyle().
				Foreground(colorOrange).
				Bold(true)

	menuItemStyle = lipgloss.NewStyle().
			Foreground(colorWhite).
			PaddingLeft(3)

	menuSelectedStyle = lipgloss.NewStyle().
				Foreground(colorOrange).
				Bold(true).
				PaddingLeft(1)

	sidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorOrange).
			Width(sidebarWidth).
			Padding(1, 1)

	contentStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorOrange).
			Width(contentWidth).
			Padding(1, 2)

	contentTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorOrange)

	dividerStyle = lipgloss.NewStyle().
			Foreground(colorOrange)

	helpStyle = lipgloss.NewStyle().
			Foreground(colorDim).
			MarginTop(0)

	labelStyle = lipgloss.NewStyle().
			Foreground(colorWhite).
			Width(14)

	descStyle = lipgloss.NewStyle().
			Foreground(colorWhite)

	successStyle = lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)

	spinnerStyle = lipgloss.NewStyle().
			Foreground(colorOrange)

	btnFocusStyle = lipgloss.NewStyle().
			Background(colorOrange).
			Foreground(lipgloss.Color("#000000")).
			Bold(true).
			Padding(0, 4)

	btnBlurStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorOrange).
			Foreground(colorOrange).
			Padding(0, 3)

	inputLabelFocusStyle = lipgloss.NewStyle().
				Foreground(colorOrange).
				Bold(true).
				Width(14)
)
