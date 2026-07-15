// Package restore-engine provides core restore logic for SQL dump files.
package restoreengine

import "time"

// Checkpoint records the restore progress for a single dump file.
// One row per dump_identity — UPSERT on each batch COMMIT.
type Checkpoint struct {
	DumpPath       string
	DumpSizeBytes  int64
	DumpIdentity   string
	ByteOffset     int64
	StatementsDone int64
	NoBackslashEsc bool
	AnsiQuotes     bool
	CurrentDelim   string
	Charset        string
	DSN            string // connection string for resume
	UpdatedAt      time.Time
}
