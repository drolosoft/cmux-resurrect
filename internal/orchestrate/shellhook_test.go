package orchestrate

import (
	"strings"
	"testing"
)

func TestShellHook_Zsh(t *testing.T) {
	hook := ShellHook("zsh")
	if hook == "" {
		t.Fatal("expected non-empty hook for zsh")
	}
	if !strings.Contains(hook, "crex watch") {
		t.Errorf("zsh hook missing 'crex watch': %q", hook)
	}
	if !strings.Contains(hook, "crex.pid") {
		t.Errorf("zsh hook missing 'crex.pid': %q", hook)
	}
}

func TestShellHook_Bash(t *testing.T) {
	hook := ShellHook("bash")
	if hook == "" {
		t.Fatal("expected non-empty hook for bash")
	}
	if !strings.Contains(hook, "crex watch") {
		t.Errorf("bash hook missing 'crex watch': %q", hook)
	}
}

func TestShellHook_Fish(t *testing.T) {
	hook := ShellHook("fish")
	if hook == "" {
		t.Fatal("expected non-empty hook for fish")
	}
	if !strings.Contains(hook, "crex watch") {
		t.Errorf("fish hook missing 'crex watch': %q", hook)
	}
	if !strings.Contains(hook, "if not") {
		t.Errorf("fish hook missing 'if not' (fish syntax): %q", hook)
	}
}

func TestShellHook_Unknown(t *testing.T) {
	hook := ShellHook("powershell")
	if hook != "" {
		t.Errorf("expected empty string for unsupported shell, got: %q", hook)
	}
}

func TestDetectShell(t *testing.T) {
	// Just ensure it doesn't panic; result is env-dependent.
	_ = DetectShell()
}
