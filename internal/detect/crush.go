package detect

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// crushDataDirName is the directory Crush keeps a project's state in, its
// database among it (defaultDataDirectory in internal/config/config.go of
// charmbracelet/crush). It lives inside the project, not under the home
// directory, so finding the database means finding the right project.
const crushDataDirName = ".crush"

// crushDBName is the file Crush opens inside its data directory (Connect in
// internal/db/connect.go).
const crushDBName = "crush.db"

// crushProjectsName is the list where Crush records, for every working
// directory it ran in, the data directory it used there (Register in
// internal/projects/projects.go). It sits next to the global crush.json.
const crushProjectsName = "projects.json"

// crushLatestSessionQuery picks the conversation to reopen. Sessions with a
// parent are agent tool runs and title generation inside another
// conversation, and Crush's own session list leaves them out the same way
// (ListSessions in internal/db/sql/sessions.sql). updated_at counts whole
// seconds, so created_at breaks a tie.
const crushLatestSessionQuery = `SELECT id FROM sessions WHERE parent_session_id IS NULL ORDER BY updated_at DESC, created_at DESC LIMIT 1;`

// detectCrush finds the latest top-level session of Crush (Charmbracelet)
// for cwd and resumes it by id. Crush keeps each project's sessions in a
// SQLite file inside the project, so the sessions table has no directory
// column: the location of the database is what ties it to cwd. We read it
// with the sqlite3 CLI like OpenCode's, to avoid CGO dependencies. Without
// a database or a session in it, Crush starts fresh, as it did before crex
// could resume it.
func detectCrush(cwd, pid string) *Session {
	bare := &Session{Tool: "crush", CWD: cwd, Command: "crush"}

	// A relative or empty cwd would make the lookup depend on the directory
	// crex runs in, and pick up a database that is not this project's.
	if !filepath.IsAbs(cwd) {
		return bare
	}

	dataDir := crushDataDir(cwd, crushGetenv(pid))
	sessionID := latestCrushSession(filepath.Join(dataDir, crushDBName))
	if sessionID == "" {
		return bare
	}

	return &Session{
		Tool:    "crush",
		CWD:     cwd,
		Command: "crush --session " + sessionID,
	}
}

// crushGetenv returns a lookup into the environment of the Crush process,
// since that is where the variables that move its files were set. When that
// environment can't be read, crex's own is the best guess left.
func crushGetenv(pid string) func(string) string {
	env := processEnv(pid)
	if env == nil {
		return os.Getenv
	}

	return func(key string) string { return env[key] }
}

// crushDataDir returns the data directory a Crush process in cwd uses. The
// directory Crush recorded for cwd in projects.json comes first: it is the
// only trace left by --data-dir and the data_directory option, which crex
// can't see. Then CRUSH_DATA_DIR, which replaces the lookup (Load in
// internal/config/load.go), and last the lookup itself, nearestCrushDataDir.
func crushDataDir(cwd string, getenv func(string) string) string {
	if dataDir := registeredCrushDataDir(cwd, getenv); dataDir != "" {
		return dataDir
	}

	if dataDir := getenv("CRUSH_DATA_DIR"); dataDir != "" {
		return crushPath(cwd, dataDir)
	}

	return nearestCrushDataDir(cwd)
}

// registeredCrushDataDir returns the data directory projects.json records
// for cwd, or "" when there is no entry or its database is gone. A deleted
// directory says nothing about where Crush looks next, so the caller falls
// back to the lookup.
func registeredCrushDataDir(cwd string, getenv func(string) string) string {
	path := crushProjectsFile(cwd, getenv)
	if path == "" {
		return ""
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	var list struct {
		Projects []struct {
			Path    string `json:"path"`
			DataDir string `json:"data_dir"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(data, &list); err != nil {
		return ""
	}

	// Crush keeps a single entry per working directory and rewrites it on
	// every start, so the first match is the latest.
	for _, project := range list.Projects {
		if project.Path != cwd || project.DataDir == "" {
			continue
		}

		dataDir := crushPath(cwd, project.DataDir)
		if _, err := os.Stat(filepath.Join(dataDir, crushDBName)); err != nil {
			return ""
		}
		return dataDir
	}
	return ""
}

// crushProjectsFile returns where projects.json is: next to Crush's global
// crush.json, which $CRUSH_GLOBAL_DATA moves, then $XDG_DATA_HOME/crush,
// ~/.local/share/crush by default (GlobalConfigData in
// internal/config/load.go). It returns "" when the home directory is
// unknown.
func crushProjectsFile(cwd string, getenv func(string) string) string {
	if dir := getenv("CRUSH_GLOBAL_DATA"); dir != "" {
		return filepath.Join(crushPath(cwd, dir), crushProjectsName)
	}

	if dir := getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(crushPath(cwd, dir), "crush", crushProjectsName)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "crush", crushProjectsName)
}

// nearestCrushDataDir walks up from cwd to the closest existing .crush, the
// way Crush does (setDefaults in internal/config/load.go and
// LookupClosestBounded in internal/fsext/lookup.go): never past the root of
// the git worktree, and only cwd itself outside a repository, so a parent
// project's state is never adopted. Without a match it is <cwd>/.crush, the
// directory Crush creates.
func nearestCrushDataDir(cwd string) string {
	boundary := repositoryRoot(cwd)
	if boundary == "" {
		boundary = cwd
	}

	// An error leaves home empty, and then no directory is skipped below.
	home, _ := os.UserHomeDir()

	dir := cwd
	for {
		candidate := filepath.Join(dir, crushDataDirName)
		if _, err := os.Stat(candidate); err == nil {
			// Crush treats a .crush in the home directory as no match and
			// ends the search there, even inside a dotfiles repository.
			if dir == home {
				break
			}
			return candidate
		}

		parent := filepath.Dir(dir)
		if dir == boundary || parent == dir {
			break
		}
		dir = parent
	}

	return filepath.Join(cwd, crushDataDirName)
}

// crushPath resolves path the way Crush resolves its data directories:
// relative to the working directory, absolute as given.
func crushPath(cwd, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}

	return filepath.Join(cwd, path)
}

// latestCrushSession returns the id of the newest top-level session in the
// Crush database at dbPath, or "" when there is none or the database can't
// be read. The sqlite3 CLI opens it read-only, so crex can never change
// Crush's data.
func latestCrushSession(dbPath string) string {
	if _, err := os.Stat(dbPath); err != nil {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "sqlite3", "-readonly", dbPath, crushLatestSessionQuery).Output()
	if err != nil {
		return ""
	}

	sessionID := strings.TrimSpace(string(out))
	if !validSessionID.MatchString(sessionID) {
		return ""
	}
	return sessionID
}
