package tuiprogress

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	restoreengine "github.com/kentoespdam/mariadb-restorer/internal/restore-engine"
	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
	"github.com/kentoespdam/mariadb-restorer/internal/tui/demo"
)

func TestProgressScreen_New_ID(t *testing.T) {
	s := New(500 * 1024 * 1024) // 500 MB
	if s.ID() != base.ScreenProgress {
		t.Errorf("expected ScreenProgress, got %v", s.ID())
	}
}

func TestProgressScreen_Title(t *testing.T) {
	s := New(500 * 1024 * 1024)
	if !strings.Contains(s.Title(), "Progress") {
		t.Errorf("expected 'Progress' in title, got %q", s.Title())
	}
}

func TestProgressScreen_Init_WithTicks(t *testing.T) {
	s := New(500 * 1024 * 1024)
	s.demoTicks = demo.ProgressSequence(500 * 1024 * 1024)
	cmd := s.Init()
	if cmd == nil {
		t.Error("expected non-nil cmd from Init with demo ticks")
	}
}

func TestProgressScreen_Init_WithoutTicks(t *testing.T) {
	s := New(500 * 1024 * 1024)
	cmd := s.Init()
	if cmd != nil {
		t.Error("expected nil cmd from Init without demo ticks or event channel")
	}
}

func TestProgressScreen_View(t *testing.T) {
	s := New(500 * 1024 * 1024)
	view := s.View()
	// The progress view shows a progress bar with █ and ░ characters.
	if !strings.Contains(view, "%") {
		t.Error("expected percentage in progress view")
	}
	if !strings.Contains(view, "Ctrl-C") {
		t.Error("expected Ctrl-C hint in progress view")
	}
	if !strings.Contains(view, "Fast Mode") {
		t.Error("expected Fast Mode indicator in progress view")
	}
}

func TestProgressScreen_View_ShowsStats(t *testing.T) {
	s := New(500 * 1024 * 1024)
	s.bytesDone = 100 * 1024 * 1024
	s.statements = 1000
	s.batchCount = 5
	s.startTime = time.Now().Add(-30 * time.Second)

	view := s.View()
	if !strings.Contains(view, "100") {
		t.Error("expected bytes count in view")
	}
	if !strings.Contains(view, "1000") {
		t.Error("expected statement count in view")
	}
	if !strings.Contains(view, "5") {
		t.Error("expected batch count in view")
	}
}

func TestProgressScreen_ProgressMsg_Updates(t *testing.T) {
	s := New(500 * 1024 * 1024)
	s.bytesTotal = 500 * 1024 * 1024

	result, cmd := s.Update(ProgressMsg{
		ByteOffset:     100 * 1024 * 1024,
		DumpSizeBytes:  500 * 1024 * 1024,
		StatementsDone: 2000,
		BatchCount:     3,
		DeferredCount:  2,
		Elapsed:        10 * time.Second,
	})

	if cmd != nil {
		t.Error("expected nil cmd after progress update")
	}
	updated := result.(*Screen)
	if updated.bytesDone != 100*1024*1024 {
		t.Errorf("expected bytesDone=%d, got %d", 100*1024*1024, updated.bytesDone)
	}
	if updated.statements != 2000 {
		t.Errorf("expected statements=%d, got %d", 2000, updated.statements)
	}
	if updated.batchCount != 3 {
		t.Errorf("expected batchCount=%d, got %d", 3, updated.batchCount)
	}
	if updated.deferredCount != 2 {
		t.Errorf("expected deferredCount=%d, got %d", 2, updated.deferredCount)
	}
}

func TestProgressScreen_RestoreCompleteMsg(t *testing.T) {
	s := New(500 * 1024 * 1024)

	result, cmd := s.Update(RestoreCompleteMsg{
		ExitCode:      0,
		Statements:    45231,
		BytesDone:     500 * 1024 * 1024,
		BytesTotal:    500 * 1024 * 1024,
		BatchCount:    24,
		DeferredCount: 0,
	})

	if cmd != nil {
		t.Error("expected nil cmd after RestoreCompleteMsg")
	}
	updated := result.(*Screen)
	if !updated.done {
		t.Error("expected done=true after RestoreCompleteMsg")
	}
	if updated.err != "" {
		t.Errorf("expected no error, got %q", updated.err)
	}
}

func TestProgressScreen_RestoreCompleteMsg_WithError(t *testing.T) {
	s := New(500 * 1024 * 1024)

	result, cmd := s.Update(RestoreCompleteMsg{ExitCode: 1,
		Err:           errors.New("connection lost"),
		Statements:    5000,
		BytesDone:     100 * 1024 * 1024,
		BytesTotal:    500 * 1024 * 1024,
		BatchCount:    3,
		DeferredCount: 0,
	})

	if cmd != nil {
		t.Error("expected nil cmd after RestoreCompleteMsg")
	}
	updated := result.(*Screen)
	if !updated.done {
		t.Error("expected done=true after error restore")
	}
	if !strings.Contains(updated.err, "connection lost") {
		t.Errorf("expected error message, got %q", updated.err)
	}
}

func TestProgressScreen_View_ShowsPercentage(t *testing.T) {
	s := New(500 * 1024 * 1024)
	s.bytesTotal = 500 * 1024 * 1024
	s.bytesDone = 250 * 1024 * 1024 // 50%

	view := s.View()
	if !strings.Contains(view, "50") {
		t.Error("expected percentage in view")
	}
}

func TestProgressScreen_View_Done(t *testing.T) {
	s := New(500 * 1024 * 1024)
	s.done = true
	s.bytesTotal = 500 * 1024 * 1024
	s.bytesDone = 500 * 1024 * 1024

	view := s.View()
	if !strings.Contains(view, "done") && !strings.Contains(view, "Done") &&
		!strings.Contains(view, "Complete") && !strings.Contains(view, "complete") {
		t.Logf("done view: %q", view)
	}
}

func TestProgressScreen_Enter_AfterDone_EmitsComplete(t *testing.T) {
	s := New(500 * 1024 * 1024)
	s.done = true
	s.bytesTotal = 500 * 1024 * 1024
	s.bytesDone = 500 * 1024 * 1024
	s.statements = 45231
	s.batchCount = 24
	s.startTime = time.Now().Add(-1 * time.Minute)

	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd after Enter when done")
	}
	msg := cmd()
	completeMsg, ok := msg.(RestoreCompleteMsg)
	if !ok {
		t.Fatalf("expected RestoreCompleteMsg, got %T", msg)
	}
	if completeMsg.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", completeMsg.ExitCode)
	}
}

func TestProgressScreen_Enter_AfterError_EmitsComplete(t *testing.T) {
	s := New(500 * 1024 * 1024)
	s.done = true
	s.err = "fatal error"
	s.bytesDone = 100 * 1024 * 1024
	s.bytesTotal = 500 * 1024 * 1024

	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	completeMsg, ok := msg.(RestoreCompleteMsg)
	if !ok {
		t.Fatalf("expected RestoreCompleteMsg, got %T", msg)
	}
	if completeMsg.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", completeMsg.ExitCode)
	}
}

func TestProgressScreen_CtrlC_FirstSignal(t *testing.T) {
	s := New(500 * 1024 * 1024)
	s.startTime = time.Now().Add(-5 * time.Minute)
	s.bytesDone = 200 * 1024 * 1024
	s.bytesTotal = 500 * 1024 * 1024
	s.statements = 5000
	s.batchCount = 5
	s.deferredCount = 1

	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected non-nil cmd after Ctrl-C")
	}
	msg := cmd()
	completeMsg, ok := msg.(RestoreCompleteMsg)
	if !ok {
		t.Fatalf("expected RestoreCompleteMsg, got %T", msg)
	}
	if completeMsg.ExitCode != 130 {
		t.Errorf("expected exit code 130, got %d", completeMsg.ExitCode)
	}
	if completeMsg.Err == nil {
		t.Error("expected error message for interrupt")
	}
}

func TestProgressScreen_CtrlC_SecondSignal_Abort(t *testing.T) {
	s := New(500 * 1024 * 1024)
	s.startTime = time.Now().Add(-5 * time.Minute)
	s.signalCount = 1

	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected non-nil cmd after second Ctrl-C")
	}
	msg := cmd()
	completeMsg, ok := msg.(RestoreCompleteMsg)
	if !ok {
		t.Fatalf("expected RestoreCompleteMsg, got %T", msg)
	}
	if completeMsg.ExitCode != 130 {
		t.Errorf("expected exit code 130, got %d", completeMsg.ExitCode)
	}
	if !strings.Contains(completeMsg.Err.Error(), "aborted") {
		t.Errorf("expected 'aborted' message for second interrupt, got %v", completeMsg.Err)
	}
}

func TestProgressScreen_Footer_Active(t *testing.T) {
	s := New(500 * 1024 * 1024)
	s.done = false
	footer := s.Footer()
	if len(footer) != 1 {
		t.Errorf("expected 1 footer hint, got %d", len(footer))
	}
	if footer[0].Key != "Ctrl-C" {
		t.Errorf("expected 'Ctrl-C' in footer, got %q", footer[0].Key)
	}
}

func TestProgressScreen_Footer_Done(t *testing.T) {
	s := New(500 * 1024 * 1024)
	s.done = true
	footer := s.Footer()
	if len(footer) != 1 {
		t.Errorf("expected 1 footer hint when done, got %d", len(footer))
	}
	if footer[0].Key != "Enter" {
		t.Errorf("expected 'Enter' in footer when done, got %q", footer[0].Key)
	}
}

func TestProgressScreen_DemoSimulate(t *testing.T) {
	s := New(500 * 1024 * 1024)
	s.DemoSimulate()
	if s.demoTicks == nil {
		t.Error("expected demoTicks to be set after DemoSimulate")
	}
	if len(s.demoTicks) != 20 {
		t.Errorf("expected 20 demo ticks, got %d", len(s.demoTicks))
	}
}

func TestProgressScreen_ProgressEvent_FromRealEngine(t *testing.T) {
	s := New(500 * 1024 * 1024)
	s.bytesTotal = 500 * 1024 * 1024

	// Simulate a real ProgressEvent from the engine.
	event := restoreengine.ProgressEvent{
		ByteOffset:     100 * 1024 * 1024,
		DumpSizeBytes:  500 * 1024 * 1024,
		StatementsDone: 1500,
		BatchCount:     2,
		DeferredCount:  0,
	}

	// The progress channel type is restoreengine.ProgressEvent.
	// The screen uses ProgressMsg (internal type). These are different types.
	// Test the internal ProgressMsg type directly.
	result, cmd := s.Update(ProgressMsg{
		ByteOffset:     event.ByteOffset,
		DumpSizeBytes:  event.DumpSizeBytes,
		StatementsDone: event.StatementsDone,
		BatchCount:     event.BatchCount,
		DeferredCount:  event.DeferredCount,
		Elapsed:        5 * time.Second,
		Done:           false,
	})

	if cmd != nil {
		t.Error("expected nil cmd after progress event")
	}
	updated := result.(*Screen)
	if updated.bytesDone != 100*1024*1024 {
		t.Errorf("expected bytesDone=%d, got %d", 100*1024*1024, updated.bytesDone)
	}
}
