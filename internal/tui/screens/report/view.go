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
	fmt.Fprintf(&b, " Statements:  %d\n", s.summary.Statements)
	fmt.Fprintf(&b, " Bytes:       %s / %s\n",
		formatBytesReport(s.summary.BytesDone), formatBytesReport(s.summary.BytesTotal))
	fmt.Fprintf(&b, " Batches:     %d\n", s.summary.BatchCount)
	fmt.Fprintf(&b, " Elapsed:     %s\n", formatElapsed(s.summary.Elapsed))

	if s.summary.DeferredCount > 0 {
		fmt.Fprintf(&b, "\n"+base.StyleWarning.Render(" Deferred:    %d objects")+"\n",
			s.summary.DeferredCount)
		for _, d := range s.summary.DeferredDescs {
			fmt.Fprintf(&b, "   • %s\n", d)
		}
	}

	if len(s.summary.VerifyFindings) > 0 {
		var fkCount, corruptCount, otherCount int
		for _, f := range s.summary.VerifyFindings {
			switch {
			case strings.HasPrefix(f, "[FK]"):
				fkCount++
			case strings.HasPrefix(f, "[CORRUPT]"):
				corruptCount++
			default:
				otherCount++
			}
		}

		b.WriteString("\n")
		b.WriteString(base.StyleWarning.Render(" Verify Findings") + "\n")

		if fkCount > 0 {
			b.WriteString("\n")
			b.WriteString(base.StyleError.Render(fmt.Sprintf(" FK Violations (%d):", fkCount)))
			b.WriteString("\n")
			for _, f := range s.summary.VerifyFindings {
				if strings.HasPrefix(f, "[FK]") {
					fmt.Fprintf(&b, "   • %s\n", f)
				}
			}
		}

		if corruptCount > 0 {
			b.WriteString("\n")
			b.WriteString(base.StyleWarning.Render(fmt.Sprintf(" Corrupt (%d, potential false positives):", corruptCount)))
			b.WriteString("\n")
			for _, f := range s.summary.VerifyFindings {
				if strings.HasPrefix(f, "[CORRUPT]") {
					fmt.Fprintf(&b, "   • %s\n", f)
				}
			}
			b.WriteString(base.StyleDim.Render(" ⚠ Corrupt may be false positive per MariaDB documentation") + "\n")
		}

		if otherCount > 0 {
			b.WriteString("\n")
			b.WriteString(" Other findings:\n")
			for _, f := range s.summary.VerifyFindings {
				if !strings.HasPrefix(f, "[FK]") && !strings.HasPrefix(f, "[CORRUPT]") {
					fmt.Fprintf(&b, "   • %s\n", f)
				}
			}
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
