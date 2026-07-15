package tuilauncher

import (
	"fmt"
	"strings"

	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
)

func (s *LauncherScreen) View() string {
	var b strings.Builder
	if s.err != "" {
		b.WriteString(base.StyleError.Render(" ⚠ "+s.err) + "\n\n")
	}

	switch s.step {
	case 0:
		s.renderStepFile(&b)
	case 1:
		s.renderStepProfile(&b)
	case 2:
		s.renderStepPassword(&b)
	case 3:
		s.renderStepConfirm(&b)
	case 4:
		s.renderStepLaunch(&b)
	}

	steps := make([]string, totalSteps)
	for i := range steps {
		if i == s.step {
			steps[i] = base.StyleHighlight.Render(fmt.Sprintf("● S%d", i+1))
		} else {
			steps[i] = base.StyleDim.Render(fmt.Sprintf("○ S%d", i+1))
		}
	}
	b.WriteString("\n\n" + strings.Join(steps, "  "))
	return b.String()
}

func (s *LauncherScreen) renderStepFile(b *strings.Builder) {
	b.WriteString(base.StyleHighlight.Render(" Step 1: Select Dump File") + "\n\n")
	b.WriteString(base.StyleDim.Render(" Enter the path to your SQL dump file:") + "\n")
	b.WriteString(" " + s.dumpFile + "▌\n")
	b.WriteString(base.StyleDim.Render("\n Type the path, then press Enter to continue"))
	b.WriteString(base.StyleDim.Render("\n Ctrl+Shift+V to paste from clipboard (or right-click)"))
}

func (s *LauncherScreen) renderStepProfile(b *strings.Builder) {
	b.WriteString(base.StyleHighlight.Render(" Step 2: Select Connection Profile") + "\n\n")
	if len(s.profiles) == 0 {
		b.WriteString(base.StyleDim.Render(" No profiles found."))
		return
	}
	for i, p := range s.profiles {
		prefix := "  "
		if i == s.selProfile {
			prefix = "▸ "
		}
		hp := fmt.Sprintf("%s:%d", p.Host, p.Port)
		pwd := ""
		if p.SealedPassword != nil {
			pwd = " 🔒"
		}
		dbDisplay := p.Database
		if dbDisplay == "" {
			dbDisplay = "-"
		}
		line := fmt.Sprintf(" %s%s@%s (%s)%s", prefix, p.User, hp, dbDisplay, pwd)
		if i == s.selProfile {
			b.WriteString(base.StyleSelected.Render(line) + "\n")
		} else {
			b.WriteString(line + "\n")
		}
	}
}

func (s *LauncherScreen) renderStepPassword(b *strings.Builder) {
	p := s.selectedProfile()
	if p == nil {
		b.WriteString(base.StyleDim.Render(" No profile selected."))
		return
	}

	if len(p.SealedPassword) > 0 {
		b.WriteString(base.StyleHighlight.Render(" Step 3: Unlock Vault Password") + "\n\n")
		b.WriteString(base.StyleDim.Render(fmt.Sprintf(
			" Profile %q has a vaulted password. Enter your master passphrase to unlock:", p.Name)) + "\n\n")
	} else {
		b.WriteString(base.StyleHighlight.Render(" Step 3: Enter MariaDB Password") + "\n\n")
		b.WriteString(base.StyleDim.Render(fmt.Sprintf(
			" Enter password for %s@%s (leave empty to use MYSQL_PWD or socket auth):", p.User, p.Host)) + "\n\n")
	}

	b.WriteString(" " + s.passwordInput.View() + "\n")
	b.WriteString(base.StyleDim.Render("\n Enter to continue  •  Esc to go back"))
}

func (s *LauncherScreen) renderStepConfirm(b *strings.Builder) {
	b.WriteString(base.StyleHighlight.Render(" Step 4: Confirm Settings") + "\n\n")
	fmt.Fprintf(b, " Dump file: %s\n", s.dumpFile)
	if p := s.selectedProfile(); p != nil {
		hp := fmt.Sprintf("%s:%d", p.Host, p.Port)
		if p.Database == "" {
			fmt.Fprintf(b, " Target:    %s@%s (no default db)\n", p.User, hp)
		} else {
			fmt.Fprintf(b, " Target:    %s@%s/%s\n", p.User, hp, p.Database)
		}
		if p.SealedPassword != nil {
			if s.passwordInput.Value() != "" {
				fmt.Fprintf(b, " Password:  %s\n", base.StyleVaulted.Render("🔒 unsealed from vault"))
			} else {
				fmt.Fprintf(b, " Password:  %s\n",
					base.StyleDim.Render("not set (use MYSQL_PWD or no auth)"))
			}
		} else if s.passwordInput.Value() != "" {
			fmt.Fprintf(b, " Password:  %s\n", base.StyleVaulted.Render("🔒 provided"))
		} else {
			fmt.Fprintf(b, " Password:  %s\n",
				base.StyleDim.Render("not set (use MYSQL_PWD or no auth)"))
		}
	}
	fmt.Fprintf(b, " Verify:    %s\n\n", yesNo(s.verify))
	b.WriteString(base.StyleDim.Render(" Press 'v' to toggle verify, 'n' or Enter to continue"))
}

func (s *LauncherScreen) renderStepLaunch(b *strings.Builder) {
	b.WriteString(base.StyleHighlight.Render(" Step 5: Ready to Launch") + "\n\n")
	fmt.Fprintf(b, " Dump:  %s\n", s.dumpFile)
	if p := s.selectedProfile(); p != nil {
		if p.Database == "" {
			fmt.Fprintf(b, " To:    %s@%s:%d (no default db)\n", p.User, p.Host, p.Port)
		} else {
			fmt.Fprintf(b, " To:    %s@%s:%d/%s\n", p.User, p.Host, p.Port, p.Database)
		}
		if s.passwordInput.Value() != "" {
			fmt.Fprintf(b, " Pass:  %s\n", base.StyleVaulted.Render("provided"))
		} else {
			fmt.Fprintf(b, " Pass:  %s\n", base.StyleDim.Render("none"))
		}
	}
	fmt.Fprintf(b, " Verify: %s\n", yesNo(s.verify))
	b.WriteString("\n" + base.StyleSuccess.Render(" Press Enter to launch restore!"))
}

func yesNo(v bool) string {
	if v {
		return base.StyleSuccess.Render("enabled")
	}
	return base.StyleDim.Render("disabled")
}
