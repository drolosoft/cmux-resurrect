package detect

import (
	"os"
	"path/filepath"
	"testing"
)

// writeAiderFile creates path, and the directories above it, with content.
func writeAiderFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// makeGitDir marks dir as the root of a repository with a .git directory.
func makeGitDir(t *testing.T, dir string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// checkAiderCommand fails the test unless s is an aider session for cwd
// that starts with command.
func checkAiderCommand(t *testing.T, s *Session, cwd, command string) {
	t.Helper()

	if s == nil {
		t.Fatal("expected session, got nil")
	}
	if s.Tool != "aider" {
		t.Errorf("Tool = %q, want aider", s.Tool)
	}
	if s.CWD != cwd {
		t.Errorf("CWD = %q, want %q", s.CWD, cwd)
	}
	if s.Command != command {
		t.Errorf("Command = %q, want %q", s.Command, command)
	}
}

// clearAiderHistoryEnv empties AIDER_CHAT_HISTORY_FILE for the test, so a
// history renamed in the shell that runs the tests does not leak into the
// ones that expect the default name.
func clearAiderHistoryEnv(t *testing.T) {
	t.Helper()

	t.Setenv("AIDER_CHAT_HISTORY_FILE", "")
}

func TestDetectAider_HistoryAtGitRootFromSubdirectory(t *testing.T) {
	clearAiderHistoryEnv(t)
	root := t.TempDir()
	makeGitDir(t, root)
	writeAiderFile(t, filepath.Join(root, ".aider.chat.history.md"), "# aider chat started\n")

	cwd := filepath.Join(root, "internal", "detect")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}

	checkAiderCommand(t, detectAider(cwd, "123"), cwd, "aider --restore-chat-history")
}

func TestDetectAider_NoGitHistoryInCWD(t *testing.T) {
	clearAiderHistoryEnv(t)
	cwd := t.TempDir()
	writeAiderFile(t, filepath.Join(cwd, ".aider.chat.history.md"), "# aider chat started\n")

	checkAiderCommand(t, detectAider(cwd, ""), cwd, "aider --restore-chat-history")
}

func TestDetectAider_EmptyHistoryIsBare(t *testing.T) {
	clearAiderHistoryEnv(t)
	root := t.TempDir()
	makeGitDir(t, root)
	writeAiderFile(t, filepath.Join(root, ".aider.chat.history.md"), "")

	checkAiderCommand(t, detectAider(root, ""), root, "aider")
}

func TestDetectAider_NoHistoryIsBare(t *testing.T) {
	clearAiderHistoryEnv(t)
	root := t.TempDir()
	makeGitDir(t, root)

	checkAiderCommand(t, detectAider(root, ""), root, "aider")
}

func TestDetectAider_GitFileMarksWorktreeRoot(t *testing.T) {
	clearAiderHistoryEnv(t)

	// A linked worktree has a .git file pointing at the main repository.
	// Aider works in the worktree, so its history is there and not in a
	// repository further up.
	outer := t.TempDir()
	makeGitDir(t, outer)
	writeAiderFile(t, filepath.Join(outer, ".aider.chat.history.md"), "# outer chat\n")

	worktree := filepath.Join(outer, "worktrees", "feature")
	writeAiderFile(t, filepath.Join(worktree, ".git"), "gitdir: /elsewhere/.git/worktrees/feature\n")
	writeAiderFile(t, filepath.Join(worktree, ".aider.chat.history.md"), "# worktree chat\n")

	cwd := filepath.Join(worktree, "cmd")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}

	checkAiderCommand(t, detectAider(cwd, ""), cwd, "aider --restore-chat-history")

	// Without its own history the worktree starts empty, even though the
	// outer repository has one.
	if err := os.Remove(filepath.Join(worktree, ".aider.chat.history.md")); err != nil {
		t.Fatal(err)
	}
	checkAiderCommand(t, detectAider(cwd, ""), cwd, "aider")
}

func TestDetectAider_HistoryInCWDIgnoredInsideRepo(t *testing.T) {
	clearAiderHistoryEnv(t)
	root := t.TempDir()
	makeGitDir(t, root)

	cwd := filepath.Join(root, "docs")
	writeAiderFile(t, filepath.Join(cwd, ".aider.chat.history.md"), "# stray chat\n")

	checkAiderCommand(t, detectAider(cwd, ""), cwd, "aider")
}

func TestDetectAider_HistoryDirectoryIsBare(t *testing.T) {
	clearAiderHistoryEnv(t)
	cwd := t.TempDir()
	if err := os.Mkdir(filepath.Join(cwd, ".aider.chat.history.md"), 0o755); err != nil {
		t.Fatal(err)
	}

	checkAiderCommand(t, detectAider(cwd, ""), cwd, "aider")
}

func TestDetectAider_EnvRenamesHistory(t *testing.T) {
	root := t.TempDir()
	makeGitDir(t, root)
	cwd := filepath.Join(root, "app")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}

	// An absolute name is used as it is.
	absolute := filepath.Join(t.TempDir(), "chats", "project.md")
	writeAiderFile(t, absolute, "# renamed chat\n")
	t.Setenv("AIDER_CHAT_HISTORY_FILE", absolute)
	checkAiderCommand(t, detectAider(cwd, ""), cwd, "aider --restore-chat-history")

	// A relative name is opened from the directory Aider runs in, not from
	// the repository root.
	writeAiderFile(t, filepath.Join(cwd, "notes.md"), "# relative chat\n")
	t.Setenv("AIDER_CHAT_HISTORY_FILE", "notes.md")
	checkAiderCommand(t, detectAider(cwd, ""), cwd, "aider --restore-chat-history")

	// The renamed file decides on its own: a default history at the root
	// does not count when the variable points somewhere empty.
	writeAiderFile(t, filepath.Join(root, ".aider.chat.history.md"), "# default chat\n")
	t.Setenv("AIDER_CHAT_HISTORY_FILE", "missing.md")
	checkAiderCommand(t, detectAider(cwd, ""), cwd, "aider")
}

// The restore command must still read as an Aider launch, or auto_accept
// would stop adding --yes-always to it.
func TestDetectAider_RestoreCommandIsAiderLaunch(t *testing.T) {
	tool, launch, ok := ToolLaunch("aider --restore-chat-history")
	if !ok || tool != "aider" || launch != "aider" {
		t.Errorf("ToolLaunch = (%q, %q, %v), want (aider, aider, true)", tool, launch, ok)
	}
}

func TestDetectAider_RelativeCWDIsBare(t *testing.T) {
	clearAiderHistoryEnv(t)

	checkAiderCommand(t, detectAider("", ""), "", "aider")
}
