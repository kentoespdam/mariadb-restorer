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
	DSN      string // MariaDB connection string (may be empty if not connected)
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

		startTime := time.Now()
		var batchCount int64
		var deferredCount int

		runErr := runWithProgress(ctx, f, executor, store, startTime, &batchCount, &deferredCount, events)
		if runErr != nil {
			events <- ProgressEvent{Done: true, Err: runErr}
			return
		}

		events <- ProgressEvent{
			ByteOffset:     executor.ByteOffset(),
			DumpSizeBytes:  executor.DumpSize(),
			StatementsDone: executor.StatementsDone(),
			BatchCount:     batchCount,
			DeferredCount:  deferredCount,
			Elapsed:        time.Since(startTime),
			Done:           true,
		}
	}()
}

// runWithProgress executes the splitter and emits progress after every batch.
func runWithProgress(ctx context.Context, f *os.File, executor *Executor, store *SQLiteStore,
	startTime time.Time, batchCount *int64, deferredCount *int, events chan<- ProgressEvent) error {

	var batchStmts int64
	var batchBytes int64

	cfg := DefaultSplitterConfig()
	splitter := NewSplitter(f, cfg)

	return splitter.Run(func(stmt Statement) {
		// Check cancellation first.
		if err := ctx.Err(); err != nil {
			events <- ProgressEvent{Done: true, Err: err}
			return
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
				events <- ProgressEvent{Done: true, Err: fmt.Errorf("checkpoint: %w", err)}
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
}
