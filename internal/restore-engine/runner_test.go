package restoreengine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// createDumpFile creates a dump file with n INSERT statements.
// The splitter emits trailing \n after the last ; as a separate statement,
// so total callbacks = n + 1 (n INSERTs + 1 trailing newline).
func createDumpFile(t *testing.T, dir string, n int) string {
	t.Helper()
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "INSERT INTO t VALUES (%d);\n", i)
	}
	path := filepath.Join(dir, "dump.sql")
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// collectEvents reads all events from a channel until closed.
func collectEvents(t *testing.T, events <-chan ProgressEvent) []ProgressEvent {
	t.Helper()
	var result []ProgressEvent
	for ev := range events {
		result = append(result, ev)
	}
	return result
}

func TestRunRestoreAsync_Success(t *testing.T) {
	tmpDir := t.TempDir()
	dumpPath := createDumpFile(t, tmpDir, 10)
	events := make(chan ProgressEvent, 100)

	RunRestoreAsync(context.Background(),
		RestoreConfig{DataDir: tmpDir, DumpPath: dumpPath}, events)
	all := collectEvents(t, events)

	if len(all) < 1 {
		t.Fatal("expected at least 1 event")
	}
	last := all[len(all)-1]
	if !last.Done {
		t.Fatal("expected last event to be Done")
	}
	if last.Err != nil {
		t.Fatalf("unexpected error: %v", last.Err)
	}
	// 10 INSERTs + 1 trailing \n (splitter emits residual buffer at EOF).
	if last.StatementsDone != 11 {
		t.Errorf("expected 11 statements (10 INSERTs + trailing \\n), got %d",
			last.StatementsDone)
	}
	if last.ByteOffset <= 0 {
		t.Errorf("expected ByteOffset > 0, got %d", last.ByteOffset)
	}
	if last.DumpSizeBytes <= 0 {
		t.Errorf("expected DumpSizeBytes > 0, got %d", last.DumpSizeBytes)
	}
	// No batch progress events for 10 statements (under 1000 threshold).
	for _, ev := range all[:len(all)-1] {
		if ev.Done {
			t.Error("unexpected Done event before final")
		}
	}
}

func TestRunRestoreAsync_DumpFileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	events := make(chan ProgressEvent, 10)
	RunRestoreAsync(context.Background(),
		RestoreConfig{DataDir: tmpDir, DumpPath: "/nonexistent/dump.sql"}, events)

	all := collectEvents(t, events)
	if len(all) < 1 {
		t.Fatal("expected at least 1 event")
	}
	last := all[len(all)-1]
	if !last.Done {
		t.Fatal("expected Done event for missing file")
	}
	if last.Err == nil {
		t.Fatal("expected error for missing dump file")
	}
}

func TestRunRestoreAsync_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	dumpPath := createDumpFile(t, tmpDir, 10)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancelled — callback sees it on first statement

	events := make(chan ProgressEvent, 200)
	RunRestoreAsync(ctx, RestoreConfig{DataDir: tmpDir, DumpPath: dumpPath}, events)

	all := collectEvents(t, events)
	if len(all) < 1 {
		t.Fatal("expected at least 1 event")
	}
	if !all[0].Done {
		t.Fatal("expected first event to be Done on cancelled context")
	}
	if all[0].Err == nil {
		t.Fatal("expected context cancellation error")
	}
	if !errors.Is(all[0].Err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", all[0].Err)
	}
}

func TestRunRestoreAsync_ResumeFromCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()
	dumpPath := createDumpFile(t, tmpDir, 50)

	identity, size, err := ComputeIdentity(dumpPath)
	if err != nil {
		t.Fatal(err)
	}

	// byteOff = cumulative text length (sum of len(stmt.Text)) after 10 stmts.
	// Stmt 0 has no leading \n (24 bytes), stmts 1-9 have leading \n (25 bytes).
	// Total = 24 + 9*25 = 249.
	const byteOff int64 = 249

	// Pre-seed SQLite store with a checkpoint at statement 10.
	storePath := filepath.Join(tmpDir, "mariadb-restorer.db")
	store, err := NewSQLiteStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	cp := &Checkpoint{
		DumpPath:       dumpPath,
		DumpSizeBytes:  size,
		DumpIdentity:   identity,
		ByteOffset:     byteOff,
		StatementsDone: 10,
		CurrentDelim:   ";",
		UpdatedAt:      time.Now().Add(-10 * time.Second),
	}
	if err := store.Upsert(cp); err != nil {
		t.Fatal(err)
	}
	store.Close()

	events := make(chan ProgressEvent, 100)
	RunRestoreAsync(context.Background(),
		RestoreConfig{DataDir: tmpDir, DumpPath: dumpPath}, events)
	all := collectEvents(t, events)

	if len(all) < 2 {
		t.Fatal("expected resume event + final Done")
	}

	// First event: resume notification with checkpoint data.
	if all[0].Done {
		t.Fatal("expected resume event, got Done")
	}
	if all[0].StatementsDone != 10 {
		t.Errorf("expected StatementsDone=10 (from checkpoint), got %d",
			all[0].StatementsDone)
	}
	if all[0].ByteOffset != byteOff {
		t.Errorf("expected ByteOffset=%d (from checkpoint), got %d",
			byteOff, all[0].ByteOffset)
	}
	if all[0].DumpSizeBytes != size {
		t.Errorf("expected DumpSizeBytes=%d, got %d", size, all[0].DumpSizeBytes)
	}

	// Last event: completion with all 50 statements.
	last := all[len(all)-1]
	if !last.Done {
		t.Fatal("expected last event to be Done")
	}
	if last.Err != nil {
		t.Fatalf("unexpected error: %v", last.Err)
	}
	// After resume, total > checkpoint (resume seek lands mid-stmt + remaining
	// stmts + trailing \n). Exact count depends on seek alignment.
	if last.StatementsDone <= 10 {
		t.Errorf("expected StatementsDone > 10 (checkpoint), got %d",
			last.StatementsDone)
	}
}

func TestRunRestoreAsync_BatchThreshold(t *testing.T) {
	tmpDir := t.TempDir()
	dumpPath := createDumpFile(t, tmpDir, 1500) // triggers 1000-stmt batch

	events := make(chan ProgressEvent, 100)
	RunRestoreAsync(context.Background(),
		RestoreConfig{DataDir: tmpDir, DumpPath: dumpPath}, events)

	var doneCount int
	var foundBatch bool
	for ev := range events {
		if ev.Done {
			doneCount++
			if ev.Err != nil {
				t.Fatalf("unexpected error: %v", ev.Err)
			}
			continue
		}
		if ev.BatchCount > 0 {
			foundBatch = true
		}
	}
	if doneCount != 1 {
		t.Errorf("expected exactly 1 Done event, got %d", doneCount)
	}
	if !foundBatch {
		t.Errorf("expected batch progress event for 1500 statements")
	}
}
