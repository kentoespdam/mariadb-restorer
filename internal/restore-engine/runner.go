package restoreengine

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"
)

// RestoreConfig holds parameters for running a restore.
type RestoreConfig struct {
	DataDir  string
	DumpPath string
	DSN      string // MariaDB connection string (user:password@tcp(host:port)/dbname?params)
	Verify   bool   // run CHECK TABLE EXTENDED after restore
}

// RunRestoreAsync runs the restore engine in a goroutine, sending progress
// events to the provided channel. Closes the channel when done.
// The ctx is checked for cancellation on each statement boundary.
func RunRestoreAsync(ctx context.Context, cfg RestoreConfig, events chan<- ProgressEvent) {
	go func() {
		defer close(events)

		store, err := NewSQLiteStore(cfg.DataDir + "/mariadb-restorer.db")
		if err != nil {
			events <- ProgressEvent{Done: true, Err: fmt.Errorf("open store: %w", err)}
			return
		}
		defer store.Close()

		executor, err := NewExecutor(store, cfg.DumpPath)
		if err != nil {
			events <- ProgressEvent{Done: true, Err: fmt.Errorf("create executor: %w", err)}
			return
		}

		// Connect to target database if DSN is provided.
		if cfg.DSN != "" {
			if err := executor.Connect(cfg.DSN); err != nil {
				events <- ProgressEvent{Done: true, Err: fmt.Errorf("connect: %w", err)}
				return
			}
			defer executor.Close()
		}

		// Resume from checkpoint.
		cp, err := executor.ResumeFromCheckpoint()
		if err != nil {
			events <- ProgressEvent{Done: true, Err: fmt.Errorf("resume checkpoint: %w", err)}
			return
		}
		if cp != nil {
			events <- ProgressEvent{
				ByteOffset:     cp.ByteOffset,
				DumpSizeBytes:  cp.DumpSizeBytes,
				StatementsDone: cp.StatementsDone,
				Elapsed:        time.Since(cp.UpdatedAt),
			}
		}

		f, err := os.Open(cfg.DumpPath)
		if err != nil {
			events <- ProgressEvent{Done: true, Err: fmt.Errorf("open dump: %w", err)}
			return
		}
		defer f.Close()

		if _, err := f.Seek(executor.ByteOffset(), io.SeekStart); err != nil {
			events <- ProgressEvent{Done: true, Err: fmt.Errorf("seek: %w", err)}
			return
		}

	// Save initial checkpoint with DSN so resume from history works.
	initCp := &Checkpoint{
		DumpPath:       executor.dumpPath,
		DumpSizeBytes:  executor.dumpSize,
		DumpIdentity:   executor.dumpIdent,
		ByteOffset:     executor.ByteOffset(),
		StatementsDone: executor.StatementsDone(),
		CurrentDelim:   ";",
		DSN:            cfg.DSN,
	}
	if err := store.Upsert(initCp); err != nil {
		events <- ProgressEvent{Done: true, Err: fmt.Errorf("initial checkpoint: %w", err)}
		return
	}

	startTime := time.Now()
	var batchCount int64
	var deferredCount int

	runErr := runWithProgress(ctx, f, executor, store, startTime, &batchCount, &deferredCount, events)
	if runErr != nil {
		events <- ProgressEvent{Done: true, Err: runErr}
		return
	}

	// All statements processed — mark byteOff as complete so Home screen
	// detects "Completed" status (byteOff tracks statement text length, not
	// file position, so it never reaches dumpSize on its own).
	executor.byteOff = executor.dumpSize

	// Save final checkpoint so restore appears in Home screen history
	// even when batch thresholds weren't reached. Do this BEFORE verify
	// so the restore history is preserved even if the optional verify step fails.
	finalCp := &Checkpoint{
		DumpPath:       executor.dumpPath,
		DumpSizeBytes:  executor.dumpSize,
		DumpIdentity:   executor.dumpIdent,
		ByteOffset:     executor.ByteOffset(),
		StatementsDone: executor.StatementsDone(),
		CurrentDelim:   ";", // hardcoded: completed checkpoint, never resumed
		DSN:            cfg.DSN,
	}
	if err := store.Upsert(finalCp); err != nil {
		events <- ProgressEvent{Done: true, Err: fmt.Errorf("final checkpoint: %w", err)}
		return
	}

	// Run verification if enabled and connected.
	var verifyFindings []string
	if cfg.Verify && executor.IsConnected() {
		vf, err := executor.VerifyTables()
		if err != nil {
			events <- ProgressEvent{Done: true, Err: fmt.Errorf("verify: %w", err)}
			return
		}
		verifyFindings = vf
	}

	events <- ProgressEvent{
		ByteOffset:     executor.ByteOffset(),
		DumpSizeBytes:  executor.DumpSize(),
		StatementsDone: executor.StatementsDone(),
		BatchCount:     batchCount,
		DeferredCount:  deferredCount,
		Elapsed:        time.Since(startTime),
		Done:           true,
		VerifyFindings: verifyFindings,
	}
	}()
}

// runWithProgress executes the splitter and emits progress after every batch.
// Returns an error if any SQL statement fails, so RunRestoreAsync does NOT
// save a false-success checkpoint or send a misleading Done event.
func runWithProgress(ctx context.Context, f *os.File, executor *Executor, store *SQLiteStore,
	startTime time.Time, batchCount *int64, deferredCount *int, events chan<- ProgressEvent) error {

	var batchStmts int64
	var batchBytes int64
	var execErr error // tracks first SQL exec error to prevent false-success Done

	cfg := DefaultSplitterConfig()
	splitter := NewSplitter(f, cfg)

	splitErr := splitter.Run(func(stmt Statement) {
		// Skip processing if a previous statement already failed.
		if execErr != nil {
			return
		}

		// Check cancellation first.
		if err := ctx.Err(); err != nil {
			execErr = err
			events <- ProgressEvent{Done: true, Err: err}
			return
		}

		// Execute the SQL statement against the pinned connection.
		if executor.IsConnected() {
			if err := executor.Exec(stmt.Text); err != nil {
				execErr = fmt.Errorf(
					"at offset %d (stmt %d): %w",
					executor.ByteOffset(), executor.StatementsDone()+1, err)
				events <- ProgressEvent{Done: true, Err: execErr}
				return
			}
		}

		executor.statements++
		executor.byteOff += int64(len(stmt.Text))
		batchStmts++
		batchBytes += int64(len(stmt.Text))

		// Commit batch at thresholds (~64MB or ~1000 statements).
		if batchBytes >= 64*1024*1024 || batchStmts >= 1000 {
			*batchCount++

			cp := &Checkpoint{
				DumpPath:       executor.dumpPath,
				DumpSizeBytes:  executor.dumpSize,
				DumpIdentity:   executor.dumpIdent,
				ByteOffset:     executor.ByteOffset(),
				StatementsDone: executor.StatementsDone(),
				CurrentDelim:   splitter.Config().Delimiter,
				Charset:        splitter.Config().Charset,
				NoBackslashEsc: splitter.Config().NoBackslashEscapes,
				AnsiQuotes:     splitter.Config().AnsiQuotes,
			}
			if err := store.Upsert(cp); err != nil {
				execErr = fmt.Errorf("checkpoint: %w", err)
				events <- ProgressEvent{Done: true, Err: execErr}
				f.Close() // force splitter to stop
				return
			}

			events <- ProgressEvent{
				ByteOffset:     executor.ByteOffset(),
				DumpSizeBytes:  executor.DumpSize(),
				StatementsDone: executor.StatementsDone(),
				BatchCount:     *batchCount,
				DeferredCount:  *deferredCount,
				Elapsed:        time.Since(startTime),
			}

			batchBytes = 0
			batchStmts = 0
		}
	})

	if splitErr != nil {
		return splitErr
	}
	return execErr
}
