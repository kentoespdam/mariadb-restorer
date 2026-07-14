package tui

import (
	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
	_ "github.com/kentoespdam/mariadb-restorer/internal/tui/screens/help" // trigger factory registration
	_ "github.com/kentoespdam/mariadb-restorer/internal/tui/screens/home"
	_ "github.com/kentoespdam/mariadb-restorer/internal/tui/screens/launcher"
	_ "github.com/kentoespdam/mariadb-restorer/internal/tui/screens/profiles"
	_ "github.com/kentoespdam/mariadb-restorer/internal/tui/screens/progress"
	_ "github.com/kentoespdam/mariadb-restorer/internal/tui/screens/report"
)

// init registers all global keyboard shortcuts.
// Screen factories are registered via init() in each screen package.
func init() {
	base.RegisterShortcut(base.ShortcutInfo{
		Key: "ctrl+c", Desc: "Quit the TUI", Quit: true,
	})
	base.RegisterShortcut(base.ShortcutInfo{
		Key: "q", Desc: "Quit the TUI", Quit: true,
	})
	base.RegisterShortcut(base.ShortcutInfo{
		Key: "esc", Desc: "Go back", Back: true,
	})
	base.RegisterShortcut(base.ShortcutInfo{
		Key: "h", Desc: "Go to Home screen", Home: true,
	})
	base.RegisterShortcut(base.ShortcutInfo{
		Key: "?", Desc: "Help screen", TargetID: base.ScreenHelp,
	})
	base.RegisterShortcut(base.ShortcutInfo{
		Key: "g", Desc: "Glossary screen", TargetID: base.ScreenGlossary,
	})
	base.RegisterShortcut(base.ShortcutInfo{
		Key: "p", Desc: "Profile Manager",
		TargetID: base.ScreenProfiles,
		OnlyOn:   base.ScreenHome,
	})
	base.RegisterShortcut(base.ShortcutInfo{
		Key: "r", Desc: "New restore (Launcher)",
		TargetID: base.ScreenLauncher,
		OnlyOn:   base.ScreenHome,
	})
}
