package tuilauncher

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	credentialvault "github.com/kentoespdam/mariadb-restorer/internal/credential-vault"
	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
)

func TestLauncherScreen_New_ID(t *testing.T) {
	s := NewLauncherScreen("/tmp/test", true)
	if s.ID() != base.ScreenLauncher {
		t.Errorf("expected ScreenLauncher, got %v", s.ID())
	}
}

func TestLauncherScreen_Title(t *testing.T) {
	s := NewLauncherScreen("/tmp/test", true)
	if !strings.Contains(s.Title(), "New Restore") {
		t.Errorf("expected 'New Restore' in title, got %q", s.Title())
	}
}

func TestLauncherScreen_Init_LoadsDemoProfiles(t *testing.T) {
	s := NewLauncherScreen("/tmp/test", true)
	cmd := s.Init()
	if cmd == nil {
		t.Fatal("expected non-nil cmd from Init")
	}
	msg := cmd()
	if msg != nil {
		// Demo mode returns nil but sets s.profiles internally.
		if len(s.profiles) != 4 {
			t.Errorf("expected 4 demo profiles, got %d", len(s.profiles))
		}
	}
}

func TestLauncherScreen_Step1_View(t *testing.T) {
	s := NewLauncherScreen("/tmp/test", true)
	s.step = 0
	s.err = ""
	s.profiles = testProfiles()
	view := s.View()

	if !strings.Contains(view, "Step 1") {
		t.Error("expected 'Step 1' in view")
	}
	if !strings.Contains(view, "Select Dump File") {
		t.Error("expected 'Select Dump File' in view")
	}
	if !strings.Contains(view, "S1") {
		t.Error("expected step indicator S1")
	}
}

func TestLauncherScreen_Step1_TypeFile(t *testing.T) {
	s := NewLauncherScreen("/tmp/test", true)
	s.step = 0
	s.profiles = testProfiles()
	s.dumpFile = ""

	// Type a character.
	result, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	updated := result.(*LauncherScreen)
	if updated.dumpFile != "/" {
		t.Errorf("expected dumpFile='/', got %q", updated.dumpFile)
	}
}

func TestLauncherScreen_Step1_Types_H(t *testing.T) {
	s := NewLauncherScreen("/tmp/test", true)
	s.step = 0
	s.profiles = testProfiles()
	s.dumpFile = ""

	result, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	updated := result.(*LauncherScreen)
	if updated.dumpFile != "h" {
		t.Errorf("expected dumpFile='h', got %q", updated.dumpFile)
	}
	// 'h' should NOT advance the step (that would indicate Home navigation).
	if updated.step != 0 {
		t.Errorf("expected step=0 after typing 'h', got %d", updated.step)
	}
}

func TestLauncherScreen_Step1_Types_G(t *testing.T) {
	s := NewLauncherScreen("/tmp/test", true)
	s.step = 0
	s.profiles = testProfiles()
	s.dumpFile = ""

	result, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	updated := result.(*LauncherScreen)
	if updated.dumpFile != "g" {
		t.Errorf("expected dumpFile='g', got %q", updated.dumpFile)
	}
	if updated.step != 0 {
		t.Errorf("expected step=0 after typing 'g', got %d", updated.step)
	}
}

func TestLauncherScreen_Step1_Types_QuestionMark(t *testing.T) {
	s := NewLauncherScreen("/tmp/test", true)
	s.step = 0
	s.profiles = testProfiles()
	s.dumpFile = ""

	result, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	updated := result.(*LauncherScreen)
	if updated.dumpFile != "?" {
		t.Errorf("expected dumpFile='?', got %q", updated.dumpFile)
	}
	if updated.step != 0 {
		t.Errorf("expected step=0 after typing '?', got %d", updated.step)
	}
}

func TestLauncherScreen_Step1_Backspace(t *testing.T) {
	s := NewLauncherScreen("/tmp/test", true)
	s.step = 0
	s.profiles = testProfiles()
	s.dumpFile = "/test"

	result, _ := s.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	updated := result.(*LauncherScreen)
	if updated.dumpFile != "/tes" {
		t.Errorf("expected dumpFile='/tes', got %q", updated.dumpFile)
	}
}

func TestLauncherScreen_Step1_N_TypesIntoDumpFile(t *testing.T) {
	s := NewLauncherScreen("/tmp/test", true)
	s.step = 0
	s.profiles = testProfiles()

	result, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	updated := result.(*LauncherScreen)
	if updated.dumpFile != "n" {
		t.Errorf("expected dumpFile='n', got %q", updated.dumpFile)
	}
	if updated.step != 0 {
		t.Errorf("expected step=0 (still text input), got %d", updated.step)
	}
}

func TestLauncherScreen_Step1_Enter_GoesToStep2(t *testing.T) {
	s := NewLauncherScreen("/tmp/test", true)
	s.step = 0
	s.profiles = testProfiles()
	s.dumpFile = "/path/to/dump.sql"

	result, _ := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(*LauncherScreen)
	if updated.step != 1 {
		t.Errorf("expected step=1 after Enter, got %d", updated.step)
	}
}

func TestLauncherScreen_Step2_View(t *testing.T) {
	s := NewLauncherScreen("/tmp/test", true)
	s.step = 1
	s.profiles = testProfiles()

	view := s.View()
	if !strings.Contains(view, "Step 2") {
		t.Error("expected 'Step 2' in view")
	}
	if !strings.Contains(view, "Select Connection") {
		t.Error("expected 'Select Connection' in view")
	}
}

func TestLauncherScreen_Step2_Nav_Down(t *testing.T) {
	s := NewLauncherScreen("/tmp/test", true)
	s.step = 1
	s.profiles = testProfiles()
	s.selProfile = 0

	result, _ := s.Update(tea.KeyMsg{Type: tea.KeyDown})
	updated := result.(*LauncherScreen)
	if updated.selProfile != 1 {
		t.Errorf("expected selProfile=1, got %d", updated.selProfile)
	}
}

func TestLauncherScreen_Step2_Nav_Up(t *testing.T) {
	s := NewLauncherScreen("/tmp/test", true)
	s.step = 1
	s.profiles = testProfiles()
	s.selProfile = 2

	// Need > 2 profiles for up to work
	s.profiles = append(s.profiles, testProfiles()...)

	result, _ := s.Update(tea.KeyMsg{Type: tea.KeyUp})
	updated := result.(*LauncherScreen)
	if updated.selProfile != 1 {
		t.Errorf("expected selProfile=1, got %d", updated.selProfile)
	}
}

func TestLauncherScreen_Step2_N_GoesToStep3(t *testing.T) {
	s := NewLauncherScreen("/tmp/test", true)
	s.step = 1
	s.profiles = testProfiles()

	result, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	updated := result.(*LauncherScreen)
	if updated.step != 2 {
		t.Errorf("expected step=2, got %d", updated.step)
	}
}

func TestLauncherScreen_Step3_View_ShowsConfirm(t *testing.T) {
	s := NewLauncherScreen("/tmp/test", true)
	s.step = 2
	s.dumpFile = "/backups/dump.sql"
	s.profiles = testProfiles()
	s.selProfile = 0

	view := s.View()
	if !strings.Contains(view, "Step 3") {
		t.Error("expected 'Step 3' in view")
	}
	if !strings.Contains(view, "Confirm") {
		t.Error("expected 'Confirm' in view")
	}
	if !strings.Contains(view, "/backups/dump.sql") {
		t.Error("expected dump file path in confirm view")
	}
	if !strings.Contains(view, "app_staging") {
		t.Error("expected database name in confirm view")
	}
}

func TestLauncherScreen_Step3_VerifyToggle(t *testing.T) {
	s := NewLauncherScreen("/tmp/test", true)
	s.step = 2
	s.profiles = testProfiles()
	s.verify = false

	result, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	updated := result.(*LauncherScreen)
	if !updated.verify {
		t.Error("expected verify=true after toggle")
	}
}

func TestLauncherScreen_Step3_VerifyToggle_Twice(t *testing.T) {
	s := NewLauncherScreen("/tmp/test", true)
	s.step = 2
	s.profiles = testProfiles()
	s.verify = true

	result, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	updated := result.(*LauncherScreen)
	if updated.verify {
		t.Error("expected verify=false after second toggle")
	}
}

func TestLauncherScreen_Step4_View(t *testing.T) {
	s := NewLauncherScreen("/tmp/test", true)
	s.step = 3
	s.dumpFile = "/backups/dump.sql"
	s.profiles = testProfiles()
	s.selProfile = 0

	view := s.View()
	if !strings.Contains(view, "Step 4") {
		t.Error("expected 'Step 4' in view")
	}
	if !strings.Contains(view, "Ready") {
		t.Error("expected 'Ready' in view")
	}
}

func TestLauncherScreen_B_GoesBack(t *testing.T) {
	s := NewLauncherScreen("/tmp/test", true)
	s.step = 2
	s.profiles = testProfiles()

	result, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	updated := result.(*LauncherScreen)
	if updated.step != 1 {
		t.Errorf("expected step=1 after back, got %d", updated.step)
	}
}

func TestLauncherScreen_B_DoesNotGoBelowStep0(t *testing.T) {
	s := NewLauncherScreen("/tmp/test", true)
	s.step = 0
	s.profiles = testProfiles()

	result, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	updated := result.(*LauncherScreen)
	if updated.step != 0 {
		t.Errorf("expected step=0 (boundary), got %d", updated.step)
	}
}

func TestLauncherScreen_Esc_NavigatesBack(t *testing.T) {
	s := NewLauncherScreen("/tmp/test", true)
	s.step = 1
	s.profiles = testProfiles()

	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(base.NavigateBackMsg); !ok {
		t.Errorf("expected NavigateBackMsg, got %T", msg)
	}
}

func TestLauncherScreen_Step3_Enter_GoesToStep4(t *testing.T) {
	s := NewLauncherScreen("/tmp/test", true)
	s.step = 2
	s.profiles = testProfiles()

	result, _ := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(*LauncherScreen)
	if updated.step != 3 {
		t.Errorf("expected step=3 after Enter on confirm, got %d", updated.step)
	}
}

func TestLauncherScreen_Footer(t *testing.T) {
	s := NewLauncherScreen("/tmp/test", true)
	footer := s.Footer()
	if len(footer) < 2 {
		t.Errorf("expected at least 2 footer hints, got %d", len(footer))
	}
}

// testProfiles returns minimal profile data for testing.
func testProfiles() []*credentialvault.Profile {
	return []*credentialvault.Profile{
		{Name: "staging", Host: "staging-db", Port: 3306, User: "app_user", Database: "app_staging", SealedPassword: []byte{1}},
		{Name: "prod", Host: "prod-db", Port: 3306, User: "admin", Database: "app_prod"},
	}
}
