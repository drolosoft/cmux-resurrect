package detect

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// stubProcessEnv replaces the process environment reader for one test.
func stubProcessEnv(t *testing.T, env map[string]map[string]string) {
	t.Helper()
	orig := processEnv
	processEnv = func(pid string) map[string]string { return env[pid] }
	t.Cleanup(func() { processEnv = orig })
}

// writeTranscript creates a Claude transcript large enough to pass the
// placeholder size filter.
func writeTranscript(t *testing.T, projectDir, sessionID string) {
	t.Helper()
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, sessionID+".jsonl"), make([]byte, 600), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectClaude_CustomConfigDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := "/src/app.io"
	configDir := filepath.Join(home, ".claude-pro")
	writeTranscript(t, filepath.Join(configDir, "projects", "-src-app-io"), "pro-session")
	stubProcessEnv(t, map[string]map[string]string{"42": {"CLAUDE_CONFIG_DIR": configDir}})

	s := detectClaude(cwd, "42")
	if s == nil {
		t.Fatal("expected session in CLAUDE_CONFIG_DIR, got nil")
	}
	want := "CLAUDE_CONFIG_DIR=" + configDir + " claude --resume pro-session"
	if s.Command != want {
		t.Errorf("Command = %q, want %q", s.Command, want)
	}
}

func TestDetectClaude_DefaultConfigDirHasNoPrefix(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := "/src/app"
	writeTranscript(t, filepath.Join(home, ".claude", "projects", "-src-app"), "default-session")
	// Explicitly pointing at the default dir must not add a prefix.
	stubProcessEnv(t, map[string]map[string]string{"7": {"CLAUDE_CONFIG_DIR": filepath.Join(home, ".claude") + "/"}})

	s := detectClaude(cwd, "7")
	if s == nil || s.Command != "claude --resume default-session" {
		t.Fatalf("got %+v, want plain claude --resume default-session", s)
	}
}

func TestDetectClaude_ProjectDirEncodesNonAlphanumerics(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Claude maps "." and "_" to "-" as well as "/".
	writeTranscript(t, filepath.Join(home, ".claude", "projects", "-src-gitlab-example-com-my-app"), "dotted")
	stubProcessEnv(t, nil)

	s := detectClaude("/src/gitlab.example.com/my_app", "")
	if s == nil || s.Command != "claude --resume dotted" {
		t.Fatalf("got %+v, want claude --resume dotted", s)
	}
}

func TestDetectClaude_SessionFilePerPID(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := "/src/app"
	configDir := filepath.Join(home, ".claude")
	projectDir := filepath.Join(configDir, "projects", "-src-app")
	writeTranscript(t, projectDir, "first")
	writeTranscript(t, projectDir, "second") // most recent
	if err := os.MkdirAll(filepath.Join(configDir, "sessions"), 0o755); err != nil {
		t.Fatal(err)
	}
	meta := `{"pid":100,"sessionId":"first","cwd":"/src/app"}`
	if err := os.WriteFile(filepath.Join(configDir, "sessions", "100.json"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
	stubProcessEnv(t, nil)

	if s := detectClaude(cwd, "100"); s == nil || s.Command != "claude --resume first" {
		t.Errorf("pid 100: got %+v, want its own session 'first'", s)
	}
	// No session file: falls back to the most recent transcript.
	if s := detectClaude(cwd, "200"); s == nil || s.Command != "claude --resume second" {
		t.Errorf("pid 200: got %+v, want fallback 'second'", s)
	}
}

func TestDetectClaude_SessionFileRejectsUnsafeID(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(filepath.Join(configDir, "sessions"), 0o755); err != nil {
		t.Fatal(err)
	}
	meta := `{"sessionId":"x;rm -rf ~","cwd":"/src/app"}`
	if err := os.WriteFile(filepath.Join(configDir, "sessions", "1.json"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
	if id, _ := claudeSessionForPID(configDir, "1", "/src/app"); id != "" {
		t.Errorf("claudeSessionForPID = %q, want empty for unsafe id", id)
	}
}

func TestClaudeResumeCommand_QuotesConfigDir(t *testing.T) {
	got := claudeResumeCommand("/Users/me/Claude Pro's", true, "abc")
	want := `CLAUDE_CONFIG_DIR='/Users/me/Claude Pro'\''s' claude --resume abc`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSplitEnvPrefix(t *testing.T) {
	tests := []struct {
		in, prefix, rest string
	}{
		{"claude --resume abc", "", "claude --resume abc"},
		{"CLAUDE_CONFIG_DIR=/x claude --resume abc", "CLAUDE_CONFIG_DIR=/x ", "claude --resume abc"},
		{`CLAUDE_CONFIG_DIR='/a b'\''c' claude`, `CLAUDE_CONFIG_DIR='/a b'\''c' `, "claude"},
		{"A=1 B=2  codex resume x", "A=1 B=2  ", "codex resume x"},
		{"FOO=bar", "", "FOO=bar"},
		{"npm run dev", "", "npm run dev"},
	}
	for _, tt := range tests {
		prefix, rest := SplitEnvPrefix(tt.in)
		if prefix != tt.prefix || rest != tt.rest {
			t.Errorf("SplitEnvPrefix(%q) = (%q, %q), want (%q, %q)", tt.in, prefix, rest, tt.prefix, tt.rest)
		}
	}
}

func TestEnvFromProcArgs(t *testing.T) {
	var buf []byte
	buf = binary.NativeEndian.AppendUint32(buf, 2) // argc
	buf = append(buf, "/usr/local/bin/claude\x00\x00\x00\x00"...)
	buf = append(buf, "claude\x00--resume\x00"...)
	buf = append(buf, "HOME=/Users/me\x00CLAUDE_CONFIG_DIR=/Users/me/.claude-pro\x00\x00junk=1\x00"...)

	env := envFromProcArgs(buf)
	if got := env["CLAUDE_CONFIG_DIR"]; got != "/Users/me/.claude-pro" {
		t.Errorf("CLAUDE_CONFIG_DIR = %q", got)
	}
	if got := env["HOME"]; got != "/Users/me" {
		t.Errorf("HOME = %q", got)
	}
	if _, ok := env["junk"]; ok {
		t.Error("parsing must stop at the end of the env block")
	}
	if envFromProcArgs([]byte{1}) != nil {
		t.Error("expected nil for a truncated buffer")
	}
}

func TestReadProcessEnv_Self(t *testing.T) {
	// Reads what the kernel recorded at exec, i.e. the environment this test
	// binary was started with (PATH is always set by `go test`).
	env := readProcessEnv(strconv.Itoa(os.Getpid()))
	if env == nil {
		t.Skip("process environment not readable on this platform")
	}
	if env["PATH"] != os.Getenv("PATH") {
		t.Errorf("PATH from process env = %q, want %q", env["PATH"], os.Getenv("PATH"))
	}
}

func TestClaudeSessionTitle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	lines := `{"type":"user","message":"hi"}
{"type":"ai-title","aiTitle":"First title","sessionId":"s"}
{"type":"ai-title","aiTitle":"Review code","sessionId":"s"}
`
	if err := os.WriteFile(path, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := claudeSessionTitle(path); got != "Review code" {
		t.Errorf("title = %q, want last ai-title", got)
	}

	// A /rename title wins over generated ones, even older.
	lines = `{"type":"custom-title","customTitle":"My name","sessionId":"s"}
{"type":"ai-title","aiTitle":"Generated","sessionId":"s"}`
	if err := os.WriteFile(path, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := claudeSessionTitle(path); got != "My name" {
		t.Errorf("title = %q, want custom title", got)
	}

	if got := claudeSessionTitle(filepath.Join(t.TempDir(), "missing.jsonl")); got != "" {
		t.Errorf("title = %q, want empty for missing transcript", got)
	}
}
