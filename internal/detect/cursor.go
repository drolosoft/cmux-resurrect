package detect

import (
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// cursorCommand starts the Cursor CLI. The installer links it as both
// `agent` and `cursor-agent`; `agent` is the name the registry launches, so
// a resumed chat starts the same binary a fresh one would.
const cursorCommand = "agent"

// cursorStore is the SQLite file that holds one chat. A chat folder without
// it has nothing `agent --resume` could reopen.
const cursorStore = "store.db"

// detectCursor finds the chat a Cursor CLI process was having in cwd. The
// CLI files each chat as <config>/chats/<md5 of the directory>/<chat id>/
// with the conversation in store.db, and `agent --resume <chat id>` reopens
// it; config is ~/.cursor unless the process environment moves it (see
// cursorConfigDir). Cursor does not document this layout; it comes from
// entireio/cli, which reads the same store, and from the CLI's own bundle.
// When no chat belongs to the directory, Cursor is relaunched fresh, as crex
// did before it read the chats.
func detectCursor(cwd, pid string) *Session {
	session := &Session{Tool: "cursor", CWD: cwd, Command: cursorCommand}

	configDir, custom := cursorConfigDir(pid)
	if configDir == "" {
		return session
	}
	chatsDir := filepath.Join(configDir, "chats")

	// The process does not say which chat it has open, so the chat written
	// last in the directory stands for it. Two Cursors in one directory
	// both resume that one.
	var chatID string
	var lastWrite time.Time
	for _, key := range cursorWorkspaceKeys(cwd) {
		id, written := latestCursorChat(filepath.Join(chatsDir, key))
		if id != "" && written.After(lastWrite) {
			chatID = id
			lastWrite = written
		}
	}
	if chatID == "" {
		return session
	}

	session.Command = cursorResumeCommand(configDir, custom, chatID)
	return session
}

// cursorConfigDir returns the folder a Cursor process keeps its chats in,
// resolved the way the CLI 2026.11.05 does it (cursor-config/dist/paths.js):
// $CURSOR_CONFIG_DIR, else $XDG_CONFIG_HOME/cursor, else ~/.cursor, where a
// variable that holds only blanks counts as unset. custom reports whether it
// differs from ~/.cursor, in which case the resume command must name it
// again. It returns "" when the folder cannot be known.
func cursorConfigDir(pid string) (dir string, custom bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	defaultDir := filepath.Join(home, ".cursor")

	env := cursorEnvironment(pid)
	dir = defaultDir
	if configured := env["CURSOR_CONFIG_DIR"]; strings.TrimSpace(configured) != "" {
		dir = configured
	} else if xdg := env["XDG_CONFIG_HOME"]; strings.TrimSpace(xdg) != "" {
		dir = filepath.Join(xdg, "cursor")
	}

	// Cursor would read a relative value from its own directory, which crex
	// cannot be sure of. No chat is better than one from another folder.
	if !filepath.IsAbs(dir) {
		return "", false
	}

	dir = filepath.Clean(dir)
	return dir, dir != defaultDir
}

// cursorEnvironment returns the variables Cursor chooses its folder from: the
// environment of the process when it can be read, and crex's own otherwise,
// since both usually come from the same login shell.
func cursorEnvironment(pid string) map[string]string {
	if env := processEnv(pid); env != nil {
		return env
	}

	return map[string]string{
		"CURSOR_CONFIG_DIR": os.Getenv("CURSOR_CONFIG_DIR"),
		"XDG_CONFIG_HOME":   os.Getenv("XDG_CONFIG_HOME"),
	}
}

// cursorResumeCommand builds the command that reopens a Cursor chat. When the
// chat lives outside ~/.cursor it sets CURSOR_CONFIG_DIR to that folder, even
// if the process found it through XDG_CONFIG_HOME: the variable names the
// folder itself, wins over XDG_CONFIG_HOME in the restored shell, and leaves
// the other programs that read XDG_CONFIG_HOME alone.
func cursorResumeCommand(configDir string, custom bool, chatID string) string {
	cmd := cursorCommand + " --resume " + chatID
	if custom {
		cmd = "CURSOR_CONFIG_DIR=" + shellQuote(configDir) + " " + cmd
	}
	return cmd
}

// cursorWorkspaceKeys returns the folder names Cursor may have filed the
// chats of cwd under. Cursor hashes the directory as it saw it, which is
// usually the path with its links resolved (/tmp is /private/tmp on macOS),
// while crex may read the linked spelling, so both are tried.
func cursorWorkspaceKeys(cwd string) []string {
	keys := []string{cursorWorkspaceKey(cwd)}

	resolved, err := filepath.EvalSymlinks(cwd)
	if err == nil && resolved != cwd {
		keys = append(keys, cursorWorkspaceKey(resolved))
	}
	return keys
}

// cursorWorkspaceKey is the folder name Cursor gives a directory under
// <config>/chats: the hex MD5 of its path. MD5 is Cursor's naming, not a
// security measure.
func cursorWorkspaceKey(path string) string {
	sum := md5.Sum([]byte(path))
	return hex.EncodeToString(sum[:])
}

// latestCursorChat returns the id of the chat in a workspace folder that was
// written last, and when. The folder name is the chat id; it is checked
// because it ends up in a shell command. Returns "" when the folder holds
// no chat.
func latestCursorChat(workspaceDir string) (chatID string, written time.Time) {
	entries, err := os.ReadDir(workspaceDir)
	if err != nil {
		return "", time.Time{}
	}

	for _, entry := range entries {
		if !entry.IsDir() || !validSessionID.MatchString(entry.Name()) {
			continue
		}
		modified := cursorChatWritten(filepath.Join(workspaceDir, entry.Name()))
		if modified.After(written) {
			chatID = entry.Name()
			written = modified
		}
	}
	return chatID, written
}

// cursorChatWritten returns when a chat was last written: the newest time
// among the files in its folder. SQLite writes into store.db and its -wal
// file in place, which leaves the folder's own time at the moment those
// files were created, so the folder alone would date a long chat by its
// first message. A folder without store.db gets the zero time.
func cursorChatWritten(chatDir string) time.Time {
	entries, err := os.ReadDir(chatDir)
	if err != nil {
		return time.Time{}
	}

	var written time.Time
	hasStore := false
	for _, entry := range entries {
		if entry.Name() == cursorStore {
			hasStore = true
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(written) {
			written = info.ModTime()
		}
	}
	if !hasStore {
		return time.Time{}
	}
	return written
}
