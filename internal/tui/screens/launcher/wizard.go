// Package tuilauncher provides the restore launcher wizard.
package tuilauncher

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	credentialvault "github.com/kentoespdam/mariadb-restorer/internal/credential-vault"
	restoreengine "github.com/kentoespdam/mariadb-restorer/internal/restore-engine"
	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
	"github.com/kentoespdam/mariadb-restorer/internal/tui/demo"
	tuiprogress "github.com/kentoespdam/mariadb-restorer/internal/tui/screens/progress"
)

func init() {
	base.RegisterFactory(base.ScreenLauncher, func(ctx base.FactoryContext) base.Screen {
		return NewLauncherScreen(ctx.DataDir, ctx.Demo)
	})
}

const totalSteps = 4

// LauncherScreen is a multi-step wizard for starting a restore.
type LauncherScreen struct {
	step       int
	dataDir    string
	demo       bool
	dumpFile   string
	profiles   []*credentialvault.Profile
	selProfile int
	loaded     bool
	verify     bool

	err string
}

// NewLauncherScreen creates a launcher wizard.
func NewLauncherScreen(dataDir string, demo bool) *LauncherScreen {
	return &LauncherScreen{dataDir: dataDir, demo: demo}
}

func (s *LauncherScreen) ID() base.ScreenID { return base.ScreenLauncher }
func (s *LauncherScreen) Title() string {
	return fmt.Sprintf("🚀 New Restore (Step %d/%d)", s.step+1, totalSteps)
}
func (s *LauncherScreen) Footer() []base.FooterHint {
	return []base.FooterHint{
		{Key: "n", Desc: "next"}, {Key: "b", Desc: "back"}, {Key: "Esc", Desc: "cancel"}}
}

func (s *LauncherScreen) Init() tea.Cmd {
	return func() tea.Msg {
		if s.loaded {
			return nil
		}
		s.loaded = true
		if s.demo {
			s.profiles = demo.SyntheticProfiles()
			return nil
		}
		store, err := base.OpenProfileStore(s.dataDir + "/mariadb-restorer.db")
		if err != nil {
			return errMsg{err}
		}
		defer store.Close()
		p, err := store.List()
		if err != nil {
			return errMsg{err}
		}
		s.profiles = p
		return nil
	}
}

type errMsg struct{ error }

func (s *LauncherScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case errMsg:
		s.err = msg.Error()
		return s, nil
	case tea.KeyMsg:
		return s.handleKey(msg)
	}
	return s, nil
}

func (s *LauncherScreen) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "n":
		if s.step < totalSteps-1 {
			s.step++
		} else {
			return s, s.launch()
		}
	case "b":
		if s.step > 0 {
			s.step--
		}
	case "esc":
		return s, func() tea.Msg { return base.NavigateBackMsg{} }
	case "enter":
		if s.step == 3 {
			return s, s.launch()
		} else if s.step < totalSteps-1 {
			s.step++
		}
	case "up", "k":
		if s.step == 1 && s.selProfile > 0 {
			s.selProfile--
		}
	case "down", "j":
		if s.step == 1 && s.selProfile < len(s.profiles)-1 {
			s.selProfile++
		}
	case "v":
		if s.step == 2 {
			s.verify = !s.verify
		}
	case "backspace":
		if s.step == 0 && len(s.dumpFile) > 0 {
			s.dumpFile = s.dumpFile[:len(s.dumpFile)-1]
		}
	default:
		if s.step == 0 && len(msg.String()) == 1 {
			s.dumpFile += msg.String()
		}
	}
	return s, nil
}

func (s *LauncherScreen) launch() tea.Cmd {
	return func() tea.Msg {
		if s.dumpFile == "" {
			return errMsg{fmt.Errorf("dump file path is required")}
		}
		if len(s.profiles) == 0 {
			return errMsg{fmt.Errorf("no connection profile selected")}
		}
		var progressScreen *tuiprogress.Screen
		if s.demo {
			progressScreen = tuiprogress.New(500 * 1024 * 1024)
			progressScreen.DemoSimulate()
		} else {
			eventCh := make(chan restoreengine.ProgressEvent, 100)
			progressScreen = tuiprogress.New(-1)
			runCtx := progressScreen.StartRestore(context.Background(), eventCh)
			restoreengine.RunRestoreAsync(runCtx,
				restoreengine.RestoreConfig{
					DataDir:  s.dataDir,
					DumpPath: s.dumpFile,
				}, eventCh)
		}
		return base.NavigateToMsg{Screen: progressScreen}
	}
}
