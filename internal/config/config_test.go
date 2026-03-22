package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.WatchInterval != 5*time.Minute {
		t.Errorf("WatchInterval = %v, want 5m", cfg.WatchInterval)
	}
	if cfg.MaxAutosaves != 10 {
		t.Errorf("MaxAutosaves = %d, want 10", cfg.MaxAutosaves)
	}
}

func TestLoad_NonexistentFile(t *testing.T) {
	cfg, err := Load("/tmp/nonexistent-cmux-resurrect-config.toml")
	if err != nil {
		t.Fatalf("load nonexistent: %v", err)
	}
	if cfg.WatchInterval != 5*time.Minute {
		t.Errorf("should use defaults, got interval=%v", cfg.WatchInterval)
	}
}

func TestLoad_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
watch_interval = "10s"
max_autosaves = 5
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.WatchInterval != 10*time.Second {
		t.Errorf("WatchInterval = %v, want 10s", cfg.WatchInterval)
	}
	if cfg.MaxAutosaves != 5 {
		t.Errorf("MaxAutosaves = %d, want 5", cfg.MaxAutosaves)
	}
}

func TestDefaultLayoutsDir(t *testing.T) {
	dir := DefaultLayoutsDir()
	if dir == "" {
		t.Error("empty layouts dir")
	}
}

func TestDefaultWorkspaceFile(t *testing.T) {
	path := DefaultWorkspaceFile()
	if path == "" {
		t.Error("empty workspace file path")
	}
	if !strings.Contains(path, "workspaces.md") {
		t.Errorf("path should contain workspaces.md: %q", path)
	}
}

func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		in, want string
	}{
		{"~/test/path", filepath.Join(home, "test/path")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"", ""},
	}
	for _, tt := range tests {
		got := ExpandHome(tt.in)
		if got != tt.want {
			t.Errorf("ExpandHome(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestLoad_WithWorkspaceFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `workspace_file = "~/my-vault/workspaces.md"`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, "my-vault/workspaces.md")
	if cfg.WorkspaceFile != want {
		t.Errorf("WorkspaceFile = %q, want %q", cfg.WorkspaceFile, want)
	}
}
