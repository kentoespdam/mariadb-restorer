package tuiprofiles

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
)

func (e *EditorScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if e.saved {
		return e, func() tea.Msg { return base.NavigateBackMsg{} }
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return e.handleKey(msg)
	}
	updated, cmd := e.inputs[e.focused].Update(msg)
	e.inputs[e.focused] = updated
	return e, cmd
}

func (e *EditorScreen) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		return e.focusNext(1)
	case "shift+tab":
		return e.focusNext(-1)
	case "enter":
		if err := e.save(); err != nil {
			e.err = err.Error()
			return e, nil
		}
		e.saved = true
		return e, nil
	case "esc":
		return e, func() tea.Msg { return base.NavigateBackMsg{} }
	case "ctrl+x":
		if e.hasPwd {
			e.clearPwd = true
			e.hasPwd = false
			e.err = "Password will be cleared on save."
		}
		return e, nil
	}
	updated, cmd := e.inputs[e.focused].Update(msg)
	e.inputs[e.focused] = updated
	return e, cmd
}

func (e *EditorScreen) focusNext(delta int) (tea.Model, tea.Cmd) {
	nf := int(e.focused) + delta
	if nf < 0 {
		nf = maxFields - 1
	} else if nf >= maxFields {
		nf = 0
	}
	e.focused = fieldIndex(nf)
	for i := range e.inputs {
		if i == int(e.focused) {
			e.inputs[i].Focus()
		} else {
			e.inputs[i].Blur()
		}
	}
	return e, textinput.Blink
}
