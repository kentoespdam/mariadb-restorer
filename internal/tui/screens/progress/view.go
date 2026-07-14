package tuiprogress

import (
	"fmt"
	"strings"
	"time"

	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
)

func (s *Screen) View() string {
	var b strings.Builder

	if s.err != "" {
		b.WriteString(base.StyleError.Render(" ❌ Error: "+s.err) + "\n\n")
		b.WriteString(base.StyleDim.Render(" Press Enter to view the report."))
		return b.String()
	}

	if s.done {
		b.WriteString(base.StyleSuccess.Render(" ✔ Restore Complete!") + "\n\n")
		fmt.Fprintf(&b, " Statements: %d\n", s.statements)
		fmt.Fprintf(&b, " Batches:    %d\n", s.batchCount)
		if s.deferredCount > 0 {
			fmt.Fprintf(&b, " Deferred:   %d objects\n", s.deferredCount)
		}
		b.WriteString(base.StyleDim.Render("\n Press Enter to view the report."))
		return b.String()
	}

	// Progress bar.
	percent := float64(0)
	if s.bytesTotal > 0 {
		percent = float64(s.bytesDone) / float64(s.bytesTotal) * 100
	}
	barWidth := 40
	filled := int(percent / 100 * float64(barWidth))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	fmt.Fprintf(&b, " %s\n", bar)
	fmt.Fprintf(&b, " %.1f%% — %s / %s\n\n",
		percent, formatBytes(s.bytesDone), formatBytes(s.bytesTotal))

	// Throughput and ETA.
	elapsed := time.Since(s.startTime)
	throughput := float64(0)
	eta := "-"
	if elapsed > 0 {
		rate := float64(s.bytesDone) / elapsed.Seconds()
		throughput = rate
		if rate > 0 && s.bytesTotal > 0 {
			remaining := float64(s.bytesTotal-s.bytesDone) / rate
			eta = formatDuration(time.Duration(remaining) * time.Second)
		}
	}

	fmt.Fprintf(&b, " Statements:  %d\n", s.statements)
	fmt.Fprintf(&b, " Batches:     %d\n", s.batchCount)
	fmt.Fprintf(&b, " Throughput:  %s/s\n", formatBytes(int64(throughput)))
	fmt.Fprintf(&b, " Elapsed:     %s\n", formatDuration(elapsed))
	fmt.Fprintf(&b, " ETA:         %s\n", eta)

	if s.deferredCount > 0 {
		fmt.Fprintf(&b, " Deferred:    %d\n", s.deferredCount)
	}

	// Fast Mode indicator.
	if s.fastMode {
		b.WriteString("\n" + base.StyleWarning.Render(" ⚡ Fast Mode: autocommit=0, unique_checks=0, fk_checks=0"))
	}

	if s.signalCount > 0 {
		b.WriteString("\n\n" + base.StyleWarning.Render(" ⚠ Draining current batch... Ctrl-C again to abort."))
	} else {
		b.WriteString(base.StyleDim.Render("\n\n Ctrl-C to interrupt (graceful drain current batch)."))
	}

	return b.String()
}

func formatBytes(b int64) string {
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

func formatDuration(d time.Duration) string {
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
