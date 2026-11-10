package detect

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The fixtures copy the layout the Cursor CLI writes, which Cursor does not
// document. It comes from entireio/cli: the merged reader in
// cmd/entire/cli/agent/cursor/images.go (ChatsBaseDir, ChatsWorkspaceKey) and
// cmd/entire/cli/agentimport/cursor_chats.go, and the table in the closed PR
// 1063 (docs/architecture/cursor-ide-archive.md):
//
//	~/.cursor/chats/<md5 of the workspace path>/<chat id>/store.db
//
// The resume flag is `--resume [chatId]` in
// https://cursor.com/docs/cli/reference/parameters.

// writeCursorChat files a chat for workspace the way the Cursor CLI does and
// dates the chat folder and its store.db at modified.
func writeCursorChat(t *testing.T, home, workspace, chatID string, modified time.Time) {
	t.Helper()

	chatDir := filepath.Join(home, ".cursor", "chats", cursorWorkspaceKey(workspace), chatID)
	if err := os.MkdirAll(chatDir, 0o755); err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(chatDir, "store.db")
	if err := os.WriteFile(store, []byte("SQLite format 3\x00"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{store, chatDir} {
		if err := os.Chtimes(path, modified, modified); err != nil {
			t.Fatal(err)
		}
	}
}

// The workspace key is the hex MD5 of the path, as `md5 -s /src/app` prints
// it. Pinning one value keeps the hash spelling from drifting.
func TestCursorWorkspaceKey(t *testing.T) {
	if got := cursorWorkspaceKey("/src/app"); got != "c7344a2fb68e27bcfcb167d33a28ad5d" {
		t.Errorf("cursorWorkspaceKey(/src/app) = %q", got)
	}
}

// A chat filed under the MD5 of the directory is resumed by its folder name.
func TestDetectCursor_FoundByWorkspaceHash(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeCursorChat(t, home, "/src/app", "0b9c1f4e-7a2d-4c3b-9e8f-1a2b3c4d5e6f", time.Now())

	s := detectCursor("/src/app", "")
	want := Session{Tool: "cursor", CWD: "/src/app", Command: "agent --resume 0b9c1f4e-7a2d-4c3b-9e8f-1a2b3c4d5e6f"}
	if s == nil || *s != want {
		t.Fatalf("detectCursor = %+v, want %+v", s, want)
	}
}

// Cursor hashes the directory with its links resolved, so a pane whose cwd
// is a link still finds the chat filed under the real path.
func TestDetectCursor_SymlinkedCwd(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	real := filepath.Join(t.TempDir(), "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(link)
	if err != nil {
		t.Fatal(err)
	}
	writeCursorChat(t, home, resolved, "linked-chat", time.Now())

	s := detectCursor(link, "")
	if s == nil || s.Command != "agent --resume linked-chat" || s.CWD != link {
		t.Fatalf("detectCursor(link) = %+v, want agent --resume linked-chat in %s", s, link)
	}
}

// With several chats in one directory, the one written last is resumed.
func TestDetectCursor_NewestOfTwoChats(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	now := time.Now()
	writeCursorChat(t, home, "/src/app", "newer-chat", now)
	writeCursorChat(t, home, "/src/app", "older-chat", now.Add(-time.Hour))

	s := detectCursor("/src/app", "")
	if s == nil || s.Command != "agent --resume newer-chat" {
		t.Fatalf("detectCursor = %+v, want agent --resume newer-chat", s)
	}
}

// A chat that keeps being written moves its store.db forward even when the
// folder itself was created long ago; the store's time is what counts.
func TestDetectCursor_StoreTimeBeatsFolderTime(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	now := time.Now()
	writeCursorChat(t, home, "/src/app", "long-running", now.Add(-2*time.Hour))
	writeCursorChat(t, home, "/src/app", "short-lived", now.Add(-time.Hour))

	store := filepath.Join(home, ".cursor", "chats", cursorWorkspaceKey("/src/app"), "long-running", "store.db")
	if err := os.Chtimes(store, now, now); err != nil {
		t.Fatal(err)
	}

	s := detectCursor("/src/app", "")
	if s == nil || s.Command != "agent --resume long-running" {
		t.Fatalf("detectCursor = %+v, want agent --resume long-running", s)
	}
}

// Folders that are not resumable chats are skipped: one without a store.db
// and one whose name could not be passed to the shell safely.
func TestDetectCursor_SkipsFoldersThatAreNotChats(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	now := time.Now()
	writeCursorChat(t, home, "/src/app", "real-chat", now.Add(-time.Hour))
	writeCursorChat(t, home, "/src/app", "x;rm -rf ~", now)

	empty := filepath.Join(home, ".cursor", "chats", cursorWorkspaceKey("/src/app"), "no-store")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}

	s := detectCursor("/src/app", "")
	if s == nil || s.Command != "agent --resume real-chat" {
		t.Fatalf("detectCursor = %+v, want agent --resume real-chat", s)
	}
}

// Without a chat for the directory, or without ~/.cursor at all, Cursor is
// relaunched fresh, exactly as before crex read its chats.
func TestDetectCursor_NoChatsFallsBackToBareCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	want := Session{Tool: "cursor", CWD: "/src/app", Command: "agent"}

	if s := detectCursor("/src/app", ""); s == nil || *s != want {
		t.Errorf("no ~/.cursor: detectCursor = %+v, want %+v", s, want)
	}

	// Chats for another directory say nothing about this one.
	writeCursorChat(t, home, "/src/other", "other-chat", time.Now())
	if s := detectCursor("/src/app", ""); s == nil || *s != want {
		t.Errorf("chats elsewhere: detectCursor = %+v, want %+v", s, want)
	}
}
