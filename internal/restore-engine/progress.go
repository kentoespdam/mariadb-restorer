package restoreengine

import (
	"fmt"
	"io"
	"time"
)

// ProgressEvent is emitted after each batch commit.
type ProgressEvent struct {
	ByteOffset     int64
	DumpSizeBytes  int64
	StatementsDone int64
	BatchCount     int64
	DeferredCount  int
	Elapsed        time.Duration
	Done           bool
	Err            error
}

// ProgressReporter formats progress events for output.
type ProgressReporter struct {
	w         io.Writer
	startTime time.Time
	batchCnt  int64
}

// NewProgressReporter creates a reporter that writes to w.
func NewProgressReporter(w io.Writer) *ProgressReporter {
	return &ProgressReporter{
		w:         w,
		startTime: time.Now(),
	}
}

// Report formats and writes a single progress event.
// For TTY output, uses carriage-return to rewrite in place.
// For plain output, prints timestamped lines.
func (p *ProgressReporter) Report(ev ProgressEvent, isTTY bool) {
	p.batchCnt = ev.BatchCount

	if ev.Done {
		if ev.Err != nil {
			fmt.Fprintf(p.w, "\n✗ Restore failed: %v\n", ev.Err)
		} else {
			elapsed := time.Since(p.startTime).Round(time.Second)
			fmt.Fprintf(p.w, "\n✓ Restore complete — %d statements, %s elapsed\n",
				ev.StatementsDone, elapsed)
		}
		return
	}

	percent := float64(0)
	if ev.DumpSizeBytes > 0 {
		percent = float64(ev.ByteOffset) / float64(ev.DumpSizeBytes) * 100
	}

	elapsed := time.Since(p.startTime).Round(time.Second)
	rate := float64(0)
	if elapsed > 0 {
		rate = float64(ev.ByteOffset) / elapsed.Seconds()
	}

	if isTTY {
		line := fmt.Sprintf("\r%.1f%% — %s / %s — %d stmts — %s elapsed",
			percent,
			formatBytes(ev.ByteOffset),
			formatBytes(ev.DumpSizeBytes),
			ev.StatementsDone,
			elapsed)
		if rate > 0 {
			line += fmt.Sprintf(" — %s/s", formatBytes(int64(rate)))
		}
		fmt.Fprint(p.w, line)
	} else {
		line := fmt.Sprintf("[%s] %.1f%% — %d statements, %s/s",
			elapsed, percent, ev.StatementsDone, formatBytes(int64(rate)))
		fmt.Fprintln(p.w, line)
	}
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
