package detect

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// gooseSessionFilters are the conditions that keep a session out of a
// resume, keyed by the column each one needs. Only a 'user' session is a
// chat someone typed in a terminal: subagent, scheduled, hidden, gateway and
// ACP runs share the table. One with archived_at was archived by the user.
// Goose added session_type in schema version 5 and archived_at in version
// 12, and naming a missing column fails the whole query, so each condition
// is used only when its column exists. The conditions are fixed text:
// nothing from the database or the environment is ever spliced into the
// query here.
var gooseSessionFilters = []struct {
	column    string
	condition string
}{
	{column: "session_type", condition: "session_type = 'user'"},
	{column: "archived_at", condition: "archived_at IS NULL"},
}

// detectGoose finds the latest chat Goose (aaif-goose/goose, formerly
// block/goose) recorded for cwd and resumes it by id. Goose keeps every
// session in <data dir>/sessions/sessions.db (SQLite), which we read with
// the sqlite3 CLI like OpenCode's, to avoid CGO dependencies.
//
// pressly/goose, the database migration tool, is also called goose. A
// process by that name is only taken for the AI agent when its database has
// a session for cwd, so without one this returns nil instead of a bare
// relaunch that could start the wrong program.
func detectGoose(cwd, pid string) *Session {
	dataDir, pathRoot := gooseDataDir(pid)
	if dataDir == "" {
		return nil
	}

	sessionID := latestGooseSession(filepath.Join(dataDir, "sessions", "sessions.db"), cwd)
	if sessionID == "" {
		return nil
	}

	return &Session{
		Tool:    "goose",
		CWD:     cwd,
		Command: gooseResumeCommand(pathRoot, sessionID),
	}
}

// gooseDataDir returns the data directory a Goose process uses, resolved as
// Goose resolves it (crates/goose/src/config/paths.rs): <root>/data when
// $GOOSE_PATH_ROOT is an absolute path, otherwise the Xdg strategy of the
// etcetera crate, which Goose uses on macOS as well as on Linux:
// $XDG_DATA_HOME/goose, ~/.local/share/goose by default. Relative values of
// either variable are ignored, as Goose and etcetera ignore them. pathRoot
// is the GOOSE_PATH_ROOT in effect, or "" when there is none. dir is ""
// when the home directory can't be found.
//
// The variables come from the environment of the Goose process, since that
// is where it wrote its sessions. When that environment can't be read,
// crex's own is the best guess left.
func gooseDataDir(pid string) (dir, pathRoot string) {
	getenv := os.Getenv
	if env := processEnv(pid); env != nil {
		getenv = func(key string) string { return env[key] }
	}

	if root := getenv("GOOSE_PATH_ROOT"); filepath.IsAbs(root) {
		root = filepath.Clean(root)
		return filepath.Join(root, "data"), root
	}

	dataHome := getenv("XDG_DATA_HOME")
	if !filepath.IsAbs(dataHome) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", ""
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "goose"), ""
}

// latestGooseSession returns the id of the most recently updated chat for
// cwd in the Goose database at dbPath, or "" when there is none or the
// database can't be read.
//
// updated_at is text in two shapes: datetime('now') writes
// '2026-11-10 09:15:00', and an imported session keeps sqlx's RFC 3339
// '2026-11-10T08:00:00+00:00'. Compared as text the T sorts after the space,
// so the column goes through julianday(), which reads both, the way Goose
// itself sorts with unixepoch(). julianday() also works on the sqlite3 of
// older systems, where unixepoch() does not exist.
func latestGooseSession(dbPath, cwd string) string {
	if _, err := os.Stat(dbPath); err != nil {
		return ""
	}

	conditions, ok := gooseSessionConditions(dbPath)
	if !ok {
		return ""
	}
	conditions = append([]string{"working_dir = '" + escapeSQLite(cwd) + "'"}, conditions...)

	query := `SELECT id FROM sessions WHERE ` + strings.Join(conditions, " AND ") + ` ORDER BY julianday(updated_at) DESC, id DESC LIMIT 1;`
	out, err := queryGooseDB(dbPath, query)
	if err != nil {
		return ""
	}

	sessionID := strings.TrimSpace(out)
	if !validSessionID.MatchString(sessionID) {
		return ""
	}
	return sessionID
}

// gooseSessionConditions returns the filters from gooseSessionFilters whose
// column the sessions table has. ok is false when the columns can't be
// read, which also means the query would fail.
func gooseSessionConditions(dbPath string) (conditions []string, ok bool) {
	out, err := queryGooseDB(dbPath, `SELECT name FROM pragma_table_info('sessions');`)
	if err != nil {
		return nil, false
	}

	columns := make(map[string]bool)
	for _, name := range strings.Fields(out) {
		columns[name] = true
	}

	// No columns at all means there is no sessions table to query.
	if len(columns) == 0 {
		return nil, false
	}

	for _, filter := range gooseSessionFilters {
		if columns[filter.column] {
			conditions = append(conditions, filter.condition)
		}
	}
	return conditions, true
}

// queryGooseDB runs one query against the database with the sqlite3 CLI in
// read-only mode, so crex can never change Goose's data, and returns what
// it prints.
func queryGooseDB(dbPath, query string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "sqlite3", "-readonly", dbPath, query).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// gooseResumeCommand builds the command that reopens a Goose session. It
// names the id with --session-id: a bare --resume reopens the latest chat
// of any directory. The command is prefixed with GOOSE_PATH_ROOT when the
// session lives under one, because without it the restored shell would
// look for the id in the default database and Goose would exit.
func gooseResumeCommand(pathRoot, sessionID string) string {
	cmd := "goose session --resume --session-id " + sessionID
	if pathRoot != "" {
		cmd = "GOOSE_PATH_ROOT=" + shellQuote(pathRoot) + " " + cmd
	}
	return cmd
}
