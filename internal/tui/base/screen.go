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
	ScreenNone     ScreenID = iota // 0 = sentinel for "no constraint"
	ScreenHome                     // 1
	ScreenProfiles                 // 2
	ScreenEditor                   // 3
	ScreenLauncher                 // 4
	ScreenProgress                 // 5
	ScreenReport                   // 6
	ScreenHelp                     // 7
	ScreenGlossary                 // 8
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

// NavigateTo returns a tea.Cmd that creates a screen by ID via the registered
// factory and emits a NavigateToMsg for the Router to push onto the stack.
func NavigateTo(id ScreenID, ctx FactoryContext) tea.Cmd {
	return func() tea.Msg {
		sc, ok := CreateScreen(id, ctx)
		if !ok {
			return nil
		}
		return NavigateToMsg{Screen: sc}
	}
}
