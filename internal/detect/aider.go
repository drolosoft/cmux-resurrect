package detect

import (
	"os"
	"path/filepath"
)

// aiderHistoryName is the file Aider appends every chat to. It has no
// sessions or ids: one history per repository is all there is to resume.
const aiderHistoryName = ".aider.chat.history.md"

// aiderHistoryEnv renames the history file. Aider reads each of its options
// from an AIDER_ variable too, and this one is --chat-history-file. Config
// files can rename it as well, but crex does not read them.
const aiderHistoryEnv = "AIDER_CHAT_HISTORY_FILE"

// detectAider relaunches Aider with --restore-chat-history when the chat it
// was having is still on disk, so the restored pane reloads the conversation.
// Without a history to reload it starts Aider empty, as before.
func detectAider(cwd, _ string) *Session {
	bare := &Session{Tool: "aider", CWD: cwd, Command: "aider"}

	// A relative or empty cwd would make the lookup depend on the directory
	// crex runs in, and pick up a history that is not Aider's.
	if !filepath.IsAbs(cwd) {
		return bare
	}

	history := aiderHistoryFile(cwd)
	if !hasContent(history) {
		return bare
	}

	return &Session{Tool: "aider", CWD: cwd, Command: "aider --restore-chat-history"}
}

// aiderHistoryFile returns where Aider running in cwd keeps its chat
// history: the file named by AIDER_CHAT_HISTORY_FILE when it is set, else
// .aider.chat.history.md at the root of the repository, or in cwd when there
// is no repository.
func aiderHistoryFile(cwd string) string {
	// Aider opens a relative name from the directory it runs in, not from the
	// repository root.
	if name := os.Getenv(aiderHistoryEnv); name != "" {
		if filepath.IsAbs(name) {
			return name
		}
		return filepath.Join(cwd, name)
	}

	root := repositoryRoot(cwd)
	if root == "" {
		root = cwd
	}

	return filepath.Join(root, aiderHistoryName)
}

// repositoryRoot walks up from dir to the first directory that holds a .git,
// and returns "" when there is none. A .git file counts too: that is how a
// linked worktree or a submodule marks its root, and Aider stops there. It
// does not run git, so it costs a few stats per pane.
func repositoryRoot(dir string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// hasContent reports whether path is a regular file with at least one byte.
// Aider writes a "chat started" line as soon as it opens the history, so an
// empty file was truncated or made by hand and there is nothing to restore.
func hasContent(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.Mode().IsRegular() && info.Size() > 0
}
