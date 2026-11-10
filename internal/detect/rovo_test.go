package detect

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The fixtures follow Atlassian's "Manage sessions in Rovo Dev CLI"
// (support.atlassian.com/rovo/docs/manage-sessions-in-rovo-dev-cli/): one
// folder per session under ~/.rovodev/sessions/, named by the session UUID,
// holding a metadata.json with the title and the workspace. The page does not
// name the fields; workspace_path is the key the crex harness notes give, and
// no public dump of the file was found to confirm it. The restore command is
// `acli rovodev run --restore <session UUID>`, from the same page and from
// support.atlassian.com/rovo/docs/rovo-dev-cli-commands/.

// writeRovoSession creates a session folder with the given metadata.json
// contents under sessionsDir and sets the file's modification time, which is
// what tells the newest session apart.
func writeRovoSession(t *testing.T, sessionsDir, id, metadata string, modified time.Time) {
	t.Helper()

	folder := filepath.Join(sessionsDir, id)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}

	metadataPath := filepath.Join(folder, "metadata.json")
	if err := os.WriteFile(metadataPath, []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(metadataPath, modified, modified); err != nil {
		t.Fatal(err)
	}
}

// rovoMetadata returns a metadata.json body for a session opened in workspace.
func rovoMetadata(workspace string) string {
	return `{"title": "Fix the login form", "workspace_path": "` + workspace + `"}`
}

// defaultRovoSessionsDir points HOME at a fresh temporary folder and returns
// the sessions folder Rovo Dev uses when config.yml does not move it.
func defaultRovoSessionsDir(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)

	return filepath.Join(home, ".rovodev", "sessions")
}

func TestDetectRovo_FindsSessionForWorkspace(t *testing.T) {
	sessionsDir := defaultRovoSessionsDir(t)
	workspace := "/Users/dev/projects/shop"
	id := "3f2b8c1e-9d4a-4e7b-a1c2-5d6e7f809a1b"
	writeRovoSession(t, sessionsDir, id, rovoMetadata(workspace), time.Now())

	session := detectRovo(workspace, "")

	want := Session{Tool: "rovo", CWD: workspace, Command: "acli rovodev run --restore " + id}
	if session == nil || *session != want {
		t.Errorf("detectRovo = %+v, want %+v", session, want)
	}
}

func TestDetectRovo_PicksNewestOfTheWorkspace(t *testing.T) {
	sessionsDir := defaultRovoSessionsDir(t)
	workspace := "/Users/dev/projects/shop"
	// The newer session sorts first by name, so the folder order cannot
	// pick it by accident.
	older := "22222222-2222-4222-8222-222222222222"
	newer := "11111111-1111-4111-8111-111111111111"
	now := time.Now()
	writeRovoSession(t, sessionsDir, newer, rovoMetadata(workspace), now)
	writeRovoSession(t, sessionsDir, older, rovoMetadata(workspace), now.Add(-time.Hour))

	session := detectRovo(workspace, "")

	if session == nil || session.Command != "acli rovodev run --restore "+newer {
		t.Errorf("detectRovo = %+v, want the session %s", session, newer)
	}
}

func TestDetectRovo_IgnoresOtherWorkspaces(t *testing.T) {
	sessionsDir := defaultRovoSessionsDir(t)
	workspace := "/Users/dev/projects/shop"
	ours := "11111111-1111-4111-8111-111111111111"
	theirs := "22222222-2222-4222-8222-222222222222"
	now := time.Now()
	writeRovoSession(t, sessionsDir, ours, rovoMetadata(workspace), now.Add(-time.Hour))
	writeRovoSession(t, sessionsDir, theirs, rovoMetadata("/Users/dev/projects/blog"), now)

	session := detectRovo(workspace, "")

	if session == nil || session.Command != "acli rovodev run --restore "+ours {
		t.Errorf("detectRovo = %+v, want the session %s", session, ours)
	}
}

func TestDetectRovo_SkipsMalformedMetadata(t *testing.T) {
	sessionsDir := defaultRovoSessionsDir(t)
	workspace := "/Users/dev/projects/shop"
	good := "11111111-1111-4111-8111-111111111111"
	broken := "22222222-2222-4222-8222-222222222222"
	now := time.Now()
	writeRovoSession(t, sessionsDir, good, rovoMetadata(workspace), now.Add(-time.Hour))
	writeRovoSession(t, sessionsDir, broken, `{"workspace_path": "`+workspace, now)

	session := detectRovo(workspace, "")

	if session == nil || session.Command != "acli rovodev run --restore "+good {
		t.Errorf("detectRovo = %+v, want the session %s", session, good)
	}
}

func TestDetectRovo_RejectsUnsafeFolderNames(t *testing.T) {
	sessionsDir := defaultRovoSessionsDir(t)
	workspace := "/Users/dev/projects/shop"
	writeRovoSession(t, sessionsDir, "abc;rm -rf ~", rovoMetadata(workspace), time.Now())

	session := detectRovo(workspace, "")

	if session == nil || session.Command != "acli rovodev run" {
		t.Errorf("detectRovo = %+v, want the bare command", session)
	}
}

func TestDetectRovo_NoSessionGivesBareCommand(t *testing.T) {
	defaultRovoSessionsDir(t)
	workspace := "/Users/dev/projects/shop"

	session := detectRovo(workspace, "")

	want := Session{Tool: "rovo", CWD: workspace, Command: "acli rovodev run"}
	if session == nil || *session != want {
		t.Errorf("detectRovo = %+v, want %+v", session, want)
	}
}

func TestDetectRovo_HonoursPersistenceDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	workspace := "/Users/dev/projects/shop"
	id := "3f2b8c1e-9d4a-4e7b-a1c2-5d6e7f809a1b"

	// The shape of the sessions block is the one in "Manage Rovo Dev CLI
	// settings" (support.atlassian.com/rovo/docs/manage-rovo-dev-cli-settings),
	// with a quoted value that starts with ~.
	config := "agent:\n" +
		"  streaming: true\n" +
		"sessions:\n" +
		"  # Directory where session data is stored\n" +
		"  persistenceDir: \"~/elsewhere/rovo-sessions\"\n" +
		"  enableWorkspaceStateSync: false\n"
	writeRovoConfig(t, home, config)
	writeRovoSession(t, filepath.Join(home, "elsewhere", "rovo-sessions"), id, rovoMetadata(workspace), time.Now())

	session := detectRovo(workspace, "")

	if session == nil || session.Command != "acli rovodev run --restore "+id {
		t.Errorf("detectRovo = %+v, want the session %s", session, id)
	}
}

func TestRovoSessionsDir(t *testing.T) {
	home := "/Users/dev"
	defaultDir := filepath.Join(home, ".rovodev", "sessions")

	tests := []struct {
		name   string
		config string
		want   string
	}{
		{"no config", "", defaultDir},
		{"no sessions block", "agent:\n  streaming: true\n", defaultDir},
		{"absolute path", "sessions:\n  persistenceDir: /data/rovo\n", "/data/rovo"},
		{"single quotes and comment", "sessions:\n  persistenceDir: '/data/rovo' # moved\n", "/data/rovo"},
		{"tilde", "sessions:\n  persistenceDir: \"~/rovo\"\n", filepath.Join(home, "rovo")},
		{"key of another block", "other:\n  persistenceDir: /data/rovo\nsessions:\n  autoRestore: false\n", defaultDir},
		{"relative path", "sessions:\n  persistenceDir: rovo\n", defaultDir},
		{"empty value", "sessions:\n  persistenceDir: \"\"\n", defaultDir},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := rovoSessionsDir(home, test.config); got != test.want {
				t.Errorf("rovoSessionsDir = %q, want %q", got, test.want)
			}
		})
	}
}

// writeRovoConfig writes ~/.rovodev/config.yml under home.
func writeRovoConfig(t *testing.T, home, config string) {
	t.Helper()

	folder := filepath.Join(home, ".rovodev")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "config.yml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
}
