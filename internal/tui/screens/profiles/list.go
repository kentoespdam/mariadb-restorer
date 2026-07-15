// Package tuiprofiles provides profile list and editor screens.
package tuiprofiles

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	credentialvault "github.com/kentoespdam/mariadb-restorer/internal/credential-vault"
	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
	"github.com/kentoespdam/mariadb-restorer/internal/tui/demo"
)

func init() {
	base.RegisterFactory(base.ScreenProfiles, func(ctx base.FactoryContext) base.Screen {
		return NewListScreen(ctx.DataDir, ctx.Demo)
	})
}

// ListScreen displays connection profiles with search/filter.
type ListScreen struct {
	profiles  []*credentialvault.Profile
	selected  int
	search    string
	searching bool
	err       error
	dataDir   string
	demo      bool
	loading   bool
}

// NewListScreen creates a profile list screen. In demo mode, loads synthetic data.
func NewListScreen(dataDir string, demo bool) *ListScreen {
	return &ListScreen{
		dataDir: dataDir,
		demo:    demo,
		loading: true,
	}
}

func (s *ListScreen) ID() base.ScreenID         { return base.ScreenProfiles }
func (s *ListScreen) Title() string             { return "📋 Connection Profiles" }
func (s *ListScreen) Footer() []base.FooterHint { return listFooter() }

func listFooter() []base.FooterHint {
	return []base.FooterHint{
		{Key: "↑/↓", Desc: "navigate"},
		{Key: "Enter", Desc: "edit"},
		{Key: "n", Desc: "new"},
		{Key: "/", Desc: "search"},
		{Key: "Esc/h", Desc: "home"},
		{Key: "?", Desc: "help"},
		{Key: "g", Desc: "glossary"},
	}
}

type profileListLoadedMsg []*credentialvault.Profile
type errMsg struct{ error }

func (s *ListScreen) Init() tea.Cmd {
	return func() tea.Msg {
		if s.demo {
			return profileListLoadedMsg(demo.SyntheticProfiles())
		}
		p, err := loadProfiles(s.dataDir)
		if err != nil {
			return errMsg{err}
		}
		return profileListLoadedMsg(p)
	}
}

func loadProfiles(dataDir string) ([]*credentialvault.Profile, error) {
	dbPath := dataDir + "/mariadb-restorer.db"
	store, err := base.OpenProfileStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open profile store: %w", err)
	}
	defer store.Close()
	return store.List()
}

func (s *ListScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case profileListLoadedMsg:
		s.profiles = msg
		s.loading = false
		return s, nil
	case errMsg:
		s.err = msg
		s.loading = false
		return s, nil
	case tea.KeyMsg:
		return s.handleKey(msg)
	}
	return s, nil
}

func (s *ListScreen) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if s.searching {
		return s.handleSearch(msg)
	}
	switch msg.String() {
	case "up", "k":
		if s.selected > 0 {
			s.selected--
		}
	case "down", "j":
		if s.selected < len(s.profiles)-1 {
			s.selected++
		}
	case "enter":
		if len(s.profiles) > 0 && s.selected >= 0 {
			ed := NewEditorScreen(s.dataDir, s.profiles[s.selected], true)
			return s, func() tea.Msg { return base.NavigateToMsg{Screen: ed} }
		}
	case "n":
		ed := NewEditorScreen(s.dataDir, nil, false)
		return s, func() tea.Msg { return base.NavigateToMsg{Screen: ed} }
	case "/":
		s.searching = true
		s.search = ""
	case "?":
		return s, base.NavigateTo(base.ScreenHelp, base.FactoryContext{DataDir: s.dataDir, Demo: s.demo})
	case "h", "esc":
		return s, func() tea.Msg { return base.NavigateBackMsg{} }
	case "g":
		return s, base.NavigateTo(base.ScreenGlossary, base.FactoryContext{DataDir: s.dataDir, Demo: s.demo})
	}
	return s, nil
}



func (s *ListScreen) handleSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		s.searching = false
	case "esc":
		s.searching = false
		s.search = ""
	case "backspace":
		if len(s.search) > 0 {
			s.search = s.search[:len(s.search)-1]
		}
	default:
		if len(msg.String()) == 1 {
			s.search += msg.String()
		}
	}
	return s, nil
}
