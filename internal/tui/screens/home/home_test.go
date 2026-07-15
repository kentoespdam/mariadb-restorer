package tuihome

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
	"github.com/kentoespdam/mariadb-restorer/internal/tui/demo"
)

func TestHomeScreen_Init_ReturnsCmd(t *testing.T) {
	s := New("/tmp/test", true)
	cmd := s.Init()
	if cmd == nil {
		t.Error("expected non-nil cmd from Init")
	}
}

func TestHomeScreen_ID(t *testing.T) {
	s := New("/tmp/test", true)
	if s.ID() != base.ScreenHome {
		t.Errorf("expected ScreenHome, got %v", s.ID())
	}
}

func TestHomeScreen_Title(t *testing.T) {
	s := New("/tmp/test", true)
	if !strings.Contains(s.Title(), "Restore") {
		t.Errorf("expected 'Restore' in title, got %q", s.Title())
	}
}

func TestHomeScreen_Init_LoadsDemoCheckpoints(t *testing.T) {
	s := New("/tmp/test", true)
	cmd := s.Init()
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}

	// Execute the returned command.
	msg := cmd()
	cps, ok := msg.(checkpointsLoadedMsg)
	if !ok {
		t.Fatalf("expected checkpointsLoadedMsg, got %T", msg)
	}
	if len(cps) < 1 {
		t.Fatal("expected at least 1 demo checkpoint")
	}
}

func TestHomeScreen_View_Loading(t *testing.T) {
	s := New("/tmp/test", true)
	s.loading = true
	view := s.View()
	if !strings.Contains(view, "Loading") {
		t.Error("expected 'Loading' in view while loading")
	}
}

func TestHomeScreen_View_DemoData(t *testing.T) {
	s := New("/tmp/test", true)
	// Simulate loading demo data.
	s.checkpoints = demo.SyntheticCheckpoints()
	s.loading = false

	view := s.View()
	if !strings.Contains(view, "Completed") {
		t.Error("expected completed status in view")
	}
	if !strings.Contains(view, "Resumable") {
		t.Error("expected resumable status in view")
	}
	if !strings.Contains(view, "/backups/") {
		t.Error("expected dump path in view")
	}
}

func TestHomeScreen_View_SelectedDetail(t *testing.T) {
	s := New("/tmp/test", true)
	s.checkpoints = demo.SyntheticCheckpoints()
	s.loading = false
	s.selected = 0 // first checkpoint selected

	view := s.View()
	if !strings.Contains(view, "a1b2c3d") {
		t.Error("expected identity prefix in selected detail")
	}
}

func TestHomeScreen_Nav_Down(t *testing.T) {
	s := New("/tmp/test", true)
	s.checkpoints = demo.SyntheticCheckpoints()
	s.loading = false
	s.selected = 0

	result, cmd := s.Update(tea.KeyMsg{Type: tea.KeyDown})
	if cmd != nil {
		t.Error("expected nil cmd for navigation")
	}
	updated := result.(*Screen)
	if updated.selected != 1 {
		t.Errorf("expected selected=1, got %d", updated.selected)
	}
}

func TestHomeScreen_Nav_Up(t *testing.T) {
	s := New("/tmp/test", true)
	s.checkpoints = demo.SyntheticCheckpoints()
	s.loading = false
	s.selected = 1

	result, cmd := s.Update(tea.KeyMsg{Type: tea.KeyUp})
	if cmd != nil {
		t.Error("expected nil cmd for navigation")
	}
	updated := result.(*Screen)
	if updated.selected != 0 {
		t.Errorf("expected selected=0, got %d", updated.selected)
	}
}

func TestHomeScreen_Nav_J_Down(t *testing.T) {
	s := New("/tmp/test", true)
	s.checkpoints = demo.SyntheticCheckpoints()
	s.loading = false
	s.selected = 0

	result, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	updated := result.(*Screen)
	if updated.selected != 1 {
		t.Errorf("expected selected=1, got %d", updated.selected)
	}
}

func TestHomeScreen_Nav_K_Up(t *testing.T) {
	s := New("/tmp/test", true)
	s.checkpoints = demo.SyntheticCheckpoints()
	s.loading = false
	s.selected = 1

	result, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	updated := result.(*Screen)
	if updated.selected != 0 {
		t.Errorf("expected selected=0, got %d", updated.selected)
	}
}

func TestHomeScreen_Nav_Boundary_Up(t *testing.T) {
	s := New("/tmp/test", true)
	s.checkpoints = demo.SyntheticCheckpoints()
	s.loading = false
	s.selected = 0

	result, _ := s.Update(tea.KeyMsg{Type: tea.KeyUp})
	updated := result.(*Screen)
	if updated.selected != 0 {
		t.Errorf("expected selected=0 (boundary), got %d", updated.selected)
	}
}

func TestHomeScreen_Nav_Boundary_Down(t *testing.T) {
	s := New("/tmp/test", true)
	s.checkpoints = demo.SyntheticCheckpoints()
	s.loading = false
	s.selected = len(s.checkpoints) - 1

	result, _ := s.Update(tea.KeyMsg{Type: tea.KeyDown})
	updated := result.(*Screen)
	if updated.selected != len(s.checkpoints)-1 {
		t.Errorf("expected selected=%d (boundary), got %d", len(s.checkpoints)-1, updated.selected)
	}
}

func TestHomeScreen_Delete_DemoMode(t *testing.T) {
	s := New("/tmp/test", true)
	s.checkpoints = demo.SyntheticCheckpoints()
	s.loading = false
	initialCount := len(s.checkpoints)
	s.selected = 0

	result, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmd != nil {
		t.Error("expected nil cmd for demo delete")
	}
	updated := result.(*Screen)
	if len(updated.checkpoints) != initialCount-1 {
		t.Errorf("expected %d checkpoints after delete, got %d", initialCount-1, len(updated.checkpoints))
	}
}

func TestHomeScreen_View_Empty(t *testing.T) {
	s := New("/tmp/test", true)
	s.checkpoints = nil
	s.loading = false

	view := s.View()
	if !strings.Contains(view, "No restore") {
		t.Error("expected empty state message")
	}
}

func TestHomeScreen_Footer(t *testing.T) {
	s := New("/tmp/test", true)
	footer := s.Footer()
	if len(footer) < 7 {
		t.Errorf("expected at least 7 footer hints, got %d", len(footer))
	}
	hasEnter := false
	for _, f := range footer {
		if f.Key == "Enter" {
			hasEnter = true
			break
		}
	}
	if !hasEnter {
		t.Error("expected 'Enter' footer hint")
	}
}

func TestHomeScreen_Error(_ *testing.T) {
	s := New("/tmp/test", true)
	s.loading = false
	s.err = nil
	_ = s.View // should not panic
}

func TestHomeScreen_Init_NonDemo(t *testing.T) {
	// Non-demo mode Init attempts SQLite access. We just check it returns a cmd.
	s := New("/tmp/test-nonexistent", false)
	s.demo = false
	cmd := s.Init()
	if cmd == nil {
		t.Error("expected non-nil cmd from Init in non-demo mode")
	}
}

func TestHomeScreen_StatusText_Completed(t *testing.T) {
	cps := demo.SyntheticCheckpoints()
	cps[0].ByteOffset = cps[0].DumpSizeBytes
	text := statusText(cps[0])
	if !strings.Contains(text, "Completed") {
		t.Errorf("expected 'Completed' status, got %q", text)
	}
}

func TestHomeScreen_StatusText_Resumable(t *testing.T) {
	cps := demo.SyntheticCheckpoints()
	// Already resumable at 300/500 MB.
	cps[1].ByteOffset = 300 * 1024 * 1024
	cps[1].DumpSizeBytes = 500 * 1024 * 1024
	text := statusText(cps[1])
	if !strings.Contains(text, "Resumable") {
		t.Errorf("expected 'Resumable' status, got %q", text)
	}
}

func TestHomeScreen_ElapsedTimeInDetail(t *testing.T) {
	s := New("/tmp/test", true)
	checkpoints := demo.SyntheticCheckpoints()
	checkpoints[0].UpdatedAt = time.Date(2026, 7, 14, 6, 50, 0, 0, time.UTC)
	s.checkpoints = checkpoints
	s.loading = false
	s.selected = 0

	view := s.View()
	if !strings.Contains(view, "2026-07-14") {
		t.Error("expected date format in detail")
	}
}
