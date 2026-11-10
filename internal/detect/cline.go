package detect

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// clineTerminalSource is the source Cline CLI gives the sessions it starts.
// The VS Code extension, the JetBrains plugin, the desktop app and the kanban
// board can keep their sessions in the same database, and none of those is
// the conversation of the terminal being restored.
const clineTerminalSource = "cli"

// clineDirVariables are the variables that move Cline's session index, from
// the widest to the narrowest (sdk/packages/shared/src/storage/paths.ts):
// CLINE_DIR replaces ~/.cline, CLINE_DATA_DIR replaces <cline dir>/data and
// CLINE_DB_DATA_DIR replaces <data dir>/db. A resume command repeats every
// one the process had, since the wider ones also move settings and
// providers, not just the database.
var clineDirVariables = []string{"CLINE_DIR", "CLINE_DATA_DIR", "CLINE_DB_DATA_DIR"}

// clineSessionFilters are the conditions that keep a child session out of a
// resume, keyed by the column each one needs. Subagent and team-task runs are
// recorded under their parent, and Cline's own history hides them the same
// way (rootOnly in sdk/packages/core/src/session/services/session-service.ts).
// Both columns arrived through legacy migrations, so an older database may
// lack them, and naming a missing column fails the whole query. The
// conditions are fixed text: nothing from the database or the environment is
// ever spliced into the query here.
var clineSessionFilters = []struct {
	column    string
	condition string
}{
	{column: "is_subagent", condition: "is_subagent = 0"},
	{column: "parent_session_id", condition: "(parent_session_id IS NULL OR parent_session_id = '')"},
}

// detectCline finds the session Cline CLI last used in cwd and resumes it by
// id. Cline indexes its sessions in <data dir>/db/sessions.db (SQLite), which
// we read with the sqlite3 CLI like OpenCode's, to avoid CGO dependencies.
// Cline has no flag that continues the latest session (-c is --cwd), so
// without a database or a session for cwd it starts fresh.
func detectCline(cwd, pid string) *Session {
	fresh := &Session{Tool: "cline", CWD: cwd, Command: "cline"}

	dbDir, overrides := clineDBDir(pid)
	if dbDir == "" {
		return fresh
	}

	sessionID := latestClineSession(filepath.Join(dbDir, "sessions.db"), cwd)
	if sessionID == "" {
		return fresh
	}

	command := "cline --id " + sessionID
	if len(overrides) > 0 {
		command = strings.Join(overrides, " ") + " " + command
	}
	return &Session{Tool: "cline", CWD: cwd, Command: command}
}

// clineDBDir returns the directory a Cline process keeps sessions.db in,
// resolved as resolveDbDataDir does from the variables in clineDirVariables,
// and the assignments a resume command needs to find it again. overrides is
// empty when the directory is the default ~/.cline/data/db, and dir is ""
// when the home directory is unknown.
//
// The variables come from the environment of the Cline process, since that
// is where it wrote its sessions. Cline ignores empty values; a relative one
// would resolve against a directory we can't know, so it counts as unset.
func clineDBDir(pid string) (dir string, overrides []string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", nil
	}
	defaultDir := filepath.Join(home, ".cline", "data", "db")

	env := processEnv(pid)
	values := make(map[string]string)
	for _, name := range clineDirVariables {
		value := strings.TrimSpace(env[name])
		if value == "" || !filepath.IsAbs(value) {
			continue
		}
		values[name] = filepath.Clean(value)
		overrides = append(overrides, name+"="+shellQuote(values[name]))
	}

	// Each variable, when set, replaces everything above it in the path.
	clineDir := filepath.Join(home, ".cline")
	if value, ok := values["CLINE_DIR"]; ok {
		clineDir = value
	}
	dataDir := filepath.Join(clineDir, "data")
	if value, ok := values["CLINE_DATA_DIR"]; ok {
		dataDir = value
	}
	dir = filepath.Join(dataDir, "db")
	if value, ok := values["CLINE_DB_DATA_DIR"]; ok {
		dir = value
	}

	if dir == defaultDir {
		return dir, nil
	}
	return dir, overrides
}

// latestClineSession returns the id of the top-level terminal session for cwd
// that was used last, or "" when there is none or the database can't be read.
// updated_at moves whenever Cline records a change of status, so a session
// resumed today sorts above one started yesterday; started_at breaks ties and
// orders a database that predates updated_at.
func latestClineSession(dbPath, cwd string) string {
	if _, err := os.Stat(dbPath); err != nil {
		return ""
	}

	columns, ok := clineSessionColumns(dbPath)
	if !ok {
		return ""
	}

	conditions := []string{
		"source = '" + clineTerminalSource + "'",
		"cwd = '" + escapeSQLite(cwd) + "'",
	}
	for _, filter := range clineSessionFilters {
		if columns[filter.column] {
			conditions = append(conditions, filter.condition)
		}
	}

	order := "started_at DESC"
	if columns["updated_at"] {
		order = "updated_at DESC, " + order
	}

	query := `SELECT session_id FROM sessions WHERE ` + strings.Join(conditions, " AND ") + ` ORDER BY ` + order + ` LIMIT 1;`
	out, err := queryClineDB(dbPath, query)
	if err != nil {
		return ""
	}

	sessionID := strings.TrimSpace(out)
	if !validSessionID.MatchString(sessionID) {
		return ""
	}
	return sessionID
}

// clineSessionColumns returns the names of the columns of the sessions
// table. ok is false when they can't be read or there is no such table, in
// which case any query on it would fail too.
func clineSessionColumns(dbPath string) (columns map[string]bool, ok bool) {
	out, err := queryClineDB(dbPath, `SELECT name FROM pragma_table_info('sessions');`)
	if err != nil {
		return nil, false
	}

	columns = make(map[string]bool)
	for _, name := range strings.Fields(out) {
		columns[name] = true
	}
	return columns, len(columns) > 0
}

// queryClineDB runs one query against the database with the sqlite3 CLI in
// read-only mode, so crex can never change Cline's data, and returns what it
// prints.
func queryClineDB(dbPath, query string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "sqlite3", "-readonly", dbPath, query).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
