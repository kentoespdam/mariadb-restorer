// Package tuiprogress provides the live progress monitor screen.
package tuiprogress

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
	"github.com/kentoespdam/mariadb-restorer/internal/tui/demo"
	restoreengine "github.com/kentoespdam/mariadb-restorer/internal/restore-engine"
)

// ProgressMsg wraps a restore progress event.
type ProgressMsg struct {
	ByteOffset     int64
	DumpSizeBytes  int64
	StatementsDone int64
	BatchCount     int64
	DeferredCount  int
	Elapsed        time.Duration
	Done           bool
	Err            error
}

// RestoreCompleteMsg signals the restore finished.
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
	eventCh       chan restoreengine.ProgressEvent
	cancel        context.CancelFunc
}

// New creates a progress screen for a restore.
func New(bytesTotal int64) *Screen {
	return &Screen{
		bytesTotal: bytesTotal,
		startTime:  time.Now(),
		fastMode:   true,
	}
}

// StartRestore configures the screen for real restore events.
// Returns a cancellable context that should be passed to RunRestoreAsync.
func (s *Screen) StartRestore(ctx context.Context, ch chan restoreengine.ProgressEvent) context.Context {
	s.eventCh = ch
	s.bytesTotal = -1
	if ctx != nil {
		cancelCtx, cancel := context.WithCancel(ctx)
		s.cancel = cancel
		return cancelCtx
	}
	return ctx
}

func (s *Screen) ID() base.ScreenID  { return base.ScreenProgress }
func (s *Screen) Title() string      { return "⏳ Restore in Progress" }

func (s *Screen) Footer() []base.FooterHint {
	if s.done || s.err != "" {
		return []base.FooterHint{{Key: "Enter", Desc: "view report"}}
	}
	return []base.FooterHint{{Key: "Ctrl-C", Desc: "interrupt"}}
}

func (s *Screen) Init() tea.Cmd {
	if s.demoTicks != nil {
		return s.demoNextTick()
	}
	if s.eventCh != nil {
		return s.nextEventCmd()
	}
	return nil
}

func (s *Screen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ProgressMsg:
		return s.handleProgress(msg)
	case RestoreCompleteMsg:
		s.done = true
		if msg.Err != nil {
			s.err = msg.Err.Error()
		}
		return s, nil
	case tea.KeyMsg:
		return s.handleKey(msg)
	}
	return s, nil
}

func (s *Screen) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		s.signalCount++
		if s.cancel != nil {
			s.cancel()
		}
		return s, func() tea.Msg {
			err := fmt.Errorf("interrupted by user (SIGINT)")
			if s.signalCount >= 2 {
				err = fmt.Errorf("aborted by user (SIGINT ×2)")
			}
			return RestoreCompleteMsg{
				ExitCode: 130, Err: err,
				Statements: s.statements, BytesDone: s.bytesDone,
				BytesTotal: s.bytesTotal, BatchCount: s.batchCount,
				DeferredCount: s.deferredCount, Elapsed: time.Since(s.startTime),
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
	return s, nil
}

func (s *Screen) emitComplete() tea.Cmd {
	return func() tea.Msg {
		var err error
		exitCode := 0
		if s.err != "" {
			exitCode = 1
			err = fmt.Errorf("%s", s.err)
		}
		return RestoreCompleteMsg{ExitCode: exitCode, Err: err,
			Statements: s.statements, BytesDone: s.bytesDone,
			BytesTotal: s.bytesTotal, BatchCount: s.batchCount,
			DeferredCount: s.deferredCount, Elapsed: time.Since(s.startTime)}
	}
}
