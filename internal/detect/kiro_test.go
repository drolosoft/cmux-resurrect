package detect

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// Kiro CLI is closed source, so the fixtures below come from what others
// have written down, and each one says how far it can be trusted.
//
// The commands are documented by Kiro: `kiro-cli chat --resume-id <ID>`
// resumes a local session by id and `--resume` the previous one of the
// directory (kiro.dev/docs/reference/cli-commands/,
// kiro.dev/docs/cli/chat/session-management/). The 3.x migration guide
// (kiro.dev/docs/cli/v3/) adds that both flags open a 2.x session in 3.x as a
// copy, so an id read from the 2.x stores still resumes after the upgrade.
//
// The files on disk are not documented anywhere by Kiro. Their layout comes
// from the third-party reader mxmehl/kiro-cli-history (commit f96f9f0,
// 2026-11-09): kiro_cli_history/main.py reads them and tests/fixtures.py
// writes them, field for field as here. It is co-written by an Amazon
// employee and reads real installs, but it is still reverse engineering of a
// format Kiro is free to change, the 3.x one most of all.

// writeKiroV3Session writes a 3.x session.json under
// <home>/.kiro/sessions/<workspaceHash>/<folder>/, as write_v3_session in
// tests/fixtures.py does. Kiro does not document how it derives the hash,
// which is why the detector walks every hash folder.
func writeKiroV3Session(t *testing.T, home, workspaceHash, folder, body string) {
	t.Helper()

	sessionDir := filepath.Join(home, ".kiro", "sessions", workspaceHash, folder)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "session.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// kiroV3Body returns a 3.x session.json with the fields the detector reads,
// in the shape and timestamp format of write_v3_session.
func kiroV3Body(id, workspace, lastModified string) string {
	return `{"id": "` + id + `", "title": "Fix the login form", ` +
		`"workspacePaths": ["` + workspace + `"], ` +
		`"createdAt": "2026-11-01T09:00:00.000Z", "lastModifiedAt": "` + lastModified + `"}`
}

// writeKiroClassicSession writes a 2.x classic-mode <id>.json under
// <home>/.kiro/sessions/cli/, as write_jsonl_session in tests/fixtures.py
// does: an ISO timestamp without a zone.
func writeKiroClassicSession(t *testing.T, home, id, workspace, updated string) {
	t.Helper()

	cliDir := filepath.Join(home, ".kiro", "sessions", "cli")
	if err := os.MkdirAll(cliDir, 0o755); err != nil {
		t.Fatal(err)
	}

	body := `{"session_id": "` + id + `", "title": "Old work", "cwd": "` + workspace + `", ` +
		`"created_at": "2026-10-01T09:00:00", "updated_at": "` + updated + `"}`
	if err := os.WriteFile(filepath.Join(cliDir, id+".json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// kiroSQLiteSchema is the conversations_v2 table of the 2.x data.sqlite3, as
// write_sqlite_v2_sessions in tests/fixtures.py creates it. created_at and
// updated_at are milliseconds since the epoch.
const kiroSQLiteSchema = `CREATE TABLE conversations_v2 (
	key TEXT NOT NULL,
	conversation_id TEXT NOT NULL,
	value TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL,
	PRIMARY KEY (key, conversation_id)
);`

// writeKiroDatabase creates the 2.x data.sqlite3 where the detector looks for
// it on this system and runs the given INSERT statements through the sqlite3
// CLI, the same tool the detector reads it with.
func writeKiroDatabase(t *testing.T, home, inserts string) {
	t.Helper()

	dbPath := kiroDatabasePath(runtime.GOOS, home, "")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command("sqlite3", dbPath, kiroSQLiteSchema+inserts).CombinedOutput()
	if err != nil {
		t.Fatalf("sqlite3: %v\n%s", err, out)
	}
}

// kiroTestHome points HOME at a fresh folder, clears XDG_DATA_HOME so Linux
// looks under it too, and gives the process no environment of its own.
func kiroTestHome(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	stubProcessEnv(t, nil)

	return home
}

func TestDetectKiro_V3SessionForWorkspace(t *testing.T) {
	home := kiroTestHome(t)
	workspace := "/Users/dev/projects/shop"
	id := "f2946a26-3735-4b08-8d05-c928010302d5"
	writeKiroV3Session(t, home, "a1b2c3d4", "sess_"+id, kiroV3Body(id, workspace, "2026-11-09T14:30:22.000Z"))

	session := detectKiro(workspace, "")

	want := Session{Tool: "kiro", CWD: workspace, Command: "kiro-cli chat --resume-id " + id}
	if session == nil || *session != want {
		t.Errorf("detectKiro = %+v, want %+v", session, want)
	}
}

func TestDetectKiro_V3PicksNewestOfTheWorkspace(t *testing.T) {
	home := kiroTestHome(t)
	workspace := "/Users/dev/projects/shop"
	// The newer session sorts first by name, so the folder order cannot pick
	// it by accident.
	newer := "11111111-1111-4111-8111-111111111111"
	older := "22222222-2222-4222-8222-222222222222"
	writeKiroV3Session(t, home, "a1b2c3d4", "sess_"+newer, kiroV3Body(newer, workspace, "2026-11-09T14:30:22.000Z"))
	writeKiroV3Session(t, home, "a1b2c3d4", "sess_"+older, kiroV3Body(older, workspace, "2026-11-08T09:15:00.000Z"))

	session := detectKiro(workspace, "")

	if session == nil || session.Command != "kiro-cli chat --resume-id "+newer {
		t.Errorf("detectKiro = %+v, want the session %s", session, newer)
	}
}

func TestDetectKiro_SQLiteSessionForWorkspace(t *testing.T) {
	home := kiroTestHome(t)
	workspace := "/Users/dev/projects/shop"
	writeKiroDatabase(t, home, `
INSERT INTO conversations_v2 (key, conversation_id, value, created_at, updated_at) VALUES
	('/Users/dev/projects/shop', '3f2b8c1e-9d4a-4e7b-a1c2-5d6e7f809a1b', '{}', 1794129000000, 1794129323000),
	('/Users/dev/projects/shop', '9a8b7c6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d', '{}', 1794234000000, 1794234622000);`)

	session := detectKiro(workspace, "")

	want := Session{Tool: "kiro", CWD: workspace, Command: "kiro-cli chat --resume-id 9a8b7c6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d"}
	if session == nil || *session != want {
		t.Errorf("detectKiro = %+v, want %+v", session, want)
	}
}

func TestDetectKiro_ClassicSessionForWorkspace(t *testing.T) {
	home := kiroTestHome(t)
	workspace := "/Users/dev/projects/shop"
	id := "3f2b8c1e-9d4a-4e7b-a1c2-5d6e7f809a1b"
	writeKiroClassicSession(t, home, id, workspace, "2026-11-09T14:30:22")

	session := detectKiro(workspace, "")

	if session == nil || session.Command != "kiro-cli chat --resume-id "+id {
		t.Errorf("detectKiro = %+v, want the session %s", session, id)
	}
}

func TestDetectKiro_NewestAcrossFormats(t *testing.T) {
	workspace := "/Users/dev/projects/shop"
	v3 := "11111111-1111-4111-8111-111111111111"
	classic := "22222222-2222-4222-8222-222222222222"
	sqlite := "33333333-3333-4333-8333-333333333333"

	// 1794234622000 ms is 2026-11-09T14:30:22Z.
	database := `
INSERT INTO conversations_v2 (key, conversation_id, value, created_at, updated_at) VALUES
	('/Users/dev/projects/shop', '` + sqlite + `', '{}', 1794234000000, 1794234622000);`

	cases := []struct {
		name        string
		v3Time      string
		classicTime string
		want        string
	}{
		{"3.x newest", "2026-11-09T15:00:00.000Z", "2026-11-09T14:00:00", v3},
		{"classic newest", "2026-11-09T13:00:00.000Z", "2026-11-09T16:00:00", classic},
		{"SQLite newest", "2026-11-09T13:00:00.000Z", "2026-11-09T14:00:00", sqlite},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := kiroTestHome(t)
			writeKiroV3Session(t, home, "a1b2c3d4", "sess_"+v3, kiroV3Body(v3, workspace, c.v3Time))
			writeKiroClassicSession(t, home, classic, workspace, c.classicTime)
			writeKiroDatabase(t, home, database)

			session := detectKiro(workspace, "")

			if session == nil || session.Command != "kiro-cli chat --resume-id "+c.want {
				t.Errorf("detectKiro = %+v, want the session %s", session, c.want)
			}
		})
	}
}

func TestDetectKiro_IgnoresOtherDirectories(t *testing.T) {
	home := kiroTestHome(t)
	workspace := "/Users/dev/projects/shop"
	ours := "11111111-1111-4111-8111-111111111111"
	theirs := "22222222-2222-4222-8222-222222222222"
	// Every store holds a newer session for another directory, and a 3.x
	// session that only added the workspace as a second folder: none of
	// them was opened here.
	writeKiroV3Session(t, home, "a1b2c3d4", "sess_"+ours, kiroV3Body(ours, workspace, "2026-11-08T09:00:00.000Z"))
	writeKiroV3Session(t, home, "e5f6a7b8", "sess_"+theirs, kiroV3Body(theirs, "/Users/dev/projects/blog", "2026-11-09T09:00:00.000Z"))
	writeKiroV3Session(t, home, "c9d0e1f2", "sess_extra",
		`{"id": "sess_extra", "workspacePaths": ["/Users/dev/projects/blog", "`+workspace+`"], "lastModifiedAt": "2026-11-09T10:00:00.000Z"}`)
	writeKiroClassicSession(t, home, "33333333-3333-4333-8333-333333333333", "/Users/dev/projects/blog", "2026-11-09T11:00:00")
	writeKiroDatabase(t, home, `
INSERT INTO conversations_v2 (key, conversation_id, value, created_at, updated_at) VALUES
	('/Users/dev/projects/blog', '44444444-4444-4444-8444-444444444444', '{}', 1794234000000, 1794300000000);`)

	session := detectKiro(workspace, "")

	if session == nil || session.Command != "kiro-cli chat --resume-id "+ours {
		t.Errorf("detectKiro = %+v, want the session %s", session, ours)
	}
}

func TestDetectKiro_SkipsMalformedSessionJSON(t *testing.T) {
	home := kiroTestHome(t)
	workspace := "/Users/dev/projects/shop"
	good := "11111111-1111-4111-8111-111111111111"
	// A session.json cut short, as when Kiro is still writing it, is newer
	// but cannot be read, so it is not taken for a session of the directory.
	writeKiroV3Session(t, home, "a1b2c3d4", "sess_"+good, kiroV3Body(good, workspace, "2026-11-08T09:00:00.000Z"))
	writeKiroV3Session(t, home, "a1b2c3d4", "sess_broken", `{"id": "sess_broken", "workspacePaths": ["`+workspace)

	session := detectKiro(workspace, "")

	if session == nil || session.Command != "kiro-cli chat --resume-id "+good {
		t.Errorf("detectKiro = %+v, want the session %s", session, good)
	}
}

func TestDetectKiro_UnsafeIDResumesLatest(t *testing.T) {
	home := kiroTestHome(t)
	workspace := "/Users/dev/projects/shop"
	// The id is typed into the terminal, so one that is not a plain id is
	// never sent. The session is still there, and --resume reaches it
	// without naming it.
	writeKiroV3Session(t, home, "a1b2c3d4", "sess_odd", kiroV3Body("abc; rm -rf ~", workspace, "2026-11-09T09:00:00.000Z"))

	session := detectKiro(workspace, "")

	want := Session{Tool: "kiro", CWD: workspace, Command: "kiro-cli chat --resume"}
	if session == nil || *session != want {
		t.Errorf("detectKiro = %+v, want %+v", session, want)
	}
}

func TestDetectKiro_QuoteInCWD(t *testing.T) {
	home := kiroTestHome(t)
	workspace := "/src/o'brien"
	// Left unescaped, the quote would end the string literal: the query
	// would fail and the detector would hand back the bare command.
	writeKiroDatabase(t, home, `
INSERT INTO conversations_v2 (key, conversation_id, value, created_at, updated_at) VALUES
	('/src/o''brien', '3f2b8c1e-9d4a-4e7b-a1c2-5d6e7f809a1b', '{}', 1794129000000, 1794129323000);`)

	session := detectKiro(workspace, "")

	if session == nil || session.Command != "kiro-cli chat --resume-id 3f2b8c1e-9d4a-4e7b-a1c2-5d6e7f809a1b" {
		t.Errorf("detectKiro = %+v, want the session of %s", session, workspace)
	}
}

func TestDetectKiro_NoSessionIsBare(t *testing.T) {
	kiroTestHome(t)

	session := detectKiro("/Users/dev/projects/shop", "")

	want := Session{Tool: "kiro", CWD: "/Users/dev/projects/shop", Command: "kiro-cli"}
	if session == nil || *session != want {
		t.Errorf("detectKiro = %+v, want %+v", session, want)
	}
}

func TestKiroDatabasePath(t *testing.T) {
	home := "/home/dev"
	stubProcessEnv(t, map[string]map[string]string{
		"42": {"XDG_DATA_HOME": "/data/dev"},
		"43": {"XDG_DATA_HOME": "relative/data"},
	})
	t.Setenv("XDG_DATA_HOME", "")

	// macOS keeps it under Application Support and Linux under the XDG data
	// folder, the dirs::data_local_dir() of the Rust crate that Amazon Q
	// Developer CLI, Kiro's open-source predecessor, used for the same file
	// (crates/chat-cli/src/util/paths.rs in aws/amazon-q-developer-cli).
	// kiro-cli-history's README gives the same two defaults.
	cases := []struct {
		name string
		goos string
		pid  string
		want string
	}{
		{"macOS", "darwin", "42", "/home/dev/Library/Application Support/kiro-cli/data.sqlite3"},
		{"Linux default", "linux", "", "/home/dev/.local/share/kiro-cli/data.sqlite3"},
		{"Linux XDG_DATA_HOME of the process", "linux", "42", "/data/dev/kiro-cli/data.sqlite3"},
		{"Linux relative XDG_DATA_HOME is ignored", "linux", "43", "/home/dev/.local/share/kiro-cli/data.sqlite3"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := kiroDatabasePath(c.goos, home, c.pid); got != c.want {
				t.Errorf("kiroDatabasePath(%q) = %q, want %q", c.goos, got, c.want)
			}
		})
	}
}
