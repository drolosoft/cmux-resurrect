package detect

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// crushSchema is the part of Crush's sessions table the detector reads. The
// columns and their types come from internal/db/migrations/
// 20250424200609_initial.sql of charmbracelet/crush (commit 1b37350), and
// internal/db/models.go has the same fields. Crush opens the file in WAL
// mode (internal/db/connect.go), so the fixture does too: a read-only open
// has to cope with it.
const crushSchema = `PRAGMA journal_mode=WAL;
CREATE TABLE sessions (
	id TEXT PRIMARY KEY,
	parent_session_id TEXT,
	title TEXT NOT NULL,
	updated_at INTEGER NOT NULL,
	created_at INTEGER NOT NULL
);`

// writeCrushDB creates <dataDir>/crush.db with the sessions table and runs
// the given INSERT statements through the sqlite3 CLI, the same tool the
// detector reads it with. The test is skipped when sqlite3 is not installed.
func writeCrushDB(t *testing.T, dataDir, inserts string) {
	t.Helper()
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 CLI not installed")
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(dataDir, "crush.db")
	out, err := exec.Command("sqlite3", dbPath, crushSchema+inserts).CombinedOutput()
	if err != nil {
		t.Fatalf("sqlite3: %v\n%s", err, out)
	}
}

// isolateCrush points HOME at a fresh directory and clears the variables
// that move Crush's files, so neither the real projects.json nor the shell
// running the tests can leak in. It returns the temp HOME.
func isolateCrush(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("CRUSH_GLOBAL_DATA", "")
	t.Setenv("CRUSH_DATA_DIR", "")
	stubProcessEnv(t, nil)
	return home
}

// makeCrushRepository creates dir with a .git directory in it, the mark
// Crush's `git rev-parse --show-toplevel` and repositoryRoot both stop at.
func makeCrushRepository(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// makeCrushDir creates dir and its parents, failing the test when it can't.
func makeCrushDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

// wantCrushCommand fails the test unless s is a crush session for cwd that
// runs command.
func wantCrushCommand(t *testing.T, s *Session, cwd, command string) {
	t.Helper()
	if s == nil {
		t.Fatalf("expected a session running %q, got nil", command)
	}
	if s.Tool != "crush" || s.CWD != cwd {
		t.Errorf("Tool, CWD = %q, %q; want crush, %s", s.Tool, s.CWD, cwd)
	}
	if s.Command != command {
		t.Errorf("Command = %q, want %q", s.Command, command)
	}
}

// The ids are UUIDs, as session.Service gives them; updated_at is a Unix
// time in seconds, kept fresh by the update_sessions_updated_at trigger.

func TestDetectCrush_SessionInCWD(t *testing.T) {
	home := isolateCrush(t)
	project := filepath.Join(home, "src", "app")
	makeCrushRepository(t, project)
	writeCrushDB(t, filepath.Join(project, ".crush"), `
INSERT INTO sessions (id, parent_session_id, title, updated_at, created_at) VALUES
	('3f0c2a9e-5b1d-4c7e-9a2f-1d8e6b4c0a71', NULL, 'refactor', 1794234622, 1794230000);`)

	s := detectCrush(project, "")
	wantCrushCommand(t, s, project, "crush --session 3f0c2a9e-5b1d-4c7e-9a2f-1d8e6b4c0a71")
}

func TestDetectCrush_SessionInAncestorInsideRepository(t *testing.T) {
	home := isolateCrush(t)
	project := filepath.Join(home, "src", "app")
	makeCrushRepository(t, project)
	// Crush started in a subdirectory adopts the .crush at the root of the
	// repository (setDefaults in internal/config/load.go uses
	// fsext.LookupClosestBounded up to the worktree root).
	cwd := filepath.Join(project, "internal", "store")
	makeCrushDir(t, cwd)
	writeCrushDB(t, filepath.Join(project, ".crush"), `
INSERT INTO sessions (id, parent_session_id, title, updated_at, created_at) VALUES
	('3f0c2a9e-5b1d-4c7e-9a2f-1d8e6b4c0a71', NULL, 'refactor', 1794234622, 1794230000);`)

	s := detectCrush(cwd, "")
	wantCrushCommand(t, s, cwd, "crush --session 3f0c2a9e-5b1d-4c7e-9a2f-1d8e6b4c0a71")
}

func TestDetectCrush_IgnoresCrushAboveRepositoryRoot(t *testing.T) {
	home := isolateCrush(t)
	// A .crush in the directory that holds several repositories belongs to
	// none of them: Crush stops the walk at the worktree root and creates
	// <cwd>/.crush instead, which has no sessions yet.
	workspace := filepath.Join(home, "src")
	writeCrushDB(t, filepath.Join(workspace, ".crush"), `
INSERT INTO sessions (id, parent_session_id, title, updated_at, created_at) VALUES
	('3f0c2a9e-5b1d-4c7e-9a2f-1d8e6b4c0a71', NULL, 'other project', 1794234622, 1794230000);`)
	project := filepath.Join(workspace, "app")
	makeCrushRepository(t, project)

	s := detectCrush(project, "")
	wantCrushCommand(t, s, project, "crush")
}

func TestDetectCrush_WithoutRepositoryOnlyCWD(t *testing.T) {
	home := isolateCrush(t)
	// Outside a repository the boundary is the directory itself
	// (projectBoundary in internal/config/load.go), so a parent's .crush is
	// not adopted either.
	parent := filepath.Join(home, "notes")
	writeCrushDB(t, filepath.Join(parent, ".crush"), `
INSERT INTO sessions (id, parent_session_id, title, updated_at, created_at) VALUES
	('3f0c2a9e-5b1d-4c7e-9a2f-1d8e6b4c0a71', NULL, 'notes', 1794234622, 1794230000);`)
	cwd := filepath.Join(parent, "drafts")
	makeCrushDir(t, cwd)

	s := detectCrush(cwd, "")
	wantCrushCommand(t, s, cwd, "crush")
}

func TestDetectCrush_IgnoresCrushInHome(t *testing.T) {
	home := isolateCrush(t)
	// A dotfiles repository makes $HOME a worktree root, but Crush never
	// adopts a .crush found there (LookupClosestBounded in
	// internal/fsext/lookup.go skips home.Dir()).
	makeCrushRepository(t, home)
	writeCrushDB(t, filepath.Join(home, ".crush"), `
INSERT INTO sessions (id, parent_session_id, title, updated_at, created_at) VALUES
	('3f0c2a9e-5b1d-4c7e-9a2f-1d8e6b4c0a71', NULL, 'dotfiles', 1794234622, 1794230000);`)
	cwd := filepath.Join(home, "scratch")
	makeCrushDir(t, cwd)

	s := detectCrush(cwd, "")
	wantCrushCommand(t, s, cwd, "crush")
}

func TestDetectCrush_SkipsChildSessions(t *testing.T) {
	home := isolateCrush(t)
	project := filepath.Join(home, "src", "app")
	makeCrushRepository(t, project)
	// The agent tool and title generation run in child sessions, which are
	// newer than their parent. Crush lists only parent_session_id IS NULL
	// (ListSessions in internal/db/sql/sessions.sql).
	writeCrushDB(t, filepath.Join(project, ".crush"), `
INSERT INTO sessions (id, parent_session_id, title, updated_at, created_at) VALUES
	('3f0c2a9e-5b1d-4c7e-9a2f-1d8e6b4c0a71', NULL, 'refactor', 1794234622, 1794230000),
	('9b7d4e10-2c3a-4f5b-8e6d-7a1c0b2d3e4f', '3f0c2a9e-5b1d-4c7e-9a2f-1d8e6b4c0a71', 'agent task', 1794234700, 1794234650);`)

	s := detectCrush(project, "")
	wantCrushCommand(t, s, project, "crush --session 3f0c2a9e-5b1d-4c7e-9a2f-1d8e6b4c0a71")
}

func TestDetectCrush_NewestOfTwo(t *testing.T) {
	home := isolateCrush(t)
	project := filepath.Join(home, "src", "app")
	makeCrushRepository(t, project)
	// The newer session was created first: the order is by updated_at, not
	// by creation or by row order.
	writeCrushDB(t, filepath.Join(project, ".crush"), `
INSERT INTO sessions (id, parent_session_id, title, updated_at, created_at) VALUES
	('5a6b7c8d-9e0f-4a1b-8c2d-3e4f5a6b7c8d', NULL, 'recent work', 1794234622, 1794100000),
	('3f0c2a9e-5b1d-4c7e-9a2f-1d8e6b4c0a71', NULL, 'old work', 1794129323, 1794129000);`)

	s := detectCrush(project, "")
	wantCrushCommand(t, s, project, "crush --session 5a6b7c8d-9e0f-4a1b-8c2d-3e4f5a6b7c8d")
}

func TestDetectCrush_NoDBIsBare(t *testing.T) {
	home := isolateCrush(t)
	project := filepath.Join(home, "src", "app")
	makeCrushRepository(t, project)

	s := detectCrush(project, "")
	wantCrushCommand(t, s, project, "crush")
}

func TestDetectCrush_OnlyChildSessionsIsBare(t *testing.T) {
	home := isolateCrush(t)
	project := filepath.Join(home, "src", "app")
	makeCrushRepository(t, project)
	writeCrushDB(t, filepath.Join(project, ".crush"), `
INSERT INTO sessions (id, parent_session_id, title, updated_at, created_at) VALUES
	('9b7d4e10-2c3a-4f5b-8e6d-7a1c0b2d3e4f', '3f0c2a9e-5b1d-4c7e-9a2f-1d8e6b4c0a71', 'agent task', 1794234700, 1794234650);`)

	s := detectCrush(project, "")
	wantCrushCommand(t, s, project, "crush")
}

func TestDetectCrush_InvalidIDIsBare(t *testing.T) {
	home := isolateCrush(t)
	project := filepath.Join(home, "src", "app")
	makeCrushRepository(t, project)
	// The id ends up in a shell command, so anything but the characters of
	// an id is refused rather than typed into the restored pane.
	writeCrushDB(t, filepath.Join(project, ".crush"), `
INSERT INTO sessions (id, parent_session_id, title, updated_at, created_at) VALUES
	('x; rm -rf ~', NULL, 'odd', 1794234622, 1794230000);`)

	s := detectCrush(project, "")
	wantCrushCommand(t, s, project, "crush")
}

func TestDetectCrush_DataDirFromProcess(t *testing.T) {
	home := isolateCrush(t)
	project := filepath.Join(home, "src", "app")
	makeCrushRepository(t, project)
	// CRUSH_DATA_DIR replaces the lookup, and a relative value resolves
	// against the working directory (Load in internal/config/load.go).
	writeCrushDB(t, filepath.Join(project, "state", "crush"), `
INSERT INTO sessions (id, parent_session_id, title, updated_at, created_at) VALUES
	('3f0c2a9e-5b1d-4c7e-9a2f-1d8e6b4c0a71', NULL, 'refactor', 1794234622, 1794230000);`)
	stubProcessEnv(t, map[string]map[string]string{"42": {"CRUSH_DATA_DIR": "state/crush"}})

	s := detectCrush(project, "42")
	wantCrushCommand(t, s, project, "crush --session 3f0c2a9e-5b1d-4c7e-9a2f-1d8e6b4c0a71")
}

func TestDetectCrush_DataDirFromProjectsList(t *testing.T) {
	home := isolateCrush(t)
	project := filepath.Join(home, "src", "app")
	makeCrushRepository(t, project)
	// --data-dir and the data_directory option leave no trace crex can read
	// in the process, but Crush records the directory it used for each
	// working directory in projects.json (Register in
	// internal/projects/projects.go), next to its global crush.json.
	dataDir := filepath.Join(home, "crush-data", "app")
	writeCrushDB(t, dataDir, `
INSERT INTO sessions (id, parent_session_id, title, updated_at, created_at) VALUES
	('3f0c2a9e-5b1d-4c7e-9a2f-1d8e6b4c0a71', NULL, 'refactor', 1794234622, 1794230000);`)
	writeCrushProjects(t, filepath.Join(home, ".local", "share", "crush"), `{
  "projects": [
    {"path": "`+filepath.Join(home, "elsewhere")+`", "data_dir": "`+filepath.Join(home, "elsewhere", ".crush")+`", "last_accessed": "2026-11-10T08:00:00Z"},
    {"path": "`+project+`", "data_dir": "`+dataDir+`", "last_accessed": "2026-11-09T18:30:00Z"}
  ]
}`)

	s := detectCrush(project, "")
	wantCrushCommand(t, s, project, "crush --session 3f0c2a9e-5b1d-4c7e-9a2f-1d8e6b4c0a71")
}

func TestDetectCrush_StaleProjectsEntryFallsBackToLookup(t *testing.T) {
	home := isolateCrush(t)
	project := filepath.Join(home, "src", "app")
	makeCrushRepository(t, project)
	// The recorded directory was deleted since; the .crush in the project is
	// what Crush will open next.
	writeCrushDB(t, filepath.Join(project, ".crush"), `
INSERT INTO sessions (id, parent_session_id, title, updated_at, created_at) VALUES
	('3f0c2a9e-5b1d-4c7e-9a2f-1d8e6b4c0a71', NULL, 'refactor', 1794234622, 1794230000);`)
	writeCrushProjects(t, filepath.Join(home, ".local", "share", "crush"), `{
  "projects": [
    {"path": "`+project+`", "data_dir": "`+filepath.Join(home, "gone")+`", "last_accessed": "2026-11-09T18:30:00Z"}
  ]
}`)

	s := detectCrush(project, "")
	wantCrushCommand(t, s, project, "crush --session 3f0c2a9e-5b1d-4c7e-9a2f-1d8e6b4c0a71")
}

func TestDetectCrush_ProjectsListFromGlobalDataEnv(t *testing.T) {
	home := isolateCrush(t)
	project := filepath.Join(home, "src", "app")
	makeCrushRepository(t, project)
	// CRUSH_GLOBAL_DATA moves crush.json, and projects.json with it
	// (GlobalConfigData in internal/config/load.go).
	globalData := filepath.Join(home, "crush-global")
	dataDir := filepath.Join(home, "crush-data", "app")
	writeCrushDB(t, dataDir, `
INSERT INTO sessions (id, parent_session_id, title, updated_at, created_at) VALUES
	('3f0c2a9e-5b1d-4c7e-9a2f-1d8e6b4c0a71', NULL, 'refactor', 1794234622, 1794230000);`)
	writeCrushProjects(t, globalData, `{"projects": [{"path": "`+project+`", "data_dir": "`+dataDir+`"}]}`)
	stubProcessEnv(t, map[string]map[string]string{"7": {"CRUSH_GLOBAL_DATA": globalData}})

	s := detectCrush(project, "7")
	wantCrushCommand(t, s, project, "crush --session 3f0c2a9e-5b1d-4c7e-9a2f-1d8e6b4c0a71")
}

// writeCrushProjects writes Crush's projects.json into dir.
func writeCrushProjects(t *testing.T, dir, content string) {
	t.Helper()
	makeCrushDir(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "projects.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
