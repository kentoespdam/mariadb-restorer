// Package tuiprofiles provides profile list and editor screens.
package tuiprofiles

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	credentialvault "github.com/kentoespdam/mariadb-restorer/internal/credential-vault"
	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
)

const maxFields = 7

type fieldIndex int

const (
	fieldName fieldIndex = iota
	fieldHost
	fieldPort
	fieldUser
	fieldDatabase
	fieldPassword
	fieldPassphrase
)

// EditorScreen provides a form to create or edit a connection profile.
type EditorScreen struct {
	inputs    [maxFields]textinput.Model
	focused   fieldIndex
	dataDir   string
	origName  string
	hasPwd    bool
	clearPwd  bool // user pressed Ctrl-X to remove vaulted password
	err       string
	saved     bool
}

// NewEditorScreen creates a profile editor, pre-populated if profile is given.
func NewEditorScreen(dataDir string, profile *credentialvault.Profile, edit bool) *EditorScreen {
	e := &EditorScreen{dataDir: dataDir, inputs: [maxFields]textinput.Model{}}
	labels := []string{"Name", "Host", "Port", "User", "Database", "Password", "Passphrase"}
	echoModes := []textinput.EchoMode{
		textinput.EchoNormal, textinput.EchoNormal, textinput.EchoNormal,
		textinput.EchoNormal, textinput.EchoNormal,
		textinput.EchoPassword, textinput.EchoPassword,
	}
	for i := range e.inputs {
		ti := textinput.New()
		ti.Placeholder = labels[i]
		ti.CharLimit = 64
		ti.EchoMode = echoModes[i]
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
			e.inputs[fieldPassword].Placeholder = "🔒 vaulted (leave empty)"
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
	hints := []base.FooterHint{
		{Key: "Tab", Desc: "next field"},
		{Key: "Enter", Desc: "save"},
		{Key: "Esc", Desc: "back"},
	}
	if e.hasPwd {
		hints = append(hints, base.FooterHint{Key: "Ctrl-X", Desc: "clear password"})
	}
	return hints
}

func (e *EditorScreen) Init() tea.Cmd { return textinput.Blink }
