package tuiprofiles

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	credentialvault "github.com/kentoespdam/mariadb-restorer/internal/credential-vault"
	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
)

const maxFields = 5

type fieldIndex int

const (
	fieldName fieldIndex = iota
	fieldHost
	fieldPort
	fieldUser
	fieldDatabase
)

// EditorScreen provides a form to create or edit a connection profile.
type EditorScreen struct {
	inputs   [maxFields]textinput.Model
	focused  fieldIndex
	dataDir  string
	origName string
	hasPwd   bool
	err      string
	saved    bool
}

// NewEditorScreen creates a profile editor, pre-populated if profile is given.
func NewEditorScreen(dataDir string, profile *credentialvault.Profile, edit bool) *EditorScreen {
	e := &EditorScreen{dataDir: dataDir, inputs: [maxFields]textinput.Model{}}
	labels := []string{"Name", "Host", "Port", "User", "Database"}
	for i := range e.inputs {
		ti := textinput.New()
		ti.Placeholder = labels[i]
		ti.CharLimit = 64
		e.inputs[i] = ti
	}
	if edit && profile != nil {
		e.inputs[fieldName].SetValue(profile.Name)
		e.inputs[fieldHost].SetValue(profile.Host)
		e.inputs[fieldPort].SetValue(fmt.Sprintf("%d", profile.Port))
		e.inputs[fieldUser].SetValue(profile.User)
		e.inputs[fieldDatabase].SetValue(profile.Database)
		e.origName = profile.Name
		if len(profile.SealedPassword) > 0 {
			e.hasPwd = true
		}
	}
	e.inputs[fieldName].Focus()
	return e
}

func (e *EditorScreen) ID() base.ScreenID { return base.ScreenEditor }
func (e *EditorScreen) Title() string {
	if e.origName != "" {
		return "✏️ Edit Profile: " + e.origName
	}
	return "✏️ New Profile"
}
func (e *EditorScreen) Footer() []base.FooterHint {
	return []base.FooterHint{
		{Key: "Tab", Desc: "next field"},
		{Key: "Enter", Desc: "save"},
		{Key: "Esc", Desc: "back"},
		{Key: "s", Desc: "set password"},
	}
}

func (e *EditorScreen) Init() tea.Cmd { return textinput.Blink }

func (e *EditorScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if e.saved {
		return e, func() tea.Msg { return base.NavigateBackMsg{} }
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			e.focused = (e.focused + 1) % maxFields
			for i := range e.inputs {
				if i == int(e.focused) {
					e.inputs[i].Focus()
				} else {
					e.inputs[i].Blur()
				}
			}
			return e, textinput.Blink
		case "shift+tab":
			e.focused = (e.focused - 1 + maxFields) % maxFields
			for i := range e.inputs {
				if i == int(e.focused) {
					e.inputs[i].Focus()
				} else {
					e.inputs[i].Blur()
				}
			}
			return e, textinput.Blink
		case "enter":
			if e.hasPwd && e.inputs[fieldName].Value() != e.origName {
				e.err = "Renaming requires Master Passphrase to re-seal."
				return e, nil
			}
			if err := e.save(); err != nil {
				e.err = err.Error()
				return e, nil
			}
			e.saved = true
			return e, nil
		case "esc":
			return e, func() tea.Msg { return base.NavigateBackMsg{} }
		case "s":
			e.err = "Password sealing requires vault. See set-password flow."
			return e, nil
		}
	}
	updated, cmd := e.inputs[e.focused].Update(msg)
	e.inputs[e.focused] = updated
	return e, cmd
}

func (e *EditorScreen) save() error {
	name := e.inputs[fieldName].Value()
	if name == "" {
		return fmt.Errorf("name is required")
	}
	_ = name
	return nil
}
