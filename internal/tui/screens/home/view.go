package tuihome

import (
	"fmt"
	"strings"

	restoreengine "github.com/kentoespdam/mariadb-restorer/internal/restore-engine"
	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
)

func (s *Screen) View() string {
	if s.loading {
		return base.StyleDim.Render("Loading restore history...")
	}
	if s.err != nil {
		return base.StyleError.Render("Error: " + s.err.Error())
	}
	if len(s.checkpoints) == 0 {
		return base.StyleDim.Render("No restore history found.\n\nPress 'r' to start a new restore, or 'p' to manage profiles.")
	}

	var b strings.Builder
	b.WriteString(base.StyleHighlight.Render(
		fmt.Sprintf(" %d restore(s) in progress/resumable", len(s.checkpoints)),
	) + "\n\n")

	for i, cp := range s.checkpoints {
		status := statusText(cp)
		prefix := " "
		if i == s.selected {
			prefix = "▸"
		}
		line := fmt.Sprintf(" %s [%s] %s", prefix, status, cp.DumpPath)
		if i == s.selected {
			percent := float64(0)
			if cp.DumpSizeBytes > 0 {
				percent = float64(cp.ByteOffset) / float64(cp.DumpSizeBytes) * 100
			}
			idPref := ""
			if len(cp.DumpIdentity) >= 8 {
				idPref = cp.DumpIdentity[:7]
			}
			detail := fmt.Sprintf("\n   %d / %d bytes (%.1f%%) — %d stmts — %s",
				cp.ByteOffset, cp.DumpSizeBytes, percent, cp.StatementsDone,
				cp.UpdatedAt.Format("2006-01-02 15:04"),
			)
			line += fmt.Sprintf("\n   📋 %s%s", idPref, detail)
			b.WriteString(base.StyleSelected.Render(line) + "\n")
		} else {
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

func statusText(cp *restoreengine.Checkpoint) string {
	if cp.ByteOffset >= cp.DumpSizeBytes && cp.DumpSizeBytes > 0 {
		return base.StyleSuccess.Render("✔ Completed")
	}
	return base.StyleWarning.Render("◌ Resumable")
}
