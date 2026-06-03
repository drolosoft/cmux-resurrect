package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectShell_Zsh(t *testing.T) {
	t.Setenv("SHELL", "/bin/zsh")
	if got := DetectShell(); got != "zsh" {
		t.Errorf("DetectShell() = %q, want zsh", got)
	}
}

func TestDetectShell_Bash(t *testing.T) {
	t.Setenv("SHELL", "/usr/bin/bash")
	if got := DetectShell(); got != "bash" {
		t.Errorf("DetectShell() = %q, want bash", got)
	}
}

func TestDetectShell_Fish(t *testing.T) {
	t.Setenv("SHELL", "/opt/homebrew/bin/fish")
	if got := DetectShell(); got != "fish" {
		t.Errorf("DetectShell() = %q, want fish", got)
	}
}

func TestDetectShell_Unknown(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	if got := DetectShell(); got != "sh" {
		t.Errorf("DetectShell() = %q, want sh", got)
	}
}

func TestHookLine_Zsh(t *testing.T) {
	got := HookLine("zsh", "^G")
	if !strings.Contains(got, `bindkey -s '^G'`) {
		t.Errorf("HookLine(zsh) = %q, missing bindkey", got)
	}
	if !strings.Contains(got, "crex pop") {
		t.Errorf("HookLine(zsh) = %q, missing crex pop", got)
	}
	if !strings.Contains(got, "# crex-pop-hook") {
		t.Errorf("HookLine(zsh) = %q, missing marker", got)
	}
}

func TestHookLine_Bash(t *testing.T) {
	got := HookLine("bash", "\\C-g")
	if !strings.Contains(got, "bind") {
		t.Errorf("HookLine(bash) = %q, missing bind", got)
	}
	if !strings.Contains(got, "# crex-pop-hook") {
		t.Errorf("HookLine(bash) = %q, missing marker", got)
	}
}

func TestHookLine_Fish(t *testing.T) {
	got := HookLine("fish", "\\cg")
	if !strings.Contains(got, "bind") {
		t.Errorf("HookLine(fish) = %q, missing bind", got)
	}
	if !strings.Contains(got, "# crex-pop-hook") {
		t.Errorf("HookLine(fish) = %q, missing marker", got)
	}
}

func TestInstallHook_Idempotent(t *testing.T) {
	dir := t.TempDir()
	rcFile := filepath.Join(dir, ".zshrc")
	os.WriteFile(rcFile, []byte("# existing config\n"), 0644)

	err := InstallHookToFile(rcFile, "zsh", "^G")
	if err != nil {
		t.Fatalf("first install: %v", err)
	}
	content1, _ := os.ReadFile(rcFile)
	if !strings.Contains(string(content1), "crex-pop-hook") {
		t.Error("hook not installed")
	}

	err = InstallHookToFile(rcFile, "zsh", "^G")
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	content2, _ := os.ReadFile(rcFile)
	if strings.Count(string(content2), "crex-pop-hook") != 1 {
		t.Error("hook duplicated")
	}
}

func TestUninstallHook(t *testing.T) {
	dir := t.TempDir()
	rcFile := filepath.Join(dir, ".zshrc")
	os.WriteFile(rcFile, []byte("# before\n"), 0644)

	InstallHookToFile(rcFile, "zsh", "^G")
	UninstallHookFromFile(rcFile)

	content, _ := os.ReadFile(rcFile)
	if strings.Contains(string(content), "crex-pop-hook") {
		t.Error("hook not removed")
	}
	if !strings.Contains(string(content), "# before") {
		t.Error("existing content removed")
	}
}
