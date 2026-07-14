package tui

import (
	"errors"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"
	"golang.org/x/term"
)

// ErrNoTTY is returned when running TUI in a non-interactive environment.
var ErrNoTTY = errors.New("TUI mode requires a terminal (TTY)")

const (
	MinWidth  = 80
	MinHeight = 24
)

// ErrTermTooSmall is returned when the terminal is too small.
var ErrTermTooSmall = fmt.Errorf("terminal must be at least %dx%d", MinWidth, MinHeight)

// Run starts the TUI application. Detects TTY, checks size, then runs Bubble Tea.
func Run(dataDir string, demo bool) error {
	if !isatty.IsTerminal(os.Stdin.Fd()) && !isatty.IsCygwinTerminal(os.Stdin.Fd()) {
		return ErrNoTTY
	}

	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		w, h = 80, 24
	}
	if w < MinWidth || h < MinHeight {
		return ErrTermTooSmall
	}

	router, err := NewRouter(dataDir, demo)
	if err != nil {
		return fmt.Errorf("init TUI: %w", err)
	}

	p := tea.NewProgram(router,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err = p.Run()
	return err
}
