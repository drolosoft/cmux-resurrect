package detect

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// clineSchema is the part of Cline's sessions table the detector reads. The
// columns and their types come from SCHEMA_STATEMENTS in
// sdk/packages/shared/src/db/sqlite-db.ts of cline/cline (commit e1f499a,
// CLI 3.0.74); the real table has twenty more columns. Cline opens the file
// in WAL mode, so the fixture does too: a read-only open has to cope with it.
const clineSchema = `PRAGMA journal_mode=WAL;
CREATE TABLE sessions (
	session_id TEXT PRIMARY KEY,
	source TEXT NOT NULL,
	started_at TEXT NOT NULL,
	cwd TEXT NOT NULL,
	workspace_root TEXT NOT NULL,
	parent_session_id TEXT,
	is_subagent INTEGER NOT NULL DEFAULT 0,
	updated_at TEXT NOT NULL
);`

// clineLegacySchema is a sessions table from before Cline recorded child
// sessions. parent_session_id and is_subagent are two of the LEGACY_MIGRATIONS
// in the same file, so older databases lack them until a newer Cline opens
// them; updated_at is left out too, to cover the fallback to started_at.
const clineLegacySchema = `CREATE TABLE sessions (
	session_id TEXT PRIMARY KEY,
	source TEXT NOT NULL,
	started_at TEXT NOT NULL,
	cwd TEXT NOT NULL
);`

// writeClineDB creates <dbDir>/sessions.db with the given schema and runs the
// INSERT statements that fill it through the sqlite3 CLI, the same tool the
// detector reads it with. The test is skipped when sqlite3 is not installed.
func writeClineDB(t *testing.T, dbDir, schema, inserts string) {
	t.Helper()
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 CLI not installed")
	}
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(dbDir, "sessions.db")
	out, err := exec.Command("sqlite3", dbPath, schema+inserts).CombinedOutput()
	if err != nil {
		t.Fatalf("sqlite3: %v\n%s", err, out)
	}
}

// clineTestHome points HOME at a fresh directory and gives the process none
// of the variables that move Cline's data, so the detector looks in
// <home>/.cline/data/db. It returns that directory.
func clineTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	return filepath.Join(home, ".cline", "data", "db")
}

// wantClineCommand fails the test unless s is a cline session for cwd that
// runs command.
func wantClineCommand(t *testing.T, s *Session, cwd, command string) {
	t.Helper()
	if s == nil {
		t.Fatalf("expected a session running %q, got nil", command)
	}
	if s.Tool != "cline" || s.CWD != cwd {
		t.Errorf("Tool, CWD = %q, %q; want cline, %s", s.Tool, s.CWD, cwd)
	}
	if s.Command != command {
		t.Errorf("Command = %q, want %q", s.Command, command)
	}
}

// The ids follow createSessionId in sdk/packages/shared/src/session/index.ts:
// the Unix time in milliseconds, an underscore and five characters from
// [0-9a-z]. Timestamps are ISO 8601 strings from nowIso() in sqlite-db.ts.

func TestDetectCline_Found(t *testing.T) {
	dbDir := clineTestHome(t)
	writeClineDB(t, dbDir, clineSchema, `
INSERT INTO sessions (session_id, source, started_at, cwd, workspace_root, parent_session_id, is_subagent, updated_at) VALUES
	('1794234622123_a1b2c', 'cli', '2026-11-09T14:30:22.123Z', '/src/app', '/src/app', NULL, 0, '2026-11-09T14:41:05.000Z');`)

	s := detectCline("/src/app", "")
	wantClineCommand(t, s, "/src/app", "cline --id 1794234622123_a1b2c")
}

func TestDetectCline_NewestOfTwo(t *testing.T) {
	dbDir := clineTestHome(t)
	// The older session was resumed after the newer one was started, so it
	// is the last one used. updated_at says so; started_at would not.
	writeClineDB(t, dbDir, clineSchema, `
INSERT INTO sessions (session_id, source, started_at, cwd, workspace_root, parent_session_id, is_subagent, updated_at) VALUES
	('1794129323000_old01', 'cli', '2026-11-08T09:15:23.000Z', '/src/app', '/src/app', NULL, 0, '2026-11-09T18:02:00.000Z'),
	('1794234622123_new02', 'cli', '2026-11-09T14:30:22.123Z', '/src/app', '/src/app', NULL, 0, '2026-11-09T15:00:00.000Z');`)

	s := detectCline("/src/app", "")
	wantClineCommand(t, s, "/src/app", "cline --id 1794129323000_old01")
}

func TestDetectCline_SkipsChildSessions(t *testing.T) {
	dbDir := clineTestHome(t)
	// Subagent and team-task runs are recorded under their parent, in the
	// same directory and later than it. Cline's own history hides them with
	// is_subagent = 0 AND an empty parent_session_id (rootOnly in
	// sdk/packages/core/src/session/services/session-service.ts).
	writeClineDB(t, dbDir, clineSchema, `
INSERT INTO sessions (session_id, source, started_at, cwd, workspace_root, parent_session_id, is_subagent, updated_at) VALUES
	('1794234622123_root1', 'cli', '2026-11-09T14:30:22.123Z', '/src/app', '/src/app', '', 0, '2026-11-09T14:31:00.000Z'),
	('1794234680000_sub01', 'subagent', '2026-11-09T14:31:20.000Z', '/src/app', '/src/app', '1794234622123_root1', 1, '2026-11-09T14:35:00.000Z'),
	('1794234690000_team1', 'cli', '2026-11-09T14:31:30.000Z', '/src/app', '/src/app', '1794234622123_root1', 0, '2026-11-09T14:36:00.000Z');`)

	s := detectCline("/src/app", "")
	wantClineCommand(t, s, "/src/app", "cline --id 1794234622123_root1")
}

func TestDetectCline_IgnoresOtherClients(t *testing.T) {
	dbDir := clineTestHome(t)
	// The VS Code extension can share the database and the folder, but its
	// conversation is not the one this terminal was running.
	writeClineDB(t, dbDir, clineSchema, `
INSERT INTO sessions (session_id, source, started_at, cwd, workspace_root, parent_session_id, is_subagent, updated_at) VALUES
	('1794129323000_cli01', 'cli', '2026-11-08T09:15:23.000Z', '/src/app', '/src/app', NULL, 0, '2026-11-08T10:00:00.000Z'),
	('1794234622123_vsc01', 'vscode', '2026-11-09T14:30:22.123Z', '/src/app', '/src/app', NULL, 0, '2026-11-09T15:00:00.000Z');`)

	s := detectCline("/src/app", "")
	wantClineCommand(t, s, "/src/app", "cline --id 1794129323000_cli01")
}

func TestDetectCline_OtherDirectoryIsBare(t *testing.T) {
	dbDir := clineTestHome(t)
	writeClineDB(t, dbDir, clineSchema, `
INSERT INTO sessions (session_id, source, started_at, cwd, workspace_root, parent_session_id, is_subagent, updated_at) VALUES
	('1794234622123_a1b2c', 'cli', '2026-11-09T14:30:22.123Z', '/src/other', '/src/other', NULL, 0, '2026-11-09T14:41:05.000Z');`)

	s := detectCline("/src/app", "")
	wantClineCommand(t, s, "/src/app", "cline")
}

func TestDetectCline_NoDBIsBare(t *testing.T) {
	clineTestHome(t)

	s := detectCline("/src/app", "")
	wantClineCommand(t, s, "/src/app", "cline")
}

func TestDetectCline_QuoteInCWD(t *testing.T) {
	dbDir := clineTestHome(t)
	cwd := "/src/o'brien"
	// Left unescaped, the quote would end the string literal: the query
	// would fail and the detector would hand back the bare command.
	writeClineDB(t, dbDir, clineSchema, `
INSERT INTO sessions (session_id, source, started_at, cwd, workspace_root, parent_session_id, is_subagent, updated_at) VALUES
	('1794234622123_a1b2c', 'cli', '2026-11-09T14:30:22.123Z', '/src/o''brien', '/src/o''brien', NULL, 0, '2026-11-09T14:41:05.000Z');`)

	s := detectCline(cwd, "")
	wantClineCommand(t, s, cwd, "cline --id 1794234622123_a1b2c")
}

func TestDetectCline_LegacySchema(t *testing.T) {
	dbDir := clineTestHome(t)
	// Filtering or ordering on a missing column would fail the whole query,
	// so the detector must leave those columns out and order by started_at.
	writeClineDB(t, dbDir, clineLegacySchema, `
INSERT INTO sessions (session_id, source, started_at, cwd) VALUES
	('1794129323000_old01', 'cli', '2026-11-08T09:15:23.000Z', '/src/app'),
	('1794234622123_new02', 'cli', '2026-11-09T14:30:22.123Z', '/src/app');`)

	s := detectCline("/src/app", "")
	wantClineCommand(t, s, "/src/app", "cline --id 1794234622123_new02")
}

func TestDetectCline_DataDirFromProcess(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// CLINE_DATA_DIR replaces ~/.cline/data, and the index lives in its db
	// directory (resolveClineDataDir and resolveDbDataDir in
	// sdk/packages/shared/src/storage/paths.ts).
	dataDir := filepath.Join(home, "work cline")
	writeClineDB(t, filepath.Join(dataDir, "db"), clineSchema, `
INSERT INTO sessions (session_id, source, started_at, cwd, workspace_root, parent_session_id, is_subagent, updated_at) VALUES
	('1794234622123_a1b2c', 'cli', '2026-11-09T14:30:22.123Z', '/src/app', '/src/app', NULL, 0, '2026-11-09T14:41:05.000Z');`)
	stubProcessEnv(t, map[string]map[string]string{"42": {"CLINE_DATA_DIR": dataDir}})

	s := detectCline("/src/app", "42")
	want := "CLINE_DATA_DIR='" + dataDir + "' cline --id 1794234622123_a1b2c"
	wantClineCommand(t, s, "/src/app", want)
}

func TestDetectCline_ClineDirFromProcess(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// CLINE_DIR replaces ~/.cline as a whole, settings included, so the
	// resume command sets it again rather than only the database path.
	clineDir := filepath.Join(home, "alt")
	writeClineDB(t, filepath.Join(clineDir, "data", "db"), clineSchema, `
INSERT INTO sessions (session_id, source, started_at, cwd, workspace_root, parent_session_id, is_subagent, updated_at) VALUES
	('1794234622123_a1b2c', 'cli', '2026-11-09T14:30:22.123Z', '/src/app', '/src/app', NULL, 0, '2026-11-09T14:41:05.000Z');`)
	stubProcessEnv(t, map[string]map[string]string{"42": {"CLINE_DIR": clineDir}})

	s := detectCline("/src/app", "42")
	want := "CLINE_DIR=" + clineDir + " cline --id 1794234622123_a1b2c"
	wantClineCommand(t, s, "/src/app", want)
}

func TestDetectCline_DefaultDirHasNoPrefix(t *testing.T) {
	dbDir := clineTestHome(t)
	writeClineDB(t, dbDir, clineSchema, `
INSERT INTO sessions (session_id, source, started_at, cwd, workspace_root, parent_session_id, is_subagent, updated_at) VALUES
	('1794234622123_a1b2c', 'cli', '2026-11-09T14:30:22.123Z', '/src/app', '/src/app', NULL, 0, '2026-11-09T14:41:05.000Z');`)
	// Pointing CLINE_DIR at the default directory changes nothing.
	home := os.Getenv("HOME")
	stubProcessEnv(t, map[string]map[string]string{"7": {"CLINE_DIR": filepath.Join(home, ".cline") + "/"}})

	s := detectCline("/src/app", "7")
	wantClineCommand(t, s, "/src/app", "cline --id 1794234622123_a1b2c")
}
