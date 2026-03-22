package cmd

import (
	"testing"
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
		icon, name := extractIconAndName(tt.title)
		if icon != tt.wantIcon {
			t.Errorf("extractIconAndName(%q) icon = %q, want %q", tt.title, icon, tt.wantIcon)
		}
		if name != tt.wantName {
			t.Errorf("extractIconAndName(%q) name = %q, want %q", tt.title, name, tt.wantName)
		}
	}
}

func TestAbbreviateHome(t *testing.T) {
	// This test depends on the actual home dir, so just test the logic.
	got := abbreviateHome("/some/other/path")
	if got != "/some/other/path" {
		t.Errorf("non-home path changed: %q", got)
	}
}

func TestTrimSpace(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"  hello  ", "hello"},
		{"hello", "hello"},
		{"  ", ""},
		{"", ""},
		{"\thello\t", "hello"},
	}
	for _, tt := range tests {
		got := trimSpace(tt.in)
		if got != tt.want {
			t.Errorf("trimSpace(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
