package detect

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// hermesTerminalSource is the source Hermes Agent gives a conversation typed
// in an interactive terminal (`hermes` or `hermes chat`). The same table
// holds Telegram, Discord, cron, one-shot and gateway sessions, and none of
// those belongs to the terminal being restored.
const hermesTerminalSource = "cli"

// detectHermes finds the latest terminal session Hermes Agent (Nous
// Research) recorded for cwd and resumes it by id. Hermes keeps every
// session in <hermes home>/state.db (SQLite), which we read with the sqlite3
// CLI like OpenCode's, to avoid CGO dependencies. Without a database or a
// session for cwd, Hermes starts fresh, as it did before crex could resume it.
func detectHermes(cwd, pid string) *Session {
	fresh := &Session{Tool: "hermes", CWD: cwd, Command: "hermes"}

	hermesHome, custom := hermesHomeDir(pid)
	if hermesHome == "" {
		return fresh
	}

	sessionID := latestHermesSession(filepath.Join(hermesHome, "state.db"), cwd)
	if sessionID == "" {
		return fresh
	}

	return &Session{
		Tool:    "hermes",
		CWD:     cwd,
		Command: hermesResumeCommand(hermesHome, custom, sessionID),
	}
}

// hermesHomeDir returns the directory a Hermes process keeps its state in:
// $HERMES_HOME from the process environment when set, ~/.hermes otherwise.
// A named profile (`hermes -p coder`) is a directory of its own under
// ~/.hermes/profiles with its own state.db, and Hermes selects it through
// HERMES_HOME. custom reports whether it differs from the default, in which
// case the resume command must set it again.
func hermesHomeDir(pid string) (dir string, custom bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	def := filepath.Join(home, ".hermes")

	env := processEnv(pid)
	v := env["HERMES_HOME"]
	if v == "" || !filepath.IsAbs(v) {
		return def, false
	}
	v = filepath.Clean(v)
	return v, v != def
}

// latestHermesSession returns the id of the most recently started terminal
// session for cwd in the Hermes database at dbPath, or "" when there is
// none or the database can't be read. A session that context compression
// split keeps its source and directory, and the newest part is the one to
// continue.
func latestHermesSession(dbPath, cwd string) string {
	if _, err := os.Stat(dbPath); err != nil {
		return ""
	}

	query := `SELECT id FROM sessions WHERE source = '` + hermesTerminalSource + `' AND cwd = '` + escapeSQLite(cwd) + `' ORDER BY started_at DESC LIMIT 1;`
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "sqlite3", "-readonly", dbPath, query).Output()
	if err != nil {
		return ""
	}

	sessionID := strings.TrimSpace(string(out))
	if !validSessionID.MatchString(sessionID) {
		return ""
	}
	return sessionID
}

// hermesResumeCommand builds the command that reopens a Hermes session,
// prefixed with HERMES_HOME when the session lives outside ~/.hermes:
// without it the restored shell would look for the id in the wrong
// database.
func hermesResumeCommand(hermesHome string, custom bool, sessionID string) string {
	cmd := "hermes --resume " + sessionID
	if custom {
		cmd = "HERMES_HOME=" + shellQuote(hermesHome) + " " + cmd
	}
	return cmd
}
