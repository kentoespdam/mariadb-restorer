package tui

import (
	_ "github.com/kentoespdam/mariadb-restorer/internal/tui/screens/help" // trigger factory registration
	_ "github.com/kentoespdam/mariadb-restorer/internal/tui/screens/home"
	_ "github.com/kentoespdam/mariadb-restorer/internal/tui/screens/launcher"
	_ "github.com/kentoespdam/mariadb-restorer/internal/tui/screens/profiles"
	_ "github.com/kentoespdam/mariadb-restorer/internal/tui/screens/progress"
	_ "github.com/kentoespdam/mariadb-restorer/internal/tui/screens/report"
)

// init imports all screen factory packages so their init() runs first.
