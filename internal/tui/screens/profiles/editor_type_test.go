package tuiprofiles

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestEditorScreen_Types_S_IntoNameField(t *testing.T) {
	e := NewEditorScreen("/tmp/test", nil, false)
	e.focused = fieldName

	result, cmd := e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Error("expected non-nil cmd after typing 's'")
	}
	updated := result.(*EditorScreen)
	if updated.err != "" {
		t.Errorf("expected no error after typing 's', got %q", updated.err)
	}
	if updated.inputs[fieldName].Value() != "s" {
		t.Errorf("expected name field to contain 's', got %q", updated.inputs[fieldName].Value())
	}
	view := updated.View()
	if !strings.Contains(view, "s") {
		t.Error("expected 's' in editor view after typing")
	}
}

func TestEditorScreen_Types_H_IntoHostField(t *testing.T) {
	e := NewEditorScreen("/tmp/test", nil, false)
	e.Update(tea.KeyMsg{Type: tea.KeyTab})

	result, cmd := e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if cmd == nil {
		t.Error("expected non-nil cmd after typing 'h'")
	}
	updated := result.(*EditorScreen)
	if updated.err != "" {
		t.Errorf("expected no error after typing 'h', got %q", updated.err)
	}
	if updated.inputs[fieldHost].Value() != "h" {
		t.Errorf("expected host field to contain 'h', got %q", updated.inputs[fieldHost].Value())
	}
}

func TestEditorScreen_Types_G_IntoUserField(t *testing.T) {
	e := NewEditorScreen("/tmp/test", nil, false)
	e.Update(tea.KeyMsg{Type: tea.KeyTab}) // host
	e.Update(tea.KeyMsg{Type: tea.KeyTab}) // port
	e.Update(tea.KeyMsg{Type: tea.KeyTab}) // user

	result, cmd := e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if cmd == nil {
		t.Error("expected non-nil cmd after typing 'g'")
	}
	updated := result.(*EditorScreen)
	if updated.err != "" {
		t.Errorf("expected no error after typing 'g', got %q", updated.err)
	}
	if updated.inputs[fieldUser].Value() != "g" {
		t.Errorf("expected user field to contain 'g', got %q", updated.inputs[fieldUser].Value())
	}
}

func TestEditorScreen_Types_Word_IntoAllFields(t *testing.T) {
	e := NewEditorScreen("/tmp/test", nil, false)

	for _, ch := range []rune{'m', 'y', 's', 'q'} {
		e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	if got := e.inputs[fieldName].Value(); got != "mysq" {
		t.Errorf("expected name 'mysq', got %q", got)
	}

	e.Update(tea.KeyMsg{Type: tea.KeyTab})
	for _, ch := range []rune{'h', 'o', 's', 't'} {
		e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	if got := e.inputs[fieldHost].Value(); got != "host" {
		t.Errorf("expected host 'host', got %q", got)
	}

	e.Update(tea.KeyMsg{Type: tea.KeyTab})
	for _, ch := range []rune{'3', '3', '0', '6'} {
		e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	if got := e.inputs[fieldPort].Value(); got != "3306" {
		t.Errorf("expected port '3306', got %q", got)
	}

	e.Update(tea.KeyMsg{Type: tea.KeyTab})
	for _, ch := range []rune{'r', 'o', 'o', 't'} {
		e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	if got := e.inputs[fieldUser].Value(); got != "root" {
		t.Errorf("expected user 'root', got %q", got)
	}

	e.Update(tea.KeyMsg{Type: tea.KeyTab})
	for _, ch := range []rune{'d', 'b', '1'} {
		e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	if got := e.inputs[fieldDatabase].Value(); got != "db1" {
		t.Errorf("expected database 'db1', got %q", got)
	}

	e.Update(tea.KeyMsg{Type: tea.KeyTab})
	for _, ch := range []rune{'s', 'e', 'c', 'r', 'e', 't'} {
		e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	if got := e.inputs[fieldPassword].Value(); got != "secret" {
		t.Errorf("expected password 'secret', got %q", got)
	}

	e.Update(tea.KeyMsg{Type: tea.KeyTab})
	for _, ch := range []rune{'m', 'a', 's', 't', 'e', 'r'} {
		e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	if got := e.inputs[fieldPassphrase].Value(); got != "master" {
		t.Errorf("expected passphrase 'master', got %q", got)
	}

	if e.err != "" {
		t.Errorf("expected no error, got %q", e.err)
	}
	view := e.View()
	for _, s := range []string{"mysq", "host", "3306", "root", "db1"} {
		if !strings.Contains(view, s) {
			t.Errorf("expected %q in view", s)
		}
	}
}
