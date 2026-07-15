package restoreengine

import (
	"bytes"
)

// delimiterMatch checks if data[i:] starts with the current delimiter.
func (s *Splitter) delimiterMatch(data []byte, i int) bool {
	return bytes.HasPrefix(data[i:], s.delim)
}

// isDelimiterByte returns true if b could start the delimiter.
func (s *Splitter) isDelimiterByte(b byte) bool {
	return b == s.delim[0]
}

// feedDefault handles the default state (outside strings/comments).
func (s *Splitter) feedDefault(b byte, data []byte, i *int, cb StatementCallback) error {
	s.buf = append(s.buf, b)

	switch {
	case b == '\'':
		s.state = StateSingleQuote

	case b == '"':
		if s.cfg.AnsiQuotes {
			s.state = StateBacktick // ansi_quotes: " acts as identifier quote
		} else {
			s.state = StateDoubleQuote
		}

	case b == '`':
		s.state = StateBacktick

	case b == '-' && *i+1 < len(data) && data[*i+1] == '-':
		s.state = StateLineComment

	case b == '#':
		s.state = StateHashComment

	case b == '/' && *i+1 < len(data) && data[*i+1] == '*':
		// Check for executable comment: /*! or /*M!
		if *i+2 < len(data) && (data[*i+2] == '!' || (data[*i+2] == 'M' && *i+3 < len(data) && data[*i+3] == '!')) {
			s.state = StateExecComment
		} else {
			s.state = StateBlockComment
		}
		// Consume the /* we already appended; the * or M will be consumed normally.

	case b == '\n':
		s.maybeHandleDelimiter(cb)

	case s.isDelimiterByte(b) && s.delimiterMatch(data, *i):
		// Advance past all delimiter bytes (important for multi-char delimiters).
		for j := 1; j < len(s.delim); j++ {
			*i++
		}
		// Remove delimiter bytes from buffer (only first byte was appended).
		s.buf = s.buf[:len(s.buf)-1]
		stmt := Statement{Text: s.buf, IsChunk: false}
		// Check for DELIMITER command (client-side, consumed).
		if isDelimiterCommand(s.buf) {
			s.handleDelimiterCommand(s.buf)
			s.buf = s.buf[:0]
			return nil
		}
		// Check for SET sql_mode / SET NAMES (observed but forwarded).
		if isSetStatement(s.buf) {
			s.observeSet(s.buf)
		}
		s.buf = s.buf[:0]
		cb(stmt)
	}

	return nil
}

// feedSingleQuote handles single-quoted string state.
func (s *Splitter) feedSingleQuote(b byte, data []byte, i *int) {
	s.buf = append(s.buf, b)

	if b == '\\' && !s.cfg.NoBackslashEscapes {
		// Backslash escape — consume next byte as literal.
		if *i+1 < len(data) {
			*i++
			s.buf = append(s.buf, data[*i])
		}
		return
	}

	if b == '\'' {
		// Check for doubled quote '' (escaped quote in SQL).
		if *i+1 < len(data) && data[*i+1] == '\'' {
			*i++
			s.buf = append(s.buf, '\'')
			return
		}
		s.state = StateDefault
	}
}

// feedDoubleQuote handles double-quoted string state.
func (s *Splitter) feedDoubleQuote(b byte, data []byte, i *int) {
	s.buf = append(s.buf, b)

	if b == '\\' && !s.cfg.NoBackslashEscapes {
		if *i+1 < len(data) {
			*i++
			s.buf = append(s.buf, data[*i])
		}
		return
	}

	if b == '"' {
		if *i+1 < len(data) && data[*i+1] == '"' {
			*i++
			s.buf = append(s.buf, '"')
			return
		}
		s.state = StateDefault
	}
}

// feedBacktick handles backtick-quoted identifier state.
func (s *Splitter) feedBacktick(b byte, data []byte, i *int) {
	s.buf = append(s.buf, b)

	if b == '`' {
		// Check for doubled backtick `` (escaped).
		if *i+1 < len(data) && data[*i+1] == '`' {
			*i++
			s.buf = append(s.buf, '`')
			return
		}
		s.state = StateDefault
	}
}

// feedBlockComment handles block comments (/* */) and executable comments.
func (s *Splitter) feedBlockComment(b byte, data []byte, i *int) {
	s.buf = append(s.buf, b)

	if b == '*' && *i+1 < len(data) && data[*i+1] == '/' {
		*i++
		s.buf = append(s.buf, '/')
		s.state = StateDefault
	}
}

// feedLineComment handles line comments (-- and #).
// After the newline, also checks for DELIMITER commands so subsequent
// standalone DELIMITER lines are detected even when preceded by comments.
func (s *Splitter) feedLineComment(b byte) {
	s.buf = append(s.buf, b)
	if b == '\n' {
		s.state = StateDefault
		// Don't call maybeHandleDelimiter here — it's called from feedDefault
		// when the next newline is encountered.
	}
}

// maybeHandleDelimiter checks if a newline (from default state) completes
// a statement that ends with the delimiter, OR if the buffer contains a
// standalone DELIMITER command preceded only by whitespace and comments.
func (s *Splitter) maybeHandleDelimiter(cb StatementCallback) {
	// Check if buffer contains a DELIMITER command on its own line,
	// preceded only by whitespace/comment content (which we discard).
	if line := extractDelimiterLine(s.buf); line != nil {
		s.handleDelimiterCommand(line)
		s.buf = s.buf[:0]
		return
	}

	// If buffer ends with delimiter, treat as complete statement.
	if len(s.buf) >= len(s.delim) && bytes.HasSuffix(s.buf, s.delim) {
		trimmed := s.buf[:len(s.buf)-len(s.delim)]
		s.buf = s.buf[:0]
		if len(trimmed) > 0 {
			stmt := Statement{Text: trimmed, IsChunk: false}
			if isSetStatement(trimmed) {
				s.observeSet(trimmed)
			}
			cb(stmt)
		}
	}
}

// extractDelimiterLine searches the buffer for a standalone DELIMITER command
// line preceded only by whitespace, empty lines, or comments. Returns the
// DELIMITER line if found, nil otherwise.
func extractDelimiterLine(buf []byte) []byte {
	lines := bytes.Split(buf, []byte("\n"))
	for i, line := range lines {
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			continue
		}
		// Check if this line is a DELIMITER command.
		upper := bytes.ToUpper(trimmed)
		if bytes.HasPrefix(upper, []byte("DELIMITER ")) {
			// Verify all preceding lines are empty or comments.
			precedingOK := true
			for j := 0; j < i; j++ {
				t := bytes.TrimSpace(lines[j])
				if len(t) > 0 && !bytes.HasPrefix(t, []byte("--")) && !bytes.HasPrefix(t, []byte("#")) {
					precedingOK = false
					break
				}
			}
			if precedingOK {
				return trimmed
			}
		}
		// If this line is NOT a comment and NOT empty, stop searching —
		// DELIMITER must be on its own line preceded only by comments.
		if !bytes.HasPrefix(trimmed, []byte("--")) && !bytes.HasPrefix(trimmed, []byte("#")) {
			return nil
		}
	}
	return nil
}
