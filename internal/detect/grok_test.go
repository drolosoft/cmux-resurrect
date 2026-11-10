package detect

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// writeGrokActiveSessions writes the registry of open Grok Build sessions.
//
// Format from xai-org/grok-build,
// crates/codegen/xai-grok-active-sessions/src/lib.rs: a pretty-printed JSON
// array of ActiveSession {session_id, pid, cwd, opened_at}, where the pid is
// std::process::id() of the TUI (xai-grok-pager/src/app/effects/mod.rs).
func writeGrokActiveSessions(t *testing.T, grokHome, content string) {
	t.Helper()

	if err := os.MkdirAll(grokHome, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(grokHome, "active_sessions.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// writeGrokSummary writes the summary.json of one Grok Build session.
//
// Layout from xai-org/grok-build: the directory is
// <grok home>/sessions/<encode_cwd_dirname(cwd)>/<session id>/
// (crates/codegen/xai-grok-config/src/paths.rs, sessions_cwd_dir_in, and
// crates/codegen/xai-grok-shell/src/session/persistence.rs, session_dir_in),
// and only a directory holding summary.json counts as resumable
// (persistence.rs, is_persisted_session_dir). The fields are those of
// Summary in persistence.rs, with Info {id, cwd} from
// crates/codegen/xai-grok-shared/src/session/info.rs.
func writeGrokSummary(t *testing.T, cwdDir, sessionID, content string) {
	t.Helper()

	sessionDir := filepath.Join(cwdDir, sessionID)
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "summary.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// grokSummary builds a summary.json for an ordinary interactive session that
// has messages, last changed at updatedAt.
func grokSummary(sessionID, cwd, updatedAt string) string {
	return `{
  "info": {"id": "` + sessionID + `", "cwd": "` + cwd + `"},
  "session_summary": "Fix the login form",
  "created_at": "2026-09-01T10:00:00.000000Z",
  "updated_at": "` + updatedAt + `",
  "num_messages": 6,
  "num_chat_messages": 3,
  "current_model_id": "grok-build"
}`
}

func TestDetectGrok_NoGrokHome(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	stubProcessEnv(t, nil)

	if s := detectGrok("/src/app", "42"); s != nil {
		t.Errorf("got %+v, want nil when neither Grok tool has written anything", s)
	}
}

func TestDetectGrok_ActiveSessionForPID(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	grokHome := filepath.Join(home, ".grok")
	cwdDir := filepath.Join(grokHome, "sessions", "%2Fsrc%2Fapp")
	older := "019a1f2e-7c3b-7d40-9a51-2b6c8e0f4a11"
	newer := "019a1f30-1d22-7e8a-b3f4-5c6d7e8f9a22"
	writeGrokSummary(t, cwdDir, older, grokSummary(older, "/src/app", "2026-09-29T09:00:00.000000Z"))
	writeGrokSummary(t, cwdDir, newer, grokSummary(newer, "/src/app", "2026-09-29T16:57:09.123456789Z"))
	writeGrokActiveSessions(t, grokHome, `[
  {
    "session_id": "`+older+`",
    "pid": 4242,
    "cwd": "/src/app",
    "opened_at": "2026-09-29T08:59:00.000000Z"
  }
]`)

	// The PID's own entry wins over the most recent session of the folder.
	if s := detectGrok("/src/app", "4242"); s == nil || s.Command != "grok --resume "+older {
		t.Errorf("pid 4242: got %+v, want grok --resume %s", s, older)
	}

	// A process without an entry gets the most recent session of the folder.
	if s := detectGrok("/src/app", "5151"); s == nil || s.Command != "grok --resume "+newer {
		t.Errorf("pid 5151: got %+v, want grok --resume %s", s, newer)
	}
}

func TestDetectGrok_ActiveSessionMustMatchFolderAndExist(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	grokHome := filepath.Join(home, ".grok")
	cwdDir := filepath.Join(grokHome, "sessions", "%2Fsrc%2Fapp")
	saved := "019a1f2e-7c3b-7d40-9a51-2b6c8e0f4a11"
	writeGrokSummary(t, cwdDir, saved, grokSummary(saved, "/src/app", "2026-09-29T09:00:00Z"))

	// Entry 1 belongs to another folder, entry 2 has no summary.json yet
	// (Grok writes it on the first message), entry 3 is not a safe id.
	writeGrokActiveSessions(t, grokHome, `[
  {"session_id": "019a0000-0000-7000-8000-000000000001", "pid": 1, "cwd": "/src/other", "opened_at": "2026-09-29T09:00:00Z"},
  {"session_id": "019a0000-0000-7000-8000-000000000002", "pid": 2, "cwd": "/src/app", "opened_at": "2026-09-29T09:00:00Z"},
  {"session_id": "x;rm -rf ~", "pid": 3, "cwd": "/src/app", "opened_at": "2026-09-29T09:00:00Z"}
]`)

	for _, pid := range []string{"1", "2", "3"} {
		if s := detectGrok("/src/app", pid); s == nil || s.Command != "grok --resume "+saved {
			t.Errorf("pid %s: got %+v, want the folder's own session %s", pid, s, saved)
		}
	}
}

func TestDetectGrok_LatestSkipsSessionsGrokWouldNotContinue(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	cwdDir := filepath.Join(home, ".grok", "sessions", "%2Fsrc%2Fapp")
	wanted := "019a0000-0000-7000-8000-00000000000a"
	writeGrokSummary(t, cwdDir, wanted, grokSummary(wanted, "/src/app", "2026-09-29T09:00:00Z"))

	// Every one of these is newer, and Grok's own --continue would skip it
	// (persistence.rs, most_recent_local_summary_for_cwd_in_view): a
	// subagent session, an explicitly hidden one, a one-shot `grok -p`, and
	// an unused husk with no messages and no title.
	writeGrokSummary(t, cwdDir, "019a0000-0000-7000-8000-00000000000b", `{
  "info": {"id": "019a0000-0000-7000-8000-00000000000b", "cwd": "/src/app"},
  "session_summary": "Explore the repo", "session_kind": "subagent",
  "updated_at": "2026-09-29T12:00:00Z", "num_messages": 9
}`)
	writeGrokSummary(t, cwdDir, "019a0000-0000-7000-8000-00000000000c", `{
  "info": {"id": "019a0000-0000-7000-8000-00000000000c", "cwd": "/src/app"},
  "session_summary": "Hidden", "hidden": true,
  "updated_at": "2026-09-29T12:00:00Z", "num_messages": 9
}`)
	writeGrokSummary(t, cwdDir, "019a0000-0000-7000-8000-00000000000d", `{
  "info": {"id": "019a0000-0000-7000-8000-00000000000d", "cwd": "/src/app"},
  "session_summary": "One shot", "session_kind": "headless",
  "updated_at": "2026-09-29T12:00:00Z", "num_messages": 2
}`)
	writeGrokSummary(t, cwdDir, "019a0000-0000-7000-8000-00000000000e", `{
  "info": {"id": "019a0000-0000-7000-8000-00000000000e", "cwd": "/src/app"},
  "session_summary": "  ", "updated_at": "2026-09-29T12:00:00Z", "num_messages": 0
}`)

	if s := detectGrok("/src/app", ""); s == nil || s.Command != "grok --resume "+wanted {
		t.Errorf("got %+v, want grok --resume %s", s, wanted)
	}
}

func TestDetectGrok_LastActiveBeatsUpdated(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	cwdDir := filepath.Join(home, ".grok", "sessions", "%2Fsrc%2Fapp")

	// Renaming an old session bumps updated_at but not last_active_at, the
	// time Grok sorts by when it has one.
	writeGrokSummary(t, cwdDir, "019a0000-0000-7000-8000-000000000001", `{
  "info": {"id": "019a0000-0000-7000-8000-000000000001", "cwd": "/src/app"},
  "session_summary": "Renamed yesterday", "num_messages": 4,
  "updated_at": "2026-09-29T15:00:00Z", "last_active_at": "2026-09-20T10:00:00Z"
}`)
	writeGrokSummary(t, cwdDir, "019a0000-0000-7000-8000-000000000002", `{
  "info": {"id": "019a0000-0000-7000-8000-000000000002", "cwd": "/src/app"},
  "session_summary": "Worked on today", "num_messages": 4,
  "updated_at": "2026-09-29T14:00:00Z", "last_active_at": "2026-09-29T14:00:00Z"
}`)

	want := "grok --resume 019a0000-0000-7000-8000-000000000002"
	if s := detectGrok("/src/app", ""); s == nil || s.Command != want {
		t.Errorf("got %+v, want %s", s, want)
	}
}

func TestDetectGrok_ContinueWhenNoSessionIsReadable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	writeGrokActiveSessions(t, filepath.Join(home, ".grok"), `[]`)

	// Grok Build is in use but nothing for this folder can be read: let
	// Grok pick the folder's most recent session itself.
	if s := detectGrok("/src/app", "42"); s == nil || s.Command != "grok --continue" {
		t.Errorf("got %+v, want grok --continue", s)
	}
}

func TestDetectGrok_GrokHomeFromProcess(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	grokHome := filepath.Join(home, "work grok")
	saved := "019a1f2e-7c3b-7d40-9a51-2b6c8e0f4a11"
	writeGrokSummary(t, filepath.Join(grokHome, "sessions", "%2Fsrc%2Fapp"), saved, grokSummary(saved, "/src/app", "2026-09-29T09:00:00Z"))
	stubProcessEnv(t, map[string]map[string]string{"42": {"GROK_HOME": grokHome}})

	// GROK_HOME replaces ~/.grok (crates/codegen/xai-dirs/src/lib.rs,
	// resolve_grok_home_from), so the resume command has to set it again.
	want := "GROK_HOME='" + grokHome + "' grok --resume " + saved
	if s := detectGrok("/src/app", "42"); s == nil || s.Command != want {
		t.Errorf("got %+v, want %s", s, want)
	}
}

func TestDetectGrok_LongFolderFoundByCwdFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	cwd := "/Users/dev/Documents/" + strings.Repeat("開発プロジェクト/", 6) + "main-branch"

	// Past 255 encoded bytes Grok names the folder <slug>-<blake3 prefix>
	// and writes the real path to a .cwd file inside it (paths.rs,
	// encode_cwd_dirname and ensure_sessions_cwd_dir_in).
	cwdDir := filepath.Join(home, ".grok", "sessions", "main-branch-0123456789abcdef")
	saved := "019a1f2e-7c3b-7d40-9a51-2b6c8e0f4a11"
	writeGrokSummary(t, cwdDir, saved, grokSummary(saved, cwd, "2026-09-29T09:00:00Z"))
	if err := os.WriteFile(filepath.Join(cwdDir, ".cwd"), []byte(cwd), 0o600); err != nil {
		t.Fatal(err)
	}

	if s := detectGrok(cwd, ""); s == nil || s.Command != "grok --resume "+saved {
		t.Errorf("got %+v, want grok --resume %s", s, saved)
	}
}

func TestGrokEncodeCWD(t *testing.T) {
	// Expected values follow the Rust urlencoding crate that paths.rs uses:
	// everything but ASCII letters, digits and -._~ becomes %XX, upper case.
	tests := []struct {
		cwd, want string
	}{
		{"/Users/foo/project", "%2FUsers%2Ffoo%2Fproject"},
		{"/tmp", "%2Ftmp"},
		{"/src/my app_v1.2~x", "%2Fsrc%2Fmy%20app_v1.2~x"},
		{"/Users/user/Documents/project-名前", "%2FUsers%2Fuser%2FDocuments%2Fproject-%E5%90%8D%E5%89%8D"},
	}
	for _, tt := range tests {
		if got := grokEncodeCWD(tt.cwd); got != tt.want {
			t.Errorf("grokEncodeCWD(%q) = %q, want %q", tt.cwd, got, tt.want)
		}
	}
}

// writeGrokCLIDatabase creates the ~/.grok/grok.db of the community grok-cli
// with the sqlite3 command line tool, skipping the test without it.
//
// Schema from superagent-ai/grok-cli, src/storage/db.ts (the path) and
// src/storage/migrations.ts (the sessions table, trimmed to the columns that
// matter here).
func writeGrokCLIDatabase(t *testing.T, home, inserts string) {
	t.Helper()

	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 not installed")
	}
	if err := os.MkdirAll(filepath.Join(home, ".grok"), 0o700); err != nil {
		t.Fatal(err)
	}
	schema := `CREATE TABLE sessions (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL,
  cwd_at_start TEXT NOT NULL,
  cwd_last TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
) STRICT;`
	out, err := exec.Command("sqlite3", filepath.Join(home, ".grok", "grok.db"), schema+inserts).CombinedOutput()
	if err != nil {
		t.Fatalf("sqlite3: %v: %s", err, out)
	}
}

func TestDetectGrok_GrokCLIDatabase(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	writeGrokCLIDatabase(t, home, `
INSERT INTO sessions VALUES ('older', 'w1', '/src/app', '/src/app', 'active', '2026-03-19T10:00:00.000Z', '2026-03-19T10:00:00.000Z');
INSERT INTO sessions VALUES ('newer', 'w1', '/src/app', '/src/app', 'active', '2026-03-19T11:00:00.000Z', '2026-03-19T17:59:23.123Z');
INSERT INTO sessions VALUES ('elsewhere', 'w2', '/src/other', '/src/other', 'active', '2026-03-20T10:00:00.000Z', '2026-03-20T10:00:00.000Z');`)

	// grok-cli resumes with --session <id> (src/index.ts); it has no
	// --continue, and its 'latest' ignores the folder.
	if s := detectGrok("/src/app", ""); s == nil || s.Command != "grok --session newer" {
		t.Errorf("got %+v, want grok --session newer", s)
	}
	if s := detectGrok("/src/none", ""); s != nil {
		t.Errorf("got %+v, want nil for a folder grok-cli never saw", s)
	}
}

func TestDetectGrok_NewestStoreWins(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)

	// Both tools install to ~/.grok/bin/grok, so a machine can hold data
	// from both. The store written last belongs to the one that runs.
	writeGrokCLIDatabase(t, home, `
INSERT INTO sessions VALUES ('cli-session', 'w1', '/src/app', '/src/app', 'active', '2026-03-19T10:00:00.000Z', '2026-03-19T10:00:00.000Z');
INSERT INTO sessions VALUES ('cli-newer', 'w2', '/src/web', '/src/web', 'active', '2026-10-01T10:00:00.000Z', '2026-10-01T10:00:00.000Z');`)
	saved := "019a1f2e-7c3b-7d40-9a51-2b6c8e0f4a11"
	writeGrokSummary(t, filepath.Join(home, ".grok", "sessions", "%2Fsrc%2Fapp"), saved, grokSummary(saved, "/src/app", "2026-09-29T09:00:00Z"))
	older := "019a0000-0000-7000-8000-000000000001"
	writeGrokSummary(t, filepath.Join(home, ".grok", "sessions", "%2Fsrc%2Fweb"), older, grokSummary(older, "/src/web", "2026-09-29T09:00:00Z"))

	if s := detectGrok("/src/app", ""); s == nil || s.Command != "grok --resume "+saved {
		t.Errorf("/src/app: got %+v, want the newer Grok Build session %s", s, saved)
	}
	if s := detectGrok("/src/web", ""); s == nil || s.Command != "grok --session cli-newer" {
		t.Errorf("/src/web: got %+v, want the newer grok-cli session cli-newer", s)
	}
}
