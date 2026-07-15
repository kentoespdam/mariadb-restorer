package tuiprofiles

import (
	"strings"

	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
)

func (e *EditorScreen) View() string {
	var b strings.Builder
	if e.err != "" {
		b.WriteString(base.StyleError.Render(" ⚠ "+e.err) + "\n\n")
	}
	if e.saved {
		return base.StyleSuccess.Render(" ✔ Profile saved!")
	}

	labels := []string{"Name", "Host", "Port", "User", "Database", "Password", "Passphrase"}
	for i, ti := range e.inputs {
		label := labels[i]
		style := base.StyleDim
		if i == int(e.focused) {
			style = base.StyleHighlight
		}
		b.WriteString(style.Render(" "+label+":") + "\n ")
		b.WriteString(ti.View() + "\n\n")
	}

	if e.hasPwd {
		b.WriteString(base.StyleVaulted.Render(" 🔒 Password is vaulted\n\n"))
	}

	b.WriteString(base.StyleDim.Render(" Enter to save • Esc to cancel • Tab to navigate"))
	return b.String()
}
