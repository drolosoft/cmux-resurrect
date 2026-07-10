package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseCommand_Simple(t *testing.T) {
	tests := []struct {
		input    string
		wantCmd  string
		wantArgs []string
	}{
		{"ls", "ls", nil},
		{"save morning", "save", []string{"morning"}},
		{"restore 2", "restore", []string{"2"}},
		{"delete my-layout", "delete", []string{"my-layout"}},
		{"use claude", "use", []string{"claude"}},
		{"now", "now", nil},
		{"help", "help", nil},
		{"exit", "exit", nil},
		{"templates", "templates", nil},
		{"watch start", "watch", []string{"start"}},
		{"watch stop", "watch", []string{"stop"}},
		{"watch status", "watch", []string{"status"}},
		{"show my-layout", "show", []string{"my-layout"}},
		{"edit morning", "edit", []string{"morning"}},
		{"import", "import", nil},
		{"import-from-md", "import-from-md", nil},
		{"export", "export", nil},
		{"export-to-md", "export-to-md", nil},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cmd, args := parseCommand(tt.input)
			if cmd != tt.wantCmd {
				t.Errorf("parseCommand(%q) cmd = %q, want %q", tt.input, cmd, tt.wantCmd)
			}
			if len(args) != len(tt.wantArgs) {
				t.Errorf("parseCommand(%q) args = %v, want %v", tt.input, args, tt.wantArgs)
			}
		})
	}
}

func TestParseCommand_Blueprint(t *testing.T) {
	tests := []struct {
		input    string
		wantCmd  string
		wantArgs []string
	}{
		{"bp add api ~/projects/api", "bp add", []string{"api", "~/projects/api"}},
		{"bp list", "bp list", nil},
		{"bp remove api", "bp remove", []string{"api"}},
		{"bp toggle 3", "bp toggle", []string{"3"}},
		{"bp rm api", "bp rm", []string{"api"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cmd, args := parseCommand(tt.input)
			if cmd != tt.wantCmd {
				t.Errorf("parseCommand(%q) cmd = %q, want %q", tt.input, cmd, tt.wantCmd)
			}
			if len(args) != len(tt.wantArgs) {
				t.Errorf("parseCommand(%q) args = %v, want %v", tt.input, args, tt.wantArgs)
				return
			}
			for i, a := range args {
				if a != tt.wantArgs[i] {
					t.Errorf("parseCommand(%q) args[%d] = %q, want %q", tt.input, i, a, tt.wantArgs[i])
				}
			}
		})
	}
}

func TestParseCommand_Template(t *testing.T) {
	tests := []struct {
		input    string
		wantCmd  string
		wantArgs []string
	}{
		{"template show claude", "template show", []string{"claude"}},
		{"template customize aside", "template customize", []string{"aside"}},
		{"template show 2", "template show", []string{"2"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cmd, args := parseCommand(tt.input)
			if cmd != tt.wantCmd {
				t.Errorf("parseCommand(%q) cmd = %q, want %q", tt.input, cmd, tt.wantCmd)
			}
			if len(args) != len(tt.wantArgs) {
				t.Errorf("parseCommand(%q) args = %v, want %v", tt.input, args, tt.wantArgs)
				return
			}
			for i, a := range args {
				if a != tt.wantArgs[i] {
					t.Errorf("parseCommand(%q) args[%d] = %q, want %q", tt.input, i, a, tt.wantArgs[i])
				}
			}
		})
	}
}

func TestParseCommand_SettingsBanner(t *testing.T) {
	tests := []struct {
		input    string
		wantCmd  string
		wantArgs []string
	}{
		{"settings banner set flame", "settings banner set", []string{"flame"}},
		{"settings banner get", "settings banner get", nil},
		{"settings banner list", "settings banner list", nil},
		{"settings banner set classic", "settings banner set", []string{"classic"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cmd, args := parseCommand(tt.input)
			if cmd != tt.wantCmd {
				t.Errorf("parseCommand(%q) cmd = %q, want %q", tt.input, cmd, tt.wantCmd)
			}
			if len(args) != len(tt.wantArgs) {
				t.Errorf("parseCommand(%q) args = %v, want %v", tt.input, args, tt.wantArgs)
				return
			}
			for i, a := range args {
				if a != tt.wantArgs[i] {
					t.Errorf("parseCommand(%q) args[%d] = %q, want %q", tt.input, i, a, tt.wantArgs[i])
				}
			}
		})
	}
}

func TestPadIcon(t *testing.T) {
	// Text-presentation emoji get an extra space for alignment.
	if got := padIcon("🖥"); got != "🖥 " {
		t.Errorf("padIcon(🖥) = %q, want %q", got, "🖥 ")
	}
	if got := padIcon("⏱"); got != "⏱ " {
		t.Errorf("padIcon(⏱) = %q, want %q", got, "⏱ ")
	}
	if got := padIcon("🗑"); got != "🗑 " {
		t.Errorf("padIcon(🗑) = %q, want %q", got, "🗑 ")
	}
	// Full-width emoji should NOT get padding.
	if got := padIcon("📋"); got != "📋" {
		t.Errorf("padIcon(📋) = %q, want %q", got, "📋")
	}
	if got := padIcon("🚀"); got != "🚀" {
		t.Errorf("padIcon(🚀) = %q, want %q", got, "🚀")
	}
}

func TestResolveNameOrNumber_ByName(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "morning"},
		{Kind: KindLayout, Name: "afternoon"},
	}
	resolved, err := resolveNameOrNumber("morning", items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != "morning" {
		t.Errorf("resolveNameOrNumber(morning) = %q, want %q", resolved, "morning")
	}
}

func TestResolveNameOrNumber_ByNumber(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "morning"},
		{Kind: KindLayout, Name: "afternoon"},
	}
	resolved, err := resolveNameOrNumber("2", items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != "afternoon" {
		t.Errorf("resolveNameOrNumber(2) = %q, want %q", resolved, "afternoon")
	}
}

func TestParseCommand_EmptyAndWhitespace(t *testing.T) {
	cmd, args := parseCommand("")
	if cmd != "" || args != nil {
		t.Errorf("empty input should return empty cmd, got %q %v", cmd, args)
	}

	cmd, args = parseCommand("   ")
	if cmd != "" || args != nil {
		t.Errorf("whitespace input should return empty cmd, got %q %v", cmd, args)
	}
}

func TestResolveNumberRef_Valid(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "morning"},
		{Kind: KindLayout, Name: "afternoon"},
		{Kind: KindLayout, Name: "evening"},
	}

	item, err := resolveNumberRef("2", items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.Name != "afternoon" {
		t.Errorf("resolveNumberRef(2) = %q, want %q", item.Name, "afternoon")
	}
}

func TestResolveNumberRef_OutOfRange(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "morning"},
	}

	_, err := resolveNumberRef("99", items)
	if err == nil {
		t.Error("expected error for out-of-range ref, got nil")
	}
}

func TestResolveNumberRef_NotANumber(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "morning"},
	}

	_, err := resolveNumberRef("abc", items)
	if err == nil {
		t.Error("expected error for non-numeric ref, got nil")
	}
}

func TestResolveNumberRef_EmptyItems(t *testing.T) {
	_, err := resolveNumberRef("1", nil)
	if err == nil {
		t.Error("expected error for empty items, got nil")
	}
}

func TestParseCommand_Skill(t *testing.T) {
	tests := []struct {
		input    string
		wantCmd  string
		wantArgs []string
	}{
		{"skill install", "skill install", nil},
		{"skill show", "skill show", nil},
		{"skill install codex", "skill install", []string{"codex"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cmd, args := parseCommand(tt.input)
			if cmd != tt.wantCmd {
				t.Errorf("parseCommand(%q) cmd = %q, want %q", tt.input, cmd, tt.wantCmd)
			}
			if len(args) != len(tt.wantArgs) {
				t.Errorf("parseCommand(%q) args = %v, want %v", tt.input, args, tt.wantArgs)
			}
		})
	}
}

func TestExecSkillInstall_WritesSkill(t *testing.T) {
	dir := t.TempDir()
	orig := skillsBaseDir
	skillsBaseDir = func(codex bool) string { return dir }
	defer func() { skillsBaseDir = orig }()

	m := &ShellModel{output: &strings.Builder{}}
	m.execSkillInstall(false)
	out := m.output.String()
	if !strings.Contains(out, dir) {
		t.Errorf("output should report installed path, got %q", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "crex", "SKILL.md")); err != nil {
		t.Errorf("skill not written: %v", err)
	}
}
