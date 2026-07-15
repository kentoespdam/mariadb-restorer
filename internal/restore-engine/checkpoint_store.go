package restoreengine

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // SQLite driver registration
)

const checkpointSchema = `
CREATE TABLE IF NOT EXISTS checkpoints (
    dump_identity   TEXT PRIMARY KEY,
    dump_path       TEXT NOT NULL,
    dump_size_bytes INTEGER NOT NULL,
    byte_offset     INTEGER NOT NULL DEFAULT 0,
    statements_done INTEGER NOT NULL DEFAULT 0,
    no_backslash_escapes INTEGER NOT NULL DEFAULT 0,
    ansi_quotes     INTEGER NOT NULL DEFAULT 0,
    current_delimiter TEXT NOT NULL DEFAULT ';',
    charset         TEXT NOT NULL DEFAULT '',
    dsn             TEXT NOT NULL DEFAULT '',
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
)`

const upsertSQL = `INSERT INTO checkpoints
    (dump_identity, dump_path, dump_size_bytes, byte_offset, statements_done,
     no_backslash_escapes, ansi_quotes, current_delimiter, charset, dsn, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
ON CONFLICT(dump_identity) DO UPDATE SET
    dump_path       = excluded.dump_path,
    dump_size_bytes = excluded.dump_size_bytes,
    byte_offset     = excluded.byte_offset,
    statements_done = excluded.statements_done,
    no_backslash_escapes = excluded.no_backslash_escapes,
    ansi_quotes     = excluded.ansi_quotes,
    current_delimiter = excluded.current_delimiter,
    charset         = excluded.charset,
    -- dsn intentionally NOT in UPDATE SET: initial checkpoint stores it,
    -- batch checkpoints (which don't carry DSN) must NOT overwrite it.
    updated_at      = datetime('now')`

const dsnMigrationSQL = `ALTER TABLE checkpoints ADD COLUMN dsn TEXT NOT NULL DEFAULT ''`

// SQLiteStore persists checkpoints to SQLite using modernc SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens or creates the checkpoint SQLite database.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	pragmas := []string{
		"PRAGMA synchronous=FULL",
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("%s: %w", p, err)
		}
	}

	if _, err := db.Exec(checkpointSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	// Migration: add dsn column for existing databases.
	db.Exec(dsnMigrationSQL) // ignore error — column may already exist

	return &SQLiteStore{db: db}, nil
}

// Get retrieves the checkpoint for a dump identity.
func (s *SQLiteStore) Get(dumpIdentity string) (*Checkpoint, error) {
	row := s.db.QueryRow(`SELECT dump_path, dump_size_bytes, dump_identity,
		byte_offset, statements_done, no_backslash_escapes, ansi_quotes,
		current_delimiter, charset, dsn, updated_at
		FROM checkpoints WHERE dump_identity = ?`, dumpIdentity)

	var cp Checkpoint
	var nb, aq int
	var updated string
	err := row.Scan(&cp.DumpPath, &cp.DumpSizeBytes, &cp.DumpIdentity,
		&cp.ByteOffset, &cp.StatementsDone, &nb, &aq,
		&cp.CurrentDelim, &cp.Charset, &cp.DSN, &updated)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get checkpoint: %w", err)
	}
	cp.NoBackslashEsc = nb != 0
	cp.AnsiQuotes = aq != 0
	cp.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updated)
	return &cp, nil
}

// Upsert creates or updates the checkpoint.
func (s *SQLiteStore) Upsert(cp *Checkpoint) error {
	nb := 0
	if cp.NoBackslashEsc {
		nb = 1
	}
	aq := 0
	if cp.AnsiQuotes {
		aq = 1
	}

	_, err := s.db.Exec(upsertSQL, cp.DumpIdentity, cp.DumpPath, cp.DumpSizeBytes,
		cp.ByteOffset, cp.StatementsDone, nb, aq, cp.CurrentDelim, cp.Charset, cp.DSN)
	if err != nil {
		return fmt.Errorf("upsert checkpoint: %w", err)
	}
	return nil
}

// Delete removes the checkpoint for a dump identity.
func (s *SQLiteStore) Delete(dumpIdentity string) error {
	_, err := s.db.Exec("DELETE FROM checkpoints WHERE dump_identity = ?", dumpIdentity)
	if err != nil {
		return fmt.Errorf("delete checkpoint: %w", err)
	}
	return nil
}

// ListAll returns all unfinished checkpoints.
func (s *SQLiteStore) ListAll() ([]*Checkpoint, error) {
	rows, err := s.db.Query(`SELECT dump_path, dump_size_bytes, dump_identity,
		byte_offset, statements_done, no_backslash_escapes, ansi_quotes,
		current_delimiter, charset, dsn, updated_at FROM checkpoints`)
	if err != nil {
		return nil, fmt.Errorf("list checkpoints: %w", err)
	}
	defer rows.Close()

	var result []*Checkpoint
	for rows.Next() {
		var cp Checkpoint
		var nb, aq int
		var updated string
		if err := rows.Scan(&cp.DumpPath, &cp.DumpSizeBytes, &cp.DumpIdentity,
			&cp.ByteOffset, &cp.StatementsDone, &nb, &aq,
			&cp.CurrentDelim, &cp.Charset, &cp.DSN, &updated); err != nil {
			return nil, fmt.Errorf("scan checkpoint: %w", err)
		}
		cp.NoBackslashEsc = nb != 0
		cp.AnsiQuotes = aq != 0
		cp.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updated)
		result = append(result, &cp)
	}
	return result, rows.Err()
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
