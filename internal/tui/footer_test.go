package tui

import (
	"strings"
	"testing"

	"github.com/kentoespdam/mariadb-restorer/internal/tui/base"
)

func TestGlobalShortcuts(t *testing.T) {
	shortcuts := GlobalShortcuts()
	if len(shortcuts) != 1 {
		t.Errorf("expected 1 global shortcut, got %d", len(shortcuts))
	}

	if shortcuts[0].Key != "Ctrl-Q" {
		t.Errorf("expected 'Ctrl-Q' shortcut, got %q", shortcuts[0].Key)
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
