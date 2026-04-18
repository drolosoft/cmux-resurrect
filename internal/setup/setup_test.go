package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/client"
)

func TestDescribeBackend(t *testing.T) {
	tests := []struct {
		backend client.DetectedBackend
		want    string
	}{
		{client.BackendCmux, "cmux"},
		{client.BackendGhostty, "Ghostty"},
		{client.BackendUnknown, "unknown"},
	}
	for _, tt := range tests {
		got := DescribeBackend(tt.backend)
		if got != tt.want {
			t.Errorf("DescribeBackend(%q) = %q, want %q", tt.backend, got, tt.want)
		}
	}
}

func TestGenerateDefaultConfig(t *testing.T) {
	content := GenerateDefaultConfig("5m", 10)
	if !strings.Contains(content, "watch_interval") {
		t.Errorf("config missing watch_interval: %q", content)
	}
	if !strings.Contains(content, "max_autosaves") {
		t.Errorf("config missing max_autosaves: %q", content)
	}
	if !strings.Contains(content, "5m") {
		t.Errorf("config missing watch_interval value '5m': %q", content)
	}
	if !strings.Contains(content, "10") {
		t.Errorf("config missing max_autosaves value '10': %q", content)
	}
}

func TestWriteConfigIfNotExists_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	created, err := WriteConfigIfNotExists(path, "5m", 10)
	if err != nil {
		t.Fatalf("WriteConfigIfNotExists: %v", err)
	}
	if !created {
		t.Error("expected created=true for new file")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "watch_interval") {
		t.Errorf("created file missing watch_interval: %q", content)
	}
	if !strings.Contains(content, "max_autosaves") {
		t.Errorf("created file missing max_autosaves: %q", content)
	}
}

func TestWriteConfigIfNotExists_SkipsExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	original := "# existing config\nwatch_interval = \"1m\"\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("setup existing file: %v", err)
	}

	created, err := WriteConfigIfNotExists(path, "5m", 10)
	if err != nil {
		t.Fatalf("WriteConfigIfNotExists: %v", err)
	}
	if created {
		t.Error("expected created=false for existing file")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file after skip: %v", err)
	}
	if string(data) != original {
		t.Errorf("existing file was overwritten: got %q, want %q", string(data), original)
	}
}
