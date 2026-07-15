package tuiprofiles

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
	"github.com/kentoespdam/mariadb-restorer/internal/tui/demo"
)

func TestListScreen_Init_ReturnsCmd(t *testing.T) {
	s := NewListScreen("/tmp/test", true)
	cmd := s.Init()
	if cmd == nil {
		t.Error("expected non-nil cmd from Init")
	}
}

func TestListScreen_ID(t *testing.T) {
	s := NewListScreen("/tmp/test", true)
	if s.ID() != base.ScreenProfiles {
		t.Errorf("expected ScreenProfiles, got %v", s.ID())
	}
}

func TestListScreen_Title(t *testing.T) {
	s := NewListScreen("/tmp/test", true)
	if !strings.Contains(s.Title(), "Profiles") {
		t.Errorf("expected 'Profiles' in title, got %q", s.Title())
	}
}

func TestListScreen_Init_LoadsDemoProfiles(t *testing.T) {
	s := NewListScreen("/tmp/test", true)
	cmd := s.Init()
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}

	msg := cmd()
	profiles, ok := msg.(profileListLoadedMsg)
	if !ok {
		t.Fatalf("expected profileListLoadedMsg, got %T", msg)
	}
	if len(profiles) < 1 {
		t.Fatal("expected at least 1 demo profile")
	}
}

func TestListScreen_DemoProfiles_HaveExpectedContent(t *testing.T) {
	profiles := demo.SyntheticProfiles()
	if len(profiles) != 4 {
		t.Errorf("expected 4 demo profiles, got %d", len(profiles))
	}

	names := make(map[string]bool)
	for _, p := range profiles {
		names[p.Name] = true
	}
	for _, name := range []string{"staging", "prod", "dev", "analytics"} {
		if !names[name] {
			t.Errorf("expected profile %q in demo data", name)
		}
	}
}

func TestListScreen_View_Loading(t *testing.T) {
	s := NewListScreen("/tmp/test", true)
	s.loading = true
	view := s.View()
	if !strings.Contains(view, "Loading") {
		t.Error("expected 'Loading' in view while loading")
	}
}

func TestListScreen_View_Empty(t *testing.T) {
	s := NewListScreen("/tmp/test", true)
	s.profiles = nil
	s.loading = false
	view := s.View()
	if !strings.Contains(view, "No profiles") {
		t.Error("expected empty state message")
	}
}

func TestListScreen_View_DemoData(t *testing.T) {
	s := NewListScreen("/tmp/test", true)
	s.profiles = demo.SyntheticProfiles()
	s.loading = false

	view := s.View()
	if !strings.Contains(view, "staging") {
		t.Error("expected 'staging' in view")
	}
	if !strings.Contains(view, "vaulted") {
		t.Error("expected vaulted indicator in view")
	}
}

func TestListScreen_View_ShowsProfileCount(t *testing.T) {
	s := NewListScreen("/tmp/test", true)
	s.profiles = demo.SyntheticProfiles()
	s.loading = false

	view := s.View()
	if !strings.Contains(view, "4 profile(s)") {
		t.Error("expected profile count in view")
	}
}

func TestListScreen_Nav_Down(t *testing.T) {
	s := NewListScreen("/tmp/test", true)
	s.profiles = demo.SyntheticProfiles()
	s.loading = false
	s.selected = 0

	result, cmd := s.Update(tea.KeyMsg{Type: tea.KeyDown})
	if cmd != nil {
		t.Error("expected nil cmd for navigation")
	}
	updated := result.(*ListScreen)
	if updated.selected != 1 {
		t.Errorf("expected selected=1, got %d", updated.selected)
	}
}

func TestListScreen_Nav_Up(t *testing.T) {
	s := NewListScreen("/tmp/test", true)
	s.profiles = demo.SyntheticProfiles()
	s.loading = false
	s.selected = 2

	result, _ := s.Update(tea.KeyMsg{Type: tea.KeyUp})
	updated := result.(*ListScreen)
	if updated.selected != 1 {
		t.Errorf("expected selected=1, got %d", updated.selected)
	}
}

func TestListScreen_Nav_J_Down(t *testing.T) {
	s := NewListScreen("/tmp/test", true)
	s.profiles = demo.SyntheticProfiles()
	s.loading = false
	s.selected = 0

	result, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	updated := result.(*ListScreen)
	if updated.selected != 1 {
		t.Errorf("expected selected=1, got %d", updated.selected)
	}
}

func TestListScreen_Nav_K_Up(t *testing.T) {
	s := NewListScreen("/tmp/test", true)
	s.profiles = demo.SyntheticProfiles()
	s.loading = false
	s.selected = 1

	result, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	updated := result.(*ListScreen)
	if updated.selected != 0 {
		t.Errorf("expected selected=0, got %d", updated.selected)
	}
}

func TestListScreen_Enter_OnSelected_NavigatesToEditor(t *testing.T) {
	s := NewListScreen("/tmp/test", true)
	s.profiles = demo.SyntheticProfiles()
	s.loading = false
	s.selected = 0

	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd for Enter on profile")
	}
	msg := cmd()
	if _, ok := msg.(base.NavigateToMsg); !ok {
		t.Errorf("expected NavigateToMsg, got %T", msg)
	}
}

func TestListScreen_N_NewProfile_NavigatesToEditor(t *testing.T) {
	s := NewListScreen("/tmp/test", true)
	s.profiles = demo.SyntheticProfiles()
	s.loading = false

	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if cmd == nil {
		t.Fatal("expected non-nil cmd for new profile")
	}
	msg := cmd()
	if _, ok := msg.(base.NavigateToMsg); !ok {
		t.Errorf("expected NavigateToMsg, got %T", msg)
	}
}

func TestListScreen_Slash_StartsSearch(t *testing.T) {
	s := NewListScreen("/tmp/test", true)
	s.profiles = demo.SyntheticProfiles()
	s.loading = false

	result, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	updated := result.(*ListScreen)
	if !updated.searching {
		t.Error("expected searching=true after '/' key")
	}
}

func TestListScreen_Search_Filters(t *testing.T) {
	s := NewListScreen("/tmp/test", true)
	s.profiles = demo.SyntheticProfiles()
	s.loading = false
	s.searching = true
	s.search = "staging"

	filtered := s.filtered()
	if len(filtered) != 1 {
		t.Errorf("expected 1 profile matching 'staging', got %d", len(filtered))
	}
	if filtered[0].Name != "staging" {
		t.Errorf("expected 'staging', got %q", filtered[0].Name)
	}
}

func TestListScreen_Search_ByHost(t *testing.T) {
	s := NewListScreen("/tmp/test", true)
	s.profiles = demo.SyntheticProfiles()
	s.loading = false
	s.searching = true
	s.search = "analytics"

	filtered := s.filtered()
	if len(filtered) != 1 {
		t.Errorf("expected 1 profile matching 'analytics', got %d", len(filtered))
	}
}

func TestListScreen_Search_Enter_ExitsSearch(t *testing.T) {
	s := NewListScreen("/tmp/test", true)
	s.profiles = demo.SyntheticProfiles()
	s.loading = false
	s.searching = true
	s.search = "test"

	result, _ := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(*ListScreen)
	if updated.searching {
		t.Error("expected searching=false after Enter in search")
	}
}

func TestListScreen_Search_Types_H(t *testing.T) {
	s := NewListScreen("/tmp/test", true)
	s.profiles = demo.SyntheticProfiles()
	s.loading = false
	s.searching = true
	s.search = ""

	result, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	updated := result.(*ListScreen)
	if updated.search != "h" {
		t.Errorf("expected search='h', got %q", updated.search)
	}
	if !updated.searching {
		t.Error("expected to remain in search mode after typing 'h'")
	}
}

func TestListScreen_Search_Types_G(t *testing.T) {
	s := NewListScreen("/tmp/test", true)
	s.profiles = demo.SyntheticProfiles()
	s.loading = false
	s.searching = true
	s.search = ""

	result, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	updated := result.(*ListScreen)
	if updated.search != "g" {
		t.Errorf("expected search='g', got %q", updated.search)
	}
	if !updated.searching {
		t.Error("expected to remain in search mode after typing 'g'")
	}
}

func TestListScreen_Search_Types_QuestionMark(t *testing.T) {
	s := NewListScreen("/tmp/test", true)
	s.profiles = demo.SyntheticProfiles()
	s.loading = false
	s.searching = true
	s.search = ""

	result, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	updated := result.(*ListScreen)
	if updated.search != "?" {
		t.Errorf("expected search='?', got %q", updated.search)
	}
	if !updated.searching {
		t.Error("expected to remain in search mode after typing '?'")
	}
}

func TestListScreen_Search_Esc_ExitsSearch(t *testing.T) {
	s := NewListScreen("/tmp/test", true)
	s.profiles = demo.SyntheticProfiles()
	s.loading = false
	s.searching = true
	s.search = "test"

	result, _ := s.Update(tea.KeyMsg{Type: tea.KeyEscape})
	updated := result.(*ListScreen)
	if updated.searching {
		t.Error("expected searching=false after Esc in search")
	}
	if updated.search != "" {
		t.Error("expected empty search after Esc")
	}
}

func TestListScreen_Footer(t *testing.T) {
	s := NewListScreen("/tmp/test", true)
	footer := s.Footer()
	if len(footer) < 7 {
		t.Errorf("expected at least 7 footer hints, got %d", len(footer))
	}
	hasHome := false
	for _, f := range footer {
		if f.Key == "Esc/h" {
			hasHome = true
			break
		}
	}
	if !hasHome {
		t.Error("expected 'Esc/h' footer hint for home navigation")
	}
}

func TestListScreen_Esc_NavigatesBack(t *testing.T) {
	s := NewListScreen("/tmp/test", true)
	s.profiles = demo.SyntheticProfiles()
	s.loading = false

	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected non-nil cmd for Esc")
	}
	msg := cmd()
	if _, ok := msg.(base.NavigateBackMsg); !ok {
		t.Errorf("expected NavigateBackMsg, got %T", msg)
	}
}

func TestListScreen_H_NavigatesBack(t *testing.T) {
	s := NewListScreen("/tmp/test", true)
	s.profiles = demo.SyntheticProfiles()
	s.loading = false

	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if cmd == nil {
		t.Fatal("expected non-nil cmd for 'h'")
	}
	msg := cmd()
	if _, ok := msg.(base.NavigateBackMsg); !ok {
		t.Errorf("expected NavigateBackMsg, got %T", msg)
	}
}

func TestListScreen_Error(t *testing.T) {
	_ = t
	s := NewListScreen("/tmp/test", true)
	s.loading = false
	s.err = nil
	_ = s.View() // should not panic
}
