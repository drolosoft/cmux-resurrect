package agentskill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContent(t *testing.T) {
	c := Content()
	if len(c) == 0 {
		t.Fatal("embedded skill is empty")
	}
	if !strings.HasPrefix(c, "---\n") {
		t.Error("skill must start with YAML frontmatter")
	}
	if !strings.Contains(c, "name: crex") {
		t.Error("frontmatter must declare name: crex")
	}
	if !strings.Contains(c, "description: Use when") {
		t.Error("description must start with 'Use when' (trigger-based, per skill spec)")
	}
	// The gaps the skill exists to close must be covered.
	for _, must := range []string{"--mode", "--dry-run", "--resume", "list --json", "--raw"} {
		if !strings.Contains(c, must) {
			t.Errorf("skill content missing required topic %q", must)
		}
	}
}

func TestInstall(t *testing.T) {
	dir := t.TempDir()
	path, err := Install(dir)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	want := filepath.Join(dir, "crex", "SKILL.md")
	if path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	if string(data) != Content() {
		t.Error("installed content differs from embedded content")
	}
}

func TestInstall_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	if _, err := Install(dir); err != nil {
		t.Fatalf("first install: %v", err)
	}
	// Corrupt the installed copy, reinstall must restore it.
	p := filepath.Join(dir, "crex", "SKILL.md")
	if err := os.WriteFile(p, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(dir); err != nil {
		t.Fatalf("reinstall: %v", err)
	}
	data, _ := os.ReadFile(p)
	if string(data) != Content() {
		t.Error("reinstall did not refresh stale content")
	}
}

func TestDefaultDirs(t *testing.T) {
	home, _ := os.UserHomeDir()
	if got := ClaudeDir(); got != filepath.Join(home, ".claude", "skills") {
		t.Errorf("ClaudeDir() = %q", got)
	}
	if got := CodexDir(); got != filepath.Join(home, ".agents", "skills") {
		t.Errorf("CodexDir() = %q", got)
	}
}
