// Package tuiprogress provides the live progress monitor screen.
package tuiprogress

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
	"github.com/kentoespdam/mariadb-restorer/internal/tui/demo"
)

// ProgressMsg wraps a restore progress event.
type ProgressMsg struct {
	ByteOffset     int64
	DumpSizeBytes  int64
	StatementsDone int64
	BatchCount     int64
	DeferredCount  int
	Done           bool
	Err            error
}

// RestoreCompleteMsg signals the restore finished. Router transitions to report screen.
type RestoreCompleteMsg struct {
	ExitCode      int
	Err           error
	Statements    int64
	BytesDone     int64
	BytesTotal    int64
	BatchCount    int64
	DeferredCount int
	Elapsed       time.Duration
	DeferredDescs []string
}

// Screen displays live restore progress.
type Screen struct {
	bytesDone     int64
	bytesTotal    int64
	statements    int64
	batchCount    int64
	deferredCount int
	startTime     time.Time
	err           string
	done          bool
	signalCount   int
	fastMode      bool
	demoTicks     []demo.DemoProgressTick
	demoIdx       int
}

// New creates a progress screen for a restore.
func New(bytesTotal int64) *Screen {
	return &Screen{
		bytesTotal: bytesTotal,
		startTime:  time.Now(),
		fastMode:   true,
	}
}

func (s *Screen) ID() base.ScreenID  { return base.ScreenProgress }
func (s *Screen) Title() string      { return "⏳ Restore in Progress" }

func (s *Screen) Footer() []base.FooterHint {
	if s.done || s.err != "" {
		return []base.FooterHint{{Key: "Enter", Desc: "view report"}}
	}
	return []base.FooterHint{{Key: "Ctrl-C", Desc: "interrupt (graceful drain)"}}
}

func (s *Screen) Init() tea.Cmd {
	if s.demoTicks != nil {
		return s.demoNextTick()
	}
	return nil
}

func (s *Screen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ProgressMsg:
		s.bytesDone = msg.ByteOffset
		s.statements = msg.StatementsDone
		s.batchCount = msg.BatchCount
		s.deferredCount = msg.DeferredCount
		s.done = msg.Done
		if msg.Err != nil {
			s.err = msg.Err.Error()
			s.done = true
		}
		// In demo mode, schedule next tick.
		if s.demoTicks != nil && !s.done {
			return s, s.demoNextTick()
		}
		return s, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			s.signalCount++
			return s, func() tea.Msg {
				exitCode := 130
				var err error
				if s.signalCount >= 2 {
					err = fmt.Errorf("aborted by user (SIGINT ×2)")
				} else {
					err = fmt.Errorf("interrupted by user (SIGINT)")
				}
				return RestoreCompleteMsg{
					ExitCode:      exitCode,
					Err:           err,
					Statements:    s.statements,
					BytesDone:     s.bytesDone,
					BytesTotal:    s.bytesTotal,
					BatchCount:    s.batchCount,
					DeferredCount: s.deferredCount,
					Elapsed:       time.Since(s.startTime),
				}
			}
		case "enter":
			if s.done || s.err != "" {
				return s, s.emitComplete()
			}
		default:
			if s.done || s.err != "" {
				return s, s.emitComplete()
			}
		}
	}
	return s, nil
}

func (s *Screen) emitComplete() tea.Cmd {
	return func() tea.Msg {
		exitCode := 0
		var err error
		if s.err != "" {
			exitCode = 1
			err = fmt.Errorf("%s", s.err)
		}
		return RestoreCompleteMsg{
			ExitCode:      exitCode,
			Err:           err,
			Statements:    s.statements,
			BytesDone:     s.bytesDone,
			BytesTotal:    s.bytesTotal,
			BatchCount:    s.batchCount,
			DeferredCount: s.deferredCount,
			Elapsed:       time.Since(s.startTime),
		}
	}
}
