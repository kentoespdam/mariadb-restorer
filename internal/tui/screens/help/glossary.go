package tuihelp

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
)

func init() {
	base.RegisterFactory(base.ScreenGlossary, func(_ base.FactoryContext) base.Screen {
		return NewGlossaryScreen()
	})
}

// GlossaryScreen defines domain terms for the tool.
type GlossaryScreen struct{}

// NewGlossaryScreen creates a glossary screen.
func NewGlossaryScreen() *GlossaryScreen {
	return &GlossaryScreen{}
}

func (s *GlossaryScreen) ID() base.ScreenID { return base.ScreenGlossary }
func (s *GlossaryScreen) Title() string     { return "📖 Glossary" }
func (s *GlossaryScreen) Footer() []base.FooterHint {
	return []base.FooterHint{
		{Key: "Esc", Desc: "back"},
		{Key: "?", Desc: "help"},
	}
}
func (s *GlossaryScreen) Init() tea.Cmd { return nil }

func (s *GlossaryScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			return s, func() tea.Msg { return base.NavigateBackMsg{} }
		case "?":
			return s, base.NavigateTo(base.ScreenHelp, base.FactoryContext{})
		}
	}
	return s, nil
}

func (s *GlossaryScreen) View() string {
	var b strings.Builder

	entries := []struct{ term, def string }{
		{"Statement Boundary", "The exact byte offset immediately after a complete SQL statement's delimiter. Checkpoints land on these, never mid-statement."},
		{"Checkpoint", "A persistent record of restore progress (byte offset, statements done, lexer state). Written after each successful batch COMMIT so the tool can resume after a crash."},
		{"Batch", "A group of SQL statements executed within one transaction. Committed after ~64MB of data or ~1000 statements, whichever comes first. A batch may trigger multiple COMMITs if it contains an Implicit-Commit Statement."},
		{"Resume Batch", "The first batch of a resumed restore. Because it repeats statements that may have already been executed, it tolerates duplicate errors (MySQL 1062, 1050) and ignores them."},
		{"Deferred Object", "A CREATE VIEW, TRIGGER, or ROUTINE that failed at restore time due to DEFINER user mismatch (ERROR 1227/1449). Captured and replayed later via the 'replay' subcommand. Results in Exit Code 3."},
		{"Credential Vault", "AES-256-GCM encrypted storage for MariaDB connection passwords. Sealed per-profile using Argon2id KDF with a user-supplied Master Passphrase. Never stored in plaintext."},
		{"Master Passphrase", "The user's secret used to derive the vault encryption key via Argon2id. Required to seal/unseal passwords. Never travels with the Data Directory."},
		{"Fast Mode", "Session variables for speed: autocommit=0, unique_checks=0, foreign_key_checks=0. Tool OWNS these and resets to 1 on every exit. Accepts FK violations; verify phase finds them."},
		{"Verify", "Optional post-restore phase (--verify) that runs CHECK TABLE EXTENDED on tables with foreign keys, surfacing FK violations and structural Corrupt. Exit Code 4 if findings exist."},
	}

	for _, e := range entries {
		term := base.StyleHighlight.Render(" " + e.term)
		def := base.StyleDim.Render("\n  " + e.def)
		b.WriteString(term + def + "\n\n")
	}

	b.WriteString(base.StyleDim.Render(" Esc to return"))
	return b.String()
}
