// Package tuireport provides the post-restore report screen.
package tuireport

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	restoreengine "github.com/kentoespdam/mariadb-restorer/internal/restore-engine"
	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
	tuiprogress "github.com/kentoespdam/mariadb-restorer/internal/tui/screens/progress"
)

// ExitCodeInfo holds the human-readable interpretation of an exit code.
type ExitCodeInfo struct {
	Code        int
	Label       string
	Description string
	Resumable   bool
	HasDeferred bool
	HasVerify   bool
}

// RestoreSummary holds the data needed to render a report.
type RestoreSummary struct {
	ExitCode      int
	Err           error
	Statements    int64
	BytesDone     int64
	BytesTotal    int64
	BatchCount    int64
	DeferredCount int
	DeferredDescs []string
	Elapsed       time.Duration
	VerifyFindings []string
	DataDir       string // for resume
	DumpPath      string // for resume
	DSN           string // for resume
}

// DecodeExitCode returns a human-readable explanation for a restore exit code.
func DecodeExitCode(code int) ExitCodeInfo {
	switch code {
	case 0:
		return ExitCodeInfo{
			Code: 0, Label: "Clean Success",
			Description: "Restore completed successfully. No deferred objects, no verify findings.",
		}
	case 1:
		return ExitCodeInfo{
			Code: 1, Label: "Fatal Error",
			Description: "Restore stopped mid-run due to a fatal error. Resumable via checkpoint.",
			Resumable:   true,
		}
	case 3:
		return ExitCodeInfo{
			Code: 3, Label: "Deferred Objects",
			Description: "Restore completed, but objects (views, triggers, routines) were deferred due to DEFINER issues. Run 'replay' to retry.",
			HasDeferred: true,
		}
	case 4:
		return ExitCodeInfo{
			Code: 4, Label: "Verify Findings",
			Description: "Restore completed with CHECK TABLE EXTENDED findings. FK violations and/or Corrupt detected. Inspect manually.",
			HasVerify:   true,
		}
	case 130:
		return ExitCodeInfo{
			Code: 130, Label: "Interrupted (SIGINT)",
			Description: "Restore deliberately stopped with Ctrl-C. Current batch drained. Resumable.",
			Resumable:   true,
		}
	case 143:
		return ExitCodeInfo{
			Code: 143, Label: "Interrupted (SIGTERM)",
			Description: "Restore terminated by SIGTERM. Resumable via checkpoint.",
			Resumable:   true,
		}
	default:
		return ExitCodeInfo{
			Code: code, Label: "Unknown",
			Description: fmt.Sprintf("Undefined exit code: %d", code),
		}
	}
}

// Screen displays the post-restore report.
type Screen struct {
	summary RestoreSummary
}

// New creates a report screen from a RestoreSummary.
func New(summary RestoreSummary) *Screen {
	return &Screen{summary: summary}
}

func (s *Screen) ID() base.ScreenID { return base.ScreenReport }
func (s *Screen) Title() string {
	info := DecodeExitCode(s.summary.ExitCode)
	return fmt.Sprintf("📊 Restore Report — %s", info.Label)
}

func (s *Screen) Footer() []base.FooterHint {
	info := DecodeExitCode(s.summary.ExitCode)
	hints := []base.FooterHint{
		{Key: "Esc", Desc: "back to Home"},
		{Key: "?", Desc: "help"},
		{Key: "g", Desc: "glossary"},
	}
	if info.Resumable {
		hints = append(hints, base.FooterHint{Key: "r", Desc: "resume restore"})
	}
	if info.HasDeferred || s.summary.DeferredCount > 0 {
		hints = append(hints, base.FooterHint{Key: "p", Desc: "replay deferred"})
	}
	return hints
}

func (s *Screen) Init() tea.Cmd { return nil }

// resumeRestore creates a progress screen and starts a resumable restore.
func (s *Screen) resumeRestore() tea.Cmd {
	return func() tea.Msg {
		eventCh := make(chan restoreengine.ProgressEvent, 100)
		progressScreen := tuiprogress.New(-1)
		progressScreen.ConfigureStore(s.summary.DataDir, s.summary.DumpPath, s.summary.DSN)
		runCtx := progressScreen.StartRestore(context.Background(), eventCh)
		restoreengine.RunRestoreAsync(runCtx,
			restoreengine.RestoreConfig{
				DataDir:  s.summary.DataDir,
				DumpPath: s.summary.DumpPath,
				DSN:      s.summary.DSN,
			},
			eventCh)
		return base.NavigateToMsg{Screen: progressScreen}
	}
}

func (s *Screen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "h":
			return s, func() tea.Msg { return base.NavigateBackMsg{} }
		case "r":
			if DecodeExitCode(s.summary.ExitCode).Resumable {
				if s.summary.DSN == "" {
					return s, func() tea.Msg {
						return base.ShowErrorMsg{Err: fmt.Errorf("resume: DSN tidak tersedia (buka dari history?)")}
					}
				}
				return s, s.resumeRestore()
			}
		case "p":
			if s.summary.DeferredCount > 0 {
				return s, func() tea.Msg {
					return base.ShowErrorMsg{Err: fmt.Errorf("replay: not yet implemented")}
				}
			}
		case "?":
			return s, base.NavigateTo(base.ScreenHelp, base.FactoryContext{})
		case "g":
			return s, base.NavigateTo(base.ScreenGlossary, base.FactoryContext{})
		}
	}
	return s, nil
}
