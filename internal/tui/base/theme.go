package base

import "github.com/charmbracelet/lipgloss"

// ANSI color palette — works on dark AND light terminals.
var (
	ColorGreen   = lipgloss.Color("2")
	ColorRed     = lipgloss.Color("1")
	ColorYellow  = lipgloss.Color("3")
	ColorBlue    = lipgloss.Color("4")
	ColorMagenta = lipgloss.Color("5")
	ColorWhite   = lipgloss.Color("7")
	ColorGray    = lipgloss.Color("8")
	ColorCyan    = lipgloss.Color("6")
)

// Reusable Lip Gloss styles.
var (
	StyleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorWhite).
			Padding(0, 1)

	StyleHighlight = lipgloss.NewStyle().
			Foreground(ColorBlue).
			Bold(true)

	StyleSuccess = lipgloss.NewStyle().
			Foreground(ColorGreen).
			Bold(true)

	StyleError = lipgloss.NewStyle().
			Foreground(ColorRed).
			Bold(true)

	StyleWarning = lipgloss.NewStyle().
			Foreground(ColorYellow)

	StyleDim = lipgloss.NewStyle().
			Foreground(ColorGray)

	StyleStatusBar = lipgloss.NewStyle().
			Background(ColorBlue).
			Foreground(ColorWhite).
			Padding(0, 1)

	StyleSelected = lipgloss.NewStyle().
			Foreground(ColorCyan).
			Bold(true)

	StyleVaulted = lipgloss.NewStyle().
			Foreground(ColorYellow).
			Italic(true)

	StyleDataDir = lipgloss.NewStyle().
			Foreground(ColorGray).
			Italic(true)

	StyleHelpKey = lipgloss.NewStyle().
			Foreground(ColorCyan).
			Bold(true)

	StyleHelpDesc = lipgloss.NewStyle().
			Foreground(ColorWhite)
)
