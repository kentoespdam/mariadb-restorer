package tuihelp

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
)

// HelpScreen tests.

func TestHelpScreen_Init(t *testing.T) {
	s := NewHelpScreen()
	cmd := s.Init()
	if cmd != nil {
		t.Error("expected nil cmd from Init")
	}
}

func TestHelpScreen_ID(t *testing.T) {
	s := NewHelpScreen()
	if s.ID() != base.ScreenHelp {
		t.Errorf("expected ScreenHelp, got %v", s.ID())
	}
}

func TestHelpScreen_Title(t *testing.T) {
	s := NewHelpScreen()
	if !strings.Contains(s.Title(), "Keyboard") {
		t.Errorf("expected Keyboard in title, got %q", s.Title())
	}
}

func TestHelpScreen_View_ContainsSections(t *testing.T) {
	s := NewHelpScreen()
	view := s.View()

	sections := []string{"Universal Shortcuts", "Navigation", "Home Screen", "Profile Manager", "Profile Editor", "Restore Launcher", "Progress Monitor", "Restore Report"}
	for _, section := range sections {
		if !strings.Contains(view, section) {
			t.Errorf("expected section %q in help view", section)
		}
	}
}

func TestHelpScreen_View_ContainsKeys(t *testing.T) {
	s := NewHelpScreen()
	view := s.View()

	keys := []string{"Ctrl-Q", "?", "g", "Esc", "p", "r", "Ctrl-X"}
	for _, key := range keys {
		if !strings.Contains(view, key) {
			t.Errorf("expected key %q in help view", key)
		}
	}
}

func TestHelpScreen_Esc_NavigatesBack(t *testing.T) {
	s := NewHelpScreen()
	result, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	// Execute the cmd and check the returned message.
	msg := cmd()
	if _, ok := msg.(base.NavigateBackMsg); !ok {
		t.Errorf("expected NavigateBackMsg, got %T", msg)
	}
	if result != s {
		t.Error("expected same screen pointer")
	}
}

func TestHelpScreen_Q_NavigatesBack(t *testing.T) {
	s := NewHelpScreen()
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(base.NavigateBackMsg); !ok {
		t.Errorf("expected NavigateBackMsg, got %T", msg)
	}
}

func TestHelpScreen_OtherKey_Ignored(t *testing.T) {
	s := NewHelpScreen()
	result, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if cmd != nil {
		t.Error("expected nil cmd for unhandled key")
	}
	if result != s {
		t.Error("expected same screen pointer")
	}
}

// GlossaryScreen tests.

func TestGlossaryScreen_Init(t *testing.T) {
	s := NewGlossaryScreen()
	cmd := s.Init()
	if cmd != nil {
		t.Error("expected nil cmd from Init")
	}
}

func TestGlossaryScreen_ID(t *testing.T) {
	s := NewGlossaryScreen()
	if s.ID() != base.ScreenGlossary {
		t.Errorf("expected ScreenGlossary, got %v", s.ID())
	}
}

func TestGlossaryScreen_View_ContainsTerms(t *testing.T) {
	s := NewGlossaryScreen()
	view := s.View()

	terms := []string{"Statement Boundary", "Checkpoint", "Batch", "Resume Batch",
		"Deferred Object", "Credential Vault", "Master Passphrase", "Fast Mode", "Verify"}
	for _, term := range terms {
		if !strings.Contains(view, term) {
			t.Errorf("expected term %q in glossary view", term)
		}
	}
}

func TestGlossaryScreen_Esc_NavigatesBack(t *testing.T) {
	s := NewGlossaryScreen()
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(base.NavigateBackMsg); !ok {
		t.Errorf("expected NavigateBackMsg, got %T", msg)
	}
}

func TestGlossaryScreen_Q_NavigatesBack(t *testing.T) {
	s := NewGlossaryScreen()
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(base.NavigateBackMsg); !ok {
		t.Errorf("expected NavigateBackMsg, got %T", msg)
	}
}

// OnboardingScreen tests.

func TestOnboardingScreen_New(t *testing.T) {
	// NewOnboardingScreen returns nil if marker exists. We can't test the marker
	// logic here because it depends on the filesystem. Instead, test basic properties.
	s := &OnboardingScreen{dataDir: "/tmp/test-onboarding"}
	if s.ID() != base.ScreenHelp {
		t.Errorf("expected ScreenHelp, got %v", s.ID())
	}
}

func TestOnboardingScreen_View_ContainsWelcome(t *testing.T) {
	s := &OnboardingScreen{dataDir: "/tmp/test"}
	view := s.View()

	welcomeTerms := []string{"Welcome", "Quick Navigation", "Data Directory"}
	for _, term := range welcomeTerms {
		if !strings.Contains(view, term) {
			t.Errorf("expected %q in onboarding view", term)
		}
	}
}

func TestOnboardingScreen_View_ContainsKeys(t *testing.T) {
	s := &OnboardingScreen{dataDir: "/tmp/test"}
	view := s.View()

	keys := []string{"?", "g", "p", "r", "h", "Esc", "Ctrl-Q"}
	for _, key := range keys {
		if !strings.Contains(view, key) {
			t.Errorf("expected key %q in onboarding view", key)
		}
	}
}

func TestOnboardingScreen_Enter_Dismisses(t *testing.T) {
	s := &OnboardingScreen{dataDir: "/tmp/test"}
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(base.NavigateBackMsg); !ok {
		t.Errorf("expected NavigateBackMsg, got %T", msg)
	}
}

func TestOnboardingScreen_Esc_Dismisses(t *testing.T) {
	s := &OnboardingScreen{dataDir: "/tmp/test"}
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(base.NavigateBackMsg); !ok {
		t.Errorf("expected NavigateBackMsg, got %T", msg)
	}
}

func TestOnboardingScreen_Space_Dismisses(t *testing.T) {
	s := &OnboardingScreen{dataDir: "/tmp/test"}
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(base.NavigateBackMsg); !ok {
		t.Errorf("expected NavigateBackMsg, got %T", msg)
	}
}
