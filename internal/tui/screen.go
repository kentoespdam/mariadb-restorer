// Package tui provides the interactive terminal UI for mariadb-restorer.
package tui

import "github.com/kentoespdam/mariadb-restorer/internal/tui/base"

// Re-exported types for convenience.
type (
	FooterHint = base.FooterHint
	ScreenID   = base.ScreenID
	Screen     = base.Screen
)

// Re-export navigation messages.
type (
	NavigateToMsg   = base.NavigateToMsg
	NavigateBackMsg = base.NavigateBackMsg
	ShowErrorMsg    = base.ShowErrorMsg
)

// Screen ID constants.
const (
	ScreenNone     = base.ScreenNone
	ScreenHome     = base.ScreenHome
	ScreenProfiles = base.ScreenProfiles
	ScreenEditor   = base.ScreenEditor
	ScreenLauncher = base.ScreenLauncher
	ScreenProgress = base.ScreenProgress
	ScreenReport   = base.ScreenReport
	ScreenHelp     = base.ScreenHelp
	ScreenGlossary = base.ScreenGlossary
)
