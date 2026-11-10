package detect

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The fixtures follow Pi's own code and docs (github.com/earendil-works/pi,
// packages/coding-agent):
//   - src/core/session-manager.ts, getDefaultSessionDirPath: the folder is
//     <agent dir>/sessions/--<cwd without its leading separator, with "/",
//     "\" and ":" turned into "-">--.
//   - session-manager.ts, newSession: the file is <ISO timestamp with ":"
//     and "." turned into "-">_<id>.jsonl and its first line is the header
//     {"type":"session","version":3,"id":...,"timestamp":...,"cwd":...}.
//   - session-manager.ts, findMostRecentSession: `pi -c` takes the newest
//     file by modification time whose header cwd matches.
//   - src/config.ts: PI_CODING_AGENT_DIR moves the agent dir (default
//     ~/.pi/agent); src/main.ts reads PI_CODING_AGENT_SESSION_DIR, a folder
//     that holds the session files directly, without the per-cwd folder.
//   - docs/session-format.md and docs/cli.md (`--session <path|id>`).

// piTestCWD is the project every Pi test runs in.
const piTestCWD = "/Users/x/proj"

// piTestFolder is how Pi names the session folder of piTestCWD.
const piTestFolder = "--Users-x-proj--"

// isolatePi points HOME at an empty temporary directory, clears the Pi
// variables the developer running the tests may have exported and makes
// the process environment unreadable, so only the test decides where Pi
// keeps its sessions. It returns the temporary HOME.
func isolatePi(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(piAgentDirVariable, "")
	t.Setenv(piSessionDirVariable, "")
	stubProcessEnv(t, nil)

	return home
}

// writePiSession writes a Pi session file whose header carries id and cwd,
// named <timestamp>_<id>.jsonl as Pi names it, and gives it the
// modification time Pi uses to pick the most recent session.
func writePiSession(t *testing.T, folder, timestamp, id, cwd string, modified time.Time) string {
	t.Helper()

	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}

	header := `{"type":"session","version":3,"id":"` + id + `","timestamp":"2026-11-10T09:00:00.000Z","cwd":"` + cwd + `"}`
	message := `{"type":"message","id":"a1b2c3d4","parentId":null,"timestamp":"2026-11-10T09:00:01.000Z","message":{"role":"user","content":"hello"}}`
	path := filepath.Join(folder, timestamp+"_"+id+".jsonl")
	if err := os.WriteFile(path, []byte(header+"\n"+message+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modified, modified); err != nil {
		t.Fatal(err)
	}

	return path
}

// defaultPiFolder is the folder Pi uses for piTestCWD when nothing moves it.
func defaultPiFolder(home string) string {
	return filepath.Join(home, ".pi", "agent", "sessions", piTestFolder)
}

func TestPiSessionFolder(t *testing.T) {
	tests := []struct {
		cwd  string
		want string
	}{
		{"/Users/x/proj", "--Users-x-proj--"},
		{"/", "----"},
		{"/srv/app.io", "--srv-app.io--"},
		{`C:\Users\x\proj`, "--C--Users-x-proj--"},
	}
	for _, test := range tests {
		if got := piSessionFolder(test.cwd); got != test.want {
			t.Errorf("piSessionFolder(%q) = %q, want %q", test.cwd, got, test.want)
		}
	}
}

func TestDetectPi_FindsSession(t *testing.T) {
	home := isolatePi(t)
	writePiSession(t, defaultPiFolder(home), "2026-11-10T09-00-00-000Z",
		"019a6f2e-7c3b-7d41-9a52-4f1e8b0c2d3a", piTestCWD, time.Now())

	session := detectPi(piTestCWD, "")

	want := Session{Tool: "pi", CWD: piTestCWD, Command: "pi --session 019a6f2e-7c3b-7d41-9a52-4f1e8b0c2d3a"}
	if session == nil || *session != want {
		t.Errorf("detectPi = %+v, want %+v", session, want)
	}
}

func TestDetectPi_NewestOfTwo(t *testing.T) {
	home := isolatePi(t)
	folder := defaultPiFolder(home)
	now := time.Now()

	// The session created first was used last: its file name is older but
	// it was written to more recently, and that is the one `pi -c` opens.
	writePiSession(t, folder, "2026-11-09T08-00-00-000Z", "019a6f2e-0000-7000-8000-000000000001", piTestCWD, now)
	writePiSession(t, folder, "2026-11-10T08-00-00-000Z", "019a6f2e-0000-7000-8000-000000000002", piTestCWD, now.Add(-time.Hour))

	session := detectPi(piTestCWD, "")

	if session == nil || session.Command != "pi --session 019a6f2e-0000-7000-8000-000000000001" {
		t.Errorf("detectPi = %+v, want the most recently written session", session)
	}
}

func TestDetectPi_AgentDirFromCrexEnvironment(t *testing.T) {
	isolatePi(t)
	agentDir := filepath.Join(t.TempDir(), "pi agent")
	t.Setenv(piAgentDirVariable, agentDir)
	writePiSession(t, filepath.Join(agentDir, "sessions", piTestFolder), "2026-11-10T09-00-00-000Z",
		"019a6f2e-0000-7000-8000-00000000000a", piTestCWD, time.Now())

	session := detectPi(piTestCWD, "")

	want := "PI_CODING_AGENT_DIR='" + agentDir + "' pi --session 019a6f2e-0000-7000-8000-00000000000a"
	if session == nil || session.Command != want {
		t.Errorf("detectPi = %+v, want Command %q", session, want)
	}
}

func TestDetectPi_AgentDirWithTilde(t *testing.T) {
	home := isolatePi(t)
	t.Setenv(piAgentDirVariable, "~/pi-work")
	writePiSession(t, filepath.Join(home, "pi-work", "sessions", piTestFolder), "2026-11-10T09-00-00-000Z",
		"019a6f2e-0000-7000-8000-00000000000b", piTestCWD, time.Now())

	session := detectPi(piTestCWD, "")

	want := "PI_CODING_AGENT_DIR=" + filepath.Join(home, "pi-work") + " pi --session 019a6f2e-0000-7000-8000-00000000000b"
	if session == nil || session.Command != want {
		t.Errorf("detectPi = %+v, want Command %q", session, want)
	}
}

func TestDetectPi_SessionDirFromProcessEnvironment(t *testing.T) {
	home := isolatePi(t)
	sessionDir := filepath.Join(t.TempDir(), "sessions")
	stubProcessEnv(t, map[string]map[string]string{"42": {
		piSessionDirVariable: sessionDir,
		// The session dir wins over the agent dir, as in Pi's main.ts.
		piAgentDirVariable: filepath.Join(home, "ignored"),
	}})

	// A session dir holds the files of every project side by side, so
	// only the header tells them apart.
	writePiSession(t, sessionDir, "2026-11-10T10-00-00-000Z", "019a6f2e-0000-7000-8000-0000000000c1", "/Users/x/other", time.Now())
	writePiSession(t, sessionDir, "2026-11-10T09-00-00-000Z", "019a6f2e-0000-7000-8000-0000000000c2", piTestCWD, time.Now().Add(-time.Minute))

	session := detectPi(piTestCWD, "42")

	want := "PI_CODING_AGENT_SESSION_DIR=" + sessionDir + " pi --session 019a6f2e-0000-7000-8000-0000000000c2"
	if session == nil || session.Command != want {
		t.Errorf("detectPi = %+v, want Command %q", session, want)
	}
}

func TestDetectPi_DefaultDirNeedsNoPrefix(t *testing.T) {
	home := isolatePi(t)
	t.Setenv(piAgentDirVariable, filepath.Join(home, ".pi", "agent")+"/")
	writePiSession(t, defaultPiFolder(home), "2026-11-10T09-00-00-000Z",
		"019a6f2e-0000-7000-8000-00000000000d", piTestCWD, time.Now())

	session := detectPi(piTestCWD, "")

	if session == nil || session.Command != "pi --session 019a6f2e-0000-7000-8000-00000000000d" {
		t.Errorf("detectPi = %+v, want no variable in front of pi", session)
	}
}

func TestDetectPi_HeaderCWDMismatchIgnored(t *testing.T) {
	home := isolatePi(t)

	// /Users/x-proj and /Users/x/proj share the folder --Users-x-proj--;
	// the header is the only thing that says whose session it is.
	writePiSession(t, defaultPiFolder(home), "2026-11-10T09-00-00-000Z",
		"019a6f2e-0000-7000-8000-00000000000e", "/Users/x-proj", time.Now())

	session := detectPi(piTestCWD, "")

	want := Session{Tool: "pi", CWD: piTestCWD, Command: "pi"}
	if session == nil || *session != want {
		t.Errorf("detectPi = %+v, want %+v", session, want)
	}
}

func TestDetectPi_SkipsFilesWithoutHeader(t *testing.T) {
	home := isolatePi(t)
	folder := defaultPiFolder(home)
	valid := writePiSession(t, folder, "2026-11-10T08-00-00-000Z",
		"019a6f2e-0000-7000-8000-00000000000f", piTestCWD, time.Now().Add(-time.Hour))

	// Newer files that are not sessions Pi could open: one starts with a
	// message, the other carries an id crex would not type into a shell.
	broken := filepath.Join(folder, "2026-11-10T09-00-00-000Z_broken.jsonl")
	if err := os.WriteFile(broken, []byte(`{"type":"message","id":"x","cwd":"/Users/x/proj"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writePiSession(t, folder, "2026-11-10T10-00-00-000Z", "bad;id", piTestCWD, time.Now())

	session := detectPi(piTestCWD, "")

	if session == nil || session.Command != "pi --session 019a6f2e-0000-7000-8000-00000000000f" {
		t.Errorf("detectPi = %+v, want the session in %s", session, valid)
	}
}

func TestDetectPi_NoSessionsRelaunchesPi(t *testing.T) {
	isolatePi(t)

	session := detectPi(piTestCWD, "")

	want := Session{Tool: "pi", CWD: piTestCWD, Command: "pi"}
	if session == nil || *session != want {
		t.Errorf("detectPi = %+v, want %+v", session, want)
	}
}
