package tui

import (
	"strings"
	"testing"

	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
)

func TestGlobalShortcuts(t *testing.T) {
	shortcuts := GlobalShortcuts()
	if len(shortcuts) != 3 {
		t.Errorf("expected 3 global shortcuts, got %d", len(shortcuts))
	}

	hasQ := false
	hasHelp := false
	hasGlossary := false
	for _, s := range shortcuts {
		switch s.Key {
		case "q":
			hasQ = true
		case "?":
			hasHelp = true
		case "g":
			hasGlossary = true
		}
	}

	if !hasQ {
		t.Error("expected 'q' shortcut")
	}
	if !hasHelp {
		t.Error("expected '?' shortcut")
	}
	if !hasGlossary {
		t.Error("expected 'g' shortcut")
	}
}

func TestHomeShortcuts(t *testing.T) {
	shortcuts := HomeShortcuts()
	if len(shortcuts) != 4 {
		t.Errorf("expected 4 home shortcuts, got %d", len(shortcuts))
	}
}

func TestProfileListShortcuts(t *testing.T) {
	shortcuts := ProfileListShortcuts()
	if len(shortcuts) != 4 {
		t.Errorf("expected 4 profile list shortcuts, got %d", len(shortcuts))
	}
}

func TestEditorShortcuts(t *testing.T) {
	shortcuts := EditorShortcuts()
	if len(shortcuts) != 4 {
		t.Errorf("expected 4 editor shortcuts, got %d", len(shortcuts))
	}
}

func TestLauncherShortcuts(t *testing.T) {
	shortcuts := LauncherShortcuts()
	if len(shortcuts) != 3 {
		t.Errorf("expected 3 launcher shortcuts, got %d", len(shortcuts))
	}
}

func TestProgressShortcuts(t *testing.T) {
	shortcuts := ProgressShortcuts()
	if len(shortcuts) != 1 {
		t.Errorf("expected 1 progress shortcut, got %d", len(shortcuts))
	}
}

func TestReportShortcuts(t *testing.T) {
	shortcuts := ReportShortcuts()
	if len(shortcuts) != 3 {
		t.Errorf("expected 3 report shortcuts, got %d", len(shortcuts))
	}
}

func TestRenderFooter_Empty(t *testing.T) {
	result := RenderFooter(nil, 80)
	if result != "" {
		t.Errorf("expected empty footer, got %q", result)
	}
}

func TestRenderFooter_SingleHint(t *testing.T) {
	hints := []base.FooterHint{{Key: "q", Desc: "quit"}}
	result := RenderFooter(hints, 80)
	if result == "" {
		t.Error("expected non-empty footer")
	}
	if !strings.Contains(result, "q") {
		t.Error("expected 'q' in footer")
	}
	if !strings.Contains(result, "quit") {
		t.Error("expected 'quit' in footer")
	}
}

func TestRenderFooter_MultipleHints(t *testing.T) {
	hints := []base.FooterHint{
		{Key: "q", Desc: "quit"},
		{Key: "?", Desc: "help"},
		{Key: "Esc", Desc: "back"},
	}
	result := RenderFooter(hints, 80)
	if result == "" {
		t.Error("expected non-empty footer")
	}
	// Should contain separators between hints.
	if !strings.Contains(result, "•") {
		t.Error("expected '•' separator between hints")
	}
}

func TestRenderFooter_CustomWidth(t *testing.T) {
	hints := []base.FooterHint{{Key: "q", Desc: "quit"}}
	result := RenderFooter(hints, 40)
	if result == "" {
		t.Error("expected non-empty footer")
	}
}
