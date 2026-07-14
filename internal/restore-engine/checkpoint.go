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
	UpdatedAt      time.Time
}

// CheckpointStore persists checkpoints to SQLite.
// Each call is fsync-durable (PRAGMA synchronous=FULL).
type CheckpointStore interface {
	// Get retrieves the checkpoint for a dump identity.
	// Returns nil, nil if no checkpoint exists.
	Get(dumpIdentity string) (*Checkpoint, error)

	// Upsert creates or updates the checkpoint for a dump.
	Upsert(cp *Checkpoint) error

	// Delete removes the checkpoint (called on full success).
	Delete(dumpIdentity string) error

	// ListAll returns all unfinished checkpoints.
	ListAll() ([]*Checkpoint, error)

	// Close shuts down the store.
	Close() error
}
