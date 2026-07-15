package tuiprogress

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// nextEventCmd returns a command that reads one event from the restore channel.
func (s *Screen) nextEventCmd() tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-s.eventCh
		if !ok {
			return RestoreCompleteMsg{
				ExitCode:      0,
				Statements:    s.statements,
				BytesDone:     s.bytesDone,
				BytesTotal:    s.bytesTotal,
				BatchCount:    s.batchCount,
				DeferredCount: s.deferredCount,
				Elapsed:       time.Since(s.startTime),
				DataDir: s.dataDir, DumpPath: s.dumpPath, DSN: s.dsn,
			}
		}
		if ev.Done {
			exitCode := 0
			var err error
			if ev.Err != nil {
				exitCode = 1
				err = ev.Err
			}
			// Exit code 4 if verify findings exist (only if no fatal error).
			if len(ev.VerifyFindings) > 0 && err == nil {
				exitCode = 4
			}
			return RestoreCompleteMsg{
				ExitCode: exitCode, Err: err,
				Statements: ev.StatementsDone, BytesDone: ev.ByteOffset,
				BytesTotal: ev.DumpSizeBytes, BatchCount: ev.BatchCount,
				DeferredCount: ev.DeferredCount, Elapsed: time.Since(s.startTime),
				VerifyFindings: ev.VerifyFindings,
				DataDir: s.dataDir, DumpPath: s.dumpPath, DSN: s.dsn,
			}
		}
		return ProgressMsg(ev)
	}
}

func (s *Screen) handleProgress(msg ProgressMsg) (tea.Model, tea.Cmd) {
	s.bytesDone = msg.ByteOffset
	if s.bytesTotal <= 0 {
		s.bytesTotal = msg.DumpSizeBytes
	}
	s.statements = msg.StatementsDone
	s.batchCount = msg.BatchCount
	s.deferredCount = msg.DeferredCount
	s.done = msg.Done
	if msg.Err != nil {
		s.err = msg.Err.Error()
		s.done = true
	}
	if s.eventCh != nil && !s.done {
		return s, s.nextEventCmd()
	}
	if s.demoTicks != nil && !s.done {
		return s, s.demoNextTick()
	}
	return s, nil
}
