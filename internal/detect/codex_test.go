package detect

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// codexThreadsSchema is the threads table of ~/.codex/state_5.sqlite as
// Codex 0.128 leaves it after its migration 44, copied from `.schema`. The
// fixtures keep every NOT NULL column so an insert that works here also
// matches what a real Codex writes.
const codexThreadsSchema = `CREATE TABLE threads (
    id TEXT PRIMARY KEY,
    rollout_path TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    source TEXT NOT NULL,
    model_provider TEXT NOT NULL,
    cwd TEXT NOT NULL,
    title TEXT NOT NULL,
    sandbox_policy TEXT NOT NULL,
    approval_mode TEXT NOT NULL,
    tokens_used INTEGER NOT NULL DEFAULT 0,
    has_user_event INTEGER NOT NULL DEFAULT 0,
    archived INTEGER NOT NULL DEFAULT 0,
    archived_at INTEGER,
    git_sha TEXT,
    git_branch TEXT,
    git_origin_url TEXT
, cli_version TEXT NOT NULL DEFAULT '', first_user_message TEXT NOT NULL DEFAULT '', agent_nickname TEXT, agent_role TEXT, memory_mode TEXT NOT NULL DEFAULT 'enabled', model TEXT, reasoning_effort TEXT, agent_path TEXT, created_at_ms INTEGER, updated_at_ms INTEGER, thread_source TEXT, preview TEXT NOT NULL DEFAULT '', recency_at INTEGER NOT NULL DEFAULT 0, recency_at_ms INTEGER NOT NULL DEFAULT 0, history_mode TEXT NOT NULL DEFAULT 'legacy', name TEXT, is_pinned INTEGER NOT NULL DEFAULT 0);
CREATE INDEX idx_threads_archived_cwd_updated_at_ms ON threads(archived, cwd, updated_at_ms DESC, id DESC);
`

// codexThread is one row of the threads table in a test fixture.
type codexThread struct {
	id       string
	cwd      string
	updated  time.Time
	archived bool
	// rollout is the rollout file the row points at. When it is empty the
	// helper creates one under sessions/, as Codex does.
	rollout string
}

// requireSQLite skips the test when the sqlite3 CLI is not installed, since
// both the detector and the fixtures go through it.
func requireSQLite(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 CLI not installed")
	}
}

// isolateCodex points HOME at a fresh directory and clears CODEX_HOME and
// the process environment, so the developer's own ~/.codex never leaks into
// a test. It returns the default Codex home inside that HOME.
func isolateCodex(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	stubProcessEnv(t, nil)

	return filepath.Join(home, ".codex")
}

// writeCodexRollout writes a Codex 0.128 rollout file whose first line is
// the session_meta record (format of ~/.codex/sessions/YYYY/MM/DD/
// rollout-*.jsonl), with instructions of the given size to make that line
// as long as needed, and a second line so the file looks like a real one.
func writeCodexRollout(t *testing.T, path, id, cwd string, instructionsSize int) {
	t.Helper()
	meta := map[string]any{
		"timestamp": "2025-01-15T10:00:00.000Z",
		"type":      "session_meta",
		"payload": map[string]any{
			"id":                id,
			"timestamp":         "2025-01-15T10:00:00.000Z",
			"cwd":               cwd,
			"originator":        "codex_cli_rs",
			"cli_version":       "0.128.0",
			"source":            "cli",
			"model_provider":    "openai",
			"base_instructions": map[string]string{"text": strings.Repeat("x", instructionsSize)},
		},
	}
	first, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	content := string(first) + "\n" + `{"type":"event_msg","payload":{"type":"task_started"}}` + "\n"

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// codexRolloutPath returns where Codex 0.128 files a session started at
// the given time: sessions/YYYY/MM/DD/rollout-<timestamp>-<id>.jsonl.
func codexRolloutPath(codexHome string, started time.Time, id string) string {
	day := filepath.Join(codexHome, "sessions", started.Format("2006"), started.Format("01"), started.Format("02"))
	return filepath.Join(day, "rollout-"+started.Format("2006-01-02T15-04-05")+"-"+id+".jsonl")
}

// writeCodexState creates state_5.sqlite in codexHome with the given
// threads, and a rollout file for every thread that does not name one.
func writeCodexState(t *testing.T, codexHome string, threads []codexThread) {
	t.Helper()
	requireSQLite(t)
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatal(err)
	}

	var script strings.Builder
	script.WriteString(codexThreadsSchema)
	for _, thread := range threads {
		rollout := thread.rollout
		if rollout == "" {
			rollout = codexRolloutPath(codexHome, thread.updated, thread.id)
			writeCodexRollout(t, rollout, thread.id, thread.cwd, 100)
		}
		archived := 0
		if thread.archived {
			archived = 1
		}
		fmt.Fprintf(&script,
			"INSERT INTO threads (id, rollout_path, created_at, updated_at, source, model_provider, cwd, title, sandbox_policy, approval_mode, archived) VALUES ('%s', '%s', %d, %d, 'cli', 'openai', '%s', 'title', 'workspace-write', 'on-request', %d);\n",
			escapeSQLite(thread.id), escapeSQLite(rollout), thread.updated.Unix(), thread.updated.Unix(), escapeSQLite(thread.cwd), archived)
	}

	cmd := exec.Command("sqlite3", filepath.Join(codexHome, "state_5.sqlite"))
	cmd.Stdin = strings.NewReader(script.String())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building state_5.sqlite: %v\n%s", err, out)
	}
}

// writeCodexLegacyRollout writes a pre-0.128 rollout-*.json in the root of
// sessions/. Real ones carry only session.{timestamp,id,instructions}; cwd
// is added only when the test passes one.
func writeCodexLegacyRollout(t *testing.T, codexHome, id, cwd string) {
	t.Helper()
	session := map[string]string{"timestamp": "2025-05-07T10:00:00.000Z", "id": id, "instructions": ""}
	if cwd != "" {
		session["cwd"] = cwd
	}
	data, err := json.Marshal(map[string]any{"session": session, "items": []any{}})
	if err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(codexHome, "sessions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rollout-2025-05-07-"+id+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// wantCodexResume fails the test unless the session resumes the given id
// with exactly the given command.
func wantCodexResume(t *testing.T, s *Session, command string) {
	t.Helper()
	if s == nil {
		t.Fatalf("got nil, want %q", command)
	}
	if s.Tool != "codex" || s.Command != command {
		t.Fatalf("got %+v, want codex with %q", s, command)
	}
}

// The thread table answers for a directory however old its last session
// is, and a newer thread elsewhere does not win.
func TestDetectCodex_StateFindsSessionOlderThan30Days(t *testing.T) {
	codexHome := isolateCodex(t)
	writeCodexState(t, codexHome, []codexThread{
		{id: "0190aaaa-0000-7000-8000-000000000001", cwd: "/work/project", updated: time.Now().AddDate(0, 0, -90)},
		{id: "0190aaaa-0000-7000-8000-000000000002", cwd: "/work/other", updated: time.Now()},
	})

	s := detectCodex("/work/project", "")
	wantCodexResume(t, s, "codex resume 0190aaaa-0000-7000-8000-000000000001")
}

// An archived thread is never resumed, even when it is the newest one in
// the directory.
func TestDetectCodex_StateSkipsArchived(t *testing.T) {
	codexHome := isolateCodex(t)
	writeCodexState(t, codexHome, []codexThread{
		{id: "0190bbbb-0000-7000-8000-000000000001", cwd: "/work/project", updated: time.Now().AddDate(0, 0, -3)},
		{id: "0190bbbb-0000-7000-8000-000000000002", cwd: "/work/project", updated: time.Now(), archived: true},
	})

	s := detectCodex("/work/project", "")
	wantCodexResume(t, s, "codex resume 0190bbbb-0000-7000-8000-000000000001")
}

// When the only thread of the directory is archived there is nothing to
// resume, and the rollout file it left in sessions/ must not bring it back.
func TestDetectCodex_StateOnlyArchivedIsNil(t *testing.T) {
	codexHome := isolateCodex(t)
	writeCodexState(t, codexHome, []codexThread{
		{id: "0190cccc-0000-7000-8000-000000000001", cwd: "/work/project", updated: time.Now(), archived: true},
	})

	if s := detectCodex("/work/project", ""); s != nil {
		t.Fatalf("got %+v, want nil for an archived thread", s)
	}
}

// A thread whose rollout file is gone cannot be resumed by Codex, so the
// next one of the directory is used.
func TestDetectCodex_StateSkipsMissingRollout(t *testing.T) {
	codexHome := isolateCodex(t)
	writeCodexState(t, codexHome, []codexThread{
		{id: "0190dddd-0000-7000-8000-000000000001", cwd: "/work/project", updated: time.Now().AddDate(0, 0, -1)},
		{id: "0190dddd-0000-7000-8000-000000000002", cwd: "/work/project", updated: time.Now(), rollout: filepath.Join(codexHome, "sessions", "gone.jsonl")},
	})

	s := detectCodex("/work/project", "")
	wantCodexResume(t, s, "codex resume 0190dddd-0000-7000-8000-000000000001")
}

// A state file sqlite3 cannot read sends the detector to the rollout files
// instead of giving up.
func TestDetectCodex_UnreadableStateFallsBackToRollouts(t *testing.T) {
	requireSQLite(t)
	codexHome := isolateCodex(t)
	id := "0190eeee-0000-7000-8000-000000000001"
	writeCodexRollout(t, codexRolloutPath(codexHome, time.Now(), id), id, "/work/project", 100)
	if err := os.WriteFile(filepath.Join(codexHome, "state_5.sqlite"), []byte("not a database"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := detectCodex("/work/project", "")
	wantCodexResume(t, s, "codex resume "+id)
}

// Without the state file, rollout files older than 30 days still count.
func TestDetectCodex_RolloutOlderThan30Days(t *testing.T) {
	codexHome := isolateCodex(t)
	id := "0190ffff-0000-7000-8000-000000000001"
	writeCodexRollout(t, codexRolloutPath(codexHome, time.Now().AddDate(0, 0, -90), id), id, "/work/project", 100)

	s := detectCodex("/work/project", "")
	wantCodexResume(t, s, "codex resume "+id)
}

// The most recently written rollout of the directory wins, even when it
// was started on an older day than another one.
func TestDetectCodex_RolloutPicksMostRecentlyWritten(t *testing.T) {
	codexHome := isolateCodex(t)
	continued := "01901111-0000-7000-8000-000000000001"
	untouched := "01901111-0000-7000-8000-000000000002"
	continuedPath := codexRolloutPath(codexHome, time.Now().AddDate(0, 0, -40), continued)
	untouchedPath := codexRolloutPath(codexHome, time.Now().AddDate(0, 0, -2), untouched)
	writeCodexRollout(t, continuedPath, continued, "/work/project", 100)
	writeCodexRollout(t, untouchedPath, untouched, "/work/project", 100)

	// The session started 40 days ago was written today; the newer one has
	// not been touched for two days.
	twoDaysAgo := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(untouchedPath, twoDaysAgo, twoDaysAgo); err != nil {
		t.Fatal(err)
	}

	s := detectCodex("/work/project", "")
	wantCodexResume(t, s, "codex resume "+continued)
}

// A session_meta line far above the old 64 KB scanner limit is still read.
func TestDetectCodex_RolloutWithLargeFirstLine(t *testing.T) {
	codexHome := isolateCodex(t)
	id := "01902222-0000-7000-8000-000000000001"
	writeCodexRollout(t, codexRolloutPath(codexHome, time.Now(), id), id, "/work/project", 300*1024)

	s := detectCodex("/work/project", "")
	wantCodexResume(t, s, "codex resume "+id)
}

// Legacy rollout files carry no directory, so they cannot prove they belong
// to this one and are never resumed.
func TestDetectCodex_LegacyWithoutCWDIsNil(t *testing.T) {
	codexHome := isolateCodex(t)
	writeCodexLegacyRollout(t, codexHome, "08e364d3-0000-4000-8000-000000000001", "")

	if s := detectCodex("/work/project", ""); s != nil {
		t.Fatalf("got %+v, want nil for a legacy file without cwd", s)
	}
}

// A legacy rollout file from another directory is not returned.
func TestDetectCodex_LegacyFromOtherDirectoryIsNil(t *testing.T) {
	codexHome := isolateCodex(t)
	writeCodexLegacyRollout(t, codexHome, "08e364d3-0000-4000-8000-000000000002", "/work/other")

	if s := detectCodex("/work/project", ""); s != nil {
		t.Fatalf("got %+v, want nil for a legacy file of another directory", s)
	}
}

// A legacy rollout file that names this directory is still resumed, even
// next to a newer one from somewhere else.
func TestDetectCodex_LegacyMatchingCWD(t *testing.T) {
	codexHome := isolateCodex(t)
	writeCodexLegacyRollout(t, codexHome, "08e364d3-0000-4000-8000-000000000003", "/work/project")
	writeCodexLegacyRollout(t, codexHome, "08e364d3-0000-4000-8000-000000000004", "/work/other")

	s := detectCodex("/work/project", "")
	wantCodexResume(t, s, "codex resume 08e364d3-0000-4000-8000-000000000003")
}

// CODEX_HOME in the environment of the Codex process decides where its
// sessions are, and the resume command sets it again.
func TestDetectCodex_CodexHomeFromProcess(t *testing.T) {
	defaultHome := isolateCodex(t)
	customHome := filepath.Join(t.TempDir(), "codex-work")
	writeCodexState(t, defaultHome, []codexThread{
		{id: "01903333-0000-7000-8000-000000000001", cwd: "/work/project", updated: time.Now()},
	})
	writeCodexState(t, customHome, []codexThread{
		{id: "01903333-0000-7000-8000-000000000002", cwd: "/work/project", updated: time.Now().AddDate(0, 0, -5)},
	})
	stubProcessEnv(t, map[string]map[string]string{"4242": {"CODEX_HOME": customHome}})

	s := detectCodex("/work/project", "4242")
	wantCodexResume(t, s, "CODEX_HOME="+customHome+" codex resume 01903333-0000-7000-8000-000000000002")
}

// When the process environment cannot be read, CODEX_HOME from crex's own
// environment is used.
func TestDetectCodex_CodexHomeFromEnvironment(t *testing.T) {
	isolateCodex(t)
	customHome := filepath.Join(t.TempDir(), "codex-env")
	id := "01904444-0000-7000-8000-000000000001"
	writeCodexRollout(t, codexRolloutPath(customHome, time.Now(), id), id, "/work/project", 100)
	t.Setenv("CODEX_HOME", customHome)

	s := detectCodex("/work/project", "")
	wantCodexResume(t, s, "CODEX_HOME="+customHome+" codex resume "+id)
}

// CODEX_HOME pointing at the default directory adds no prefix.
func TestDetectCodex_DefaultCodexHomeHasNoPrefix(t *testing.T) {
	codexHome := isolateCodex(t)
	id := "01905555-0000-7000-8000-000000000001"
	writeCodexRollout(t, codexRolloutPath(codexHome, time.Now(), id), id, "/work/project", 100)
	stubProcessEnv(t, map[string]map[string]string{"7": {"CODEX_HOME": codexHome + "/"}})

	s := detectCodex("/work/project", "7")
	wantCodexResume(t, s, "codex resume "+id)
}

// A thread id with shell characters is never put in a command.
func TestDetectCodex_StateRejectsUnsafeID(t *testing.T) {
	codexHome := isolateCodex(t)
	writeCodexState(t, codexHome, []codexThread{
		{id: "x; rm -rf ~", cwd: "/work/project", updated: time.Now()},
	})

	if s := detectCodex("/work/project", ""); s != nil {
		t.Fatalf("got %+v, want nil for an unsafe id", s)
	}
}

// A directory with a single quote is matched, not broken out of.
func TestDetectCodex_StateQuotesCWD(t *testing.T) {
	codexHome := isolateCodex(t)
	writeCodexState(t, codexHome, []codexThread{
		{id: "01906666-0000-7000-8000-000000000001", cwd: "/work/o'brien", updated: time.Now()},
	})

	s := detectCodex("/work/o'brien", "")
	wantCodexResume(t, s, "codex resume 01906666-0000-7000-8000-000000000001")
}

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
