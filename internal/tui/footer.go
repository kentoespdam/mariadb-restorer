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
	return base.StyleDim.Copy().Width(width).Render(line)
}

// GlobalShortcuts returns shortcuts available on every screen.
func GlobalShortcuts() []base.FooterHint {
	return []base.FooterHint{
		{Key: "q", Desc: "quit"},
		{Key: "?", Desc: "help"},
		{Key: "g", Desc: "glossary"},
	}
}

// Screen-specific shortcut sets.
func HomeShortcuts() []base.FooterHint {
	return []base.FooterHint{
		{Key: "↑/↓", Desc: "navigate"},
		{Key: "p", Desc: "profiles"},
		{Key: "r", Desc: "new restore"},
		{Key: "d", Desc: "delete"},
	}
}

func ProfileListShortcuts() []base.FooterHint {
	return []base.FooterHint{
		{Key: "↑/↓", Desc: "navigate"},
		{Key: "Enter", Desc: "edit"},
		{Key: "n", Desc: "new"},
		{Key: "/", Desc: "search"},
	}
}

func EditorShortcuts() []base.FooterHint {
	return []base.FooterHint{
		{Key: "Tab", Desc: "next field"},
		{Key: "Enter", Desc: "save"},
		{Key: "Esc", Desc: "back"},
		{Key: "s", Desc: "set password"},
	}
}

func LauncherShortcuts() []base.FooterHint {
	return []base.FooterHint{
		{Key: "n", Desc: "next step"},
		{Key: "b", Desc: "back"},
		{Key: "Esc", Desc: "cancel"},
	}
}

func ProgressShortcuts() []base.FooterHint {
	return []base.FooterHint{
		{Key: "Ctrl-C", Desc: "interrupt (graceful)"},
	}
}

func ReportShortcuts() []base.FooterHint {
	return []base.FooterHint{
		{Key: "Esc", Desc: "back to Home"},
		{Key: "r", Desc: "resume"},
		{Key: "p", Desc: "replay"},
	}
}
