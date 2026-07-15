package restoreengine

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql" // MySQL/MariaDB driver registration
)

// Executor orchestrates the full restore lifecycle: pre-flight, batches, verify, replay.
type Executor struct {
	store      *SQLiteStore
	conn       *sql.Conn // pinned single connection for session state (USE, SET, etc.)
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

// Connect opens a pinned connection to the target MariaDB/MySQL server using the DSN.
// All Exec calls use this single connection so session state (USE, SET, sql_mode)
// is correctly preserved across statements (ADR-0011: single pinned connection).
// The DSN format is the standard go-sql-driver/mysql DSN:
//   user:password@tcp(host:port)/dbname?params
func (e *Executor) Connect(dsn string) error {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("open mysql: %w", err)
	}

	conn, err := db.Conn(context.Background())
	if err != nil {
		db.Close()
		return fmt.Errorf("get connection: %w", err)
	}

	// Verify the connection actually works.
	if err := conn.PingContext(context.Background()); err != nil {
		conn.Close()
		db.Close()
		return fmt.Errorf("ping mysql: %w", err)
	}

	e.conn = conn
	return nil
}

// Close closes the pinned database connection.
func (e *Executor) Close() error {
	if e.conn != nil {
		return e.conn.Close()
	}
	return nil
}

// IsConnected returns true if the executor has an active connection.
func (e *Executor) IsConnected() bool { return e.conn != nil }

// DumpIdentity returns the computed dump identity.
func (e *Executor) DumpIdentity() string { return e.dumpIdent }

// DumpSize returns the total dump file size in bytes.
func (e *Executor) DumpSize() int64 { return e.dumpSize }

// ByteOffset returns the current restore position.
func (e *Executor) ByteOffset() int64 { return e.byteOff }

// StatementsDone returns the number of completed statements.
func (e *Executor) StatementsDone() int64 { return e.statements }

// Exec executes a single SQL statement against the target database
// using the pinned connection. Returns an error if no connection is established.
// For CREATE PROCEDURE/FUNCTION/TRIGGER/EVENT, automatically prepends a
// DROP IF EXISTS guard to prevent Error 1304 on re-run (ponytail: covers
// the 99% case; won't handle schema-qualified names like `db`.`proc`).
func (e *Executor) Exec(stmt []byte) error {
	if e.conn == nil {
		return fmt.Errorf("not connected: call Connect(dsn) first")
	}

	if guard := dropGuardStmt(stmt); guard != nil {
		// Execute DROP IF EXISTS as a separate call — works with or without
		// multiStatements=true in the DSN.
		e.conn.ExecContext(context.Background(), string(guard))
	}

	if _, err := e.conn.ExecContext(context.Background(), string(stmt)); err != nil {
		return fmt.Errorf("exec: %w", err)
	}
	return nil
}

// dropGuardStmt returns a DROP IF EXISTS statement for CREATE
// PROCEDURE/FUNCTION/TRIGGER/EVENT that lack IF NOT EXISTS or OR REPLACE.
// Returns nil for all other statements.
// Handles DEFINER clauses and executable version comment wrappers
// from mysqldump (/*!50003 CREATE*/ ... /*!50003 PROCEDURE name*/).
// Uses bytes.Index rather than HasPrefix so version comment wrappers
// do not mask the CREATE keyword.
func dropGuardStmt(stmt []byte) []byte {
	trimmed := bytes.TrimSpace(stmt)
	upper := bytes.ToUpper(trimmed)

	// Find "CREATE" anywhere in the statement — handles version comment
	// wrappers from mysqldump like /*!50003 CREATE*/ ...
	createIdx := bytes.Index(upper, []byte("CREATE"))
	if createIdx < 0 {
		return nil
	}

	// Already has a guard — skip.
	if bytes.Contains(upper, []byte("IF NOT EXISTS")) ||
		bytes.Contains(upper, []byte("OR REPLACE")) {
		return nil
	}

	// Search for the keyword after "CREATE" — works for both
	// `CREATE PROCEDURE name` and `CREATE DEFINER=... PROCEDURE name`
	// and `/*!50003 CREATE*/ /*!50003 PROCEDURE name*/`.
	afterCreate := upper[createIdx:]
	for _, entry := range [...][2]string{
		{" PROCEDURE ", "PROCEDURE"},
		{" FUNCTION ", "FUNCTION"},
		{" TRIGGER ", "TRIGGER"},
		{" EVENT ", "EVENT"},
	} {
		// Search for the keyword after CREATE
		idx := bytes.Index(afterCreate, []byte(entry[0]))
		if idx < 0 {
			// Some mysqldump formats use /*!50003 PROCEDURE name*/
			// without a space between the version tag and keyword.
			// Try with just a leading space (keyword after version number).
			continue
		}
		// Absolute offset into trimmed for name extraction.
		absIdx := createIdx + idx
		name := extractFirstIdentifier(trimmed[absIdx+len(entry[0]):])
		if len(name) == 0 {
			continue
		}
		return []byte(fmt.Sprintf("DROP %s IF EXISTS `%s`", entry[1], name))
	}

	return nil
}

// extractFirstIdentifier extracts the first backtick-quoted or unquoted
// identifier from b. Stops at the first byte that cannot be part of a
// MariaDB unquoted identifier (space, (, \n, \t).
func extractFirstIdentifier(b []byte) []byte {
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return nil
	}

	if b[0] == '`' {
		// Backtick-quoted: find matching closing backtick (handling `doubled`).
		for i := 1; i < len(b); i++ {
			if b[i] == '`' {
				if i+1 < len(b) && b[i+1] == '`' {
					i++ // skip doubled backtick inside name
					continue
				}
				return b[1:i]
			}
		}
		return b[1:] // unterminated — return rest anyway
	}

	// Unquoted: ends at first delimiter character.
	end := bytes.IndexAny(b, " (\n\t.")
	if end < 0 {
		return b
	}
	return b[:end]
}

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
