// Package restoreengine provides core restore logic for SQL dump files.
package restoreengine

import (
	"bytes"
	"strings"
)

// isDelimiterCommand checks if the buffer contains a DELIMITER command.
func isDelimiterCommand(buf []byte) bool {
	trimmed := bytes.TrimSpace(buf)
	upper := bytes.ToUpper(trimmed)
	return bytes.HasPrefix(upper, []byte("DELIMITER "))
}

// isSetStatement checks if the buffer starts with SET.
func isSetStatement(buf []byte) bool {
	trimmed := bytes.TrimSpace(buf)
	upper := bytes.ToUpper(trimmed)
	return bytes.HasPrefix(upper, []byte("SET "))
}

// handleDelimiterCommand extracts the new delimiter from a DELIMITER command.
func (s *Splitter) handleDelimiterCommand(buf []byte) {
	trimmed := bytes.TrimSpace(buf)
	parts := bytes.Fields(trimmed)
	if len(parts) >= 2 {
		newDelim := string(parts[1])
		s.cfg.Delimiter = newDelim
		s.delim = []byte(newDelim)
	}
}

// observeSet updates the splitter config based on SET statements.
// It observes SET sql_mode and SET NAMES / SET CHARACTER SET.
func (s *Splitter) observeSet(buf []byte) {
	lower := strings.ToLower(string(buf))

	if strings.Contains(lower, "set names ") {
		parts := strings.Fields(lower)
		for i, p := range parts {
			if p == "names" && i+1 < len(parts) {
				s.cfg.Charset = strings.TrimRight(strings.TrimRight(parts[i+1], ";"), ",")
				return
			}
		}
	}

	if strings.Contains(lower, "character set ") && !strings.Contains(lower, "character_set_results") {
		parts := strings.Fields(lower)
		for i, p := range parts {
			if p == "set" && i+1 < len(parts) && parts[i+1] == "character" && i+2 < len(parts) && parts[i+2] == "set" && i+3 < len(parts) {
				s.cfg.Charset = strings.TrimRight(strings.TrimRight(parts[i+3], ";"), ",")
				return
			}
		}
	}

	if strings.Contains(lower, "sql_mode") {
		s.parseSQLMode(lower)
	}
}

// parseSQLMode extracts lexer-relevant modes from sql_mode.
// Detects: NO_BACKSLASH_ESCAPES, ANSI_QUOTES, PIPES_AS_CONCAT, ORACLE.
func (s *Splitter) parseSQLMode(lower string) {
	eqIdx := strings.IndexByte(lower, '=')
	if eqIdx < 0 {
		return
	}
	val := strings.TrimSpace(lower[eqIdx+1:])
	val = strings.Trim(val, "'\"")
	modes := strings.Split(val, ",")

	// Reset to defaults first.
	s.cfg.NoBackslashEscapes = false
	s.cfg.AnsiQuotes = false
	s.cfg.PipesAsConcat = false

	for _, m := range modes {
		m = strings.TrimSpace(m)
		switch strings.ToUpper(m) {
		case "NO_BACKSLASH_ESCAPES":
			s.cfg.NoBackslashEscapes = true
		case "ANSI_QUOTES":
			s.cfg.AnsiQuotes = true
		case "PIPES_AS_CONCAT":
			s.cfg.PipesAsConcat = true
		case "ORACLE":
			// ORACLE compatibility enables ANSI_QUOTES, PIPES_AS_CONCAT,
			// NO_BACKSLASH_ESCAPES, IGNORE_SPACE, and more.
			s.cfg.AnsiQuotes = true
			s.cfg.PipesAsConcat = true
			s.cfg.NoBackslashEscapes = true
		}
	}
}
