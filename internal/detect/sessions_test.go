package detect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestDetectClaude_NoProjectDir(t *testing.T) {
	s := detectClaude("/nonexistent/path/that/does/not/exist", "")
	if s != nil {
		t.Error("expected nil for nonexistent CWD")
	}
}

func TestDetectClaude_EmptyProjectDir(t *testing.T) {
	dir := t.TempDir()
	home := os.Getenv("HOME")
	// Claude uses HOME-relative paths; this test won't match unless
	// we create the exact expected path. Test the "no jsonl files" case
	// by creating the project dir structure.
	projectPath := strings.ReplaceAll(dir, "/", "-")
	projectDir := filepath.Join(home, ".claude", "projects", projectPath)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Skip("cannot create test dir in ~/.claude/projects")
	}
	defer os.RemoveAll(projectDir)

	s := detectClaude(dir, "")
	if s != nil {
		t.Error("expected nil for empty project dir")
	}
}

func TestDetectClaude_WithSession(t *testing.T) {
	dir := t.TempDir()
	home := os.Getenv("HOME")
	projectPath := strings.ReplaceAll(dir, "/", "-")
	projectDir := filepath.Join(home, ".claude", "projects", projectPath)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Skip("cannot create test dir")
	}
	defer os.RemoveAll(projectDir)

	// Create a fake session file (must be >= 500 bytes to pass the size filter).
	sessionID := "abc123-def456-789"
	sessionFile := filepath.Join(projectDir, sessionID+".jsonl")
	content := make([]byte, 600)
	for i := range content {
		content[i] = 'x'
	}
	if err := os.WriteFile(sessionFile, content, 0o644); err != nil {
		t.Fatal(err)
	}

	s := detectClaude(dir, "")
	if s == nil {
		t.Fatal("expected session, got nil")
	}
	if s.Tool != "claude" {
		t.Errorf("Tool = %q, want claude", s.Tool)
	}
	if s.Command != "claude --resume "+sessionID {
		t.Errorf("Command = %q", s.Command)
	}
}

func TestDetectClaude_PicksMostRecent(t *testing.T) {
	dir := t.TempDir()
	home := os.Getenv("HOME")
	projectPath := strings.ReplaceAll(dir, "/", "-")
	projectDir := filepath.Join(home, ".claude", "projects", projectPath)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Skip("cannot create test dir")
	}
	defer os.RemoveAll(projectDir)

	// Create two session files with different timestamps (>= 500 bytes each).
	pad := make([]byte, 600)
	old := filepath.Join(projectDir, "old-session.jsonl")
	os.WriteFile(old, pad, 0o644)
	time.Sleep(10 * time.Millisecond)
	recent := filepath.Join(projectDir, "recent-session.jsonl")
	os.WriteFile(recent, pad, 0o644)

	s := detectClaude(dir, "")
	if s == nil {
		t.Fatal("expected session")
	}
	if s.Command != "claude --resume recent-session" {
		t.Errorf("expected most recent session, got %q", s.Command)
	}
}

func TestDetectClaude_InvalidSessionID(t *testing.T) {
	dir := t.TempDir()
	home := os.Getenv("HOME")
	projectPath := strings.ReplaceAll(dir, "/", "-")
	projectDir := filepath.Join(home, ".claude", "projects", projectPath)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Skip("cannot create test dir")
	}
	defer os.RemoveAll(projectDir)

	// Session ID with shell metacharacters.
	bad := filepath.Join(projectDir, "foo;rm -rf.jsonl")
	os.WriteFile(bad, []byte("bad"), 0o644)

	s := detectClaude(dir, "")
	if s != nil {
		t.Error("expected nil for invalid session ID")
	}
}

func TestReadCodexJSONLMeta(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	meta := map[string]interface{}{
		"type":    "session_meta",
		"payload": map[string]string{"id": "abc-123", "cwd": "/Users/test/project"},
	}
	data, _ := json.Marshal(meta)
	os.WriteFile(path, data, 0o644)

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
	os.WriteFile(path, data, 0o644)

	id, _ := readCodexJSONLMeta(path)
	if id != "" {
		t.Error("expected empty id for non-session_meta type")
	}
}

func TestReadCodexJSONLMeta_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")
	os.WriteFile(path, []byte(""), 0o644)

	id, _ := readCodexJSONLMeta(path)
	if id != "" {
		t.Error("expected empty id for empty file")
	}
}

func TestReadCodexJSONLMeta_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.jsonl")
	os.WriteFile(path, []byte("not json"), 0o644)

	id, _ := readCodexJSONLMeta(path)
	if id != "" {
		t.Error("expected empty id for invalid JSON")
	}
}

func TestDetectOpenCode_NoDB(t *testing.T) {
	s := detectOpenCode("/nonexistent/path", "")
	if s != nil {
		t.Error("expected nil when DB doesn't exist")
	}
}

func TestBatchCWDs_EmptyInput(t *testing.T) {
	result := batchCWDs(nil)
	if len(result) != 0 {
		t.Error("expected empty map for nil input")
	}
}

func TestAmpThreadCache_SeedAndLookup(t *testing.T) {
	resetAmpCache()
	seedAmpThread("17752", "T-aaaa-bbbb")
	if got := ampCache.threadFor("17752"); got != "T-aaaa-bbbb" {
		t.Errorf("threadFor known pid = %q", got)
	}
	if got := ampCache.threadFor("99999"); got != "" {
		t.Errorf("threadFor unknown pid = %q", got)
	}
}

func TestResetAmpCache(t *testing.T) {
	seedAmpThread("123", "T-y")
	resetAmpCache()
	if ampCache.threadFor("123") != "" {
		t.Error("expected empty after reset")
	}
}

func TestDetectAmp_NoPID(t *testing.T) {
	if s := detectAmp("/some/path", ""); s != nil {
		t.Error("expected nil when pid is empty")
	}
}

func TestDetectAmp_UnseededPID(t *testing.T) {
	resetAmpCache()
	if s := detectAmp("/some/path", "12345"); s != nil {
		t.Error("expected nil when pid not in cache")
	}
}

func TestThreadIDFromLogPath(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{
			"/Users/me/.cache/amp/logs/threads/T-019e4c62-93b2-7359-9148-2ecd027fcda0.log",
			"T-019e4c62-93b2-7359-9148-2ecd027fcda0",
		},
		{"/Users/me/.cache/amp/logs/cli.log", ""},                  // not in threads/
		{"/Users/me/.cache/amp/logs/threads/foo.txt", ""},          // wrong suffix
		{"/Users/me/.cache/amp/logs/threads/not-a-thread.log", ""}, // missing T- prefix
		{"/some/nested/path/logs/threads/T-019e4c62-aaaa-bbbb-cccc-ddddeeeeffff.log", "T-019e4c62-aaaa-bbbb-cccc-ddddeeeeffff"},
		{"", ""},
	}
	for _, c := range cases {
		if got := threadIDFromLogPath(c.input); got != c.want {
			t.Errorf("threadIDFromLogPath(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}
