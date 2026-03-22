package cmd

import (
	"testing"

	"github.com/juanatsap/cmux-resurrect/internal/orchestrate"
)

func TestExtractIconAndName(t *testing.T) {
	tests := []struct {
		title    string
		wantIcon string
		wantName string
	}{
		{"0 🥌 ioc-events", "🥌", "ioc-events"},
		{"1 🗿Obsidian", "🗿", "Obsidian"},
		{"2 🏟️ LaPorrA", "🏟️", "LaPorrA"},
		{"3 🗾 immich-photo-manager", "🗾", "immich-photo-manager"},
		{"plain-title", "📁", "plain-title"},
	}

	for _, tt := range tests {
		icon, name := orchestrate.ExtractIconAndName(tt.title)
		if icon != tt.wantIcon {
			t.Errorf("ExtractIconAndName(%q) icon = %q, want %q", tt.title, icon, tt.wantIcon)
		}
		if name != tt.wantName {
			t.Errorf("ExtractIconAndName(%q) name = %q, want %q", tt.title, name, tt.wantName)
		}
	}
}

func TestAbbreviateHome(t *testing.T) {
	got := orchestrate.AbbreviateHome("/some/other/path")
	if got != "/some/other/path" {
		t.Errorf("non-home path changed: %q", got)
	}
}
