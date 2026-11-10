package detect

import (
	"testing"
)

func TestValidSessionID(t *testing.T) {
	valid := []string{
		"c52c23e5-5cbe-4786-b046-528839201e7a",
		"019dfd00-d9d9-7780-a206-e338595fc436",
		"ses_20590cf55ffe6n9PUYlGssP4HP",
		"simple-id",
		"abc123",
	}
	invalid := []string{
		"foo; rm -rf /",
		"id with spaces",
		"id\nwith\nnewlines",
		"$(whoami)",
		"",
		"id|pipe",
		"id>redirect",
	}
	for _, id := range valid {
		if !validSessionID.MatchString(id) {
			t.Errorf("expected valid: %q", id)
		}
	}
	for _, id := range invalid {
		if validSessionID.MatchString(id) {
			t.Errorf("expected invalid: %q", id)
		}
	}
}

func TestEscapeSQLite(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"simple", "simple"},
		{"/Users/test/project", "/Users/test/project"},
		{"it's", "it''s"},
		{"it''s", "it''''s"},
		{"no'quotes'here", "no''quotes''here"},
	}
	for _, tt := range tests {
		got := escapeSQLite(tt.input)
		if got != tt.want {
			t.Errorf("escapeSQLite(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTitlePatterns(t *testing.T) {
	patterns := TitlePatterns()
	if len(patterns) != len(registry) {
		t.Fatalf("TitlePatterns returned %d tools, registry has %d", len(patterns), len(registry))
	}
	for _, d := range registry {
		p, ok := patterns[d.Name]
		if !ok {
			t.Errorf("missing patterns for tool %q", d.Name)
		}
		if len(p) == 0 {
			t.Errorf("empty patterns for tool %q", d.Name)
		}
	}
}

func TestRegistryNames(t *testing.T) {
	names := make(map[string]bool)
	processNames := make(map[string]bool)
	for _, d := range registry {
		if names[d.Name] {
			t.Errorf("duplicate tool name: %q", d.Name)
		}
		names[d.Name] = true
		if processNames[d.ProcessName] {
			t.Errorf("duplicate process name: %q", d.ProcessName)
		}
		processNames[d.ProcessName] = true
		if d.Detect == nil {
			t.Errorf("nil Detect func for %q", d.Name)
		}
		if len(d.TitlePatterns) == 0 {
			t.Errorf("no title patterns for %q", d.Name)
		}
	}
}

func TestBatchCWDs_EmptyInput(t *testing.T) {
	result := batchCWDs(nil)
	if len(result) != 0 {
		t.Error("expected empty map for nil input")
	}
}

func TestAutoAcceptFlags(t *testing.T) {
	flags := AutoAcceptFlags()

	tier1 := map[string]string{
		"claude":   "--dangerously-skip-permissions",
		"opencode": "--yolo",
		"codex":    "--full-auto",
		"amp":      "--dangerously-allow-all",
		"gemini":   "--sandbox",
		"copilot":  "--allow-all",
		"grok":     "--always-approve",
	}
	for tool, want := range tier1 {
		got, ok := flags[tool]
		if !ok {
			t.Errorf("AutoAcceptFlags missing tool %q", tool)
			continue
		}
		if got != want {
			t.Errorf("AutoAcceptFlags[%q] = %q, want %q", tool, got, want)
		}
	}

	tier2 := map[string]string{
		"aider":     "--yes",
		"pi":        "--approve",
		"hermes":    "--yolo",
		"codebuddy": "--dangerously-skip-permissions",
		"factory":   "--skip-permissions-unsafe",
		"qoder":     "--permission-mode auto",
	}
	for tool, want := range tier2 {
		got, ok := flags[tool]
		if !ok {
			t.Errorf("AutoAcceptFlags missing tool %q", tool)
			continue
		}
		if got != want {
			t.Errorf("AutoAcceptFlags[%q] = %q, want %q", tool, got, want)
		}
	}

	for _, noFlag := range []string{"cursor", "rovo"} {
		if flag, ok := flags[noFlag]; ok {
			t.Errorf("AutoAcceptFlags[%q] = %q, should not be present", noFlag, flag)
		}
	}
}
