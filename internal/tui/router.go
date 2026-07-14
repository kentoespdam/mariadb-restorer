package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
	tuihelp "github.com/kentoespdam/mariadb-restorer/internal/tui/screens/help"
	tuihome "github.com/kentoespdam/mariadb-restorer/internal/tui/screens/home"
	tuilauncher "github.com/kentoespdam/mariadb-restorer/internal/tui/screens/launcher"
	tuiprofiles "github.com/kentoespdam/mariadb-restorer/internal/tui/screens/profiles"
	tuiprogress "github.com/kentoespdam/mariadb-restorer/internal/tui/screens/progress"
	tuireport "github.com/kentoespdam/mariadb-restorer/internal/tui/screens/report"
)

// Router is the top-level Bubble Tea model managing a screen stack.
type Router struct {
	stack   []Screen
	dataDir string
	demo    bool
	err     error
	width   int
}

// NewRouter creates a router. In demo mode, uses synthetic data and shows a banner.
func NewRouter(dataDir string, demo bool) (*Router, error) {
	home := tuihome.New(dataDir, demo)
	r := &Router{
		stack:   []Screen{home},
		dataDir: dataDir,
		demo:    demo,
		width:   80,
	}
	if !demo {
		if onboarding := tuihelp.NewOnboardingScreen(dataDir); onboarding != nil {
			r.stack = append(r.stack, onboarding)
		}
	}
	return r, nil
}

func (r *Router) Init() tea.Cmd { return r.active().Init() }

func (r *Router) active() Screen {
	if len(r.stack) == 0 {
		return &emptyScreen{}
	}
	return r.stack[len(r.stack)-1]
}

func (r *Router) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if r.err != nil {
		return r, tea.Quit
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.width = msg.Width

	case base.NavigateToMsg:
		r.stack = append(r.stack, msg.Screen)
		return r, msg.Screen.Init()

	case base.NavigateBackMsg:
		if len(r.stack) > 1 {
			r.stack = r.stack[:len(r.stack)-1]
		}
		return r, nil

	case base.ShowErrorMsg:
		r.err = msg.Err
		return r, nil

	case tuiprogress.RestoreCompleteMsg:
		summary := tuireport.RestoreSummary{
			ExitCode:      msg.ExitCode,
			Err:           msg.Err,
			Statements:    msg.Statements,
			BytesDone:     msg.BytesDone,
			BytesTotal:    msg.BytesTotal,
			BatchCount:    msg.BatchCount,
			DeferredCount: msg.DeferredCount,
			DeferredDescs: msg.DeferredDescs,
			Elapsed:       msg.Elapsed,
		}
		report := tuireport.New(summary)
		r.stack = append(r.stack, report)
		return r, report.Init()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return r, tea.Quit
		case "?":
			help := tuihelp.NewHelpScreen()
			r.stack = append(r.stack, help)
			return r, help.Init()
		case "g":
			gloss := tuihelp.NewGlossaryScreen()
			r.stack = append(r.stack, gloss)
			return r, gloss.Init()
		case "esc":
			if len(r.stack) > 1 {
				r.stack = r.stack[:len(r.stack)-1]
			}
			return r, nil
		case "h":
			r.stack = r.stack[:1]
			return r, nil
		case "p":
			if r.active().ID() == base.ScreenHome {
				prof := tuiprofiles.NewListScreen(r.dataDir, r.demo)
				r.stack = append(r.stack, prof)
				return r, prof.Init()
			}
			return r, nil
		case "r":
			if r.active().ID() == base.ScreenHome {
				launch := tuilauncher.NewLauncherScreen(r.dataDir, r.demo)
				r.stack = append(r.stack, launch)
				return r, launch.Init()
			}
			return r, nil
		}
	}

	updated, cmd := r.active().Update(msg)
	if screen, ok := updated.(Screen); ok {
		r.stack[len(r.stack)-1] = screen
	}
	return r, cmd
}

func (r *Router) View() string {
	if r.err != nil {
		return base.StyleError.Render("Fatal: "+r.err.Error()) + "\n\nPress any key."
	}
	var banner string
	if r.demo {
		banner = base.StyleWarning.Render(" 🎪 DEMO MODE — no actual operations") + "\n"
	}
	active := r.active()
	content := active.View()
	title := base.StyleStatusBar.Render(active.Title())
	dirInfo := base.StyleDataDir.Render("📁 " + r.dataDir)
	hints := append(active.Footer(), GlobalShortcuts()...)
	footer := RenderFooter(hints, r.width)
	return fmt.Sprintf("%s%s\n%s\n\n%s", banner, title, content, dirInfo+"\n"+footer)
}

