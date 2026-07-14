package tuiprogress

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kentoespdam/mariadb-restorer/internal/tui/demo"
)

// DemoSimulate schedules simulated progress ticks for demo mode.
// The progress ticks are emitted via the Bubble Tea command system.
func (s *Screen) DemoSimulate() {
	s.demoTicks = demo.ProgressSequence(s.bytesTotal)
	s.demoIdx = 0
}

// demoNextTick returns a command that sends the next simulated progress event.
func (s *Screen) demoNextTick() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(800 * time.Millisecond)
		if s.demoIdx >= len(s.demoTicks) {
			return RestoreCompleteMsg{
				ExitCode:      0,
				Statements:    s.statements,
				BytesDone:     s.bytesDone,
				BytesTotal:    s.bytesTotal,
				BatchCount:    s.batchCount,
				DeferredCount: s.deferredCount,
				Elapsed:       time.Since(s.startTime),
			}
		}
		tick := s.demoTicks[s.demoIdx]
		s.demoIdx++
		return ProgressMsg{
			ByteOffset:     tick.ByteOffset,
			DumpSizeBytes:  s.bytesTotal,
			StatementsDone: tick.StatementsDone,
			BatchCount:     tick.BatchCount,
		}
	}
}
