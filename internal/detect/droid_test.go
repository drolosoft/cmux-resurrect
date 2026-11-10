package detect

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The fixtures below copy the layout Factory Droid writes, as read by
// github.com/entireio/cli at commit b7b42d0 (cmd/entire/cli/agent/
// factoryaidroid/factoryaidroid.go and subagent_session.go,
// cmd/entire/cli/agentimport/factory_test.go): one transcript per session in
// <home>/.factory/sessions/<cwd with every non-alphanumeric character as
// "-">/<id>.jsonl, opened by a line like
// {"type":"session_start","id":"s0","version":2,"cwd":"/repo"}. Worker
// sessions started by the Task tool live in the same folder and name their
// parent in callingSessionId. The resume flag is `droid --resume <id>` from
// docs.factory.com/reference/cli-reference.

// writeDroidSession creates a Droid transcript in the session folder of
// folderCWD whose header records headerCWD, and dates it age ago so tests can
// order several sessions. The folder and the header take separate paths
// because two directories can encode to the same folder name.
func writeDroidSession(t *testing.T, home, folderCWD, headerCWD, sessionID string, age time.Duration) {
	t.Helper()
	header := `{"type":"session_start","id":"` + sessionID + `","version":2,"cwd":"` + headerCWD + `"}`
	writeDroidTranscript(t, home, folderCWD, sessionID, header, age)
}

// writeDroidTranscript writes a transcript with the given first line, followed
// by one message, and sets its modification time to age ago.
func writeDroidTranscript(t *testing.T, home, folderCWD, sessionID, header string, age time.Duration) {
	t.Helper()
	sessionDir := filepath.Join(home, ".factory", "sessions", nonAlnum.ReplaceAllString(folderCWD, "-"))
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(sessionDir, sessionID+".jsonl")
	message := `{"type":"message","id":"u1","message":{"role":"user","content":"hello"}}`
	if err := os.WriteFile(path, []byte(header+"\n"+message+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	modified := time.Now().Add(-age)
	if err := os.Chtimes(path, modified, modified); err != nil {
		t.Fatal(err)
	}
}

func TestDetectDroid_ResumesSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	cwd := "/Users/dev/proj"
	sessionID := "5734e7ee-1c2d-4e5f-8a9b-0c1d2e3f4a5b"

	// The folder keeps the leading dash: /Users/dev/proj → -Users-dev-proj.
	writeDroidSession(t, home, cwd, cwd, sessionID, time.Minute)
	if _, err := os.Stat(filepath.Join(home, ".factory", "sessions", "-Users-dev-proj", sessionID+".jsonl")); err != nil {
		t.Fatalf("fixture not where Droid writes it: %v", err)
	}

	s := detectDroid(cwd, "")
	if s == nil {
		t.Fatal("expected session, got nil")
	}
	if s.Tool != "factory" {
		t.Errorf("Tool = %q, want factory", s.Tool)
	}
	if s.CWD != cwd {
		t.Errorf("CWD = %q, want %q", s.CWD, cwd)
	}
	if want := "droid --resume " + sessionID; s.Command != want {
		t.Errorf("Command = %q, want %q", s.Command, want)
	}
}

func TestDetectDroid_NewestOfTwo(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	cwd := "/src/app.io"

	writeDroidSession(t, home, cwd, cwd, "older", time.Hour)
	writeDroidSession(t, home, cwd, cwd, "newer", time.Minute)

	s := detectDroid(cwd, "")
	if s == nil || s.Command != "droid --resume newer" {
		t.Fatalf("got %+v, want droid --resume newer", s)
	}
}

func TestDetectDroid_HeaderCWDTellsCollidingFoldersApart(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)

	// Both directories encode to -w-acme-foo-bar, so they share one folder.
	first := "/w/acme/foo-bar"
	second := "/w/acme-foo/bar"
	writeDroidSession(t, home, first, first, "first-session", time.Hour)
	writeDroidSession(t, home, second, second, "second-session", time.Minute)

	if s := detectDroid(first, ""); s == nil || s.Command != "droid --resume first-session" {
		t.Errorf("%s: got %+v, want its own older session", first, s)
	}
	if s := detectDroid(second, ""); s == nil || s.Command != "droid --resume second-session" {
		t.Errorf("%s: got %+v, want second-session", second, s)
	}
}

func TestDetectDroid_HomeOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := "/src/app"
	override := filepath.Join(t.TempDir(), "droid-home")

	// A newer session under the real home must not win over the one the
	// process actually writes to.
	writeDroidSession(t, override, cwd, cwd, "overridden", time.Hour)
	writeDroidSession(t, home, cwd, cwd, "default", time.Minute)
	stubProcessEnv(t, map[string]map[string]string{"42": {"FACTORY_HOME_OVERRIDE": override}})

	s := detectDroid(cwd, "42")
	if s == nil {
		t.Fatal("expected session under FACTORY_HOME_OVERRIDE, got nil")
	}

	// The resume has to look in the same place, so it sets the override again.
	want := "FACTORY_HOME_OVERRIDE=" + override + " droid --resume overridden"
	if s.Command != want {
		t.Errorf("Command = %q, want %q", s.Command, want)
	}
}

func TestDetectDroid_RelativeOverrideIsIgnored(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := "/src/app"
	writeDroidSession(t, home, cwd, cwd, "default", time.Minute)
	stubProcessEnv(t, map[string]map[string]string{"7": {"FACTORY_HOME_OVERRIDE": "relative/home"}})

	s := detectDroid(cwd, "7")
	if s == nil || s.Command != "droid --resume default" {
		t.Fatalf("got %+v, want plain droid --resume default", s)
	}
}

func TestDetectDroid_SkipsWorkerSessions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	cwd := "/src/app"

	// The Task tool's Worker writes the newest transcript, but it belongs to
	// the parent session the user is talking to.
	writeDroidSession(t, home, cwd, cwd, "parent", time.Hour)
	worker := `{"type":"session_start","id":"worker","cwd":"/src/app","callingSessionId":"parent","callingToolUseId":"toolu_1"}`
	writeDroidTranscript(t, home, cwd, "worker", worker, time.Minute)

	s := detectDroid(cwd, "")
	if s == nil || s.Command != "droid --resume parent" {
		t.Fatalf("got %+v, want droid --resume parent", s)
	}
}

func TestDetectDroid_RejectsUnsafeID(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	cwd := "/src/app"
	header := `{"type":"session_start","id":"x;rm -rf ~","cwd":"/src/app"}`
	writeDroidTranscript(t, home, cwd, "unsafe", header, time.Minute)

	s := detectDroid(cwd, "")
	if s == nil || s.Command != "droid" {
		t.Fatalf("got %+v, want a fresh droid", s)
	}
}

func TestDetectDroid_NoSessionStartsFresh(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	cwd := "/src/app"

	// Without any session, and with only another directory's session in the
	// shared folder, Droid starts as it did before crex could resume it.
	for _, setup := range []string{"no session", "other directory"} {
		if setup == "other directory" {
			writeDroidSession(t, home, cwd, "/src-app", "someone-else", time.Minute)
		}

		s := detectDroid(cwd, "")
		want := Session{Tool: "factory", CWD: cwd, Command: "droid"}
		if s == nil || *s != want {
			t.Errorf("%s: got %+v, want %+v", setup, s, want)
		}
	}
}
