package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
	tuihelp "github.com/kentoespdam/mariadb-restorer/internal/tui/screens/help"
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
	home, ok := base.CreateScreen(base.ScreenHome, base.FactoryContext{DataDir: dataDir, Demo: demo})
	if !ok {
		return nil, fmt.Errorf("home screen factory not registered")
	}
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

// push navigates to a new screen, pushing it onto the stack.
func (r *Router) push(s Screen, cmd tea.Cmd) (tea.Model, tea.Cmd) {
	r.stack = append(r.stack, s)
	return r, cmd
}

// pop removes the top screen from the stack. The root screen is never removed.
// Re-initializes the newly active screen so it reloads fresh data.
func (r *Router) pop() (tea.Model, tea.Cmd) {
	if len(r.stack) > 1 {
		r.stack = r.stack[:len(r.stack)-1]
	}
	return r, r.active().Init()
}

// goHome pops all screens except the root (home) screen.
func (r *Router) goHome() (tea.Model, tea.Cmd) {
	r.stack = r.stack[:1]
	return r, nil
}

func (r *Router) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if r.err != nil {
		return r, tea.Quit
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.width = msg.Width
		return r, nil

	case base.NavigateToMsg:
		return r.push(msg.Screen, msg.Screen.Init())

	case base.NavigateBackMsg:
		return r.pop()

	case base.ShowErrorMsg:
		r.err = msg.Err
		return r, nil

	case tuiprogress.RestoreCompleteMsg:
		return r.handleRestoreComplete(msg)

	case tea.KeyMsg:
		return r.handleKey(msg)
	}

	return r.delegateToActive(msg)
}

// handleRestoreComplete transitions from the progress screen to the report screen.
func (r *Router) handleRestoreComplete(msg tuiprogress.RestoreCompleteMsg) (tea.Model, tea.Cmd) {
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
	return r.push(report, report.Init())
}

// handleKey delegates all key presses to the active screen.
// Each screen handles its own keybindings — no global shortcut layer.
func (r *Router) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Hardcoded universal quit — control characters never conflict with text input.
	switch msg.String() {
	case "ctrl+c", "ctrl+q":
		return r, tea.Quit
	}

	// Delegate all other keys to the active screen.
	return r.delegateToActive(msg)
}

// delegateToActive forwards a message to the active screen's Update method.
func (r *Router) delegateToActive(msg tea.Msg) (tea.Model, tea.Cmd) {
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

	active := r.active()
	content := active.View()
	title := base.StyleStatusBar.Render(active.Title())
	dirInfo := base.StyleDataDir.Render("📁 " + r.dataDir)
	hints := append(active.Footer(), GlobalShortcuts()...)
	footer := RenderFooter(hints, r.width)

	var banner string
	if r.demo {
		banner = base.StyleWarning.Render(" 🎪 DEMO MODE — no actual operations") + "\n"
	}

	return fmt.Sprintf("%s%s\n%s\n\n%s\n%s", banner, title, content, dirInfo, footer)
}
