package restoreengine

import (
	"fmt"
)

// Executor orchestrates the full restore lifecycle: pre-flight, batches, verify, replay.
type Executor struct {
	store      *SQLiteStore
	dumpPath   string
	dumpSize   int64
	dumpIdent  string
	byteOff    int64
	statements int64
}

// NewExecutor creates a restore executor for the given dump file.
func NewExecutor(store *SQLiteStore, dumpPath string) (*Executor, error) {
	identity, size, err := ComputeIdentity(dumpPath)
	if err != nil {
		return nil, fmt.Errorf("compute identity: %w", err)
	}
	return &Executor{
		store:     store,
		dumpPath:  dumpPath,
		dumpSize:  size,
		dumpIdent: identity,
	}, nil
}

// DumpIdentity returns the computed dump identity.
func (e *Executor) DumpIdentity() string { return e.dumpIdent }

// DumpSize returns the total dump file size in bytes.
func (e *Executor) DumpSize() int64 { return e.dumpSize }

// ByteOffset returns the current restore position.
func (e *Executor) ByteOffset() int64 { return e.byteOff }

// StatementsDone returns the number of completed statements.
func (e *Executor) StatementsDone() int64 { return e.statements }

// ResumeFromCheckpoint seeks to the last checkpoint position and returns
// the checkpoint data. Returns nil if no checkpoint exists or identity changed.
// On identity mismatch, the checkpoint is discarded and the caller should restart
// from byte 0 (ADR-0019: auto-restart on mismatch).
func (e *Executor) ResumeFromCheckpoint() (*Checkpoint, error) {
	cp, err := e.store.Get(e.dumpIdent)
	if err != nil {
		return nil, fmt.Errorf("get checkpoint: %w", err)
	}
	if cp == nil {
		return nil, nil
	}

	// Verify dump identity matches (ADR-0019).
	if cp.DumpIdentity != e.dumpIdent {
		// Identity changed — discard stale checkpoint, restart from byte 0.
		if err := e.store.Delete(e.dumpIdent); err != nil {
			return nil, fmt.Errorf("delete stale checkpoint: %w", err)
		}
		return nil, nil
	}

	// Restore splitter state from checkpoint.
	cfg := DefaultSplitterConfig()
	if cp.CurrentDelim != "" {
		cfg.Delimiter = cp.CurrentDelim
	}
	cfg.NoBackslashEscapes = cp.NoBackslashEsc
	cfg.AnsiQuotes = cp.AnsiQuotes
	cfg.Charset = cp.Charset

	e.byteOff = cp.ByteOffset
	e.statements = cp.StatementsDone
	return cp, nil
}


