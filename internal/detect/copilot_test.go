package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectCopilot_NoSessionDir(t *testing.T) {
	s := detectCopilot("/nonexistent/path", "")
	if s != nil {
		t.Error("expected nil for nonexistent CWD")
	}
}

func TestDetectCopilot_WithSession(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	home := os.Getenv("HOME")

	sessionID := "c52c23e5-5cbe-4786-b046-528839201e7a"
	sessionDir := filepath.Join(home, ".copilot", "session-state", sessionID)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}

	yaml := "cwd: " + dir + "\nname: test-session\n"
	if err := os.WriteFile(filepath.Join(sessionDir, "workspace.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	s := detectCopilot(dir, "")
	if s == nil {
		t.Fatal("expected session, got nil")
	}
	if s.Tool != "copilot" {
		t.Errorf("Tool = %q, want copilot", s.Tool)
	}
	if s.Command != "copilot --continue" {
		t.Errorf("Command = %q, want 'copilot --continue'", s.Command)
	}
}
