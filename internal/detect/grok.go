package detect

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// detectGrok finds the active session for a Grok Build CLI instance.
// Sessions are stored in ~/.grok/grok.db (SQLite).
// Uses `grok --continue` which auto-selects the latest session by CWD.
func detectGrok(cwd, _ string) *Session {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	dbPath := filepath.Join(home, ".grok", "grok.db")
	if _, err := os.Stat(dbPath); err != nil {
		return nil
	}

	query := `SELECT id FROM sessions WHERE cwd = '` + escapeSQLite(cwd) + `' ORDER BY updated_at DESC LIMIT 1;`
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "sqlite3", "-readonly", dbPath, query).Output()
	if err != nil {
		// SQLite unavailable — fall back to --continue.
		return &Session{Tool: "grok", CWD: cwd, Command: "grok --continue"}
	}

	sessionID := strings.TrimSpace(string(out))
	if sessionID == "" {
		return nil
	}

	return &Session{Tool: "grok", CWD: cwd, Command: "grok --continue"}
}
