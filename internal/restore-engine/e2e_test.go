//go:build e2e

package restoreengine

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func TestE2E_RestoreDumpToMariaDB(t *testing.T) {
	dsn := "root:rootpassword@tcp(127.0.0.1:3307)/exampledb?multiStatements=true&parseTime=true"
	tmpDir := t.TempDir()
	dumpPath := "../../testdata/dump.sql"

	t.Log("=== E2E Test: Restore dump.sql to MariaDB ===")
	t.Logf("DSN:     %s", dsn)
	t.Logf("Dump:    %s", dumpPath)
	t.Logf("DataDir: %s", tmpDir)

	// 1. Connect to MariaDB and prepare
	db, err := sql.Open("mysql", "root:rootpassword@tcp(127.0.0.1:3307)/?multiStatements=true")
	if err != nil {
		t.Fatalf("connect to MariaDB: %v", err)
	}
	defer db.Close()

	db.Exec("DROP DATABASE IF EXISTS test_restore")
	db.Exec("CREATE DATABASE IF NOT EXISTS exampledb")
	t.Log("✓ MariaDB connected, databases prepared")

	// 2. Run restore
	events := make(chan ProgressEvent, 100)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	RunRestoreAsync(ctx, RestoreConfig{
		DataDir:  tmpDir,
		DumpPath: dumpPath,
		DSN:      dsn,
		Verify:   false,
	}, events)

	var lastEv ProgressEvent
	for ev := range events {
		lastEv = ev
		if ev.Done && ev.Err != nil {
			t.Fatalf("restore failed: %v", ev.Err)
		}
	}

	elapsed := time.Since(start)
	t.Logf("✓ Restore completed in %v", elapsed)
	t.Logf("  Statements: %d", lastEv.StatementsDone)
	t.Logf("  Bytes:      %d", lastEv.ByteOffset)
	t.Logf("  Batches:    %d", lastEv.BatchCount)

	// 3. Verify data
	t.Log()
	t.Log("=== Verification ===")

	db2, err := sql.Open("mysql", "root:rootpassword@tcp(127.0.0.1:3307)/test_restore")
	if err != nil {
		t.Fatalf("connect to test_restore: %v", err)
	}
	defer db2.Close()

	// Check tables
	rows, err := db2.Query("SHOW TABLES")
	if err != nil {
		t.Fatalf("SHOW TABLES: %v", err)
	}
	t.Log("Tables:")
	var tables []string
	for rows.Next() {
		var name string
		rows.Scan(&name)
		tables = append(tables, name)
		t.Logf("  - %s", name)
	}
	rows.Close()

	expectedTables := map[string]bool{"categories": false, "products": false, "product_summary": false}
	for _, t := range tables {
		if _, ok := expectedTables[t]; ok {
			expectedTables[t] = true
		}
	}
	for tbl, found := range expectedTables {
		if !found {
			t.Errorf("missing table: %s", tbl)
		}
	}

	// Count rows
	var count int
	db2.QueryRow("SELECT COUNT(*) FROM products").Scan(&count)
	t.Logf("Products count: %d (expected 18)", count)
	if count != 18 {
		t.Errorf("expected 18 products, got %d", count)
	}

	// Check triggers
	var triggerCount int
	db2.QueryRow("SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA = 'test_restore'").Scan(&triggerCount)
	t.Logf("Triggers: %d (expected 1)", triggerCount)

	// Check procedures
	var procCount int
	db2.QueryRow("SELECT COUNT(*) FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA = 'test_restore' AND ROUTINE_TYPE = 'PROCEDURE'").Scan(&procCount)
	t.Logf("Procedures: %d (expected 2)", procCount)

	// Check functions
	var funcCount int
	db2.QueryRow("SELECT COUNT(*) FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA = 'test_restore' AND ROUTINE_TYPE = 'FUNCTION'").Scan(&funcCount)
	t.Logf("Functions: %d (expected 1)", funcCount)

	// Check views
	var viewCount int
	db2.QueryRow("SELECT COUNT(*) FROM information_schema.VIEWS WHERE TABLE_SCHEMA = 'test_restore'").Scan(&viewCount)
	t.Logf("Views: %d (expected 1)", viewCount)

	// Check events
	var eventCount int
	db2.QueryRow("SELECT COUNT(*) FROM information_schema.EVENTS WHERE EVENT_SCHEMA = 'test_restore'").Scan(&eventCount)
	t.Logf("Events: %d (expected 1)", eventCount)

	// Sample data
	var productName string
	db2.QueryRow("SELECT name FROM products WHERE id = 1").Scan(&productName)
	t.Logf("Sample product: %q", productName)
}
