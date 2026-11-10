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
//
// The folder that holds chats/ is not always ~/.cursor. The CLI 2026.11.05
// resolves it in cursor-config/dist/paths.js as $CURSOR_CONFIG_DIR, else
// $XDG_CONFIG_HOME/cursor, else ~/.cursor, each variable counting only when
// it holds more than blanks.

// cursorTestHome points HOME at a fresh directory and leaves neither the
// Cursor process nor crex with CURSOR_CONFIG_DIR or XDG_CONFIG_HOME, so the
// chats are looked for in <home>/.cursor whatever the machine running the
// tests has set.
func cursorTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CURSOR_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	stubProcessEnv(t, nil)
	return home
}

// writeCursorChat files a chat for workspace under configDir the way the
// Cursor CLI does and dates the chat folder and its store.db at modified.
func writeCursorChat(t *testing.T, configDir, workspace, chatID string, modified time.Time) {
	t.Helper()

	chatDir := filepath.Join(configDir, "chats", cursorWorkspaceKey(workspace), chatID)
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
	home := cursorTestHome(t)
	writeCursorChat(t, filepath.Join(home, ".cursor"), "/src/app", "0b9c1f4e-7a2d-4c3b-9e8f-1a2b3c4d5e6f", time.Now())

	s := detectCursor("/src/app", "")
	want := Session{Tool: "cursor", CWD: "/src/app", Command: "agent --resume 0b9c1f4e-7a2d-4c3b-9e8f-1a2b3c4d5e6f"}
	if s == nil || *s != want {
		t.Fatalf("detectCursor = %+v, want %+v", s, want)
	}
}

// Cursor hashes the directory with its links resolved, so a pane whose cwd
// is a link still finds the chat filed under the real path.
func TestDetectCursor_SymlinkedCwd(t *testing.T) {
	home := cursorTestHome(t)

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
	writeCursorChat(t, filepath.Join(home, ".cursor"), resolved, "linked-chat", time.Now())

	s := detectCursor(link, "")
	if s == nil || s.Command != "agent --resume linked-chat" || s.CWD != link {
		t.Fatalf("detectCursor(link) = %+v, want agent --resume linked-chat in %s", s, link)
	}
}

// With several chats in one directory, the one written last is resumed.
func TestDetectCursor_NewestOfTwoChats(t *testing.T) {
	home := cursorTestHome(t)
	now := time.Now()
	writeCursorChat(t, filepath.Join(home, ".cursor"), "/src/app", "newer-chat", now)
	writeCursorChat(t, filepath.Join(home, ".cursor"), "/src/app", "older-chat", now.Add(-time.Hour))

	s := detectCursor("/src/app", "")
	if s == nil || s.Command != "agent --resume newer-chat" {
		t.Fatalf("detectCursor = %+v, want agent --resume newer-chat", s)
	}
}

// A chat that keeps being written moves its store.db forward even when the
// folder itself was created long ago; the store's time is what counts.
func TestDetectCursor_StoreTimeBeatsFolderTime(t *testing.T) {
	home := cursorTestHome(t)
	now := time.Now()
	writeCursorChat(t, filepath.Join(home, ".cursor"), "/src/app", "long-running", now.Add(-2*time.Hour))
	writeCursorChat(t, filepath.Join(home, ".cursor"), "/src/app", "short-lived", now.Add(-time.Hour))

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
	home := cursorTestHome(t)
	now := time.Now()
	writeCursorChat(t, filepath.Join(home, ".cursor"), "/src/app", "real-chat", now.Add(-time.Hour))
	writeCursorChat(t, filepath.Join(home, ".cursor"), "/src/app", "x;rm -rf ~", now)

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
	home := cursorTestHome(t)
	want := Session{Tool: "cursor", CWD: "/src/app", Command: "agent"}

	if s := detectCursor("/src/app", ""); s == nil || *s != want {
		t.Errorf("no ~/.cursor: detectCursor = %+v, want %+v", s, want)
	}

	// Chats for another directory say nothing about this one.
	writeCursorChat(t, filepath.Join(home, ".cursor"), "/src/other", "other-chat", time.Now())
	if s := detectCursor("/src/app", ""); s == nil || *s != want {
		t.Errorf("chats elsewhere: detectCursor = %+v, want %+v", s, want)
	}
}

// A Cursor started with XDG_CONFIG_HOME set files its chats under
// $XDG_CONFIG_HOME/cursor, as the lab install showed. A newer chat under
// ~/.cursor belongs to some other Cursor and must not win, and the resume has
// to look in the same folder, so it names it again.
func TestDetectCursor_XDGConfigHomeFromProcess(t *testing.T) {
	home := cursorTestHome(t)
	xdg := filepath.Join(home, ".config")
	now := time.Now()
	writeCursorChat(t, filepath.Join(xdg, "cursor"), "/src/app", "xdg-chat", now.Add(-time.Hour))
	writeCursorChat(t, filepath.Join(home, ".cursor"), "/src/app", "default-chat", now)
	stubProcessEnv(t, map[string]map[string]string{"42": {"XDG_CONFIG_HOME": xdg}})

	s := detectCursor("/src/app", "42")
	want := "CURSOR_CONFIG_DIR=" + filepath.Join(xdg, "cursor") + " agent --resume xdg-chat"
	if s == nil || s.Command != want {
		t.Fatalf("detectCursor = %+v, want Command %q", s, want)
	}
}

// CURSOR_CONFIG_DIR comes before XDG_CONFIG_HOME, and it names the folder
// itself, without a "cursor" below it.
func TestDetectCursor_ConfigDirBeatsXDG(t *testing.T) {
	home := cursorTestHome(t)
	configDir := filepath.Join(home, "cursor-work")
	xdg := filepath.Join(home, ".config")
	now := time.Now()
	writeCursorChat(t, configDir, "/src/app", "work-chat", now.Add(-time.Hour))
	writeCursorChat(t, filepath.Join(xdg, "cursor"), "/src/app", "xdg-chat", now)
	stubProcessEnv(t, map[string]map[string]string{"42": {"CURSOR_CONFIG_DIR": configDir, "XDG_CONFIG_HOME": xdg}})

	s := detectCursor("/src/app", "42")
	want := "CURSOR_CONFIG_DIR=" + configDir + " agent --resume work-chat"
	if s == nil || s.Command != want {
		t.Fatalf("detectCursor = %+v, want Command %q", s, want)
	}
}

// Variables that hold only blanks count as unset for Cursor, and a value
// that names ~/.cursor needs no prefix on the resume.
func TestDetectCursor_BlankOrDefaultValueHasNoPrefix(t *testing.T) {
	home := cursorTestHome(t)
	writeCursorChat(t, filepath.Join(home, ".cursor"), "/src/app", "default-chat", time.Now())
	stubProcessEnv(t, map[string]map[string]string{
		"7": {"CURSOR_CONFIG_DIR": "  ", "XDG_CONFIG_HOME": ""},
		"8": {"CURSOR_CONFIG_DIR": filepath.Join(home, ".cursor") + "/"},
	})

	for _, pid := range []string{"7", "8"} {
		if s := detectCursor("/src/app", pid); s == nil || s.Command != "agent --resume default-chat" {
			t.Errorf("pid %s: detectCursor = %+v, want agent --resume default-chat", pid, s)
		}
	}
}

// When the Cursor process environment cannot be read, crex's own is the best
// guess at it: both usually come from the same login shell.
func TestDetectCursor_UnreadableProcessUsesCrexEnvironment(t *testing.T) {
	home := cursorTestHome(t)
	xdg := filepath.Join(home, ".config")
	writeCursorChat(t, filepath.Join(xdg, "cursor"), "/src/app", "xdg-chat", time.Now())
	t.Setenv("XDG_CONFIG_HOME", xdg)

	s := detectCursor("/src/app", "99")
	want := "CURSOR_CONFIG_DIR=" + filepath.Join(xdg, "cursor") + " agent --resume xdg-chat"
	if s == nil || s.Command != want {
		t.Fatalf("detectCursor = %+v, want Command %q", s, want)
	}
}

// A process whose environment was read and has neither variable uses
// ~/.cursor, even when crex itself runs with XDG_CONFIG_HOME set.
func TestDetectCursor_ProcessWithoutVariablesIgnoresCrexEnvironment(t *testing.T) {
	home := cursorTestHome(t)
	xdg := filepath.Join(home, ".config")
	now := time.Now()
	writeCursorChat(t, filepath.Join(xdg, "cursor"), "/src/app", "xdg-chat", now)
	writeCursorChat(t, filepath.Join(home, ".cursor"), "/src/app", "default-chat", now.Add(-time.Hour))
	t.Setenv("XDG_CONFIG_HOME", xdg)
	stubProcessEnv(t, map[string]map[string]string{"42": {"PATH": "/usr/bin"}})

	s := detectCursor("/src/app", "42")
	if s == nil || s.Command != "agent --resume default-chat" {
		t.Fatalf("detectCursor = %+v, want agent --resume default-chat", s)
	}
}

// Cursor reads a relative CURSOR_CONFIG_DIR from its own directory, which crex
// cannot be sure of, so it starts Cursor fresh instead of guessing a folder
// that could hold someone else's chats.
func TestDetectCursor_RelativeConfigDirStartsFresh(t *testing.T) {
	home := cursorTestHome(t)
	writeCursorChat(t, filepath.Join(home, ".cursor"), "/src/app", "default-chat", time.Now())
	stubProcessEnv(t, map[string]map[string]string{"42": {"CURSOR_CONFIG_DIR": "cursor-config"}})

	s := detectCursor("/src/app", "42")
	want := Session{Tool: "cursor", CWD: "/src/app", Command: "agent"}
	if s == nil || *s != want {
		t.Fatalf("detectCursor = %+v, want %+v", s, want)
	}
}
