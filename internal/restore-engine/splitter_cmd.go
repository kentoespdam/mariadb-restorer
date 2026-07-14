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
		// parts[0] is "DELIMITER", parts[1] is the new delimiter.
		newDelim := string(parts[1])
		s.cfg.Delimiter = newDelim
		s.delim = []byte(newDelim)
	}
	// parts beyond the second are ignored per SQL convention.
}

// observeSet updates the splitter config based on SET statements.
// It observes SET sql_mode and SET NAMES / SET CHARACTER SET.
func (s *Splitter) observeSet(buf []byte) {
	lower := strings.ToLower(string(buf))

	// SET NAMES <charset>
	if strings.Contains(lower, "set names ") {
		parts := strings.Fields(lower)
		for i, p := range parts {
			if p == "names" && i+1 < len(parts) {
				s.cfg.Charset = strings.TrimRight(strings.TrimRight(parts[i+1], ";"), ",")
				return
			}
		}
	}

	// SET CHARACTER SET <charset>
	if strings.Contains(lower, "character set ") && !strings.Contains(lower, "character_set_results") {
		parts := strings.Fields(lower)
		for i, p := range parts {
			if p == "set" && i+1 < len(parts) && parts[i+1] == "character" && i+2 < len(parts) && parts[i+2] == "set" && i+3 < len(parts) {
				s.cfg.Charset = strings.TrimRight(strings.TrimRight(parts[i+3], ";"), ",")
				return
			}
		}
	}

	// SET sql_mode = '...' or SET SESSION sql_mode = '...'
	if strings.Contains(lower, "sql_mode") {
		s.parseSQLMode(lower)
	}
}

// parseSQLMode extracts no_backslash_escapes and ansi_quotes from sql_mode.
func (s *Splitter) parseSQLMode(lower string) {
	// Extract the value part: everything after '='.
	eqIdx := strings.IndexByte(lower, '=')
	if eqIdx < 0 {
		return
	}
	val := strings.TrimSpace(lower[eqIdx+1:])
	// Strip surrounding quotes.
	val = strings.Trim(val, "'\"")
	// Split on comma.
	modes := strings.Split(val, ",")

	// Reset to defaults first.
	s.cfg.NoBackslashEscapes = false
	s.cfg.AnsiQuotes = false

	for _, m := range modes {
		m = strings.TrimSpace(m)
		switch strings.ToUpper(m) {
		case "NO_BACKSLASH_ESCAPES":
			s.cfg.NoBackslashEscapes = true
		case "ANSI_QUOTES":
			s.cfg.AnsiQuotes = true
		}
	}
}
