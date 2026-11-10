package detect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The fixtures below copy what Qwen Code writes, read from
// github.com/QwenLM/qwen-code at commit a34557e (0.27.1):
//
//   - packages/core/src/config/storage.ts: the runtime base is
//     $QWEN_RUNTIME_DIR, else $QWEN_HOME, else ~/.qwen (getRuntimeBaseDir,
//     getGlobalQwenDir), and a project lives in <base>/projects/<sanitizeCwd>
//     (getProjectDir).
//   - packages/core/src/utils/paths.ts: sanitizeCwd turns every
//     non-alphanumeric character into "-".
//   - packages/core/src/services/sessionService.ts: transcripts are
//     chats/<sessionId>.jsonl, named after SESSION_FILE_PATTERN, archived ones
//     go to chats/archive/, and loadSession refuses a transcript whose first
//     record names another cwd or that holds no conversation record.
//   - packages/core/src/services/chatRecordingService.ts: createBaseRecord
//     stamps uuid, parentUuid, sessionId, timestamp, type and cwd on every line.
//   - packages/core/src/utils/transcript-records.ts: the system subtypes that
//     are not conversation.
//   - packages/cli/src/runtime/agent-session-source.ts: the session_source
//     types of machine-owned sessions.

// qwenSessionID and qwenOtherSessionID are UUIDs as Qwen Code names its
// transcripts: anything else in the chats folder is not a session.
const (
	qwenSessionID      = "4f1d2c3b-5a6e-4f70-8a9b-0c1d2e3f4a5b"
	qwenOtherSessionID = "9e8d7c6b-5a49-4382-9170-6f5e4d3c2b1a"
)

// qwenChatsDir returns the chats folder Qwen Code keeps the sessions of cwd
// in, under a runtime base such as ~/.qwen.
func qwenChatsDir(base, cwd string) string {
	return filepath.Join(base, "projects", nonAlnum.ReplaceAllString(cwd, "-"), "chats")
}

// qwenRecordLine builds one transcript line the way createBaseRecord does, for
// a record of the given type and subtype stamped with cwd. An empty subtype is
// left out, as Qwen Code leaves it out of plain messages.
func qwenRecordLine(sessionID, cwd, recordType, subtype string) string {
	line := `{"uuid":"` + recordType + `-` + subtype + `","parentUuid":null,"sessionId":"` + sessionID + `",` +
		`"timestamp":"2026-11-10T09:00:00.000Z","type":"` + recordType + `"`
	if subtype != "" {
		line += `,"subtype":"` + subtype + `"`
	}
	line += `,"cwd":"` + cwd + `","version":"0.27.1"`
	if recordType == "user" {
		line += `,"message":{"role":"user","parts":[{"text":"fix the login form"}]}`
	}
	return line + "}"
}

// writeQwenTranscript writes lines as the transcript of sessionID in
// chatsDir and dates it age ago, so tests can order several sessions.
func writeQwenTranscript(t *testing.T, chatsDir, sessionID string, lines []string, age time.Duration) {
	t.Helper()
	if err := os.MkdirAll(chatsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := ""
	if len(lines) > 0 {
		content = strings.Join(lines, "\n") + "\n"
	}
	path := filepath.Join(chatsDir, sessionID+".jsonl")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	modified := time.Now().Add(-age)
	if err := os.Chtimes(path, modified, modified); err != nil {
		t.Fatal(err)
	}
}

// writeQwenSession writes a session started in recordedCWD into the chats
// folder of folderCWD: a model record, then a user prompt. The two paths are
// separate because different directories share one folder.
func writeQwenSession(t *testing.T, base, folderCWD, recordedCWD, sessionID string, age time.Duration) {
	t.Helper()
	lines := []string{
		qwenRecordLine(sessionID, recordedCWD, "system", "session_model"),
		qwenRecordLine(sessionID, recordedCWD, "user", ""),
	}
	writeQwenTranscript(t, qwenChatsDir(base, folderCWD), sessionID, lines, age)
}

func TestDetectQwen_ResumesSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	cwd := "/Users/dev/my.proj"

	// sanitizeCwd keeps the leading dash: /Users/dev/my.proj → -Users-dev-my-proj.
	writeQwenSession(t, filepath.Join(home, ".qwen"), cwd, cwd, qwenSessionID, time.Minute)
	if _, err := os.Stat(filepath.Join(home, ".qwen", "projects", "-Users-dev-my-proj", "chats", qwenSessionID+".jsonl")); err != nil {
		t.Fatalf("fixture not where Qwen Code writes it: %v", err)
	}

	s := detectQwen(cwd, "")

	want := Session{Tool: "qwen", CWD: cwd, Command: "qwen --resume " + qwenSessionID}
	if s == nil || *s != want {
		t.Fatalf("got %+v, want %+v", s, want)
	}
}

func TestDetectQwen_NewestOfTwo(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	base := filepath.Join(home, ".qwen")
	cwd := "/src/app"

	writeQwenSession(t, base, cwd, cwd, qwenOtherSessionID, time.Hour)
	writeQwenSession(t, base, cwd, cwd, qwenSessionID, time.Minute)

	s := detectQwen(cwd, "")

	if s == nil || s.Command != "qwen --resume "+qwenSessionID {
		t.Fatalf("got %+v, want qwen --resume %s", s, qwenSessionID)
	}
}

func TestDetectQwen_IgnoresArchivedSessions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	base := filepath.Join(home, ".qwen")
	cwd := "/src/app"

	// An archived session is newer, but `qwen --resume <id>` only opens
	// active ones (loadSession reads the active state).
	writeQwenSession(t, base, cwd, cwd, qwenOtherSessionID, time.Hour)
	archived := []string{
		qwenRecordLine(qwenSessionID, cwd, "user", ""),
	}
	writeQwenTranscript(t, filepath.Join(qwenChatsDir(base, cwd), "archive"), qwenSessionID, archived, time.Minute)

	s := detectQwen(cwd, "")

	if s == nil || s.Command != "qwen --resume "+qwenOtherSessionID {
		t.Fatalf("got %+v, want qwen --resume %s", s, qwenOtherSessionID)
	}
}

func TestDetectQwen_RecordedCWDTellsCollidingFoldersApart(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	base := filepath.Join(home, ".qwen")

	// Both directories encode to -w-acme-foo-bar, so they share one folder.
	first := "/w/acme/foo-bar"
	second := "/w/acme-foo/bar"
	writeQwenSession(t, base, first, first, qwenOtherSessionID, time.Hour)
	writeQwenSession(t, base, second, second, qwenSessionID, time.Minute)

	if s := detectQwen(first, ""); s == nil || s.Command != "qwen --resume "+qwenOtherSessionID {
		t.Errorf("%s: got %+v, want its own older session", first, s)
	}
	if s := detectQwen(second, ""); s == nil || s.Command != "qwen --resume "+qwenSessionID {
		t.Errorf("%s: got %+v, want its own session", second, s)
	}
}

func TestDetectQwen_RuntimeDirOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := "/src/app"
	runtimeDir := filepath.Join(t.TempDir(), "qwen runtime")

	// A newer session under ~/.qwen must not win over the folder the process
	// actually writes to.
	writeQwenSession(t, runtimeDir, cwd, cwd, qwenOtherSessionID, time.Hour)
	writeQwenSession(t, filepath.Join(home, ".qwen"), cwd, cwd, qwenSessionID, time.Minute)
	stubProcessEnv(t, map[string]map[string]string{"42": {"QWEN_RUNTIME_DIR": runtimeDir}})

	s := detectQwen(cwd, "42")

	// The resume has to look in the same place, so it sets the variable again.
	want := "QWEN_RUNTIME_DIR='" + runtimeDir + "' qwen --resume " + qwenOtherSessionID
	if s == nil || s.Command != want {
		t.Fatalf("got %+v, want %q", s, want)
	}
}

func TestDetectQwen_QwenHomeOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := "/src/app"
	qwenHome := filepath.Join(home, ".qwen-work")

	// QWEN_HOME replaces ~/.qwen as a whole, so the sessions follow it when
	// QWEN_RUNTIME_DIR is not set. "~" is expanded the way Qwen Code does.
	writeQwenSession(t, qwenHome, cwd, cwd, qwenSessionID, time.Minute)
	stubProcessEnv(t, map[string]map[string]string{"7": {"QWEN_HOME": "~/.qwen-work"}})

	s := detectQwen(cwd, "7")

	want := "QWEN_HOME=" + qwenHome + " qwen --resume " + qwenSessionID
	if s == nil || s.Command != want {
		t.Fatalf("got %+v, want %q", s, want)
	}
}

func TestDetectQwen_DefaultRuntimeDirNeedsNoPrefix(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := "/src/app"
	writeQwenSession(t, filepath.Join(home, ".qwen"), cwd, cwd, qwenSessionID, time.Minute)

	// Naming the default folder, or a relative one crex cannot place, moves
	// nothing: the command stays plain.
	for _, value := range []string{filepath.Join(home, ".qwen"), "relative/qwen"} {
		stubProcessEnv(t, map[string]map[string]string{"9": {"QWEN_RUNTIME_DIR": value}})

		s := detectQwen(cwd, "9")

		if s == nil || s.Command != "qwen --resume "+qwenSessionID {
			t.Errorf("QWEN_RUNTIME_DIR=%s: got %+v, want plain qwen --resume %s", value, s, qwenSessionID)
		}
	}
}

func TestDetectQwen_SkipsSessionsWithoutConversation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	base := filepath.Join(home, ".qwen")
	cwd := "/src/app"
	writeQwenSession(t, base, cwd, cwd, qwenOtherSessionID, time.Hour)

	// ensureConversationFile creates the file empty, and a transcript holding
	// only artifact records reconstructs to no messages. loadSession returns
	// nothing for either, and `qwen --resume <id>` exits with "No saved
	// session found".
	chatsDir := qwenChatsDir(base, cwd)
	writeQwenTranscript(t, chatsDir, "0a1b2c3d-0000-4000-8000-000000000001", nil, 2*time.Minute)
	artifactsOnly := []string{
		qwenRecordLine("0a1b2c3d-0000-4000-8000-000000000002", cwd, "system", "session_artifact_snapshot"),
		qwenRecordLine("0a1b2c3d-0000-4000-8000-000000000002", cwd, "system", "session_sources_snapshot"),
	}
	writeQwenTranscript(t, chatsDir, "0a1b2c3d-0000-4000-8000-000000000002", artifactsOnly, time.Minute)

	s := detectQwen(cwd, "")

	if s == nil || s.Command != "qwen --resume "+qwenOtherSessionID {
		t.Fatalf("got %+v, want qwen --resume %s", s, qwenOtherSessionID)
	}
}

func TestDetectQwen_SkipsAgentSessions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	base := filepath.Join(home, ".qwen")
	cwd := "/src/app"
	writeQwenSession(t, base, cwd, cwd, qwenOtherSessionID, time.Hour)

	// A session an agent host opens records its source near the start.
	// `qwen --continue` passes these over and no listing shows them, so a
	// terminal pane is never running one.
	source := `{"uuid":"src","parentUuid":null,"sessionId":"` + qwenSessionID + `","timestamp":"2026-11-10T09:00:00.000Z",` +
		`"type":"system","subtype":"session_source","cwd":"` + cwd + `","systemPayload":{"sourceType":"agent","sourceId":"reviewer"}}`
	agentSession := []string{source, qwenRecordLine(qwenSessionID, cwd, "user", "")}
	writeQwenTranscript(t, qwenChatsDir(base, cwd), qwenSessionID, agentSession, time.Minute)

	s := detectQwen(cwd, "")

	if s == nil || s.Command != "qwen --resume "+qwenOtherSessionID {
		t.Fatalf("got %+v, want qwen --resume %s", s, qwenOtherSessionID)
	}
}

func TestDetectQwen_IgnoresFilesThatAreNotSessions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	base := filepath.Join(home, ".qwen")
	cwd := "/src/app"
	writeQwenSession(t, base, cwd, cwd, qwenSessionID, time.Hour)

	// The prompt ledger sits next to the transcript as <id>.ledger.jsonl, and
	// a name that is not a UUID could carry shell syntax into the command.
	lines := []string{qwenRecordLine(qwenOtherSessionID, cwd, "user", "")}
	chatsDir := qwenChatsDir(base, cwd)
	writeQwenTranscript(t, chatsDir, qwenOtherSessionID+".ledger", lines, time.Minute)
	writeQwenTranscript(t, chatsDir, "x;rm -rf ~", lines, time.Minute)

	s := detectQwen(cwd, "")

	if s == nil || s.Command != "qwen --resume "+qwenSessionID {
		t.Fatalf("got %+v, want qwen --resume %s", s, qwenSessionID)
	}
}

func TestDetectQwen_NoSessionStartsBare(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	cwd := "/src/app"

	// Without any session, and then with only another directory's session in
	// the shared folder, Qwen Code starts as it did before crex resumed it.
	for _, setup := range []string{"no session", "other directory"} {
		if setup == "other directory" {
			writeQwenSession(t, filepath.Join(home, ".qwen"), cwd, "/src-app", qwenSessionID, time.Minute)
		}

		s := detectQwen(cwd, "")

		want := Session{Tool: "qwen", CWD: cwd, Command: "qwen"}
		if s == nil || *s != want {
			t.Errorf("%s: got %+v, want %+v", setup, s, want)
		}
	}
}
