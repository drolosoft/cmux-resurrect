package config

import (
	"os"
	"path/filepath"
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
	cfg, err := Load("/tmp/nonexistent-cmux-persist-config.toml")
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
