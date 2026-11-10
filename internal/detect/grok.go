package detect

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// detectGrok finds the session of a grok process. Two unrelated tools
// install themselves as ~/.grok/bin/grok: xAI's Grok Build, which keeps one
// directory per session under <grok home>/sessions/, and the community
// grok-cli (superagent-ai/grok-cli), which keeps a SQLite database at
// ~/.grok/grok.db.
//
// Grok Build lists every open TUI with its PID in active_sessions.json,
// which names the exact conversation. Without an entry, the most recent
// session of the directory is used, from whichever store was written last.
// When Grok Build is in use but nothing can be read for the directory,
// `grok --continue` lets Grok choose.
func detectGrok(cwd, pid string) *Session {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	grokHome, custom := grokHomeDir(home, pid)
	if grokHome == "" {
		return nil
	}

	if sessionID := grokActiveSession(grokHome, pid, cwd); sessionID != "" {
		return &Session{Tool: "grok", CWD: cwd, Command: grokCommand(grokHome, custom, "--resume "+sessionID)}
	}

	buildID, buildActivity := latestGrokBuildSession(grokHome, cwd)
	cliID, cliActivity := latestGrokCLISession(home, cwd)

	// The tool that is running is the one that wrote last, so an older
	// session of the other one never wins.
	if buildID != "" && !cliActivity.After(buildActivity) {
		return &Session{Tool: "grok", CWD: cwd, Command: grokCommand(grokHome, custom, "--resume "+buildID)}
	}
	if cliID != "" {
		return &Session{Tool: "grok", CWD: cwd, Command: "grok --session " + cliID}
	}

	if grokBuildInUse(grokHome) {
		return &Session{Tool: "grok", CWD: cwd, Command: grokCommand(grokHome, custom, "--continue")}
	}
	return nil
}

// grokHomeDir returns the Grok Build home a process uses: $GROK_HOME from
// the process environment when set, ~/.grok otherwise (resolve_grok_home_from
// in crates/codegen/xai-dirs/src/lib.rs). custom reports whether it differs
// from the default, in which case the resume command must set it again.
// Grok takes a relative GROK_HOME as is, relative to wherever it runs, so
// crex can't know which directory it means and returns "".
func grokHomeDir(home, pid string) (dir string, custom bool) {
	defaultHome := filepath.Join(home, ".grok")

	value := processEnv(pid)["GROK_HOME"]
	if value == "" {
		return defaultHome, false
	}
	if !filepath.IsAbs(value) {
		return "", false
	}

	value = filepath.Clean(value)
	return value, value != defaultHome
}

// grokActiveSession returns the session a running Grok Build TUI registered
// in <grok home>/active_sessions.json, the exact conversation of that
// process even when several run in one directory. Returns "" when the PID
// has no entry, the entry belongs to another directory, or the session has
// no summary.json yet: `grok --resume` only finds sessions that have one.
func grokActiveSession(grokHome, pid, cwd string) string {
	if pid == "" {
		return ""
	}

	data, err := os.ReadFile(filepath.Join(grokHome, "active_sessions.json"))
	if err != nil {
		return ""
	}

	// ActiveSession in crates/codegen/xai-grok-active-sessions/src/lib.rs;
	// opened_at is left out because nothing here needs it.
	var entries []struct {
		SessionID string `json:"session_id"`
		PID       int    `json:"pid"`
		CWD       string `json:"cwd"`
	}
	if json.Unmarshal(data, &entries) != nil {
		return ""
	}

	cwdDir := grokSessionsCWDDir(grokHome, cwd)
	if cwdDir == "" {
		return ""
	}

	for _, entry := range entries {
		if strconv.Itoa(entry.PID) != pid || entry.CWD != cwd || !validSessionID.MatchString(entry.SessionID) {
			continue
		}
		if _, err := os.Stat(filepath.Join(cwdDir, entry.SessionID, "summary.json")); err == nil {
			return entry.SessionID
		}
	}
	return ""
}

// grokSessionSummary holds the summary.json fields that decide which
// session Grok Build's own --continue picks (Summary in
// crates/codegen/xai-grok-shell/src/session/persistence.rs).
type grokSessionSummary struct {
	SessionSummary  string     `json:"session_summary"`
	GeneratedTitle  string     `json:"generated_title"`
	UpdatedAt       time.Time  `json:"updated_at"`
	LastActiveAt    *time.Time `json:"last_active_at"`
	NumMessages     int        `json:"num_messages"`
	SessionKind     string     `json:"session_kind"`
	Hidden          *bool      `json:"hidden"`
	ParentSessionID string     `json:"parent_session_id"`
	ForkedAt        *time.Time `json:"forked_at"`
}

// continuable reports whether Grok Build's --continue would consider the
// session. It skips hidden sessions (subagents are hidden unless they say
// otherwise), one-shot `grok -p` sessions, and the husks the TUI leaves
// when it opens without a message or a title, unless they come from a fork.
func (summary grokSessionSummary) continuable() bool {
	hidden := strings.HasPrefix(summary.SessionKind, "subagent")
	if summary.Hidden != nil {
		hidden = *summary.Hidden
	}
	if hidden || summary.SessionKind == "headless" {
		return false
	}

	forked := summary.SessionKind == "fork" || summary.ParentSessionID != "" || summary.ForkedAt != nil
	if forked {
		return true
	}

	title := strings.TrimSpace(summary.GeneratedTitle)
	if title == "" {
		title = strings.TrimSpace(summary.SessionSummary)
	}
	return summary.NumMessages > 0 || title != ""
}

// lastActivity returns the time Grok Build sorts sessions by: when content
// was last added, or the last write of any kind for sessions that predate
// that field.
func (summary grokSessionSummary) lastActivity() time.Time {
	if summary.LastActiveAt != nil {
		return *summary.LastActiveAt
	}
	return summary.UpdatedAt
}

// latestGrokBuildSession returns the Grok Build session that `grok
// --continue` would open in cwd, and when it last changed. Like Grok, a
// tie goes to the smaller ID. Returns "" when the directory has none.
func latestGrokBuildSession(grokHome, cwd string) (sessionID string, activity time.Time) {
	cwdDir := grokSessionsCWDDir(grokHome, cwd)
	if cwdDir == "" {
		return "", time.Time{}
	}

	entries, err := os.ReadDir(cwdDir)
	if err != nil {
		return "", time.Time{}
	}

	for _, entry := range entries {
		if !entry.IsDir() || !validSessionID.MatchString(entry.Name()) {
			continue
		}

		data, err := os.ReadFile(filepath.Join(cwdDir, entry.Name(), "summary.json"))
		if err != nil {
			continue
		}
		var summary grokSessionSummary
		if json.Unmarshal(data, &summary) != nil || !summary.continuable() {
			continue
		}

		changed := summary.lastActivity()
		newer := changed.After(activity)
		tieWon := changed.Equal(activity) && entry.Name() < sessionID
		if sessionID == "" || newer || tieWon {
			sessionID = entry.Name()
			activity = changed
		}
	}
	return sessionID, activity
}

// grokMaxDirName is the longest file name APFS, ext4 and NTFS accept. Grok
// Build names a directory's session folder after its URL-encoded path up to
// this length and switches to a hashed name past it.
const grokMaxDirName = 255

// grokSessionsCWDDir returns the folder that holds the Grok Build sessions
// of cwd: <grok home>/sessions/<URL-encoded cwd>, as encode_cwd_dirname in
// crates/codegen/xai-grok-config/src/paths.rs builds it. A long path gets a
// <slug>-<blake3 prefix> name instead; rather than hash it, crex finds that
// folder by the .cwd file Grok writes inside it. Returns "" when there is
// no such folder.
func grokSessionsCWDDir(grokHome, cwd string) string {
	sessionsDir := filepath.Join(grokHome, "sessions")

	encoded := grokEncodeCWD(cwd)
	if len(encoded) <= grokMaxDirName {
		return filepath.Join(sessionsDir, encoded)
	}

	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(sessionsDir, entry.Name())
		recorded, err := os.ReadFile(filepath.Join(dir, ".cwd"))
		if err == nil && strings.TrimSpace(string(recorded)) == cwd {
			return dir
		}
	}
	return ""
}

// grokEncodeCWD URL-encodes a path the way the Rust urlencoding crate that
// Grok Build uses does: ASCII letters, digits and -._~ stay, every other
// byte becomes %XX in upper case. Go's escapers keep other characters
// (PathEscape leaves ":" and "@", QueryEscape turns a space into "+"), so
// they would name a different folder.
func grokEncodeCWD(cwd string) string {
	var encoded strings.Builder
	for i := 0; i < len(cwd); i++ {
		c := cwd[i]
		switch {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
			encoded.WriteByte(c)
		case c == '-' || c == '.' || c == '_' || c == '~':
			encoded.WriteByte(c)
		default:
			fmt.Fprintf(&encoded, "%%%02X", c)
		}
	}
	return encoded.String()
}

// grokBuildInUse reports whether Grok Build has written anything to its
// home, which tells it apart from a grok-cli install that only has grok.db.
func grokBuildInUse(grokHome string) bool {
	for _, name := range []string{"active_sessions.json", "sessions"} {
		if _, err := os.Stat(filepath.Join(grokHome, name)); err == nil {
			return true
		}
	}
	return false
}

// latestGrokCLISession returns the most recently updated session the
// community grok-cli recorded for cwd, and when. grok-cli always keeps its
// database at ~/.grok/grok.db and ignores GROK_HOME (src/storage/db.ts);
// the sessions table and its cwd_last column, the directory a session last
// ran in, come from src/storage/migrations.ts. We shell out to the sqlite3
// CLI (ships with macOS) to avoid CGO dependencies, as for OpenCode.
func latestGrokCLISession(home, cwd string) (sessionID string, updatedAt time.Time) {
	dbPath := filepath.Join(home, ".grok", "grok.db")
	if _, err := os.Stat(dbPath); err != nil {
		return "", time.Time{}
	}

	query := `SELECT id, updated_at FROM sessions WHERE cwd_last = '` + escapeSQLite(cwd) + `' ORDER BY updated_at DESC LIMIT 1;`
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "sqlite3", "-readonly", dbPath, query).Output()
	if err != nil {
		return "", time.Time{}
	}

	// sqlite3 separates columns with "|"; updated_at is an ISO 8601 string
	// from JavaScript's toISOString.
	sessionID, updated, _ := strings.Cut(strings.TrimSpace(string(out)), "|")
	if !validSessionID.MatchString(sessionID) {
		return "", time.Time{}
	}
	updatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return sessionID, updatedAt
}

// grokCommand builds a Grok Build command with the given arguments,
// re-setting GROK_HOME when the sessions live outside ~/.grok so Grok looks
// for them in the right place.
func grokCommand(grokHome string, custom bool, args string) string {
	command := "grok " + args
	if custom {
		command = "GROK_HOME=" + shellQuote(grokHome) + " " + command
	}
	return command
}
