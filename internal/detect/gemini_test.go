package detect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectGemini_NoSessionDir(t *testing.T) {
	s := detectGemini("/nonexistent/path/that/does/not/exist", "")
	if s != nil {
		t.Error("expected nil for nonexistent CWD")
	}
}

func TestDetectGemini_WithSession(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	home := os.Getenv("HOME")

	hash := geminiProjectHash(dir)
	chatsDir := filepath.Join(home, ".gemini", "tmp", hash, "chats")
	if err := os.MkdirAll(chatsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	sessionFile := filepath.Join(chatsDir, "session-2026-05-27T14-30-abc12345.json")
	if err := os.WriteFile(sessionFile, []byte(`{"id":"abc12345"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	s := detectGemini(dir, "")
	if s == nil {
		t.Fatal("expected session, got nil")
	}
	if s.Tool != "gemini" {
		t.Errorf("Tool = %q, want gemini", s.Tool)
	}
	if !strings.HasPrefix(s.Command, "gemini --resume ") {
		t.Errorf("Command = %q, want prefix 'gemini --resume '", s.Command)
	}
}

func TestGeminiProjectHash(t *testing.T) {
	hash := geminiProjectHash("/tmp")
	if len(hash) != 64 {
		t.Errorf("hash length = %d, want 64", len(hash))
	}
	if geminiProjectHash("/tmp") != hash {
		t.Error("hash should be deterministic")
	}
	if geminiProjectHash("/var") == hash {
		t.Error("different paths should give different hashes")
	}
}
