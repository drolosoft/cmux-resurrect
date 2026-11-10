package detect

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// detectOpenCode finds the active session for an OpenCode instance by CWD.
// Sessions are stored in ~/.local/share/opencode/opencode.db (SQLite).
// We shell out to the sqlite3 CLI (ships with macOS) to avoid CGO dependencies.
func detectOpenCode(cwd, _ string) *Session {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	dbPath := filepath.Join(home, ".local", "share", "opencode", "opencode.db")
	if _, err := os.Stat(dbPath); err != nil {
		return nil
	}

	// Use the sqlite3 CLI to query the database in read-only mode.
	query := `SELECT id FROM session WHERE directory = '` + escapeSQLite(cwd) + `' ORDER BY time_updated DESC LIMIT 1;`
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "sqlite3", "-readonly", dbPath, query).Output()
	if err != nil {
		return nil
	}

	sessionID := strings.TrimSpace(string(out))
	if sessionID == "" || !validSessionID.MatchString(sessionID) {
		return nil
	}

	return &Session{
		Tool:    "opencode",
		CWD:     cwd,
		Command: "opencode --session " + sessionID,
	}
}
