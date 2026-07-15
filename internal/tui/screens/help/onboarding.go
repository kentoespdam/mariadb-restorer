package tuihelp

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
)

const markerFileName = ".mariadb-restorer-welcomed"

// OnboardingScreen is a dismissable first-launch overlay.
type OnboardingScreen struct {
	dataDir string
}

// NewOnboardingScreen creates an onboarding overlay if this is the first launch.
// Returns nil if the marker file exists (already shown).
func NewOnboardingScreen(dataDir string) *OnboardingScreen {
	marker := filepath.Join(dataDir, markerFileName)
	if _, err := os.Stat(marker); err == nil {
		return nil // already shown
	}
	return &OnboardingScreen{dataDir: dataDir}
}

func (s *OnboardingScreen) ID() base.ScreenID { return base.ScreenHelp }
func (s *OnboardingScreen) Title() string     { return "👋 Welcome to MariaDB Restorer" }
func (s *OnboardingScreen) Footer() []base.FooterHint {
	return []base.FooterHint{
		{Key: "Enter/Esc", Desc: "dismiss"},
		{Key: "?", Desc: "help"},
		{Key: "g", Desc: "glossary"},
	}
}

func (s *OnboardingScreen) Init() tea.Cmd { return nil }

func (s *OnboardingScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", "esc", " ":
			// Write marker file so it's not shown again.
			s.writeMarker()
			return s, func() tea.Msg { return base.NavigateBackMsg{} }
		case "?":
			return s, base.NavigateTo(base.ScreenHelp, base.FactoryContext{DataDir: s.dataDir})
		case "g":
			return s, base.NavigateTo(base.ScreenGlossary, base.FactoryContext{DataDir: s.dataDir})
		}
	}
	return s, nil
}



func (s *OnboardingScreen) writeMarker() {
	marker := filepath.Join(s.dataDir, markerFileName)
	f, err := os.Create(marker)
	if err != nil {
		return // non-fatal
	}
	f.Close()
}

func (s *OnboardingScreen) View() string {
	var b strings.Builder

	b.WriteString(base.StyleHighlight.Render(
		" Welcome to MariaDB Restorer!",
	) + "\n\n")

	b.WriteString(base.StyleDim.Render(
		" This tool helps you restore MariaDB/MySQL SQL dump files with",
	) + "\n")
	b.WriteString(base.StyleDim.Render(
		" crash-resume, constant memory, and speed optimizations.",
	) + "\n\n")

	b.WriteString(base.StyleHighlight.Render(" Quick Navigation") + "\n")
	b.WriteString(renderOnboardingItem("?", "Help screen — all keyboard shortcuts"))
	b.WriteString(renderOnboardingItem("g", "Glossary — domain terms explained"))
	b.WriteString(renderOnboardingItem("p", "Profile Manager — connection profiles"))
	b.WriteString(renderOnboardingItem("r", "New Restore — guided launcher wizard"))
	b.WriteString(renderOnboardingItem("h", "Home — restore history at a glance"))
	b.WriteString(renderOnboardingItem("↑/↓", "Navigate lists"))
	b.WriteString(renderOnboardingItem("Esc", "Go back"))
	b.WriteString(renderOnboardingItem("Ctrl-Q", "Quit TUI") + "\n")

	b.WriteString(base.StyleDim.Render(
		" The tool operates on the Data Directory: "+s.dataDir,
	) + "\n\n")

	b.WriteString(base.StyleSuccess.Render(" Press Enter or Esc to dismiss this message."))

	return b.String()
}

func renderOnboardingItem(key, desc string) string {
	k := base.StyleHelpKey.Render("  " + key)
	d := base.StyleHelpDesc.Render("  " + desc)
	return k + d + "\n"
}

