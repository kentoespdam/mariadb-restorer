package tuiprofiles

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
)

func TestEditorScreen_SavePersistsToSQLite(t *testing.T) {
	dataDir := t.TempDir()
	e := NewEditorScreen(dataDir, nil, false)

	// Type name.
	for _, ch := range []rune{'t', 'e', 's', 't', '-', 'p', 'r', 'o', 'f', 'i', 'l', 'e'} {
		e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	// Host.
	e.Update(tea.KeyMsg{Type: tea.KeyTab})
	for _, ch := range []rune{'d', 'b', '.', 'e', 'x', 'a', 'm', 'p', 'l', 'e', '.', 'c', 'o', 'm'} {
		e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	// Port.
	e.Update(tea.KeyMsg{Type: tea.KeyTab})
	for _, ch := range []rune{'3', '3', '0', '7'} {
		e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	// User.
	e.Update(tea.KeyMsg{Type: tea.KeyTab})
	for _, ch := range []rune{'t', 'e', 's', 't', 'e', 'r'} {
		e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	// Database.
	e.Update(tea.KeyMsg{Type: tea.KeyTab})
	for _, ch := range []rune{'t', 'e', 's', 't', '_', 'd', 'b'} {
		e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	// Press Enter to save.
	result, _ := e.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(*EditorScreen)
	if !updated.saved {
		t.Fatal("expected saved=true after Enter")
	}

	// Verify persistence.
	store, err := base.OpenProfileStore(dataDir + "/mariadb-restorer.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	profile, err := store.Get("test-profile")
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	if profile == nil {
		t.Fatal("expected profile to exist after save")
	}
	if profile.Host != "db.example.com" {
		t.Errorf("expected host 'db.example.com', got %q", profile.Host)
	}
	if profile.Port != 3307 {
		t.Errorf("expected port 3307, got %d", profile.Port)
	}
	if profile.User != "tester" {
		t.Errorf("expected user 'tester', got %q", profile.User)
	}
	if profile.Database != "test_db" {
		t.Errorf("expected database 'test_db', got %q", profile.Database)
	}
	if profile.SealedPassword != nil {
		t.Errorf("expected nil SealedPassword, got %v", profile.SealedPassword)
	}
}

func TestEditorScreen_Save_WithPassword(t *testing.T) {
	dataDir := t.TempDir()
	e := NewEditorScreen(dataDir, nil, false)

	// Fill all fields.
	for _, ch := range []rune{'m', 'y', 'p', 'r', 'o', 'f'} {
		e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	// Host.
	e.Update(tea.KeyMsg{Type: tea.KeyTab})
	for _, ch := range []rune{'l', 'o', 'c', 'a', 'l', 'h', 'o', 's', 't'} {
		e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	// Port (skip - default 3306).
	e.Update(tea.KeyMsg{Type: tea.KeyTab})
	// User.
	e.Update(tea.KeyMsg{Type: tea.KeyTab})
	for _, ch := range []rune{'r', 'o', 'o', 't'} {
		e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	// Database.
	e.Update(tea.KeyMsg{Type: tea.KeyTab})
	for _, ch := range []rune{'t', 'e', 's', 't'} {
		e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	// Password.
	e.Update(tea.KeyMsg{Type: tea.KeyTab})
	for _, ch := range []rune{'p', '@', 's', 's'} {
		e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	// Passphrase.
	e.Update(tea.KeyMsg{Type: tea.KeyTab})
	for _, ch := range []rune{'m', 'y', '-', 'k', 'e', 'y'} {
		e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	// Save.
	result, _ := e.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(*EditorScreen)
	if !updated.saved {
		t.Fatal("expected saved=true after Enter with password")
	}

	// Verify password was sealed.
	store, err := base.OpenProfileStore(dataDir + "/mariadb-restorer.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	profile, err := store.Get("myprof")
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	if profile == nil {
		t.Fatal("expected profile to exist")
	}
	if profile.SealedPassword == nil {
		t.Fatal("expected SealedPassword to be non-nil (password was sealed)")
	}
}

func TestEditorScreen_Save_WithoutPassphrase_ShowsError(t *testing.T) {
	e := NewEditorScreen(t.TempDir(), nil, false)
	e.inputs[fieldName].SetValue("myprof")
	e.inputs[fieldPassword].SetValue("secret")

	// save should fail because passphrase is empty.
	err := e.save()
	if err == nil {
		t.Fatal("expected error when password is set without passphrase")
	}
	if !strings.Contains(err.Error(), "passphrase") {
		t.Errorf("expected error mentioning 'passphrase', got %q", err)
	}
}
