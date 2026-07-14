package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
)

type emptyScreen struct{}

func (e *emptyScreen) Init() tea.Cmd                       { return nil }
func (e *emptyScreen) Update(tea.Msg) (tea.Model, tea.Cmd) { return e, nil }
func (e *emptyScreen) View() string                        { return "" }
func (e *emptyScreen) ID() base.ScreenID                   { return ScreenHome }
func (e *emptyScreen) Footer() []base.FooterHint           { return nil }
func (e *emptyScreen) Title() string                       { return "mariadb-restorer" }
