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
		s.renderStepConfirm(&b)
	case 3:
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
	b.WriteString(base.StyleDim.Render("\n Ctrl-V to paste from clipboard"))
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
		line := fmt.Sprintf(" %s%s@%s (%s)%s", prefix, p.User, hp, p.Database, pwd)
		if i == s.selProfile {
			b.WriteString(base.StyleSelected.Render(line) + "\n")
		} else {
			b.WriteString(line + "\n")
		}
	}
}

func (s *LauncherScreen) renderStepConfirm(b *strings.Builder) {
	b.WriteString(base.StyleHighlight.Render(" Step 3: Confirm Settings") + "\n\n")
	fmt.Fprintf(b, " Dump file: %s\n", s.dumpFile)
	if s.selProfile < len(s.profiles) {
		p := s.profiles[s.selProfile]
		hp := fmt.Sprintf("%s:%d", p.Host, p.Port)
		fmt.Fprintf(b, " Target:    %s@%s/%s\n", p.User, hp, p.Database)
		if p.SealedPassword != nil {
			fmt.Fprintf(b, " Password:  %s\n", base.StyleVaulted.Render("🔒 vault"))
		} else {
			fmt.Fprintf(b, " Password:  %s\n",
				base.StyleDim.Render("not set (use --password or MYSQL_PWD)"))
		}
	}
	fmt.Fprintf(b, " Verify:    %s\n\n", yesNo(s.verify))
	b.WriteString(base.StyleDim.Render(" Press 'v' to toggle verify, 'n' to continue"))
}

func (s *LauncherScreen) renderStepLaunch(b *strings.Builder) {
	b.WriteString(base.StyleHighlight.Render(" Step 4: Ready to Launch") + "\n\n")
	fmt.Fprintf(b, " Dump:  %s\n", s.dumpFile)
	if s.selProfile < len(s.profiles) {
		p := s.profiles[s.selProfile]
		fmt.Fprintf(b, " To:    %s@%s:%d/%s\n", p.User, p.Host, p.Port, p.Database)
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
