// Package base provides shared types for the TUI screens.
// It is imported by the router AND all screen packages, so it MUST NOT
// import any screen or router package.
package base

import tea "github.com/charmbracelet/bubbletea"

// FooterHint describes a single keyboard shortcut shown in the footer bar.
type FooterHint struct {
	Key  string
	Desc string
}

// ScreenID identifies the active screen type for router dispatch.
type ScreenID int

const (
	ScreenHome      ScreenID = iota
	ScreenProfiles
	ScreenEditor
	ScreenLauncher
	ScreenProgress
	ScreenReport
	ScreenHelp
	ScreenGlossary
)

// Screen is a navigable Bubble Tea screen.
type Screen interface {
	tea.Model
	ID() ScreenID
	Footer() []FooterHint
	Title() string
}

// NavigationMsg types — emitted by screens, handled by Router.
type NavigateToMsg struct {
	Screen Screen
}
type NavigateBackMsg struct{}
type ShowErrorMsg struct{ Err error }
