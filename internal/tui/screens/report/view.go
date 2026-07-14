package tuireport

import (
	"fmt"
	"strings"
	"time"

	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
)

func (s *Screen) View() string {
	var b strings.Builder
	info := DecodeExitCode(s.summary.ExitCode)

	switch s.summary.ExitCode {
	case 0:
		b.WriteString(base.StyleSuccess.Render(fmt.Sprintf(" Exit Code %d — %s", s.summary.ExitCode, info.Label)) + "\n")
	case 3, 4:
		b.WriteString(base.StyleWarning.Render(fmt.Sprintf(" Exit Code %d — %s", s.summary.ExitCode, info.Label)) + "\n")
	default:
		b.WriteString(base.StyleError.Render(fmt.Sprintf(" Exit Code %d — %s", s.summary.ExitCode, info.Label)) + "\n")
	}
	b.WriteString("\n " + info.Description + "\n\n")

	if s.summary.Err != nil {
		b.WriteString(base.StyleError.Render(" Error: "+s.summary.Err.Error()) + "\n\n")
	}

	b.WriteString(base.StyleHighlight.Render(" Summary") + "\n")
	b.WriteString(fmt.Sprintf(" Statements:  %d\n", s.summary.Statements))
	b.WriteString(fmt.Sprintf(" Bytes:       %s / %s\n",
		formatBytesReport(s.summary.BytesDone), formatBytesReport(s.summary.BytesTotal)))
	b.WriteString(fmt.Sprintf(" Batches:     %d\n", s.summary.BatchCount))
	b.WriteString(fmt.Sprintf(" Elapsed:     %s\n", formatElapsed(s.summary.Elapsed)))

	if s.summary.DeferredCount > 0 {
		b.WriteString(fmt.Sprintf("\n"+base.StyleWarning.Render(" Deferred:    %d objects")+"\n",
			s.summary.DeferredCount))
		for _, d := range s.summary.DeferredDescs {
			b.WriteString(fmt.Sprintf("   • %s\n", d))
		}
	}

	b.WriteString("\n" + base.StyleHighlight.Render(" Actions") + "\n")
	b.WriteString(base.StyleDim.Render("  Esc:  Return to Home screen"))
	if info.Resumable {
		b.WriteString(base.StyleDim.Render("\n  r:    Resume restore from checkpoint"))
	}
	if info.HasDeferred || s.summary.DeferredCount > 0 {
		b.WriteString(base.StyleDim.Render("\n  p:    Replay deferred objects"))
	}

	return b.String()
}

func formatBytesReport(b int64) string {
	switch {
	case b >= 1024*1024*1024:
		return fmt.Sprintf("%.1f GB", float64(b)/(1024*1024*1024))
	case b >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
	case b >= 1024:
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func formatElapsed(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	if h > 0 {
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
