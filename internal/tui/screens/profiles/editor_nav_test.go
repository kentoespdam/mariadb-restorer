package tuiprofiles

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

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
	e.focused = 6 // last field (fieldPassphrase)

	result, _ := e.Update(tea.KeyMsg{Type: tea.KeyTab})
	updated := result.(*EditorScreen)
	if updated.focused != 0 {
		t.Errorf("expected focused=0 (wrap), got %d", updated.focused)
	}
}

func TestEditorScreen_Tab_GoesThroughPasswordFields(t *testing.T) {
	e := NewEditorScreen("/tmp/test", nil, false)
	e.focused = 4 // fieldDatabase

	// Tab to Password.
	result, _ := e.Update(tea.KeyMsg{Type: tea.KeyTab})
	updated := result.(*EditorScreen)
	if updated.focused != 5 {
		t.Errorf("expected focused=5 (Password) after Tab from Database, got %d", updated.focused)
	}

	// Tab to Passphrase.
	result, _ = updated.Update(tea.KeyMsg{Type: tea.KeyTab})
	updated = result.(*EditorScreen)
	if updated.focused != 6 {
		t.Errorf("expected focused=6 (Passphrase) after Tab from Password, got %d", updated.focused)
	}
}

func TestEditorScreen_ShiftTab_FromPasswordGoesBack(t *testing.T) {
	e := NewEditorScreen("/tmp/test", nil, false)
	e.focused = 5 // fieldPassword

	result, _ := e.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	updated := result.(*EditorScreen)
	if updated.focused != 4 {
		t.Errorf("expected focused=4 (Database) after Shift+Tab from Password, got %d", updated.focused)
	}
}
