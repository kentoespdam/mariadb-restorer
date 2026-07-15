package restoreengine

import (
	"context"
	"fmt"
	"strings"
)

// VerifyTables runs CHECK TABLE ... EXTENDED on all user tables in the current
// database. Returns findings with prefixes [FK] for foreign key violations and
// [CORRUPT] for structural corruption (potentially false positives per MariaDB).
// Returns empty slice if all tables pass verification or no tables exist.
func (e *Executor) VerifyTables() ([]string, error) {
	if e.conn == nil {
		return nil, fmt.Errorf("not connected: call Connect(dsn) first")
	}

	rows, err := e.conn.QueryContext(context.Background(), "SHOW TABLES")
	if err != nil {
		return nil, fmt.Errorf("show tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan table name: %w", err)
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	if len(tables) == 0 {
		return nil, nil
	}

	query := "CHECK TABLE " + strings.Join(tables, ", ") + " EXTENDED"
	checkRows, err := e.conn.QueryContext(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("check table: %w", err)
	}
	defer checkRows.Close()

	var findings []string
	for checkRows.Next() {
		var table, op, msgType, msgText string
		if err := checkRows.Scan(&table, &op, &msgType, &msgText); err != nil {
			return nil, fmt.Errorf("scan check result: %w", err)
		}
		if msgType == "status" && msgText == "OK" {
			continue
		}
		// Classify findings per CONTEXT.md: FK violations vs Corrupt.
		if strings.Contains(msgText, "foreign key constraint fails") {
			findings = append(findings, fmt.Sprintf("[FK] %s: %s", table, msgText))
		} else if strings.Contains(msgText, "Corrupt") {
			findings = append(findings, fmt.Sprintf("[CORRUPT] %s: %s", table, msgText))
		} else {
			findings = append(findings, fmt.Sprintf("[OTHER] %s (%s): %s", table, msgType, msgText))
		}
	}
	if err := checkRows.Err(); err != nil {
		return nil, fmt.Errorf("check rows iteration: %w", err)
	}

	return findings, nil
}
