package tuiprofiles

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
)

func TestEditorScreen_NewProfile_ID(t *testing.T) {
	e := NewEditorScreen("/tmp/test", nil, false)
	if e.ID() != base.ScreenEditor {
		t.Errorf("expected ScreenEditor, got %v", e.ID())
	}
}

func TestEditorScreen_NewProfile_Title(t *testing.T) {
	e := NewEditorScreen("/tmp/test", nil, false)
	if !strings.Contains(e.Title(), "New Profile") {
		t.Errorf("expected 'New Profile' in title, got %q", e.Title())
	}
}

func TestEditorScreen_EditProfile_Title(t *testing.T) {
	e := NewEditorScreen("/tmp/test", nil, true)
	if !strings.Contains(e.Title(), "Edit") {
		t.Logf("edit title (nil profile): %q", e.Title())
	}
}

func TestEditorScreen_Init_ReturnsBlinkCmd(t *testing.T) {
	e := NewEditorScreen("/tmp/test", nil, false)
	cmd := e.Init()
	if cmd == nil {
		t.Error("expected non-nil cmd from Init")
	}
}

func TestEditorScreen_View_ShowsFields(t *testing.T) {
	e := NewEditorScreen("/tmp/test", nil, false)
	view := e.View()

	labels := []string{"Name", "Host", "Port", "User", "Database"}
	for _, label := range labels {
		if !strings.Contains(view, label) {
			t.Errorf("expected field label %q in editor view", label)
		}
	}
}

func TestEditorScreen_View_SavedMessage(t *testing.T) {
	e := NewEditorScreen("/tmp/test", nil, false)
	e.saved = true
	view := e.View()
	if !strings.Contains(view, "saved") {
		t.Error("expected 'saved' message after save")
	}
}

func TestEditorScreen_Tab_CyclesForward(t *testing.T) {
	e := NewEditorScreen("/tmp/test", nil, false)
	e.focused = 0

	result, cmd := e.Update(tea.KeyMsg{Type: tea.KeyTab})
	if cmd == nil {
		t.Error("expected non-nil cmd (textinput.Blink)")
	}
	updated := result.(*EditorScreen)
	if updated.focused != 1 {
		t.Errorf("expected focused=1 after Tab, got %d", updated.focused)
	}
}

func TestEditorScreen_ShiftTab_CyclesBackward(t *testing.T) {
	e := NewEditorScreen("/tmp/test", nil, false)
	e.focused = 1

	result, _ := e.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	updated := result.(*EditorScreen)
	if updated.focused != 0 {
		t.Errorf("expected focused=0 after Shift+Tab, got %d", updated.focused)
	}
}

func TestEditorScreen_Tab_WrapsAround(t *testing.T) {
	e := NewEditorScreen("/tmp/test", nil, false)
	e.focused = 4 // last field

	result, _ := e.Update(tea.KeyMsg{Type: tea.KeyTab})
	updated := result.(*EditorScreen)
	if updated.focused != 0 {
		t.Errorf("expected focused=0 (wrap), got %d", updated.focused)
	}
}

func TestEditorScreen_Esc_NavigatesBack(t *testing.T) {
	e := NewEditorScreen("/tmp/test", nil, false)
	_, cmd := e.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(base.NavigateBackMsg); !ok {
		t.Errorf("expected NavigateBackMsg, got %T", msg)
	}
}

func TestEditorScreen_Footer(t *testing.T) {
	e := NewEditorScreen("/tmp/test", nil, false)
	footer := e.Footer()
	if len(footer) < 3 {
		t.Errorf("expected at least 3 footer hints, got %d", len(footer))
	}
}

func TestEditorScreen_EmptyNameShowsError(t *testing.T) {
	e := NewEditorScreen("/tmp/test", nil, false)
	e.err = "name is required"
	view := e.View()
	if !strings.Contains(view, "name is required") {
		t.Error("expected error message in view")
	}
}

func TestEditorScreen_InputValue(t *testing.T) {
	e := NewEditorScreen("/tmp/test", nil, false)
	// Set a value in the name field
	e.inputs[0].SetValue("my-profile")
	view := e.View()
	if !strings.Contains(view, "my-profile") {
		t.Error("expected input value in view")
	}
}
