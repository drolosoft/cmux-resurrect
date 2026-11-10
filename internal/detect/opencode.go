package detect

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// openCodeMemoryDB is the OPENCODE_DB value that keeps the database in
// memory. Nothing reaches the disk, so there is no session to resume.
const openCodeMemoryDB = ":memory:"

// openCodeSessionFilters are the conditions that keep a session out of a
// resume, keyed by the column each one needs. A session with a parent_id is
// a subagent run inside another conversation; one with time_archived was
// archived by the user. Older databases lack one or both columns, and
// naming a missing column fails the whole query, so each condition is used
// only when its column exists. The conditions are fixed text: nothing from
// the database or the environment is ever spliced into the query here.
var openCodeSessionFilters = []struct {
	column    string
	condition string
}{
	{column: "parent_id", condition: "parent_id IS NULL"},
	{column: "time_archived", condition: "time_archived IS NULL"},
}

// detectOpenCode finds the active session for an OpenCode instance by CWD:
// the most recently updated top-level, non-archived session in that
// directory. The database is found the way OpenCode finds it, see
// openCodeDBPath. We shell out to the sqlite3 CLI (ships with macOS) to
// avoid CGO dependencies.
func detectOpenCode(cwd, pid string) *Session {
	dbPath := openCodeDBPath(pid)
	if dbPath == "" {
		return nil
	}
	if _, err := os.Stat(dbPath); err != nil {
		return nil
	}

	conditions, ok := openCodeSessionConditions(dbPath)
	if !ok {
		return nil
	}
	conditions = append([]string{"directory = '" + escapeSQLite(cwd) + "'"}, conditions...)

	query := `SELECT id FROM session WHERE ` + strings.Join(conditions, " AND ") + ` ORDER BY time_updated DESC LIMIT 1;`
	out, err := queryOpenCodeDB(dbPath, query)
	if err != nil {
		return nil
	}

	sessionID := strings.TrimSpace(out)
	if sessionID == "" || !validSessionID.MatchString(sessionID) {
		return nil
	}

	return &Session{
		Tool:    "opencode",
		CWD:     cwd,
		Command: "opencode --session " + sessionID,
	}
}

// openCodeDBPath returns the database an OpenCode process uses, resolved as
// OpenCode resolves it (packages/cli/src/database-path.ts and
// packages/util/src/global-roots.ts): $OPENCODE_DB when set, under the data
// directory when relative; opencode.db in the data directory otherwise. The
// data directory is $XDG_DATA_HOME/opencode, ~/.local/share/opencode by
// default. It returns "" when there is no file to read.
//
// The variables come from the environment of the OpenCode process, since
// that is where it wrote its sessions. When that environment can't be read,
// crex's own is the best guess left.
func openCodeDBPath(pid string) string {
	getenv := os.Getenv
	if env := processEnv(pid); env != nil {
		getenv = func(key string) string { return env[key] }
	}

	// A relative XDG_DATA_HOME is invalid by the XDG spec; it would resolve
	// against a directory we can't know, so it falls back like an unset one.
	dataHome := getenv("XDG_DATA_HOME")
	if !filepath.IsAbs(dataHome) {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	dataDir := filepath.Join(dataHome, "opencode")

	// Release builds name the file opencode.db; only local and preview
	// channels add a suffix, and those set OPENCODE_DB when it matters.
	dbPath := getenv("OPENCODE_DB")
	switch {
	case dbPath == "":
		return filepath.Join(dataDir, "opencode.db")
	case dbPath == openCodeMemoryDB:
		return ""
	case filepath.IsAbs(dbPath):
		return filepath.Clean(dbPath)
	default:
		return filepath.Join(dataDir, dbPath)
	}
}

// openCodeSessionConditions returns the filters from
// openCodeSessionFilters whose column the session table has. ok is false
// when the columns can't be read, which also means the query would fail.
func openCodeSessionConditions(dbPath string) (conditions []string, ok bool) {
	out, err := queryOpenCodeDB(dbPath, `SELECT name FROM pragma_table_info('session');`)
	if err != nil {
		return nil, false
	}

	columns := make(map[string]bool)
	for _, name := range strings.Fields(out) {
		columns[name] = true
	}

	// No columns at all means there is no session table to query.
	if len(columns) == 0 {
		return nil, false
	}

	for _, filter := range openCodeSessionFilters {
		if columns[filter.column] {
			conditions = append(conditions, filter.condition)
		}
	}
	return conditions, true
}

// queryOpenCodeDB runs one query against the database with the sqlite3 CLI
// in read-only mode, so crex can never change OpenCode's data, and returns
// what it prints.
func queryOpenCodeDB(dbPath, query string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "sqlite3", "-readonly", dbPath, query).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
