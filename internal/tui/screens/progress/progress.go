// Package tuiprogress provides the live progress monitor screen.
package tuiprogress

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
)

// Screen displays live restore progress.
type Screen struct {
	bytesDone     int64
	bytesTotal    int64
	statements    int64
	batchCount    int64
	deferredCount int
	err           string
	done          bool
}

// New creates a progress screen for a restore.
func New(bytesTotal int64) *Screen {
	return &Screen{bytesTotal: bytesTotal}
}

func (s *Screen) ID() base.ScreenID         { return base.ScreenProgress }
func (s *Screen) Title() string             { return "⏳ Restore in Progress" }
func (s *Screen) Footer() []base.FooterHint {
	return []base.FooterHint{
		{Key: "Ctrl-C", Desc: "interrupt (graceful)"},
	}
}

func (s *Screen) Init() tea.Cmd { return nil }

func (s *Screen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return s, func() tea.Msg { return base.NavigateBackMsg{} }
		}
	}
	return s, nil
}

func (s *Screen) View() string {
	var b strings.Builder

	if s.err != "" {
		b.WriteString(base.StyleError.Render(" ❌ Error: "+s.err) + "\n\n")
		b.WriteString(base.StyleDim.Render(" The restore can be resumed later from Home."))
		return b.String()
	}

	if s.done {
		b.WriteString(base.StyleSuccess.Render(" ✔ Restore Complete!") + "\n\n")
		b.WriteString(fmt.Sprintf(" Statements: %d\n", s.statements))
		b.WriteString(fmt.Sprintf(" Batches:    %d\n", s.batchCount))
		if s.deferredCount > 0 {
			b.WriteString(fmt.Sprintf(" Deferred:   %d objects\n", s.deferredCount))
		}
		b.WriteString(base.StyleDim.Render("\n Press any key to return to Home."))
		return b.String()
	}

	// Progress bar.
	percent := float64(0)
	if s.bytesTotal > 0 {
		percent = float64(s.bytesDone) / float64(s.bytesTotal) * 100
	}
	barWidth := 40
	filled := int(percent / 100 * float64(barWidth))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	b.WriteString(fmt.Sprintf(" %s\n", bar))
	b.WriteString(fmt.Sprintf(" %.1f%% — %d / %d bytes\n\n", percent, s.bytesDone, s.bytesTotal))
	b.WriteString(fmt.Sprintf(" Statements: %d\n", s.statements))
	b.WriteString(fmt.Sprintf(" Batches:    %d\n", s.batchCount))
	if s.deferredCount > 0 {
		b.WriteString(fmt.Sprintf(" Deferred:   %d\n", s.deferredCount))
	}
	b.WriteString(base.StyleDim.Render("\n Ctrl-C to interrupt (drain current batch)."))

	return b.String()
}
