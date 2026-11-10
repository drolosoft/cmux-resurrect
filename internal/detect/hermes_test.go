package detect

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// removeHermesSideFiles deletes the -wal and -shm files next to the
// database, leaving it as Hermes does when its last connection closes: the
// sqlite3 CLI on macOS keeps them, Hermes's own SQLite does not.
func removeHermesSideFiles(t *testing.T, hermesHome string) {
	t.Helper()
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(filepath.Join(hermesHome, "state.db"+suffix)); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
}

// hermesTestHome points HOME at a fresh directory and gives the process no
// HERMES_HOME and no terminal, so the detector reads <home>/.hermes/state.db.
func hermesTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	stubProcessTerminal(t, nil)
	return home
}

// hermesProcess is what the stubbed ps reports about one process.
type hermesProcess struct {
	tty     string
	started time.Time
}

// stubProcessTerminal replaces the terminal reader for one test. A pid that
// is not in processes has no terminal.
func stubProcessTerminal(t *testing.T, processes map[string]hermesProcess) {
	t.Helper()
	orig := processTerminal
	processTerminal = func(pid string) (string, time.Time, bool) {
		process, ok := processes[pid]
		return process.tty, process.started, ok
	}
	t.Cleanup(func() { processTerminal = orig })
}

// writeHermesBreadcrumb writes the file Hermes keeps for one terminal in
// <hermesHome>/terminal-sessions, in the shape the lab's Hermes v0.23.1
// wrote it: {"session_id": ..., "cwd": ..., "ts": <Unix seconds>}.
func writeHermesBreadcrumb(t *testing.T, hermesHome, name, sessionID, cwd string, written time.Time) {
	t.Helper()
	dir := filepath.Join(hermesHome, "terminal-sessions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	seconds := float64(written.UnixNano()) / float64(time.Second)
	crumb := fmt.Sprintf(`{"session_id": %q, "cwd": %q, "ts": %f}`, sessionID, cwd, seconds)
	if err := os.WriteFile(filepath.Join(dir, name), []byte(crumb), 0o644); err != nil {
		t.Fatal(err)
	}
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
	stubProcessTerminal(t, nil)
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
	stubProcessTerminal(t, nil)
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

// The tests below use the id shape Hermes v0.23.1 wrote in the lab,
// local-<64 hex digits>, shortened.

// `hermes --tui` records its sessions with source 'tui' (seen in the lab with
// v0.23.1). It is a conversation in the terminal like a 'cli' one.
func TestDetectHermes_TUISession(t *testing.T) {
	home := hermesTestHome(t)
	writeHermesDB(t, filepath.Join(home, ".hermes"), `
INSERT INTO sessions (id, source, started_at, ended_at, cwd, title) VALUES
	('local-6300aa326ade0f44', 'cli', 1794283417.5, NULL, '/src/app', NULL),
	('local-c82be6a3021b7e24', 'tui', 1794283488.7, NULL, '/src/app', NULL);`)

	s := detectHermes("/src/app", "")
	if s == nil || s.Command != "hermes --resume local-c82be6a3021b7e24" {
		t.Errorf("got %+v, want the tui session", s)
	}
}

// When no Hermes has the database open, it is a WAL database with no -wal or
// -shm file, and `sqlite3 -readonly` cannot open it (error 14). Everything is
// in the main file then, which the lab showed matches what Hermes wrote, so
// the immutable read finds the session. The space in the home checks that
// the path survives the file: URI.
func TestDetectHermes_ClosedDatabaseWithoutSideFiles(t *testing.T) {
	home := filepath.Join(t.TempDir(), "my home")
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	stubProcessTerminal(t, nil)
	hermesHome := filepath.Join(home, ".hermes")
	writeHermesDB(t, hermesHome, `
INSERT INTO sessions (id, source, started_at, ended_at, cwd, title) VALUES
	('local-c7ab2bbe7dfaaee6', 'cli', 1794284947.6, NULL, '/src/app', NULL);`)
	removeHermesSideFiles(t, hermesHome)

	dbPath := filepath.Join(hermesHome, "state.db")
	if exec.Command("sqlite3", "-readonly", dbPath, "SELECT 1;").Run() == nil {
		t.Log("this sqlite3 opens a WAL database without its -shm file; only the first read runs")
	}

	s := detectHermes("/src/app", "")
	if s == nil || s.Command != "hermes --resume local-c7ab2bbe7dfaaee6" {
		t.Errorf("got %+v, want the session from the closed database", s)
	}
}

// A -wal file with pages the main file does not have yet means the
// immutable read would miss the newest rows, as it did in the lab while
// Hermes ran. If the read-only open fails then (here the folder cannot take
// the -shm file it needs), the detector must not resume the older session.
func TestDetectHermes_PendingWALIsNotReadImmutably(t *testing.T) {
	home := hermesTestHome(t)
	hermesHome := filepath.Join(home, ".hermes")
	writeHermesDB(t, hermesHome, `
INSERT INTO sessions (id, source, started_at, ended_at, cwd, title) VALUES
	('local-old0000000000000', 'cli', 1794283417.5, NULL, '/src/app', NULL);`)

	// Copy the main file and the WAL while a second session is only in the
	// WAL, then put the copies back once sqlite3 has checkpointed and left.
	script := `PRAGMA wal_autocheckpoint=0;
INSERT INTO sessions (id, source, started_at, ended_at, cwd, title) VALUES
	('local-new0000000000000', 'cli', 1794284947.6, NULL, '/src/app', NULL);
.shell cp state.db db.copy
.shell cp state.db-wal wal.copy
`
	command := exec.Command("sqlite3", "state.db")
	command.Dir = hermesHome
	command.Stdin = strings.NewReader(script)
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("sqlite3: %v\n%s", err, out)
	}
	removeHermesSideFiles(t, hermesHome)
	for saved, original := range map[string]string{"db.copy": "state.db", "wal.copy": "state.db-wal"} {
		if err := os.Rename(filepath.Join(hermesHome, saved), filepath.Join(hermesHome, original)); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.Chmod(hermesHome, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(hermesHome, 0o755) })

	// A user who may write anywhere gets the -shm file and the new session;
	// anyone else gets a fresh Hermes. The old session is wrong either way.
	s := detectHermes("/src/app", "")
	if s == nil || (s.Command != "hermes" && s.Command != "hermes --resume local-new0000000000000") {
		t.Errorf("got %+v, want a fresh hermes or the session only the WAL holds", s)
	}
}

// A database sqlite3 cannot read at all gives the bare command.
func TestDetectHermes_UnreadableDatabaseIsBare(t *testing.T) {
	home := hermesTestHome(t)
	hermesHome := filepath.Join(home, ".hermes")
	if err := os.MkdirAll(hermesHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hermesHome, "state.db"), []byte("not a database"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := detectHermes("/src/app", "")
	if s == nil || s.Command != "hermes" {
		t.Errorf("got %+v, want the bare hermes command", s)
	}
}

// The lab's Hermes on /dev/ttys002 wrote terminal-sessions/tty-dev-ttys002;
// Hermes names the file after the device path, so a Linux pts/3 becomes
// tty-dev-pts-3.
func TestHermesTerminalID(t *testing.T) {
	cases := map[string]string{
		"ttys002": "tty-dev-ttys002",
		"pts/3":   "tty-dev-pts-3",
	}
	for tty, want := range cases {
		if got := hermesTerminalID(tty); got != want {
			t.Errorf("hermesTerminalID(%q) = %q, want %q", tty, got, want)
		}
	}
}

// ps prints the elapsed time as [[dd-]hh:]mm:ss on both macOS and Linux.
func TestParseElapsed(t *testing.T) {
	cases := []struct {
		value string
		want  time.Duration
		ok    bool
	}{
		{"00:25", 25 * time.Second, true},
		{"02:34:51", 2*time.Hour + 34*time.Minute + 51*time.Second, true},
		{"3-01:02:03", 3*24*time.Hour + time.Hour + 2*time.Minute + 3*time.Second, true},
		{"25", 0, false},
		{"x-01:02", 0, false},
		{"", 0, false},
	}
	for _, tc := range cases {
		got, ok := parseElapsed(tc.value)
		if got != tc.want || ok != tc.ok {
			t.Errorf("parseElapsed(%q) = %v, %v; want %v, %v", tc.value, got, ok, tc.want, tc.ok)
		}
	}
}

// Two Hermes in one directory on two terminals: the database's newest
// session belongs to the other one, and the breadcrumb of this process's
// terminal names its own.
func TestDetectHermes_TerminalBreadcrumbWins(t *testing.T) {
	home := hermesTestHome(t)
	hermesHome := filepath.Join(home, ".hermes")
	writeHermesDB(t, hermesHome, `
INSERT INTO sessions (id, source, started_at, ended_at, cwd, title) VALUES
	('local-this0000000000000', 'cli', 1794284884.2, NULL, '/src/app', NULL),
	('local-other000000000000', 'cli', 1794284947.6, NULL, '/src/app', NULL);`)

	started := time.Now().Add(-time.Minute)
	writeHermesBreadcrumb(t, hermesHome, "tty-dev-ttys000", "local-this0000000000000", "/src/app", started.Add(400*time.Millisecond))
	writeHermesBreadcrumb(t, hermesHome, "tty-dev-ttys002", "local-other000000000000", "/src/app", started.Add(time.Second))
	stubProcessTerminal(t, map[string]hermesProcess{"42": {tty: "ttys000", started: started}})

	s := detectHermes("/src/app", "42")
	if s == nil || s.Command != "hermes --resume local-this0000000000000" {
		t.Errorf("got %+v, want the session of this terminal", s)
	}
}

// `hermes --tui` writes no breadcrumb (seen in the lab), so the file can
// still name the session an earlier `hermes` had on the same terminal, in
// the same directory. A breadcrumb older than the process is not its own.
func TestDetectHermes_BreadcrumbOlderThanProcessIsIgnored(t *testing.T) {
	home := hermesTestHome(t)
	hermesHome := filepath.Join(home, ".hermes")
	writeHermesDB(t, hermesHome, `
INSERT INTO sessions (id, source, started_at, ended_at, cwd, title) VALUES
	('local-earlier000000000', 'cli', 1794284947.6, NULL, '/src/app', NULL),
	('local-tuisession000000', 'tui', 1794285027.7, NULL, '/src/app', NULL);`)

	started := time.Now().Add(-time.Minute)
	writeHermesBreadcrumb(t, hermesHome, "tty-dev-ttys002", "local-earlier000000000", "/src/app", started.Add(-time.Hour))
	stubProcessTerminal(t, map[string]hermesProcess{"42": {tty: "ttys002", started: started}})

	s := detectHermes("/src/app", "42")
	if s == nil || s.Command != "hermes --resume local-tuisession000000" {
		t.Errorf("got %+v, want the newest session in the database", s)
	}
}

// A breadcrumb for another directory, or one that cannot be read, says
// nothing about this process; the database decides.
func TestDetectHermes_BreadcrumbForAnotherDirectoryIsIgnored(t *testing.T) {
	home := hermesTestHome(t)
	hermesHome := filepath.Join(home, ".hermes")
	writeHermesDB(t, hermesHome, `
INSERT INTO sessions (id, source, started_at, ended_at, cwd, title) VALUES
	('local-fromdatabase0000', 'cli', 1794284947.6, NULL, '/src/app', NULL);`)

	started := time.Now().Add(-time.Minute)
	writeHermesBreadcrumb(t, hermesHome, "tty-dev-ttys002", "local-elsewhere0000000", "/src/other", started.Add(time.Second))
	if err := os.WriteFile(filepath.Join(hermesHome, "terminal-sessions", "tty-dev-ttys003"), []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	stubProcessTerminal(t, map[string]hermesProcess{
		"42": {tty: "ttys002", started: started},
		"43": {tty: "ttys003", started: started},
	})

	for _, pid := range []string{"42", "43"} {
		if s := detectHermes("/src/app", pid); s == nil || s.Command != "hermes --resume local-fromdatabase0000" {
			t.Errorf("pid %s: got %+v, want the session from the database", pid, s)
		}
	}
}
