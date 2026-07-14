package restoreengine

import (
	"fmt"
	"io"
)

// LexState represents the current parsing state of the statement splitter.
type LexState int

const (
	StateDefault      LexState = iota // outside any string/comment
	StateSingleQuote                  // inside ' string
	StateDoubleQuote                  // inside " string (or identifier if ansi_quotes)
	StateBacktick                     // inside ` identifier
	StateLineComment                  // after -- (until newline)
	StateHashComment                  // after # (until newline)
	StateBlockComment                 // inside /* */
	StateExecComment                  // inside /*! ... */ or /*M! ... */
)

// SplitterConfig holds the dynamic configuration of the splitter.
// These are updated by observing SET sql_mode in the stream.
type SplitterConfig struct {
	NoBackslashEscapes bool   // from sql_mode
	AnsiQuotes         bool   // from sql_mode
	Delimiter          string // current delimiter (default: ";")
	Charset            string // from SET NAMES / SET CHARACTER SET
}

// DefaultSplitterConfig returns the default configuration.
func DefaultSplitterConfig() SplitterConfig {
	return SplitterConfig{
		Delimiter: ";",
	}
}

// Statement represents a single SQL statement extracted from the dump.
type Statement struct {
	Text    []byte // the raw SQL text, including the delimiter
	IsChunk bool   // true if this is a chunk of a larger statement (mid-stream split for size)
}

// StatementCallback is called for each complete statement found.
type StatementCallback func(stmt Statement)

// Splitter is a state-aware streaming lexer that consumes a dump byte stream
// and emits statements on true boundaries. It is NOT a full SQL parser — it
// only tracks regions where the delimiter is significant.
type Splitter struct {
	cfg    SplitterConfig
	state  LexState
	buf    []byte // accumulated bytes for the current statement
	reader io.Reader
	delim  []byte // current delimiter bytes for fast comparison
}

// NewSplitter creates a new splitter reading from r with the given config.
func NewSplitter(r io.Reader, cfg SplitterConfig) *Splitter {
	return &Splitter{
		cfg:    cfg,
		state:  StateDefault,
		reader: r,
		delim:  []byte(cfg.Delimiter),
	}
}

// Config returns the current splitter configuration.
func (s *Splitter) Config() SplitterConfig { return s.cfg }

// State returns the current lexer state.
func (s *Splitter) State() LexState { return s.state }

// Run reads the entire stream, calling cb for each complete statement.
// Blocks until EOF or error.
func (s *Splitter) Run(cb StatementCallback) error {
	chunk := make([]byte, 64*1024) // 64KB read buffer
	for {
		n, err := s.reader.Read(chunk)
		if n > 0 {
			if err := s.feed(chunk[:n], cb); err != nil {
				return err
			}
		}
		if err == io.EOF {
			// Emit any remaining bytes as a final statement.
			if len(s.buf) > 0 {
				cb(Statement{Text: s.buf, IsChunk: false})
				s.buf = s.buf[:0]
			}
			return nil
		}
		if err != nil {
			return fmt.Errorf("read dump: %w", err)
		}
	}
}

// feed processes a chunk of bytes through the state machine.
func (s *Splitter) feed(data []byte, cb StatementCallback) error {
	for i := 0; i < len(data); i++ {
		b := data[i]

		switch s.state {
		case StateDefault:
			if err := s.feedDefault(b, data, &i, cb); err != nil {
				return err
			}
		case StateSingleQuote:
			s.feedSingleQuote(b, data, &i)
		case StateDoubleQuote:
			s.feedDoubleQuote(b, data, &i)
		case StateBacktick:
			s.feedBacktick(b, data, &i)
		case StateLineComment:
			s.feedLineComment(b)
		case StateHashComment:
			s.feedLineComment(b) // same as line comment
		case StateBlockComment, StateExecComment:
			s.feedBlockComment(b, data, &i)
		}
	}
	return nil
}
