package detect

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeCopilotSession creates one Copilot CLI session folder under
// stateDir and dates its event log at lastUsed.
//
// The layout is the one GitHub Copilot CLI 1.0.108 writes, taken from the
// published npm package (@github/copilot-darwin-arm64, the bundled
// cli-main.js): "~/.copilot/session-state/<session-id>/ (under
// $COPILOT_HOME when set), with its event log (events.jsonl), metadata
// (workspace.yaml) ...". The session's directory is the `cwd` key of
// workspace.yaml (loadPersistedSessionCwd reads it from there), and the
// keys below are the ones the CLI compares when it reloads that file. The
// events.jsonl and the session-state/<id>/ folder are also documented in
// docs.github.com, "cli-config-dir-reference".
func writeCopilotSession(t *testing.T, stateDir, sessionID, cwd string, lastUsed time.Time) string {
	t.Helper()
	sessionDir := filepath.Join(stateDir, sessionID)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}

	workspace := "id: " + sessionID + "\n" +
		"cwd: " + cwd + "\n" +
		"git_root: " + cwd + "\n" +
		"branch: main\n" +
		"created_at: 2026-11-09T10:00:00.000Z\n" +
		"updated_at: 2026-11-09T10:05:00.000Z\n"
	if err := os.WriteFile(filepath.Join(sessionDir, "workspace.yaml"), []byte(workspace), 0o644); err != nil {
		t.Fatal(err)
	}

	events := filepath.Join(sessionDir, "events.jsonl")
	if err := os.WriteFile(events, []byte(`{"type":"session.start"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(events, lastUsed, lastUsed); err != nil {
		t.Fatal(err)
	}
	return sessionDir
}

// copilotTestHome points HOME at a fresh directory, clears COPILOT_HOME and
// returns the default session-state folder inside it.
func copilotTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("COPILOT_HOME", "")
	stubProcessEnv(t, nil)
	return filepath.Join(home, ".copilot", "session-state")
}

func TestDetectCopilot_NoSessionDir(t *testing.T) {
	copilotTestHome(t)

	if s := detectCopilot("/nonexistent/path", ""); s != nil {
		t.Errorf("expected nil without a session-state folder, got %+v", s)
	}
}

func TestDetectCopilot_EachDirectoryResumesItsOwnSession(t *testing.T) {
	stateDir := copilotTestHome(t)
	now := time.Now()
	writeCopilotSession(t, stateDir, "c52c23e5-5cbe-4786-b046-528839201e7a", "/src/api", now.Add(-time.Hour))
	writeCopilotSession(t, stateDir, "9f1d0b7a-2e44-4c1b-8f0e-3a6d5c7b9e21", "/src/web", now)

	// /src/web holds the globally most recent session: `--continue` would
	// have opened it from /src/api too.
	cases := map[string]string{
		"/src/api": "copilot --resume c52c23e5-5cbe-4786-b046-528839201e7a",
		"/src/web": "copilot --resume 9f1d0b7a-2e44-4c1b-8f0e-3a6d5c7b9e21",
	}
	for cwd, want := range cases {
		s := detectCopilot(cwd, "")
		if s == nil {
			t.Fatalf("%s: expected a session, got nil", cwd)
		}
		if s.Tool != "copilot" || s.CWD != cwd {
			t.Errorf("%s: got Tool=%q CWD=%q", cwd, s.Tool, s.CWD)
		}
		if s.Command != want {
			t.Errorf("%s: Command = %q, want %q", cwd, s.Command, want)
		}
	}
}

func TestDetectCopilot_MostRecentSessionOfTheDirectory(t *testing.T) {
	stateDir := copilotTestHome(t)
	now := time.Now()
	writeCopilotSession(t, stateDir, "old-session", "/src/api", now.Add(-2*time.Hour))
	writeCopilotSession(t, stateDir, "new-session", "/src/api", now)
	writeCopilotSession(t, stateDir, "middle-session", "/src/api", now.Add(-time.Hour))

	s := detectCopilot("/src/api", "")
	if s == nil {
		t.Fatal("expected a session, got nil")
	}
	if s.Command != "copilot --resume new-session" {
		t.Errorf("Command = %q, want the most recent session", s.Command)
	}
}

func TestDetectCopilot_NoSessionForDirectory(t *testing.T) {
	stateDir := copilotTestHome(t)
	writeCopilotSession(t, stateDir, "other-session", "/src/other", time.Now())

	// Never fall back to `--continue`: it would open /src/other's session.
	if s := detectCopilot("/src/api", ""); s != nil {
		t.Errorf("expected nil for a directory without sessions, got %+v", s)
	}
}

func TestDetectCopilot_QuotedCWD(t *testing.T) {
	stateDir := copilotTestHome(t)
	sessionDir := writeCopilotSession(t, stateDir, "quoted-session", "/unused", time.Now())

	// The yaml package quotes a scalar that would not read back as plain
	// text, such as a path containing ": ".
	workspace := "id: quoted-session\ncwd: \"/src/a: b\"\n"
	if err := os.WriteFile(filepath.Join(sessionDir, "workspace.yaml"), []byte(workspace), 0o644); err != nil {
		t.Fatal(err)
	}

	s := detectCopilot("/src/a: b", "")
	if s == nil || s.Command != "copilot --resume quoted-session" {
		t.Errorf("got %+v, want the quoted-session resume", s)
	}
}

func TestDetectCopilot_InUseLockOfTheProcessWins(t *testing.T) {
	stateDir := copilotTestHome(t)
	now := time.Now()
	held := writeCopilotSession(t, stateDir, "held-session", "/src/api", now.Add(-time.Hour))
	writeCopilotSession(t, stateDir, "newer-session", "/src/api", now)

	// Copilot CLI 1.0.108 claims a session by writing inuse.<pid>.lock in
	// its folder: the native module (prebuilds/darwin-arm64/cli-native.node,
	// src/native/cli/persistence/session_in_use.rs) builds the name from
	// "inuse." and ".lock", and cli-main.js registers it in the session's
	// own folder with process.pid on create and on resume.
	if err := os.WriteFile(filepath.Join(held, "inuse.4242.lock"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	s := detectCopilot("/src/api", "4242")
	if s == nil || s.Command != "copilot --resume held-session" {
		t.Errorf("got %+v, want the session pid 4242 holds", s)
	}

	// Another process in the same directory still gets the newest session.
	s = detectCopilot("/src/api", "5151")
	if s == nil || s.Command != "copilot --resume newer-session" {
		t.Errorf("got %+v, want the newest session for an unknown pid", s)
	}
}

func TestDetectCopilot_CopilotHomeFromProcess(t *testing.T) {
	copilotTestHome(t)
	copilotHome := filepath.Join(t.TempDir(), "work copilot")
	writeCopilotSession(t, filepath.Join(copilotHome, "session-state"), "work-session", "/src/api", time.Now())
	stubProcessEnv(t, map[string]map[string]string{"42": {"COPILOT_HOME": copilotHome}})

	s := detectCopilot("/src/api", "42")
	if s == nil {
		t.Fatal("expected a session under the process's COPILOT_HOME, got nil")
	}

	// A restore must look in the same place, or --resume finds nothing.
	want := "COPILOT_HOME=" + shellQuote(copilotHome) + " copilot --resume work-session"
	if s.Command != want {
		t.Errorf("Command = %q, want %q", s.Command, want)
	}
}

func TestDetectCopilot_CopilotHomeFromEnvironment(t *testing.T) {
	copilotTestHome(t)
	copilotHome := t.TempDir()
	writeCopilotSession(t, filepath.Join(copilotHome, "session-state"), "env-session", "/src/api", time.Now())
	t.Setenv("COPILOT_HOME", copilotHome)

	s := detectCopilot("/src/api", "")
	want := "COPILOT_HOME=" + shellQuote(copilotHome) + " copilot --resume env-session"
	if s == nil || s.Command != want {
		t.Errorf("got %+v, want %q", s, want)
	}
}
