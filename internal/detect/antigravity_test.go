package detect

import (
	"os"
	"path/filepath"
	"testing"
)

// The fixtures follow Google's Antigravity CLI docs. The resume guide
// (antigravity.google/docs/cli/commands/resume) gives the cache file,
// ~/.gemini/antigravity-cli/cache/last_conversations.json, as "a JSON map
// associating absolute workspace directory paths with their most recently
// active conversation ID", with UUIDs as the ids in its example, and the
// command `agy --conversation <conversation-id>`; the CLI reference
// (antigravity.google/docs/cli/reference) lists the same flag. The guide on
// conversations (antigravity.google/docs/cli/conversations) says histories
// are scoped "directly to your current working directory", so a key is the
// directory agy started in and not the root of its repository. Neither page
// says whether links in that path are resolved. Nothing here was checked
// against a real agy install.

// writeAntigravityCache writes last_conversations.json with the given
// contents under home.
func writeAntigravityCache(t *testing.T, home, contents string) {
	t.Helper()

	folder := filepath.Join(home, ".gemini", "antigravity-cli", "cache")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "last_conversations.json"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

// antigravityHome points HOME at a fresh temporary folder and returns it.
func antigravityHome(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)

	return home
}

func TestDetectAntigravity_FindsConversationForWorkspace(t *testing.T) {
	home := antigravityHome(t)
	workspace := "/Users/dev/projects/shop"
	id := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	writeAntigravityCache(t, home, `{"`+workspace+`": "`+id+`"}`)

	session := detectAntigravity(workspace, "")

	want := Session{Tool: "antigravity", CWD: workspace, Command: "agy --conversation " + id}
	if session == nil || *session != want {
		t.Errorf("detectAntigravity = %+v, want %+v", session, want)
	}
}

func TestDetectAntigravity_IgnoresOtherWorkspaces(t *testing.T) {
	home := antigravityHome(t)
	workspace := "/Users/dev/projects/shop"
	ours := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	theirs := "f9e8d7c6-b5a4-3210-fedc-ba9876543210"
	cache := `{
		"/Users/dev/projects/blog": "` + theirs + `",
		"` + workspace + `": "` + ours + `"
	}`
	writeAntigravityCache(t, home, cache)

	session := detectAntigravity(workspace, "")

	if session == nil || session.Command != "agy --conversation "+ours {
		t.Errorf("detectAntigravity = %+v, want the conversation %s", session, ours)
	}
}

// The cache is keyed by the directory agy started in, so a subfolder of a
// workspace does not take over the conversation of its parent.
func TestDetectAntigravity_SubfolderIsAnotherWorkspace(t *testing.T) {
	home := antigravityHome(t)
	writeAntigravityCache(t, home, `{"/Users/dev/projects/shop": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"}`)

	session := detectAntigravity("/Users/dev/projects/shop/web", "")

	if session == nil || session.Command != "agy" {
		t.Errorf("detectAntigravity = %+v, want the bare command", session)
	}
}

// A pane whose cwd is a link still finds the conversation filed under the
// real path, in case agy records the directory with its links resolved.
func TestDetectAntigravity_SymlinkedCwd(t *testing.T) {
	home := antigravityHome(t)

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
	id := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	writeAntigravityCache(t, home, `{"`+resolved+`": "`+id+`"}`)

	session := detectAntigravity(link, "")

	want := Session{Tool: "antigravity", CWD: link, Command: "agy --conversation " + id}
	if session == nil || *session != want {
		t.Errorf("detectAntigravity = %+v, want %+v", session, want)
	}
}

func TestDetectAntigravity_MalformedCacheGivesBareCommand(t *testing.T) {
	home := antigravityHome(t)
	workspace := "/Users/dev/projects/shop"
	writeAntigravityCache(t, home, `{"`+workspace+`": "a1b2c3d4-e5f6`)

	session := detectAntigravity(workspace, "")

	if session == nil || session.Command != "agy" {
		t.Errorf("detectAntigravity = %+v, want the bare command", session)
	}
}

func TestDetectAntigravity_RejectsUnsafeIDs(t *testing.T) {
	home := antigravityHome(t)
	workspace := "/Users/dev/projects/shop"
	writeAntigravityCache(t, home, `{"`+workspace+`": "abc; rm -rf ~"}`)

	session := detectAntigravity(workspace, "")

	if session == nil || session.Command != "agy" {
		t.Errorf("detectAntigravity = %+v, want the bare command", session)
	}
}

func TestDetectAntigravity_NoCacheGivesBareCommand(t *testing.T) {
	antigravityHome(t)
	workspace := "/Users/dev/projects/shop"

	session := detectAntigravity(workspace, "")

	want := Session{Tool: "antigravity", CWD: workspace, Command: "agy"}
	if session == nil || *session != want {
		t.Errorf("detectAntigravity = %+v, want %+v", session, want)
	}
}

// The keys are absolute paths, so a relative cwd can only match by
// accident.
func TestDetectAntigravity_RelativeCwdGivesBareCommand(t *testing.T) {
	home := antigravityHome(t)
	writeAntigravityCache(t, home, `{"shop": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"}`)

	session := detectAntigravity("shop", "")

	if session == nil || session.Command != "agy" {
		t.Errorf("detectAntigravity = %+v, want the bare command", session)
	}
}
