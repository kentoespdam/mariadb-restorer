package tuiprofiles

import (
	"fmt"
	"strings"

	credentialvault "github.com/kentoespdam/mariadb-restorer/internal/credential-vault"
	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
)

func (s *ListScreen) View() string {
	if s.loading {
		return base.StyleDim.Render("Loading profiles...")
	}
	if s.err != nil {
		return base.StyleError.Render("Error: " + s.err.Error())
	}

	var b strings.Builder
	if s.searching {
		b.WriteString(base.StyleHighlight.Render(" Search: ") + s.search + "▌\n\n")
	}

	filtered := s.filtered()
	if len(filtered) == 0 {
		return base.StyleDim.Render("No profiles found.\n\nPress 'n' to create one.")
	}

	b.WriteString(base.StyleHighlight.Render(
		fmt.Sprintf(" %d profile(s)", len(filtered)),
	) + "\n\n")

	header := fmt.Sprintf(" %-20s %-22s %-12s %-15s %s",
		"Name", "Host:Port", "User", "Database", "Password")
	b.WriteString(base.StyleDim.Render(header) + "\n")

	for _, prof := range filtered {
		actualIdx := s.indexOf(prof)
		prefix := " "
		if actualIdx == s.selected {
			prefix = "▸"
		}
		hp := fmt.Sprintf("%s:%d", prof.Host, prof.Port)
		pwd := "—"
		if prof.SealedPassword != nil {
			pwd = base.StyleVaulted.Render("🔒 vaulted")
		}
		line := fmt.Sprintf(" %s %-20s %-22s %-12s %-15s %s",
			prefix, prof.Name, hp, prof.User, prof.Database, pwd)

		if actualIdx == s.selected {
			b.WriteString(base.StyleSelected.Render(line) + "\n")
		} else {
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

func (s *ListScreen) filtered() []*credentialvault.Profile {
	if s.search == "" {
		return s.profiles
	}
	q := strings.ToLower(s.search)
	var res []*credentialvault.Profile
	for _, p := range s.profiles {
		if strings.Contains(strings.ToLower(p.Name), q) ||
			strings.Contains(strings.ToLower(p.Host), q) {
			res = append(res, p)
		}
	}
	return res
}

func (s *ListScreen) indexOf(prof *credentialvault.Profile) int {
	for i, x := range s.profiles {
		if x.Name == prof.Name {
			return i
		}
	}
	return -1
}
