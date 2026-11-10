package detect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The fixtures below are built from the Gemini CLI sources, since the tool
// is not installed on the machines these tests run on:
//
//   - projects.json and tmp/<slug>/: packages/core/src/config/storage.ts
//     (initialize, getProjectTempDir) and projectRegistry.ts (RegistryData).
//   - session-<timestamp>-<first 8 of id>.jsonl, one JSON record per line,
//     metadata first: packages/core/src/services/chatRecordingService.ts
//     (doInitialize, appendRecord) and chatRecordingTypes.ts.
//   - --resume takes the whole UUID, an index or "latest", and only offers
//     sessions with conversation in them: packages/cli/src/utils/sessionUtils.ts
//     (getAllSessionFiles, findSession, resolveSession).

// geminiSessionID is a full session UUID. Gemini CLI names the file after
// its first 8 characters only.
const geminiSessionID = "3f2a9c1e-7b4d-4e8a-9c21-5d6e7f8a9b0c"

// geminiOlderSessionID belongs to an earlier conversation in the same folder.
const geminiOlderSessionID = "a81c0d42-19e5-4f37-b6a0-2c9d8e7f6a51"

// geminiMetadata returns the first line of a .jsonl session: the metadata
// record written by ChatRecordingService.doInitialize when a session starts
// (chatRecordingService.ts, "initialMetadata").
func geminiMetadata(sessionID, kind string) string {
	return `{"sessionId":"` + sessionID + `","projectHash":"5f0c3b2a","startTime":"2026-11-10T09:15:02.118Z","lastUpdated":"2026-11-10T09:15:02.118Z","kind":"` + kind + `"}`
}

// geminiUserPrompt is a user message record as appended by recordMessage
// (chatRecordingService.ts; MessageRecord in chatRecordingTypes.ts). The
// content is a PartListUnion, here an array of text parts.
const geminiUserPrompt = `{"id":"9d1e2f30-0001","timestamp":"2026-11-10T09:15:40.511Z","type":"user","content":[{"text":"explain the restore flow"}]}`

// geminiModelReply is the model turn that answers geminiUserPrompt
// (chatRecordingService.ts, recordMessage with type "gemini").
const geminiModelReply = `{"id":"9d1e2f30-0002","timestamp":"2026-11-10T09:15:44.020Z","type":"gemini","content":"The restore flow reads the layout first.","model":"gemini-2.5-pro"}`

// geminiTouch is a metadata update appended after each turn
// (chatRecordingService.ts, updateMetadata writes {"$set": ...}).
const geminiTouch = `{"$set":{"lastUpdated":"2026-11-10T09:15:44.020Z"}}`

// geminiInjectedContext is a user record Gemini CLI writes itself. It does
// not count as conversation (isIgnoredUserContent in
// packages/core/src/utils/sessionUtils.ts).
const geminiInjectedContext = `{"id":"9d1e2f30-0000","timestamp":"2026-11-10T09:15:02.200Z","type":"user","content":[{"text":"<session_context>cwd: /src/app</session_context>"}]}`

// writeGeminiRegistry writes ~/.gemini/projects.json mapping cwd to slug, in
// the RegistryData shape of projectRegistry.ts.
func writeGeminiRegistry(t *testing.T, home, cwd, slug string) {
	t.Helper()
	geminiDir := filepath.Join(home, ".gemini")
	if err := os.MkdirAll(geminiDir, 0o755); err != nil {
		t.Fatal(err)
	}

	registry := `{
  "projects": {
    "` + cwd + `": "` + slug + `"
  }
}`
	if err := os.WriteFile(filepath.Join(geminiDir, "projects.json"), []byte(registry), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeGeminiSession writes a session file whose lines are the given
// records and sets its modification time, which decides the newest one.
func writeGeminiSession(t *testing.T, chatsDir, name string, modTime time.Time, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(chatsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(chatsDir, name)
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatal(err)
	}
}

// geminiSlugChats returns ~/.gemini/tmp/<slug>/chats under home.
func geminiSlugChats(home, slug string) string {
	return filepath.Join(home, ".gemini", "tmp", slug, "chats")
}

func TestDetectGemini_NoSessionDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	s := detectGemini("/nonexistent/path/that/does/not/exist", "")
	if s != nil {
		t.Error("expected nil for nonexistent CWD")
	}
}

func TestDetectGemini_CurrentLayout(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := "/src/app"
	writeGeminiRegistry(t, home, cwd, "app")
	writeGeminiSession(t, geminiSlugChats(home, "app"), "session-2026-11-10T09-15-3f2a9c1e.jsonl", time.Now(),
		geminiMetadata(geminiSessionID, "main"), geminiInjectedContext, geminiUserPrompt, geminiModelReply, geminiTouch)

	s := detectGemini(cwd, "")
	if s == nil {
		t.Fatal("expected session, got nil")
	}
	if s.Tool != "gemini" || s.CWD != cwd {
		t.Errorf("Tool, CWD = %q, %q; want gemini, %q", s.Tool, s.CWD, cwd)
	}
	if want := "gemini --resume " + geminiSessionID; s.Command != want {
		t.Errorf("Command = %q, want %q", s.Command, want)
	}
}

func TestDetectGemini_NewestSessionWins(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := "/src/app"
	chatsDir := geminiSlugChats(home, "app")
	writeGeminiRegistry(t, home, cwd, "app")

	// The older conversation has the later file name: it was resumed and
	// is the one still being written to, so its modification time decides.
	now := time.Now()
	writeGeminiSession(t, chatsDir, "session-2026-11-10T09-15-3f2a9c1e.jsonl", now.Add(-time.Hour),
		geminiMetadata(geminiSessionID, "main"), geminiUserPrompt)
	writeGeminiSession(t, chatsDir, "session-2026-11-09T18-02-a81c0d42.jsonl", now,
		geminiMetadata(geminiOlderSessionID, "main"), geminiUserPrompt, geminiModelReply)

	s := detectGemini(cwd, "")
	if s == nil {
		t.Fatal("expected session, got nil")
	}
	if want := "gemini --resume " + geminiOlderSessionID; s.Command != want {
		t.Errorf("Command = %q, want %q", s.Command, want)
	}
}

func TestDetectGemini_SkipsSessionsWithoutConversation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := "/src/app"
	chatsDir := geminiSlugChats(home, "app")
	writeGeminiRegistry(t, home, cwd, "app")

	// Gemini CLI creates the file as soon as it starts. Until something is
	// asked, --resume with that UUID answers "Invalid session identifier".
	now := time.Now()
	writeGeminiSession(t, chatsDir, "session-2026-11-09T18-02-a81c0d42.jsonl", now.Add(-time.Hour),
		geminiMetadata(geminiOlderSessionID, "main"), geminiUserPrompt, geminiModelReply)
	writeGeminiSession(t, chatsDir, "session-2026-11-10T09-15-3f2a9c1e.jsonl", now,
		geminiMetadata(geminiSessionID, "main"), geminiInjectedContext)

	s := detectGemini(cwd, "")
	if s == nil {
		t.Fatal("expected the older session, got nil")
	}
	if want := "gemini --resume " + geminiOlderSessionID; s.Command != want {
		t.Errorf("Command = %q, want %q", s.Command, want)
	}
}

func TestDetectGemini_OnlyEmptySessions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := "/src/app"
	writeGeminiRegistry(t, home, cwd, "app")
	writeGeminiSession(t, geminiSlugChats(home, "app"), "session-2026-11-10T09-15-3f2a9c1e.jsonl", time.Now(),
		geminiMetadata(geminiSessionID, "main"), geminiInjectedContext)

	if s := detectGemini(cwd, ""); s != nil {
		t.Errorf("expected nil with nothing to resume, got %q", s.Command)
	}
}

func TestDetectGemini_SkipsSubagentSessions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := "/src/app"
	chatsDir := geminiSlugChats(home, "app")
	writeGeminiRegistry(t, home, cwd, "app")

	// Gemini CLI hides subagent sessions from --resume (getAllSessionFiles
	// in packages/cli/src/utils/sessionUtils.ts).
	now := time.Now()
	writeGeminiSession(t, chatsDir, "session-2026-11-09T18-02-a81c0d42.jsonl", now.Add(-time.Hour),
		geminiMetadata(geminiOlderSessionID, "main"), geminiUserPrompt)
	writeGeminiSession(t, chatsDir, "session-2026-11-10T09-15-3f2a9c1e.jsonl", now,
		geminiMetadata(geminiSessionID, "subagent"), geminiUserPrompt, geminiModelReply)

	s := detectGemini(cwd, "")
	if s == nil {
		t.Fatal("expected the main session, got nil")
	}
	if want := "gemini --resume " + geminiOlderSessionID; s.Command != want {
		t.Errorf("Command = %q, want %q", s.Command, want)
	}
}

func TestDetectGemini_UnreadableIDFallsBackToLatest(t *testing.T) {
	cases := map[string][]string{
		"no metadata line": {geminiUserPrompt, geminiModelReply},
		"broken metadata":  {`{"sessionId":"3f2a9c1e-7b4d`, geminiUserPrompt},
		"unsafe id":        {geminiMetadata("3f2a9c1e; rm -rf ~", "main"), geminiUserPrompt},
	}
	for name, lines := range cases {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			cwd := "/src/app"
			writeGeminiRegistry(t, home, cwd, "app")
			writeGeminiSession(t, geminiSlugChats(home, "app"), "session-2026-11-10T09-15-3f2a9c1e.jsonl", time.Now(), lines...)

			s := detectGemini(cwd, "")
			if s == nil {
				t.Fatal("expected a session, got nil")
			}
			if want := "gemini --resume latest"; s.Command != want {
				t.Errorf("Command = %q, want %q", s.Command, want)
			}
		})
	}
}

func TestDetectGemini_OtherProjectIsNotUsed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeGeminiRegistry(t, home, "/src/other", "other")
	writeGeminiSession(t, geminiSlugChats(home, "other"), "session-2026-11-10T09-15-3f2a9c1e.jsonl", time.Now(),
		geminiMetadata(geminiSessionID, "main"), geminiUserPrompt, geminiModelReply)

	if s := detectGemini("/src/app", ""); s != nil {
		t.Errorf("expected nil for a directory Gemini never saw, got %q", s.Command)
	}
}

func TestDetectGemini_RejectsSlugOutsideTmp(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := "/src/app"
	writeGeminiRegistry(t, home, cwd, "../escape")
	writeGeminiSession(t, filepath.Join(home, ".gemini", "escape", "chats"), "session-2026-11-10T09-15-3f2a9c1e.jsonl", time.Now(),
		geminiMetadata(geminiSessionID, "main"), geminiUserPrompt)

	if s := detectGemini(cwd, ""); s != nil {
		t.Errorf("expected nil for a slug outside tmp/, got %q", s.Command)
	}
}

func TestDetectGemini_LegacyLayout(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := t.TempDir()

	// Before the project registry, the folder was the SHA-256 of the
	// project root and the session a single JSON document (storage.ts,
	// performMigration; chatRecordingService.ts migrates ".json" to
	// ".jsonl" on resume).
	chatsDir := filepath.Join(home, ".gemini", "tmp", geminiProjectHash(cwd), "chats")
	legacy := `{
  "sessionId": "` + geminiSessionID + `",
  "projectHash": "` + geminiProjectHash(cwd) + `",
  "startTime": "2026-05-27T14:30:11.402Z",
  "lastUpdated": "2026-05-27T14:31:02.877Z",
  "messages": [
    {"id": "m1", "timestamp": "2026-05-27T14:30:40.000Z", "type": "user", "content": "add a flag"},
    {"id": "m2", "timestamp": "2026-05-27T14:31:02.877Z", "type": "gemini", "content": "Done."}
  ]
}`
	writeGeminiSession(t, chatsDir, "session-2026-05-27T14-30-3f2a9c1e.json", time.Now(), legacy)

	s := detectGemini(cwd, "")
	if s == nil {
		t.Fatal("expected session, got nil")
	}
	if want := "gemini --resume " + geminiSessionID; s.Command != want {
		t.Errorf("Command = %q, want %q", s.Command, want)
	}
}

func TestDetectGemini_GeminiCLIHome(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	customHome := t.TempDir()
	cwd := "/src/app"
	stubProcessEnv(t, map[string]map[string]string{"4242": {"GEMINI_CLI_HOME": customHome}})

	// GEMINI_CLI_HOME replaces the home directory for everything Gemini
	// CLI stores (homedir in packages/core/src/utils/paths.ts).
	writeGeminiRegistry(t, customHome, cwd, "app")
	writeGeminiSession(t, geminiSlugChats(customHome, "app"), "session-2026-11-10T09-15-3f2a9c1e.jsonl", time.Now(),
		geminiMetadata(geminiSessionID, "main"), geminiUserPrompt, geminiModelReply)

	s := detectGemini(cwd, "4242")
	if s == nil {
		t.Fatal("expected session, got nil")
	}
	if want := "gemini --resume " + geminiSessionID; s.Command != want {
		t.Errorf("Command = %q, want %q", s.Command, want)
	}
}

func TestGeminiContentText(t *testing.T) {
	cases := map[string]string{
		`"plain text"`:                     "plain text",
		`[{"text":"one "},{"text":"two"}]`: "one two",
		`["bare ", {"text":"part"}]`:       "bare part",
		`{"text":"single part"}`:           "single part",
		`[{"functionCall":{"name":"ls"}}]`: "",
		``:                                 "",
	}
	for raw, want := range cases {
		if got := geminiContentText([]byte(raw)); got != want {
			t.Errorf("geminiContentText(%s) = %q, want %q", raw, got, want)
		}
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
