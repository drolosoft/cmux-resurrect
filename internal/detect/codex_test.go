package detect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReadCodexJSONLMeta(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	meta := map[string]interface{}{
		"type":    "session_meta",
		"payload": map[string]string{"id": "abc-123", "cwd": "/Users/test/project"},
	}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	id, cwd := readCodexJSONLMeta(path)
	if id != "abc-123" {
		t.Errorf("id = %q, want abc-123", id)
	}
	if cwd != "/Users/test/project" {
		t.Errorf("cwd = %q", cwd)
	}
}

func TestReadCodexJSONLMeta_WrongType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	meta := map[string]interface{}{
		"type":    "message",
		"payload": map[string]string{"id": "abc"},
	}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	id, _ := readCodexJSONLMeta(path)
	if id != "" {
		t.Error("expected empty id for non-session_meta type")
	}
}

func TestReadCodexJSONLMeta_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	id, _ := readCodexJSONLMeta(path)
	if id != "" {
		t.Error("expected empty id for empty file")
	}
}

func TestReadCodexJSONLMeta_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.jsonl")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	id, _ := readCodexJSONLMeta(path)
	if id != "" {
		t.Error("expected empty id for invalid JSON")
	}
}
