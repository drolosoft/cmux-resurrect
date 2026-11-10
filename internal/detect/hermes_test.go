package detect

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// hermesSchema is the part of Hermes Agent's sessions table the detector
// reads. The columns and their types come from SCHEMA_SQL in
// hermes_state_common.py of NousResearch/hermes-agent (commit e782db3,
// schema version 31); website/docs/developer-guide/session-storage.md
// documents the same table, abridged. Hermes runs the file in WAL mode, so
// the fixture does too: a read-only open has to cope with it.
const hermesSchema = `PRAGMA journal_mode=WAL;
CREATE TABLE sessions (
	id TEXT PRIMARY KEY,
	source TEXT NOT NULL,
	started_at REAL NOT NULL,
	ended_at REAL,
	cwd TEXT,
	title TEXT
);`

// writeHermesDB creates <hermesHome>/state.db with the sessions table and
// runs the given INSERT statements through the sqlite3 CLI, the same tool
// the detector reads it with.
func writeHermesDB(t *testing.T, hermesHome, inserts string) {
	t.Helper()
	if err := os.MkdirAll(hermesHome, 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(hermesHome, "state.db")
	out, err := exec.Command("sqlite3", dbPath, hermesSchema+inserts).CombinedOutput()
	if err != nil {
		t.Fatalf("sqlite3: %v\n%s", err, out)
	}
}

// hermesTestHome points HOME at a fresh directory and gives the process no
// HERMES_HOME, so the detector looks in <home>/.hermes.
func hermesTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	return home
}

// The ids follow the format of website/docs/user-guide/sessions.md:
// YYYYMMDD_HHMMSS_<hex>, six hex digits for CLI sessions and eight for
// gateway ones. started_at is a Unix time in seconds.

func TestDetectHermes_LatestCLISession(t *testing.T) {
	home := hermesTestHome(t)
	writeHermesDB(t, filepath.Join(home, ".hermes"), `
INSERT INTO sessions (id, source, started_at, ended_at, cwd, title) VALUES
	('20261108_091523_a1b2c3', 'cli', 1794129323.0, 1794132923.0, '/src/app', 'old work'),
	('20261109_143022_d4e5f6', 'cli', 1794234622.5, NULL, '/src/app', 'new work');`)

	s := detectHermes("/src/app", "")
	if s == nil {
		t.Fatal("expected a session, got nil")
	}
	if s.Tool != "hermes" || s.CWD != "/src/app" {
		t.Errorf("Tool, CWD = %q, %q; want hermes, /src/app", s.Tool, s.CWD)
	}
	if want := "hermes --resume 20261109_143022_d4e5f6"; s.Command != want {
		t.Errorf("Command = %q, want %q", s.Command, want)
	}
}

func TestDetectHermes_IgnoresNewerGatewaySession(t *testing.T) {
	home := hermesTestHome(t)
	// A Telegram chat handled by the gateway can record the same directory,
	// and it is newer, but it is not the conversation of this terminal.
	writeHermesDB(t, filepath.Join(home, ".hermes"), `
INSERT INTO sessions (id, source, started_at, ended_at, cwd, title) VALUES
	('20261108_091523_a1b2c3', 'cli', 1794129323.0, NULL, '/src/app', NULL),
	('20261109_143022_d4e5f6a7', 'telegram', 1794234622.0, NULL, '/src/app', NULL);`)

	s := detectHermes("/src/app", "")
	if s == nil || s.Command != "hermes --resume 20261108_091523_a1b2c3" {
		t.Errorf("got %+v, want the cli session", s)
	}
}

func TestDetectHermes_OtherDirectoryIsBare(t *testing.T) {
	home := hermesTestHome(t)
	writeHermesDB(t, filepath.Join(home, ".hermes"), `
INSERT INTO sessions (id, source, started_at, ended_at, cwd, title) VALUES
	('20261108_091523_a1b2c3', 'cli', 1794129323.0, NULL, '/src/other', NULL);`)

	s := detectHermes("/src/app", "")
	if s == nil || s.Command != "hermes" || s.Tool != "hermes" || s.CWD != "/src/app" {
		t.Errorf("got %+v, want the bare hermes command for /src/app", s)
	}
}

func TestDetectHermes_NoDBIsBare(t *testing.T) {
	hermesTestHome(t)

	s := detectHermes("/src/app", "")
	if s == nil || s.Command != "hermes" || s.Tool != "hermes" || s.CWD != "/src/app" {
		t.Errorf("got %+v, want the bare hermes command for /src/app", s)
	}
}

func TestDetectHermes_QuoteInCWD(t *testing.T) {
	home := hermesTestHome(t)
	cwd := "/src/o'brien"
	// Left unescaped, the quote would end the string literal: the query
	// would fail and the detector would hand back the bare command.
	writeHermesDB(t, filepath.Join(home, ".hermes"), `
INSERT INTO sessions (id, source, started_at, ended_at, cwd, title) VALUES
	('20261108_091523_a1b2c3', 'cli', 1794129323.0, NULL, '/src/o''brien', NULL);`)

	s := detectHermes(cwd, "")
	if s == nil || s.Command != "hermes --resume 20261108_091523_a1b2c3" {
		t.Errorf("got %+v, want the session of %s", s, cwd)
	}
}

func TestDetectHermes_HermesHomeFromProcess(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// A named profile lives in its own directory with its own state.db, and
	// Hermes tells the process which one through HERMES_HOME
	// (session-storage.md, "Hermes home and profile isolation").
	profile := filepath.Join(home, ".hermes", "profiles", "coder")
	writeHermesDB(t, profile, `
INSERT INTO sessions (id, source, started_at, ended_at, cwd, title) VALUES
	('20261109_143022_d4e5f6', 'cli', 1794234622.0, NULL, '/src/app', NULL);`)
	stubProcessEnv(t, map[string]map[string]string{"42": {"HERMES_HOME": profile}})

	s := detectHermes("/src/app", "42")
	want := "HERMES_HOME=" + profile + " hermes --resume 20261109_143022_d4e5f6"
	if s == nil || s.Command != want {
		t.Errorf("got %+v, want Command %q", s, want)
	}
}

func TestDetectHermes_DefaultHermesHomeHasNoPrefix(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeHermesDB(t, filepath.Join(home, ".hermes"), `
INSERT INTO sessions (id, source, started_at, ended_at, cwd, title) VALUES
	('20261109_143022_d4e5f6', 'cli', 1794234622.0, NULL, '/src/app', NULL);`)
	// Pointing HERMES_HOME at the default directory changes nothing.
	stubProcessEnv(t, map[string]map[string]string{"7": {"HERMES_HOME": filepath.Join(home, ".hermes") + "/"}})

	s := detectHermes("/src/app", "7")
	if s == nil || s.Command != "hermes --resume 20261109_143022_d4e5f6" {
		t.Errorf("got %+v, want no HERMES_HOME prefix", s)
	}
}
