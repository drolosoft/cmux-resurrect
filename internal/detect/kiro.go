package detect

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// kiroCommand starts Kiro CLI from a shell. With no subcommand kiro-cli opens
// a chat, so a directory without a session gets back exactly what crex typed
// before it could resume Kiro.
const kiroCommand = "kiro-cli"

// kiroResumeByID reopens one session by its id. Under 3.x it also takes the
// id of a 2.x session and opens a 3.x copy of it (kiro.dev/docs/cli/v3/), so
// an id from either generation of stores is worth typing.
const kiroResumeByID = "kiro-cli chat --resume-id "

// kiroResumeLatest reopens the newest session of the directory the shell is
// in. It stands in when a session for cwd is on disk but its id is not safe
// to type into a terminal.
const kiroResumeLatest = "kiro-cli chat --resume"

// kiroV3SessionPrefix starts the name of every 3.x session folder. The
// workspace folders around them have names Kiro does not document, so only
// this prefix tells a session apart.
const kiroV3SessionPrefix = "sess_"

// kiroTimeLayouts are the forms Kiro's JSON timestamps take: RFC 3339 with a
// zone in 3.x, and the same without one in the 2.x classic files, which
// time.Parse then reads as UTC. Fractional seconds parse with either.
var kiroTimeLayouts = []string{time.RFC3339, "2006-01-02T15:04:05"}

// kiroSession is one session a Kiro store holds for a directory. id is empty
// when the session is there but its id is not safe to type.
type kiroSession struct {
	found   bool
	id      string
	updated time.Time
}

// newerThan reports whether s should replace other as the session to resume:
// it exists, and other does not or was updated earlier. On a tie the session
// already chosen stays.
func (s kiroSession) newerThan(other kiroSession) bool {
	if !s.found {
		return false
	}
	return !other.found || s.updated.After(other.updated)
}

// detectKiro finds the latest Kiro CLI (AWS, formerly Amazon Q Developer CLI)
// session opened in cwd and resumes it by id.
//
// Kiro keeps sessions in a different store per version, and an upgrade
// leaves the old one on disk, so all of them are read and the session
// updated last wins:
//
//   - 3.x, the default from Kiro CLI 3.0.0 on: one folder per session at
//     ~/.kiro/sessions/<workspace hash>/sess_*/session.json.
//   - 2.x classic mode: one <id>.json per session in ~/.kiro/sessions/cli/.
//   - 2.x: the conversations_v2 table of data.sqlite3 in the user data
//     folder, read with the sqlite3 CLI like OpenCode's, to avoid CGO
//     dependencies.
//
// Kiro is closed source and documents none of these files; the layouts come
// from the reader mxmehl/kiro-cli-history (see kiro_test.go). With no session
// for cwd, Kiro starts fresh, as it did before crex could resume it.
func detectKiro(cwd, pid string) *Session {
	session := &Session{Tool: "kiro", CWD: cwd, Command: kiroCommand}

	home, err := os.UserHomeDir()
	if err != nil {
		return session
	}

	// The order breaks ties between stores: the newer format first, as the
	// 3.x copy of a 2.x session is the one Kiro reopens.
	sessionsDir := filepath.Join(home, ".kiro", "sessions")
	candidates := []kiroSession{
		newestKiroV3Session(sessionsDir, cwd),
		newestKiroClassicSession(filepath.Join(sessionsDir, "cli"), cwd),
		newestKiroSQLiteSession(kiroDatabasePath(runtime.GOOS, home, pid), cwd),
	}

	var newest kiroSession
	for _, candidate := range candidates {
		if candidate.newerThan(newest) {
			newest = candidate
		}
	}

	switch {
	case !newest.found:
		return session
	case newest.id == "":
		session.Command = kiroResumeLatest
	default:
		session.Command = kiroResumeByID + newest.id
	}
	return session
}

// newestKiroV3Session returns the newest 3.x session whose workspace is cwd.
// The workspace hash in the path is undocumented, so every hash folder is
// walked and each session.json says which directory it belongs to.
func newestKiroV3Session(sessionsDir, cwd string) kiroSession {
	var newest kiroSession

	workspaces, err := os.ReadDir(sessionsDir)
	if err != nil {
		return newest
	}

	for _, workspace := range workspaces {
		if !workspace.IsDir() {
			continue
		}

		workspaceDir := filepath.Join(sessionsDir, workspace.Name())
		folders, err := os.ReadDir(workspaceDir)
		if err != nil {
			continue
		}

		for _, folder := range folders {
			if !folder.IsDir() || !strings.HasPrefix(folder.Name(), kiroV3SessionPrefix) {
				continue
			}

			candidate := readKiroV3Session(filepath.Join(workspaceDir, folder.Name(), "session.json"), cwd)
			if candidate.newerThan(newest) {
				newest = candidate
			}
		}
	}
	return newest
}

// readKiroV3Session reads one 3.x session.json and returns it when its
// workspace is cwd. A file that is missing or not valid JSON, as when Kiro is
// still writing it, is skipped rather than guessed at.
//
// Only the first of workspacePaths counts, as in kiro-cli-history: it is the
// folder the session was started in, which is what lsof reports for the
// process. A terminal in one of the other folders did not start that session,
// and giving it the session would resume one conversation in two panes.
func readKiroV3Session(path, cwd string) kiroSession {
	data, err := os.ReadFile(path)
	if err != nil {
		return kiroSession{}
	}

	var metadata struct {
		ID             string   `json:"id"`
		WorkspacePaths []string `json:"workspacePaths"`
		LastModifiedAt string   `json:"lastModifiedAt"`
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return kiroSession{}
	}
	if len(metadata.WorkspacePaths) == 0 || filepath.Clean(metadata.WorkspacePaths[0]) != cwd {
		return kiroSession{}
	}

	return kiroSession{
		found:   true,
		id:      safeKiroID(metadata.ID),
		updated: kiroTimestamp(metadata.LastModifiedAt, path),
	}
}

// newestKiroClassicSession returns the newest 2.x classic-mode session whose
// cwd is cwd. Each session is an <id>.json next to an <id>.jsonl with the
// messages; only the .json files hold the metadata.
func newestKiroClassicSession(cliDir, cwd string) kiroSession {
	var newest kiroSession

	entries, err := os.ReadDir(cliDir)
	if err != nil {
		return newest
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		candidate := readKiroClassicSession(filepath.Join(cliDir, entry.Name()), cwd)
		if candidate.newerThan(newest) {
			newest = candidate
		}
	}
	return newest
}

// readKiroClassicSession reads one 2.x classic-mode <id>.json and returns it
// when it was opened in cwd, skipping a file that is missing or not valid
// JSON.
func readKiroClassicSession(path, cwd string) kiroSession {
	data, err := os.ReadFile(path)
	if err != nil {
		return kiroSession{}
	}

	var metadata struct {
		SessionID string `json:"session_id"`
		CWD       string `json:"cwd"`
		UpdatedAt string `json:"updated_at"`
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return kiroSession{}
	}
	if metadata.CWD == "" || filepath.Clean(metadata.CWD) != cwd {
		return kiroSession{}
	}

	return kiroSession{
		found:   true,
		id:      safeKiroID(metadata.SessionID),
		updated: kiroTimestamp(metadata.UpdatedAt, path),
	}
}

// newestKiroSQLiteSession returns the newest 2.x session for cwd in the
// conversations_v2 table of the database at dbPath, whose key column is the
// directory and whose updated_at is in milliseconds. A missing database, or
// one from 1.x without that table, gives no session.
func newestKiroSQLiteSession(dbPath, cwd string) kiroSession {
	if _, err := os.Stat(dbPath); err != nil {
		return kiroSession{}
	}

	query := `SELECT conversation_id, updated_at FROM conversations_v2 WHERE key = '` + escapeSQLite(cwd) + `' ORDER BY updated_at DESC LIMIT 1;`
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "sqlite3", "-readonly", dbPath, query).Output()
	if err != nil {
		return kiroSession{}
	}

	// The timestamp is cut from the end, because the id is the column that
	// could itself contain the separator.
	row := strings.TrimSpace(string(out))
	separator := strings.LastIndex(row, "|")
	if separator < 0 {
		return kiroSession{}
	}
	millis, err := strconv.ParseInt(row[separator+1:], 10, 64)
	if err != nil {
		return kiroSession{}
	}

	return kiroSession{
		found:   true,
		id:      safeKiroID(row[:separator]),
		updated: time.UnixMilli(millis),
	}
}

// kiroDatabasePath returns where Kiro CLI 2.x keeps data.sqlite3 on goos: the
// user data folder that the Rust dirs crate's data_local_dir() gives, under
// kiro-cli. On macOS that is ~/Library/Application Support; elsewhere it is
// $XDG_DATA_HOME, or ~/.local/share when that is unset or not absolute, as
// the XDG spec asks. XDG_DATA_HOME is read from the Kiro process, which is
// what Kiro saw, and from crex's own environment when that can't be read.
func kiroDatabasePath(goos, home, pid string) string {
	if goos == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "kiro-cli", "data.sqlite3")
	}

	env := processEnv(pid)
	dataHome := env["XDG_DATA_HOME"]
	if env == nil {
		dataHome = os.Getenv("XDG_DATA_HOME")
	}
	if !filepath.IsAbs(dataHome) {
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "kiro-cli", "data.sqlite3")
}

// safeKiroID returns id when it is safe to type as the resume argument, and
// "" otherwise, which makes the caller fall back to --resume.
func safeKiroID(id string) string {
	if !validSessionID.MatchString(id) {
		return ""
	}
	return id
}

// kiroTimestamp parses a timestamp from a Kiro session file. When the field
// is missing or in a form nobody has seen, the file's modification time
// stands in: Kiro saves the session on every turn
// (kiro.dev/docs/reference/cli-commands/), so it still orders the sessions.
func kiroTimestamp(value, path string) time.Time {
	for _, layout := range kiroTimeLayouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
