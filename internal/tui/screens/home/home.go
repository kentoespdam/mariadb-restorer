// Package tuihome provides the Home screen showing restore history.
package tuihome

import (
	tea "github.com/charmbracelet/bubbletea"

	restoreengine "github.com/kentoespdam/mariadb-restorer/internal/restore-engine"
	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
	"github.com/kentoespdam/mariadb-restorer/internal/tui/demo"
	tuireport "github.com/kentoespdam/mariadb-restorer/internal/tui/screens/report"
)

func init() {
	base.RegisterFactory(base.ScreenHome, func(ctx base.FactoryContext) base.Screen {
		return New(ctx.DataDir, ctx.Demo)
	})
}

// Screen displays restore history from the Checkpoint Store.
type Screen struct {
	checkpoints []*restoreengine.Checkpoint
	selected    int
	err         error
	dataDir     string
	demo        bool
	loading     bool
}

// New creates a Home screen. In demo mode, loads synthetic data.
func New(dataDir string, demo bool) *Screen {
	return &Screen{
		dataDir: dataDir,
		demo:    demo,
		loading: true,
	}
}

func (s *Screen) ID() base.ScreenID { return base.ScreenHome }
func (s *Screen) Title() string     { return "🏠 Restore History" }

func (s *Screen) Footer() []base.FooterHint {
	return []base.FooterHint{
		{Key: "↑/↓", Desc: "navigate"},
		{Key: "Enter", Desc: "open report"},
		{Key: "p", Desc: "profiles"},
		{Key: "r", Desc: "new restore"},
		{Key: "d", Desc: "delete"},
		{Key: "?", Desc: "help"},
		{Key: "g", Desc: "glossary"},
	}
}

type checkpointsLoadedMsg []*restoreengine.Checkpoint
type errMsg struct{ error }

func (s *Screen) Init() tea.Cmd {
	return func() tea.Msg {
		if s.demo {
			return checkpointsLoadedMsg(demo.SyntheticCheckpoints())
		}
		return s.loadFromSQLite()
	}
}

func (s *Screen) loadFromSQLite() tea.Msg {
	dbPath := s.dataDir + "/mariadb-restorer.db"
	store, err := restoreengine.NewSQLiteStore(dbPath)
	if err != nil {
		return errMsg{err}
	}
	defer store.Close()
	cps, err := store.ListAll()
	if err != nil {
		return errMsg{err}
	}
	return checkpointsLoadedMsg(cps)
}

func (s *Screen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case checkpointsLoadedMsg:
		s.checkpoints = msg
		s.loading = false
		return s, nil
	case errMsg:
		s.err = msg
		s.loading = false
		return s, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if s.selected > 0 {
				s.selected--
			}
		case "down", "j":
			if s.selected < len(s.checkpoints)-1 {
				s.selected++
			}
		case "d":
			if len(s.checkpoints) > 0 && s.demo {
				s.checkpoints = append(s.checkpoints[:s.selected], s.checkpoints[s.selected+1:]...)
				if s.selected >= len(s.checkpoints) {
					s.selected = len(s.checkpoints) - 1
				}
			} else if len(s.checkpoints) > 0 {
				return s, s.deleteSelected()
			}
		case "enter":
			return s.openSelected()
		case "p":
			return s, base.NavigateTo(base.ScreenProfiles, base.FactoryContext{DataDir: s.dataDir, Demo: s.demo})
		case "r":
			return s, base.NavigateTo(base.ScreenLauncher, base.FactoryContext{DataDir: s.dataDir, Demo: s.demo})
		case "?":
			return s, base.NavigateTo(base.ScreenHelp, base.FactoryContext{DataDir: s.dataDir, Demo: s.demo})
		case "g":
			return s, base.NavigateTo(base.ScreenGlossary, base.FactoryContext{DataDir: s.dataDir, Demo: s.demo})
		}
	}
	return s, nil
}

// openSelected navigates to the report screen for the selected checkpoint.
func (s *Screen) openSelected() (tea.Model, tea.Cmd) {
	if s.selected < 0 || s.selected >= len(s.checkpoints) {
		return s, nil
	}
	cp := s.checkpoints[s.selected]

	exitCode := 0
	if cp.ByteOffset < cp.DumpSizeBytes {
		exitCode = 130 // Interrupted — resumable
	}

	summary := tuireport.RestoreSummary{
		ExitCode:   exitCode,
		Statements: cp.StatementsDone,
		BytesDone:  cp.ByteOffset,
		BytesTotal: cp.DumpSizeBytes,
	}

	report := tuireport.New(summary)
	return s, func() tea.Msg { return base.NavigateToMsg{Screen: report} }
}

func (s *Screen) deleteSelected() tea.Cmd {
	return func() tea.Msg {
		if s.selected < 0 || s.selected >= len(s.checkpoints) {
			return nil
		}
		dbPath := s.dataDir + "/mariadb-restorer.db"
		store, err := restoreengine.NewSQLiteStore(dbPath)
		if err != nil {
			return errMsg{err}
		}
		defer store.Close()
		if err := store.Delete(s.checkpoints[s.selected].DumpIdentity); err != nil {
			return errMsg{err}
		}
		cps, err := store.ListAll()
		if err != nil {
			return errMsg{err}
		}
		s.selected = 0
		return checkpointsLoadedMsg(cps)
	}
}
