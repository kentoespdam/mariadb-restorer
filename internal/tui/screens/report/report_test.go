package tuireport

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
)

// Exit code decoding tests.

func TestDecodeExitCode_0_Success(t *testing.T) {
	info := DecodeExitCode(0)
	if info.Code != 0 {
		t.Errorf("expected code 0, got %d", info.Code)
	}
	if info.Label != "Clean Success" {
		t.Errorf("expected 'Clean Success', got %q", info.Label)
	}
	if info.Resumable {
		t.Error("expected resumable=false for exit code 0")
	}
	if info.HasDeferred {
		t.Error("expected deferred=false for exit code 0")
	}
	if info.HasVerify {
		t.Error("expected verify=false for exit code 0")
	}
}

func TestDecodeExitCode_1_Fatal(t *testing.T) {
	info := DecodeExitCode(1)
	if info.Code != 1 {
		t.Errorf("expected code 1, got %d", info.Code)
	}
	if info.Label != "Fatal Error" {
		t.Errorf("expected 'Fatal Error', got %q", info.Label)
	}
	if !info.Resumable {
		t.Error("expected resumable=true for exit code 1")
	}
}

func TestDecodeExitCode_3_Deferred(t *testing.T) {
	info := DecodeExitCode(3)
	if info.Code != 3 {
		t.Errorf("expected code 3, got %d", info.Code)
	}
	if info.Label != "Deferred Objects" {
		t.Errorf("expected 'Deferred Objects', got %q", info.Label)
	}
	if !info.HasDeferred {
		t.Error("expected deferred=true for exit code 3")
	}
}

func TestDecodeExitCode_4_Verify(t *testing.T) {
	info := DecodeExitCode(4)
	if info.Code != 4 {
		t.Errorf("expected code 4, got %d", info.Code)
	}
	if info.Label != "Verify Findings" {
		t.Errorf("expected 'Verify Findings', got %q", info.Label)
	}
	if !info.HasVerify {
		t.Error("expected verify=true for exit code 4")
	}
}

func TestDecodeExitCode_130_SIGINT(t *testing.T) {
	info := DecodeExitCode(130)
	if info.Code != 130 {
		t.Errorf("expected code 130, got %d", info.Code)
	}
	if info.Label != "Interrupted (SIGINT)" {
		t.Errorf("expected 'Interrupted (SIGINT)', got %q", info.Label)
	}
	if !info.Resumable {
		t.Error("expected resumable=true for exit code 130")
	}
}

func TestDecodeExitCode_143_SIGTERM(t *testing.T) {
	info := DecodeExitCode(143)
	if info.Code != 143 {
		t.Errorf("expected code 143, got %d", info.Code)
	}
	if info.Label != "Interrupted (SIGTERM)" {
		t.Errorf("expected 'Interrupted (SIGTERM)', got %q", info.Label)
	}
	if !info.Resumable {
		t.Error("expected resumable=true for exit code 143")
	}
}

func TestDecodeExitCode_Unknown(t *testing.T) {
	info := DecodeExitCode(99)
	if info.Code != 99 {
		t.Errorf("expected code 99, got %d", info.Code)
	}
	if info.Label != "Unknown" {
		t.Errorf("expected 'Unknown', got %q", info.Label)
	}
}

// Report screen tests.

func TestReportScreen_ID(t *testing.T) {
	s := New(RestoreSummary{ExitCode: 0})
	if s.ID() != base.ScreenReport {
		t.Errorf("expected ScreenReport, got %v", s.ID())
	}
}

func TestReportScreen_Title_ExitCode0(t *testing.T) {
	s := New(RestoreSummary{ExitCode: 0})
	if !strings.Contains(s.Title(), "Clean Success") {
		t.Errorf("expected 'Clean Success' in title, got %q", s.Title())
	}
}

func TestReportScreen_Title_ExitCode1(t *testing.T) {
	s := New(RestoreSummary{ExitCode: 1})
	if !strings.Contains(s.Title(), "Fatal Error") {
		t.Errorf("expected 'Fatal Error' in title, got %q", s.Title())
	}
}

func TestReportScreen_View_ExitCode0_Success(t *testing.T) {
	s := New(RestoreSummary{
		ExitCode:   0,
		Statements: 45231,
		BytesDone:  5 * 1024 * 1024 * 1024,
		BytesTotal: 5 * 1024 * 1024 * 1024,
		BatchCount: 24,
		Elapsed:    2 * time.Hour,
	})
	view := s.View()

	if !strings.Contains(view, "Clean Success") {
		t.Error("expected 'Clean Success' in view")
	}
	if !strings.Contains(view, "45231") {
		t.Error("expected statement count in view")
	}
	if !strings.Contains(view, "5.0 GB") {
		t.Error("expected '5.0 GB' in view")
	}
	if !strings.Contains(view, "24") {
		t.Error("expected batch count in view")
	}
	if !strings.Contains(view, "2h") {
		t.Error("expected elapsed time in view")
	}
}

func TestReportScreen_View_ExitCode1_Error(t *testing.T) {
	s := New(RestoreSummary{
		ExitCode:   1,
		Err:        base.ShowErrorMsg{Err: nil}.Err,
		Statements: 12894,
		BytesDone:  300 * 1024 * 1024,
		BytesTotal: 500 * 1024 * 1024,
		BatchCount: 8,
		Elapsed:    45 * time.Minute,
	})
	view := s.View()

	if !strings.Contains(view, "Fatal Error") {
		t.Error("expected 'Fatal Error' in view")
	}
	if !strings.Contains(view, "300.0 MB") {
		t.Error("expected bytes processed in view")
	}
	if !strings.Contains(view, "45m") {
		t.Error("expected elapsed time in view")
	}
}

func TestReportScreen_View_ExitCode3_Deferred(t *testing.T) {
	s := New(RestoreSummary{
		ExitCode:      3,
		Statements:    45231,
		BytesDone:     5 * 1024 * 1024 * 1024,
		BytesTotal:    5 * 1024 * 1024 * 1024,
		BatchCount:    24,
		DeferredCount: 3,
		DeferredDescs: []string{"view_v1", "trigger_t1", "proc_p1"},
	})
	view := s.View()

	if !strings.Contains(view, "Deferred Objects") {
		t.Error("expected 'Deferred Objects' in view")
	}
	if !strings.Contains(view, "3 objects") {
		t.Error("expected deferred count in view")
	}
	if !strings.Contains(view, "view_v1") {
		t.Error("expected deferred object description in view")
	}
}

func TestReportScreen_View_ExitCode4_Verify(t *testing.T) {
	s := New(RestoreSummary{
		ExitCode:   4,
		Statements: 45231,
		BytesDone:  5 * 1024 * 1024 * 1024,
		BytesTotal: 5 * 1024 * 1024 * 1024,
		BatchCount: 24,
		Elapsed:    3 * time.Hour,
	})
	view := s.View()

	if !strings.Contains(view, "Verify Findings") {
		t.Error("expected 'Verify Findings' in view")
	}
}

func TestReportScreen_View_ExitCode130_Interrupted(t *testing.T) {
	s := New(RestoreSummary{
		ExitCode:   130,
		Statements: 12894,
		BytesDone:  300 * 1024 * 1024,
		BytesTotal: 500 * 1024 * 1024,
		BatchCount: 8,
		Elapsed:    10 * time.Minute,
	})
	view := s.View()

	if !strings.Contains(view, "Interrupted") {
		t.Error("expected 'Interrupted' in view")
	}
	if !strings.Contains(view, "Resume") {
		t.Error("expected resume action in view for exit code 130")
	}
}

func TestReportScreen_Init_Nil(t *testing.T) {
	s := New(RestoreSummary{ExitCode: 0})
	cmd := s.Init()
	if cmd != nil {
		t.Error("expected nil cmd from Init")
	}
}

func TestReportScreen_Esc_NavigatesBack(t *testing.T) {
	s := New(RestoreSummary{ExitCode: 0})
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(base.NavigateBackMsg); !ok {
		t.Errorf("expected NavigateBackMsg, got %T", msg)
	}
}

func TestReportScreen_H_NavigatesBack(t *testing.T) {
	s := New(RestoreSummary{ExitCode: 0})
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(base.NavigateBackMsg); !ok {
		t.Errorf("expected NavigateBackMsg, got %T", msg)
	}
}

func TestReportScreen_R_ResumeReturnsError(t *testing.T) {
	s := New(RestoreSummary{ExitCode: 130, BytesDone: 300, BytesTotal: 500})
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Fatal("expected non-nil cmd for resume")
	}
	msg := cmd()
	if _, ok := msg.(base.ShowErrorMsg); !ok {
		t.Errorf("expected ShowErrorMsg for resume, got %T", msg)
	}
}

func TestReportScreen_P_ReplayReturnsError(t *testing.T) {
	s := New(RestoreSummary{ExitCode: 3, DeferredCount: 3})
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if cmd == nil {
		t.Fatal("expected non-nil cmd for replay")
	}
	msg := cmd()
	if _, ok := msg.(base.ShowErrorMsg); !ok {
		t.Errorf("expected ShowErrorMsg for replay, got %T", msg)
	}
}

func TestReportScreen_Footer_ExitCode0(t *testing.T) {
	s := New(RestoreSummary{ExitCode: 0})
	footer := s.Footer()
	if len(footer) != 3 {
		t.Errorf("expected 3 footer hints for exit code 0, got %d", len(footer))
	}
	if footer[0].Key != "Esc" {
		t.Errorf("expected Esc in footer, got %q", footer[0].Key)
	}
}

func TestReportScreen_Footer_ExitCode1_Resumable(t *testing.T) {
	s := New(RestoreSummary{ExitCode: 1})
	footer := s.Footer()
	if len(footer) != 4 {
		t.Errorf("expected 4 footer hints for exit code 1, got %d", len(footer))
	}
	if footer[3].Key != "r" {
		t.Errorf("expected 'r' hint for resume, got %q", footer[3].Key)
	}
}

func TestReportScreen_Footer_ExitCode3_Deferred(t *testing.T) {
	s := New(RestoreSummary{ExitCode: 3, DeferredCount: 5})
	footer := s.Footer()
	if len(footer) != 4 {
		t.Errorf("expected 4 footer hints for exit code 3, got %d", len(footer))
	}
	if footer[3].Key != "p" {
		t.Errorf("expected 'p' hint for replay, got %q", footer[3].Key)
	}
}
