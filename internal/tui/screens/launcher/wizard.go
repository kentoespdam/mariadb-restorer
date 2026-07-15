// Package tuilauncher provides the restore launcher wizard.
package tuilauncher

import (
	"context"
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
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

const totalSteps = 5

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

	passwordInput textinput.Model
	err           string
}

// NewLauncherScreen creates a launcher wizard.
func NewLauncherScreen(dataDir string, demo bool) *LauncherScreen {
	ti := textinput.New()
	ti.Placeholder = "password"
	ti.EchoMode = textinput.EchoPassword
	ti.CharLimit = 256
	return &LauncherScreen{dataDir: dataDir, demo: demo, passwordInput: ti}
}

func (s *LauncherScreen) ID() base.ScreenID { return base.ScreenLauncher }
func (s *LauncherScreen) Title() string {
	return fmt.Sprintf("🚀 New Restore (Step %d/%d)", s.step+1, totalSteps)
}
func (s *LauncherScreen) Footer() []base.FooterHint {
	hints := []base.FooterHint{
		{Key: "n", Desc: "next"}, {Key: "b", Desc: "back"}, {Key: "Esc", Desc: "cancel"},
		{Key: "?", Desc: "help"}, {Key: "g", Desc: "glossary"},
	}
	if s.step == 2 {
		hints = []base.FooterHint{
			{Key: "Enter", Desc: "next"}, {Key: "Esc", Desc: "back"},
			{Key: "?", Desc: "help"}, {Key: "g", Desc: "glossary"},
		}
	}
	return hints
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

func (s *LauncherScreen) selectedProfile() *credentialvault.Profile {
	if s.selProfile >= 0 && s.selProfile < len(s.profiles) {
		return s.profiles[s.selProfile]
	}
	return nil
}

func (s *LauncherScreen) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Step 0 is text input mode: single chars go to dump_file path.
	if s.step == 0 {
		if msg.Paste {
			s.dumpFile += strings.TrimSpace(string(msg.Runes))
			return s, nil
		}
		switch key {
		case "backspace":
			if len(s.dumpFile) > 0 {
				s.dumpFile = s.dumpFile[:len(s.dumpFile)-1]
			}
		case "enter":
			s.step++
		case "esc":
			return s, func() tea.Msg { return base.NavigateBackMsg{} }
		case "ctrl+v":
			text, err := clipboard.ReadAll()
			if err == nil {
				s.dumpFile += strings.TrimSpace(text)
			}
		default:
			if len(key) == 1 {
				s.dumpFile += key
			}
		}
		return s, nil
	}

	// Step 2 is password/passphrase input.
	if s.step == 2 {
		switch key {
		case "enter":
			s.step++
			return s, nil
		case "esc":
			if s.step > 0 {
				s.step--
			}
			return s, nil
		case "ctrl+v":
			text, err := clipboard.ReadAll()
			if err == nil {
				s.passwordInput.SetValue(s.passwordInput.Value() + strings.TrimSpace(text))
			}
			return s, nil
		case "backspace":
			v := s.passwordInput.Value()
			if len(v) > 0 {
				s.passwordInput.SetValue(v[:len(v)-1])
			}
			return s, nil
		default:
			if len(key) == 1 {
				s.passwordInput.SetValue(s.passwordInput.Value() + key)
			}
			return s, nil
		}
	}

	// Steps 1, 3, 4: command mode.
	switch key {
	case "n":
		if s.step == totalSteps-1 {
			return s, s.launch()
		}
		s.nextStep()
	case "b":
		if s.step > 0 {
			s.step--
		}
	case "esc":
		return s, func() tea.Msg { return base.NavigateBackMsg{} }
	case "enter":
		if s.step == totalSteps-1 {
			return s, s.launch()
		}
		s.nextStep()
	case "up", "k":
		if s.step == 1 && s.selProfile > 0 {
			s.selProfile--
		}
	case "down", "j":
		if s.step == 1 && s.selProfile < len(s.profiles)-1 {
			s.selProfile++
		}
	case "v":
		if s.step == 3 {
			s.verify = !s.verify
		}
	case "?":
		return s, base.NavigateTo(base.ScreenHelp, base.FactoryContext{DataDir: s.dataDir, Demo: s.demo})
	case "g":
		return s, base.NavigateTo(base.ScreenGlossary, base.FactoryContext{DataDir: s.dataDir, Demo: s.demo})
	}
	return s, nil
}

// nextStep advances to the next step.
// Always shows the password step (step 2) so user can enter plain text password
// or master passphrase to unseal a vaulted password.
// Does NOT call launch() — handleKey checks for last-step launch separately.
func (s *LauncherScreen) nextStep() {
	s.step++
	if s.step >= totalSteps-1 {
		s.step = totalSteps - 1
	}
}

func (s *LauncherScreen) launch() tea.Cmd {
	return func() tea.Msg {
		if s.dumpFile == "" {
			return errMsg{fmt.Errorf("dump file path is required")}
		}
		p := s.selectedProfile()
		if p == nil {
			return errMsg{fmt.Errorf("no connection profile selected")}
		}

		// Build DSN from profile.
		// Omit /dbname if Database is empty — dump likely contains CREATE DATABASE.
		// go-sql-driver/mysql requires a bare "/" even when no db is specified.
		password := s.resolvePassword(p)

		dbPart := "/" + p.Database
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)%s?multiStatements=true&parseTime=true",
			p.User, password, p.Host, p.Port, dbPart)

		var progressScreen *tuiprogress.Screen
		if s.demo {
			progressScreen = tuiprogress.New(500 * 1024 * 1024)
			progressScreen.DemoSimulate()
		} else {
			eventCh := make(chan restoreengine.ProgressEvent, 100)
			progressScreen = tuiprogress.New(-1)
			progressScreen.ConfigureStore(s.dataDir, s.dumpFile, dsn)
			runCtx := progressScreen.StartRestore(context.Background(), eventCh)
			restoreengine.RunRestoreAsync(runCtx,
				restoreengine.RestoreConfig{
					DataDir:  s.dataDir,
					DumpPath: s.dumpFile,
					DSN:      dsn,
					Verify:   s.verify,
				}, eventCh)
		}
		return base.NavigateToMsg{Screen: progressScreen}
	}
}

// resolvePassword returns the MariaDB password: unseals from vault if possible,
// otherwise uses the password input value directly.
func (s *LauncherScreen) resolvePassword(p *credentialvault.Profile) string {
	if len(p.SealedPassword) > 0 && s.passwordInput.Value() != "" {
		pwd, err := credentialvault.UnsealPassword(p.SealedPassword, s.passwordInput.Value(), p.Name)
		if err == nil {
			return pwd
		}
		// Unseal failed (wrong passphrase) — return empty so MariaDB rejects
		// auth rather than sending garbage as the password.
		return ""
	}
	return s.passwordInput.Value()
}
