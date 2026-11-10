package detect

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// openCodeSchema is the part of OpenCode's session table the detector reads,
// with the column names of the real schema (OpenCode 1.14.48).
const openCodeSchema = `CREATE TABLE session (
	id text PRIMARY KEY,
	parent_id text,
	directory text NOT NULL,
	time_updated integer NOT NULL,
	time_archived integer
);`

// openCodeOldSchema is a session table from before OpenCode tracked subagent
// children and archiving: neither parent_id nor time_archived exists.
const openCodeOldSchema = `CREATE TABLE session (
	id text PRIMARY KEY,
	directory text NOT NULL,
	time_updated integer NOT NULL
);`

// writeOpenCodeDB creates an OpenCode database at dbPath with the sqlite3
// CLI, the same tool the detector reads it with, and runs the statements
// that fill it. The test is skipped when sqlite3 is not installed.
func writeOpenCodeDB(t *testing.T, dbPath, schema, rows string) {
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

// isolateOpenCode points HOME at a temp dir and clears the variables that
// move OpenCode's database, so neither the real database nor the
// environment of the shell running the tests can leak in. It returns the
// default database path inside the temp HOME.
func isolateOpenCode(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("OPENCODE_DB", "")
	stubProcessEnv(t, nil)
	return filepath.Join(home, ".local", "share", "opencode", "opencode.db")
}

// wantOpenCodeSession fails the test unless s resumes sessionID.
func wantOpenCodeSession(t *testing.T, s *Session, sessionID string) {
	t.Helper()
	if s == nil {
		t.Fatalf("expected session %s, got nil", sessionID)
	}
	want := "opencode --session " + sessionID
	if s.Command != want {
		t.Errorf("Command = %q, want %q", s.Command, want)
	}
}

func TestDetectOpenCode_NoDB(t *testing.T) {
	isolateOpenCode(t)
	s := detectOpenCode("/nonexistent/path", "")
	if s != nil {
		t.Error("expected nil when DB doesn't exist")
	}
}

// TestDetectOpenCode_SkipsChildSessions covers a subagent run: the child
// session is the newest row for the directory, but resuming it would drop
// the user into the subagent instead of their conversation.
func TestDetectOpenCode_SkipsChildSessions(t *testing.T) {
	dbPath := isolateOpenCode(t)
	writeOpenCodeDB(t, dbPath, openCodeSchema, `
		INSERT INTO session (id, parent_id, directory, time_updated) VALUES
			('ses_parent', NULL, '/src/app', 100),
			('ses_child', 'ses_parent', '/src/app', 200);`)

	wantOpenCodeSession(t, detectOpenCode("/src/app", ""), "ses_parent")
}

// TestDetectOpenCode_SkipsArchivedSessions checks that a session the user
// archived is never brought back, even when it was the last one touched.
func TestDetectOpenCode_SkipsArchivedSessions(t *testing.T) {
	dbPath := isolateOpenCode(t)
	writeOpenCodeDB(t, dbPath, openCodeSchema, `
		INSERT INTO session (id, directory, time_updated, time_archived) VALUES
			('ses_active', '/src/app', 100, NULL),
			('ses_archived', '/src/app', 200, 250);`)

	wantOpenCodeSession(t, detectOpenCode("/src/app", ""), "ses_active")
}

// TestDetectOpenCode_OnlyChildOrArchived checks that a directory whose only
// sessions are filtered out yields nothing rather than a filtered session.
func TestDetectOpenCode_OnlyChildOrArchived(t *testing.T) {
	dbPath := isolateOpenCode(t)
	writeOpenCodeDB(t, dbPath, openCodeSchema, `
		INSERT INTO session (id, parent_id, directory, time_updated, time_archived) VALUES
			('ses_child', 'ses_gone', '/src/app', 200, NULL),
			('ses_archived', NULL, '/src/app', 100, 150);`)

	if s := detectOpenCode("/src/app", ""); s != nil {
		t.Errorf("expected nil, got %q", s.Command)
	}
}

// TestDetectOpenCode_OldSchema checks a database from an OpenCode release
// that has neither parent_id nor time_archived: filtering on a missing
// column would fail the whole query, so the detector must leave it out.
func TestDetectOpenCode_OldSchema(t *testing.T) {
	dbPath := isolateOpenCode(t)
	writeOpenCodeDB(t, dbPath, openCodeOldSchema, `
		INSERT INTO session (id, directory, time_updated) VALUES
			('ses_old', '/src/app', 100),
			('ses_new', '/src/app', 200);`)

	wantOpenCodeSession(t, detectOpenCode("/src/app", ""), "ses_new")
}

// TestDetectOpenCode_SchemaWithoutArchive checks the middle case: subagent
// children are known but archiving is not, so only parent_id filters.
func TestDetectOpenCode_SchemaWithoutArchive(t *testing.T) {
	dbPath := isolateOpenCode(t)
	writeOpenCodeDB(t, dbPath, `CREATE TABLE session (
		id text PRIMARY KEY,
		parent_id text,
		directory text NOT NULL,
		time_updated integer NOT NULL
	);`, `
		INSERT INTO session (id, parent_id, directory, time_updated) VALUES
			('ses_parent', NULL, '/src/app', 100),
			('ses_child', 'ses_parent', '/src/app', 200);`)

	wantOpenCodeSession(t, detectOpenCode("/src/app", ""), "ses_parent")
}

// TestDetectOpenCode_ProcessOpenCodeDB checks that OPENCODE_DB in the
// OpenCode process environment wins over the default database.
func TestDetectOpenCode_ProcessOpenCodeDB(t *testing.T) {
	defaultDB := isolateOpenCode(t)
	writeOpenCodeDB(t, defaultDB, openCodeSchema, `
		INSERT INTO session (id, directory, time_updated) VALUES ('ses_default', '/src/app', 100);`)
	customDB := filepath.Join(t.TempDir(), "work", "custom.db")
	writeOpenCodeDB(t, customDB, openCodeSchema, `
		INSERT INTO session (id, directory, time_updated) VALUES ('ses_custom', '/src/app', 100);`)
	stubProcessEnv(t, map[string]map[string]string{"42": {"OPENCODE_DB": customDB}})

	wantOpenCodeSession(t, detectOpenCode("/src/app", "42"), "ses_custom")
}

// TestDetectOpenCode_OwnOpenCodeDB checks the fallback when the process
// environment can't be read: crex's own OPENCODE_DB is used instead.
func TestDetectOpenCode_OwnOpenCodeDB(t *testing.T) {
	isolateOpenCode(t)
	customDB := filepath.Join(t.TempDir(), "custom.db")
	writeOpenCodeDB(t, customDB, openCodeSchema, `
		INSERT INTO session (id, directory, time_updated) VALUES ('ses_custom', '/src/app', 100);`)
	t.Setenv("OPENCODE_DB", customDB)

	wantOpenCodeSession(t, detectOpenCode("/src/app", ""), "ses_custom")
}

// TestDetectOpenCode_RelativeOpenCodeDB checks that a relative OPENCODE_DB
// resolves under the data directory, as OpenCode resolves it.
func TestDetectOpenCode_RelativeOpenCodeDB(t *testing.T) {
	defaultDB := isolateOpenCode(t)
	channelDB := filepath.Join(filepath.Dir(defaultDB), "opencode-beta.db")
	writeOpenCodeDB(t, channelDB, openCodeSchema, `
		INSERT INTO session (id, directory, time_updated) VALUES ('ses_beta', '/src/app', 100);`)
	stubProcessEnv(t, map[string]map[string]string{"42": {"OPENCODE_DB": "opencode-beta.db"}})

	wantOpenCodeSession(t, detectOpenCode("/src/app", "42"), "ses_beta")
}

// TestDetectOpenCode_MemoryDB checks that an in-memory database, which
// leaves nothing on disk to resume, yields no session and no default
// database lookup.
func TestDetectOpenCode_MemoryDB(t *testing.T) {
	defaultDB := isolateOpenCode(t)
	writeOpenCodeDB(t, defaultDB, openCodeSchema, `
		INSERT INTO session (id, directory, time_updated) VALUES ('ses_default', '/src/app', 100);`)
	stubProcessEnv(t, map[string]map[string]string{"42": {"OPENCODE_DB": ":memory:"}})

	if s := detectOpenCode("/src/app", "42"); s != nil {
		t.Errorf("expected nil for an in-memory database, got %q", s.Command)
	}
}

// TestDetectOpenCode_XDGDataHome checks that XDG_DATA_HOME moves the data
// directory, and with it the default database.
func TestDetectOpenCode_XDGDataHome(t *testing.T) {
	isolateOpenCode(t)
	dataHome := t.TempDir()
	writeOpenCodeDB(t, filepath.Join(dataHome, "opencode", "opencode.db"), openCodeSchema, `
		INSERT INTO session (id, directory, time_updated) VALUES ('ses_xdg', '/src/app', 100);`)
	stubProcessEnv(t, map[string]map[string]string{"42": {"XDG_DATA_HOME": dataHome}})

	wantOpenCodeSession(t, detectOpenCode("/src/app", "42"), "ses_xdg")
}

// TestDetectOpenCode_QuotedDirectory checks that a directory holding a
// single quote is matched literally instead of breaking the query.
func TestDetectOpenCode_QuotedDirectory(t *testing.T) {
	dbPath := isolateOpenCode(t)
	writeOpenCodeDB(t, dbPath, openCodeSchema, `
		INSERT INTO session (id, directory, time_updated) VALUES
			('ses_quote', '/src/it''s', 100),
			('ses_other', '/src/other', 200);`)

	wantOpenCodeSession(t, detectOpenCode("/src/it's", ""), "ses_quote")
}
