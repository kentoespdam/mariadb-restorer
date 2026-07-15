package restoreengine

import "time"

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
	VerifyFindings []string // non-nil when verify phase ran; empty slice = clean
}
