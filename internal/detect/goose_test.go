package detect

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// gooseSchema is the part of Goose's sessions table the detector reads. The
// columns and their types come from create_schema in
// crates/goose/src/session/session_manager.rs of aaif-goose/goose (commit
// 72b8db8, schema version 18). Goose opens the file in WAL mode
// (SessionStorage::create_pool), so the fixture does too: a read-only open
// has to cope with it.
const gooseSchema = `PRAGMA journal_mode=WAL;
CREATE TABLE sessions (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	session_type TEXT NOT NULL DEFAULT 'user',
	working_dir TEXT NOT NULL,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	archived_at TIMESTAMP
);`

// gooseSchemaWithoutArchive is a sessions table from schema versions 5 to
// 11: migration 5 added session_type, and archived_at only arrived with
// migration 12.
const gooseSchemaWithoutArchive = `PRAGMA journal_mode=WAL;
CREATE TABLE sessions (
	id TEXT PRIMARY KEY,
	session_type TEXT NOT NULL DEFAULT 'user',
	working_dir TEXT NOT NULL,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);`

// gooseOldSchema is a sessions table from before migration 5, when Goose
// kept no session_type and every row was a chat.
const gooseOldSchema = `PRAGMA journal_mode=WAL;
CREATE TABLE sessions (
	id TEXT PRIMARY KEY,
	working_dir TEXT NOT NULL,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);`

// The ids follow create_session: the UTC date as YYYYMMDD, an underscore and
// a counter for that day. updated_at is written by datetime('now'), so it is
// text in the form 'YYYY-MM-DD HH:MM:SS'.

// writeGooseDB creates a Goose database at dbPath with the sqlite3 CLI, the
// same tool the detector reads it with, and runs the statements that fill
// it. The test is skipped when sqlite3 is not installed.
func writeGooseDB(t *testing.T, dbPath, schema, rows string) {
	t.Helper()
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 CLI not installed")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command("sqlite3", dbPath, schema+rows).CombinedOutput()
	if err != nil {
		t.Fatalf("sqlite3: %v\n%s", err, out)
	}
}

// isolateGoose points HOME at a temp dir and clears the variables that move
// Goose's data directory, so neither the real database nor the environment
// of the shell running the tests can leak in. It returns the default
// database path inside the temp HOME, which is the same on macOS and Linux.
func isolateGoose(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("GOOSE_PATH_ROOT", "")
	stubProcessEnv(t, nil)
	return filepath.Join(home, ".local", "share", "goose", "sessions", "sessions.db")
}

// wantGooseSession fails the test unless s resumes sessionID in /src/app
// with the given command.
func wantGooseSession(t *testing.T, s *Session, command string) {
	t.Helper()
	if s == nil {
		t.Fatalf("expected %q, got nil", command)
	}
	if s.Tool != "goose" || s.CWD != "/src/app" {
		t.Errorf("Tool, CWD = %q, %q; want goose, /src/app", s.Tool, s.CWD)
	}
	if s.Command != command {
		t.Errorf("Command = %q, want %q", s.Command, command)
	}
}

func TestDetectGoose_Found(t *testing.T) {
	dbPath := isolateGoose(t)
	writeGooseDB(t, dbPath, gooseSchema, `
INSERT INTO sessions (id, session_type, working_dir, updated_at) VALUES
	('20261109_1', 'user', '/src/app', '2026-11-09 14:30:22');`)

	wantGooseSession(t, detectGoose("/src/app", ""), "goose session --resume --session-id 20261109_1")
}

func TestDetectGoose_NewestOfTwo(t *testing.T) {
	dbPath := isolateGoose(t)
	writeGooseDB(t, dbPath, gooseSchema, `
INSERT INTO sessions (id, session_type, working_dir, updated_at) VALUES
	('20261108_3', 'user', '/src/app', '2026-11-10 09:15:00'),
	('20261109_1', 'user', '/src/app', '2026-11-09 14:30:22');`)

	wantGooseSession(t, detectGoose("/src/app", ""), "goose session --resume --session-id 20261108_3")
}

// TestDetectGoose_MixedTimestampFormats covers a session brought in with
// `goose session import`: import_session binds the chrono value, which sqlx
// writes as RFC 3339 text ('2026-11-10T08:00:00+00:00'), while every later
// change writes datetime('now') ('2026-11-10 09:15:00'). Compared as text
// the T sorts after the space and the older import would win.
func TestDetectGoose_MixedTimestampFormats(t *testing.T) {
	dbPath := isolateGoose(t)
	writeGooseDB(t, dbPath, gooseSchema, `
INSERT INTO sessions (id, session_type, working_dir, updated_at) VALUES
	('20261110_1', 'user', '/src/app', '2026-11-10T08:00:00.123456+00:00'),
	('20261110_2', 'user', '/src/app', '2026-11-10 09:15:00');`)

	wantGooseSession(t, detectGoose("/src/app", ""), "goose session --resume --session-id 20261110_2")
}

// TestDetectGoose_SkipsArchivedAndNonUser checks that only a chat the user
// started is resumed. Subagent, scheduled and hidden (`goose run
// --no-session`) rows share the table and can be newer, and an archived
// session was put away on purpose.
func TestDetectGoose_SkipsArchivedAndNonUser(t *testing.T) {
	dbPath := isolateGoose(t)
	writeGooseDB(t, dbPath, gooseSchema, `
INSERT INTO sessions (id, session_type, working_dir, updated_at, archived_at) VALUES
	('20261109_1', 'user', '/src/app', '2026-11-09 10:00:00', NULL),
	('20261109_2', 'sub_agent', '/src/app', '2026-11-09 11:00:00', NULL),
	('20261109_3', 'scheduled', '/src/app', '2026-11-09 12:00:00', NULL),
	('20261109_4', 'hidden', '/src/app', '2026-11-09 13:00:00', NULL),
	('20261109_5', 'user', '/src/app', '2026-11-09 14:00:00', '2026-11-09 14:05:00');`)

	wantGooseSession(t, detectGoose("/src/app", ""), "goose session --resume --session-id 20261109_1")
}

// TestDetectGoose_OnlyFilteredRowsIsNil checks that a directory whose only
// sessions are filtered out yields nothing, not a bare `goose`.
func TestDetectGoose_OnlyFilteredRowsIsNil(t *testing.T) {
	dbPath := isolateGoose(t)
	writeGooseDB(t, dbPath, gooseSchema, `
INSERT INTO sessions (id, session_type, working_dir, updated_at, archived_at) VALUES
	('20261109_2', 'sub_agent', '/src/app', '2026-11-09 11:00:00', NULL),
	('20261109_5', 'user', '/src/app', '2026-11-09 14:00:00', '2026-11-09 14:05:00');`)

	if s := detectGoose("/src/app", ""); s != nil {
		t.Errorf("expected nil, got %q", s.Command)
	}
}

func TestDetectGoose_OtherDirectoryIsNil(t *testing.T) {
	dbPath := isolateGoose(t)
	writeGooseDB(t, dbPath, gooseSchema, `
INSERT INTO sessions (id, session_type, working_dir, updated_at) VALUES
	('20261109_1', 'user', '/src/other', '2026-11-09 14:30:22');`)

	if s := detectGoose("/src/app", ""); s != nil {
		t.Errorf("expected nil, got %q", s.Command)
	}
}

// TestDetectGoose_OldSchema checks a database from before migration 5:
// filtering on a missing column would fail the whole query, so the detector
// must leave both conditions out.
func TestDetectGoose_OldSchema(t *testing.T) {
	dbPath := isolateGoose(t)
	writeGooseDB(t, dbPath, gooseOldSchema, `
INSERT INTO sessions (id, working_dir, updated_at) VALUES
	('20250921_1', '/src/app', '2025-09-21 10:00:00'),
	('20250921_2', '/src/app', '2025-09-21 11:00:00');`)

	wantGooseSession(t, detectGoose("/src/app", ""), "goose session --resume --session-id 20250921_2")
}

// TestDetectGoose_SchemaWithoutArchive checks the middle case: session_type
// is known but archiving is not, so only the type filters.
func TestDetectGoose_SchemaWithoutArchive(t *testing.T) {
	dbPath := isolateGoose(t)
	writeGooseDB(t, dbPath, gooseSchemaWithoutArchive, `
INSERT INTO sessions (id, session_type, working_dir, updated_at) VALUES
	('20251015_1', 'user', '/src/app', '2025-10-15 10:00:00'),
	('20251015_2', 'sub_agent', '/src/app', '2025-10-15 11:00:00');`)

	wantGooseSession(t, detectGoose("/src/app", ""), "goose session --resume --session-id 20251015_1")
}

// TestDetectGoose_NoSessionsTableIsNil checks a database file with no
// sessions table: there are no columns to read, so there is no session.
func TestDetectGoose_NoSessionsTableIsNil(t *testing.T) {
	dbPath := isolateGoose(t)
	writeGooseDB(t, dbPath, `CREATE TABLE schema_version (version INTEGER PRIMARY KEY);`, "")

	if s := detectGoose("/src/app", ""); s != nil {
		t.Errorf("expected nil, got %q", s.Command)
	}
}

// TestDetectGoose_QuoteInCWD checks that a directory holding a single quote
// is matched literally instead of breaking the query.
func TestDetectGoose_QuoteInCWD(t *testing.T) {
	dbPath := isolateGoose(t)
	writeGooseDB(t, dbPath, gooseSchema, `
INSERT INTO sessions (id, session_type, working_dir, updated_at) VALUES
	('20261109_1', 'user', '/src/o''brien', '2026-11-09 10:00:00'),
	('20261109_2', 'user', '/src/other', '2026-11-09 11:00:00');`)

	s := detectGoose("/src/o'brien", "")
	if s == nil || s.Command != "goose session --resume --session-id 20261109_1" {
		t.Errorf("got %+v, want the session of /src/o'brien", s)
	}
}

// TestDetectGoose_NoDBIsNil checks the name clash with pressly/goose, the
// migration tool: without a Goose database the process is not taken for an
// AI session, so the detector returns nothing rather than a bare `goose`.
func TestDetectGoose_NoDBIsNil(t *testing.T) {
	isolateGoose(t)

	if s := detectGoose("/src/app", ""); s != nil {
		t.Errorf("expected nil when the database doesn't exist, got %q", s.Command)
	}
}

// TestDetectGoose_XDGDataHome checks that XDG_DATA_HOME in the Goose
// process environment moves the data directory (etcetera's Xdg strategy,
// which Goose uses on macOS too). The restored shell is expected to share
// that variable, so the command carries no prefix.
func TestDetectGoose_XDGDataHome(t *testing.T) {
	isolateGoose(t)
	dataHome := t.TempDir()
	writeGooseDB(t, filepath.Join(dataHome, "goose", "sessions", "sessions.db"), gooseSchema, `
INSERT INTO sessions (id, session_type, working_dir, updated_at) VALUES
	('20261109_1', 'user', '/src/app', '2026-11-09 14:30:22');`)
	stubProcessEnv(t, map[string]map[string]string{"42": {"XDG_DATA_HOME": dataHome}})

	wantGooseSession(t, detectGoose("/src/app", "42"), "goose session --resume --session-id 20261109_1")
}

// TestDetectGoose_PathRootFromProcess checks GOOSE_PATH_ROOT, which moves
// all of Goose's directories under one root (crates/goose/src/config/
// paths.rs) and wins over XDG_DATA_HOME. The restored shell must set it
// again, or Goose would look for the id in the default database and exit.
func TestDetectGoose_PathRootFromProcess(t *testing.T) {
	defaultDB := isolateGoose(t)
	writeGooseDB(t, defaultDB, gooseSchema, `
INSERT INTO sessions (id, session_type, working_dir, updated_at) VALUES
	('20261109_1', 'user', '/src/app', '2026-11-09 14:30:22');`)
	root := filepath.Join(t.TempDir(), "goose root")
	writeGooseDB(t, filepath.Join(root, "data", "sessions", "sessions.db"), gooseSchema, `
INSERT INTO sessions (id, session_type, working_dir, updated_at) VALUES
	('20261109_7', 'user', '/src/app', '2026-11-09 10:00:00');`)
	stubProcessEnv(t, map[string]map[string]string{"42": {
		"GOOSE_PATH_ROOT": root,
		"XDG_DATA_HOME":   t.TempDir(),
	}})

	want := "GOOSE_PATH_ROOT=" + shellQuote(root) + " goose session --resume --session-id 20261109_7"
	wantGooseSession(t, detectGoose("/src/app", "42"), want)
}

// TestDetectGoose_RelativePathRootIgnored checks that a relative
// GOOSE_PATH_ROOT is ignored, as Paths::validated_path_root ignores it.
func TestDetectGoose_RelativePathRootIgnored(t *testing.T) {
	defaultDB := isolateGoose(t)
	writeGooseDB(t, defaultDB, gooseSchema, `
INSERT INTO sessions (id, session_type, working_dir, updated_at) VALUES
	('20261109_1', 'user', '/src/app', '2026-11-09 14:30:22');`)
	stubProcessEnv(t, map[string]map[string]string{"42": {"GOOSE_PATH_ROOT": "relative/root"}})

	wantGooseSession(t, detectGoose("/src/app", "42"), "goose session --resume --session-id 20261109_1")
}

// TestDetectGoose_OwnPathRoot checks the fallback when the process
// environment can't be read: crex's own GOOSE_PATH_ROOT is used instead.
func TestDetectGoose_OwnPathRoot(t *testing.T) {
	isolateGoose(t)
	root := t.TempDir()
	writeGooseDB(t, filepath.Join(root, "data", "sessions", "sessions.db"), gooseSchema, `
INSERT INTO sessions (id, session_type, working_dir, updated_at) VALUES
	('20261109_7', 'user', '/src/app', '2026-11-09 10:00:00');`)
	t.Setenv("GOOSE_PATH_ROOT", root)

	want := "GOOSE_PATH_ROOT=" + shellQuote(root) + " goose session --resume --session-id 20261109_7"
	wantGooseSession(t, detectGoose("/src/app", ""), want)
}
