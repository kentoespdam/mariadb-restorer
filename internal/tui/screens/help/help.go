// Package tuihelp provides Help and Glossary screens.
package tuihelp

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
)

func init() {
	base.RegisterFactory(base.ScreenHelp, func(_ base.FactoryContext) base.Screen {
		return NewHelpScreen()
	})
}

// HelpScreen lists all keyboard shortcuts.
type HelpScreen struct{}

// NewHelpScreen creates a help screen.
func NewHelpScreen() *HelpScreen {
	return &HelpScreen{}
}

func (s *HelpScreen) ID() base.ScreenID { return base.ScreenHelp }
func (s *HelpScreen) Title() string     { return "❓ Keyboard Shortcuts" }
func (s *HelpScreen) Footer() []base.FooterHint {
	return []base.FooterHint{{Key: "Esc", Desc: "back"}}
}
func (s *HelpScreen) Init() tea.Cmd { return nil }

func (s *HelpScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			return s, func() tea.Msg { return base.NavigateBackMsg{} }
		}
	}
	return s, nil
}

func (s *HelpScreen) View() string {
	var b strings.Builder

	// Global shortcuts.
	b.WriteString(base.StyleHighlight.Render("Global Shortcuts") + "\n")
	b.WriteString(renderShortcuts([][2]string{
		{"q / Ctrl-C", "Quit the TUI"},
		{"?", "Show this help screen"},
		{"g", "Show glossary"},
		{"h", "Go to Home screen"},
		{"Esc", "Go back to previous screen"},
		{"↑/k / ↓/j", "Navigate lists"},
	}) + "\n")

	b.WriteString(base.StyleHighlight.Render("Home Screen") + "\n")
	b.WriteString(renderShortcuts([][2]string{
		{"p", "Go to Profile Manager"},
		{"r", "Start new restore (Launcher)"},
		{"d", "Delete selected checkpoint"},
	}) + "\n")

	b.WriteString(base.StyleHighlight.Render("Profile Manager") + "\n")
	b.WriteString(renderShortcuts([][2]string{
		{"Enter", "Edit selected profile"},
		{"n", "Create new profile"},
		{"/", "Search/filter profiles"},
		{"Delete", "Delete selected profile"},
	}) + "\n")

	b.WriteString(base.StyleHighlight.Render("Profile Editor") + "\n")
	b.WriteString(renderShortcuts([][2]string{
		{"Tab / Shift+Tab", "Navigate form fields"},
		{"Enter", "Save profile"},
		{"s", "Set/change vaulted password"},
	}) + "\n")

	b.WriteString(base.StyleHighlight.Render("Restore Launcher") + "\n")
	b.WriteString(renderShortcuts([][2]string{
		{"n", "Next step"},
		{"b", "Previous step"},
		{"Esc", "Cancel launcher"},
	}) + "\n")

	b.WriteString(base.StyleDim.Render("\n Esc to return"))
	return b.String()
}

func renderShortcuts(keys [][2]string) string {
	var b strings.Builder
	for _, kv := range keys {
		key := base.StyleHelpKey.Render("  " + kv[0])
		desc := base.StyleHelpDesc.Render("  " + kv[1])
		b.WriteString(key + desc + "\n")
	}
	return b.String()
}
