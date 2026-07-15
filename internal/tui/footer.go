package tui

import (
	"strings"

	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
)

// RenderFooter builds a footer hint bar from a list of shortcuts.
func RenderFooter(hints []base.FooterHint, width int) string {
	if len(hints) == 0 {
		return ""
	}
	var parts []string
	for _, h := range hints {
		key := base.StyleHelpKey.Render(h.Key)
		desc := base.StyleHelpDesc.Render(h.Desc)
		parts = append(parts, key+": "+desc)
	}
	line := strings.Join(parts, "  •  ")
	return base.StyleDim.Width(width).Render(line)
}

// GlobalShortcuts returns shortcuts available on every screen.
// Only truly universal control-key shortcuts live here.
func GlobalShortcuts() []base.FooterHint {
	return []base.FooterHint{
		{Key: "Ctrl-Q", Desc: "quit"},
	}
}
