package detect

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// hermesTerminalSources are the sources Hermes Agent gives a conversation
// typed in an interactive terminal, as an SQL list: 'cli' for `hermes` or
// `hermes chat`, 'tui' for `hermes --tui` (seen with v0.23.1). The same
// table holds Telegram, Discord, cron, one-shot and gateway sessions, and
// none of those belongs to the terminal being restored.
const hermesTerminalSources = `'cli', 'tui'`

// hermesStartSlack widens the moment a process started when it is compared
// with a breadcrumb's time. ps gives the elapsed time in whole seconds, so
// the start it implies can be up to a second late, and Hermes writes the
// breadcrumb well under a second after it starts.
const hermesStartSlack = 2 * time.Second

// detectHermes finds the terminal session of a Hermes Agent (Nous Research)
// process and resumes it by id. The exact one is in the breadcrumb Hermes
// keeps for the process's terminal (see hermesTerminalSession). Without it,
// the latest terminal session recorded for cwd in <hermes home>/state.db
// (SQLite) stands for it, read with the sqlite3 CLI like OpenCode's, to
// avoid CGO dependencies. Without a session for cwd, or when the database
// cannot be read, Hermes starts fresh, as it did before crex could resume it.
func detectHermes(cwd, pid string) *Session {
	fresh := &Session{Tool: "hermes", CWD: cwd, Command: "hermes"}

	hermesHome, custom := hermesHomeDir(pid)
	if hermesHome == "" {
		return fresh
	}

	sessionID := hermesTerminalSession(hermesHome, pid, cwd)
	if sessionID == "" {
		sessionID = latestHermesSession(filepath.Join(hermesHome, "state.db"), cwd)
	}
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

	query := `SELECT id FROM sessions WHERE source IN (` + hermesTerminalSources + `) AND cwd = '` + escapeSQLite(cwd) + `' ORDER BY started_at DESC LIMIT 1;`
	out, err := queryHermesDB(dbPath, query)

	// Hermes keeps state.db in WAL mode and removes the -wal and -shm files
	// when its last connection closes. A read-only open cannot create the
	// -shm file again, so with no Hermes running it fails with error 14.
	// Everything is in the main file then, and an immutable open reads it as
	// it is. With a -wal file that holds pages, the main file may lack the
	// newest sessions (the lab saw exactly that while Hermes ran), so the
	// failure stands and Hermes starts fresh rather than in an older session.
	if err != nil && !hermesWALPending(dbPath) {
		immutable :=url.URL{Scheme: "file", Path: dbPath, RawQuery: "immutable=1"}
		out, err = queryHermesDB(immutable.String(), query)
	}
	if err != nil {
		return ""
	}

	sessionID := strings.TrimSpace(string(out))
	if !validSessionID.MatchString(sessionID) {
		return ""
	}
	return sessionID
}

// queryHermesDB runs one query on the Hermes database with the sqlite3 CLI,
// read-only. database is a path or a file: URI.
func queryHermesDB(database, query string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	return exec.CommandContext(ctx, "sqlite3", "-readonly", database, query).Output()
}

// hermesWALPending reports whether the database at dbPath has a -wal file
// with anything in it, which is to say pages the main file may not have.
// A file that cannot be checked counts as pending.
func hermesWALPending(dbPath string) bool {
	info, err := os.Stat(dbPath + "-wal")
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		return true
	}
	return info.Size() > 0
}

// hermesBreadcrumb is the file Hermes writes for a terminal under
// <hermes home>/terminal-sessions every time the session on it changes:
// {"session_id": ..., "cwd": ..., "ts": <Unix seconds>}. `hermes -c` reads
// it to continue the session of that terminal.
type hermesBreadcrumb struct {
	SessionID string  `json:"session_id"`
	CWD       string  `json:"cwd"`
	Written   float64 `json:"ts"`
}

// hermesTerminalSession returns the session Hermes recorded as the current
// one on the terminal of process pid, or "" when that is unknown. It is the
// exact session of that pane even when several Hermes share the directory.
// The breadcrumb only counts for cwd, and only when it was written after the
// process started: `hermes --tui` writes none (seen in the lab), so the file
// can still name the session of an earlier Hermes on the same terminal.
func hermesTerminalSession(hermesHome, pid, cwd string) string {
	tty, started, ok := processTerminal(pid)
	if !ok {
		return ""
	}

	data, err := os.ReadFile(filepath.Join(hermesHome, "terminal-sessions", hermesTerminalID(tty)))
	if err != nil {
		return ""
	}
	var crumb hermesBreadcrumb
	if json.Unmarshal(data, &crumb) != nil {
		return ""
	}
	if crumb.CWD != cwd || !validSessionID.MatchString(crumb.SessionID) {
		return ""
	}

	written := time.Unix(0, int64(crumb.Written*float64(time.Second)))
	if written.Before(started.Add(-hermesStartSlack)) {
		return ""
	}
	return crumb.SessionID
}

// hermesUnsafeName matches the characters Hermes replaces with "-" when it
// turns a terminal's identity into a file name.
var hermesUnsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// hermesTerminalID returns the name of the breadcrumb Hermes keeps for a
// terminal as ps names it: "tty-" and the device path without its leading
// "/", every character outside [A-Za-z0-9._-] as "-" (ttys002 is
// /dev/ttys002 and becomes tty-dev-ttys002, pts/3 becomes tty-dev-pts-3).
// This is get_terminal_id in hermes_cli/terminal_breadcrumbs.py of v0.23.1.
func hermesTerminalID(tty string) string {
	return "tty-" + hermesUnsafeName.ReplaceAllString("dev/"+tty, "-")
}

// processTerminal returns the terminal a process runs on, as ps names it,
// and when the process started. ok is false for a process with no terminal
// or that ps cannot report on. It is a variable so tests can stub it.
var processTerminal = readProcessTerminal

// readProcessTerminal asks ps for the terminal and the elapsed time of pid.
// The start time it gives is as precise as ps's whole seconds.
func readProcessTerminal(pid string) (tty string, started time.Time, ok bool) {
	if pid == "" {
		return "", time.Time{}, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ps", "-o", "tty=", "-o", "etime=", "-p", pid).Output()
	if err != nil {
		return "", time.Time{}, false
	}

	fields := strings.Fields(string(out))
	if len(fields) != 2 {
		return "", time.Time{}, false
	}
	tty = TTYName(fields[0])
	elapsed, valid := parseElapsed(fields[1])
	if tty == "" || !valid {
		return "", time.Time{}, false
	}
	return tty, time.Now().Add(-elapsed), true
}

// parseElapsed reads the elapsed time ps prints for a process,
// [[dd-]hh:]mm:ss on both macOS and Linux. ok is false for anything else.
func parseElapsed(value string) (elapsed time.Duration, ok bool) {
	days := 0
	if before, after, found := strings.Cut(value, "-"); found {
		count, err := strconv.Atoi(before)
		if err != nil {
			return 0, false
		}
		days = count
		value = after
	}

	parts := strings.Split(value, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}

	seconds := 0
	for _, part := range parts {
		number, err := strconv.Atoi(part)
		if err != nil {
			return 0, false
		}
		seconds = seconds*60 + number
	}
	return time.Duration(days)*24*time.Hour + time.Duration(seconds)*time.Second, true
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
