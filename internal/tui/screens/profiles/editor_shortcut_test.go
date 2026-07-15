package tuiprofiles

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
)

func TestEditorScreen_Types_AllShortcuts_NoNavigation(t *testing.T) {
	e := NewEditorScreen("/tmp/test", nil, false)

	for _, ch := range []rune{'p', 'q', 'h', 'g', 's', '?', 'r'} {
		_, cmd := e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		if cmd == nil {
			t.Errorf("expected non-nil cmd after typing '%c'", ch)
			continue
		}
		msg := cmd()
		if _, ok := msg.(base.NavigateBackMsg); ok {
			t.Errorf("'%c' should not trigger NavigateBackMsg", ch)
		}
		if _, ok := msg.(base.NavigateToMsg); ok {
			t.Errorf("'%c' should not trigger NavigateToMsg", ch)
		}
	}
	if got := e.inputs[fieldName].Value(); got != "pqhgs?r" {
		t.Errorf("expected 'pqhgs?r' in name field, got %q", got)
	}
}

func TestEditorScreen_CtrlX_RemovesPassword(t *testing.T) {
	e := NewEditorScreen("/tmp/test", nil, false)
	e.hasPwd = true

	_, cmd := e.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if cmd != nil {
		t.Log("cmd is non-nil")
	}
	if !e.clearPwd {
		t.Error("expected clearPwd=true after Ctrl-X")
	}
	if e.hasPwd {
		t.Error("expected hasPwd=false after Ctrl-X")
	}
}

func TestEditorScreen_CtrlX_OnNewProfile_Ignored(t *testing.T) {
	e := NewEditorScreen("/tmp/test", nil, false)
	e.hasPwd = false

	_, cmd := e.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	// When hasPwd is false, Ctrl-X passes through to textinput which may or may not return a cmd.
	// Just verify hasPwd stays false and clearPwd stays false.
	if e.clearPwd {
		t.Error("expected clearPwd=false when hasPwd is false")
	}
	if e.hasPwd {
		t.Error("expected hasPwd to remain false")
	}
	_ = cmd // cmd may be nil or non-nil depending on textinput impl
}

func TestEditorScreen_PasswordField_IsMasked(t *testing.T) {
	e := NewEditorScreen("/tmp/test", nil, false)
	if e.inputs[fieldPassword].EchoMode != textinput.EchoPassword {
		t.Error("expected EchoPassword mode for password field")
	}
	if e.inputs[fieldPassphrase].EchoMode != textinput.EchoPassword {
		t.Error("expected EchoPassword mode for passphrase field")
	}
}
