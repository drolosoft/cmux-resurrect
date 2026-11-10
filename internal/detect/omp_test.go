package detect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The fixtures follow oh-my-pi's own code and docs (github.com/can1357/oh-my-pi):
//   - docs/session.md, "On-Disk Layout": sessions live in
//     ~/.omp/agent/sessions/<encoded-cwd>/<timestamp>_<sessionId>.jsonl.
//   - packages/coding-agent/src/session/session-paths.ts,
//     getDefaultSessionDirName: the canonical cwd is "-<relative>" under
//     home, "-tmp-<relative>" under os.tmpdir() (checked first), and
//     "--<absolute>--" anywhere else, with "/", "\" and ":" turned into "-".
//     encodeHashedSessionDirName is the <scope>-<basename>-<sha256> name of
//     17.2.5 to 17.2.8.
//   - docs/session.md, "File Format", and src/session/session-title-slot.ts:
//     current files open with a 256-byte {"type":"title","v":1,...} slot,
//     newline included, before the {"type":"session",...} header; legacy
//     files open with the header.
//   - packages/utils/src/dirs.ts and docs/environment-variables.md:
//     PI_CONFIG_DIR, OMP_PROFILE over PI_PROFILE, PI_CODING_AGENT_DIR (default
//     profile only), XDG_DATA_HOME/omp when it exists; src/cli/args.ts reads
//     PI_CODING_AGENT_SESSION_DIR, a folder that holds the files directly.
//   - docs/cli-reference.md: `--resume [id]` takes an id prefix or a path.
//   - Bun's src/js/node/os.ts: tmpdir() is $TMPDIR, $TMP or $TEMP without a
//     trailing "/", else "/tmp".

// ompTestSessionID is a UUIDv7 like the ones omp mints for new sessions.
const ompTestSessionID = "019c625b-b900-7000-8000-000000000001"

// ompTestTitle is a session title as omp generates it: an icon and a short
// code in front of the title (docs/config-usage.md).
const ompTestTitle = "🦀 CREX: add the oh-my-pi detector"

// isolateOmp points HOME and TMPDIR at two empty sibling directories, clears
// the variables that move omp's sessions and makes the process environment
// unreadable, so only the test decides where omp keeps them. t.TempDir()
// lives under the real temp root, so without its own TMPDIR every test
// directory would count as a temp-root one. It returns HOME and the temp
// root, both with their links resolved.
func isolateOmp(t *testing.T) (home, tempRoot string) {
	t.Helper()

	tempRoot = resolvedPath(t.TempDir())
	home = resolvedPath(t.TempDir())
	t.Setenv("HOME", home)
	t.Setenv("TMPDIR", tempRoot)

	for _, name := range []string{
		piSessionDirVariable,
		piAgentDirVariable,
		ompConfigDirVariable,
		ompProfileVariable,
		ompLegacyProfileVariable,
		ompDataHomeVariable,
	} {
		t.Setenv(name, "")
	}
	stubProcessEnv(t, nil)

	return home, tempRoot
}

// makeOmpProject creates a project directory, because omp names the folder
// after the directory with its links resolved and only an existing directory
// can be resolved.
func makeOmpProject(t *testing.T, path string) string {
	t.Helper()

	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// ompTitleSlot returns the first line of a current omp session file: the
// title record padded with spaces to exactly 256 bytes, newline included,
// as serializeTitleSlot writes it.
func ompTitleSlot(t *testing.T, title string) string {
	t.Helper()

	record := `{"type":"title","v":1,"title":"` + title + `","source":"auto","updatedAt":"2026-11-10T09:00:00.000Z","pad":""}`
	padding := 256 - len(record) - len("\n")
	if padding < 0 {
		t.Fatalf("title %q does not fit in the slot", title)
	}

	return strings.Replace(record, `"pad":""`, `"pad":"`+strings.Repeat(" ", padding)+`"`, 1) + "\n"
}

// writeOmpSession writes an omp session file whose header carries id and
// cwd, behind a title slot when withTitleSlot is set, named
// <timestamp>_<id>.jsonl, with the modification time omp sorts sessions by.
func writeOmpSession(t *testing.T, folder, timestamp, id, cwd string, withTitleSlot bool, modified time.Time) string {
	t.Helper()

	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}

	var content strings.Builder
	if withTitleSlot {
		content.WriteString(ompTitleSlot(t, ompTestTitle))
	}
	content.WriteString(`{"type":"session","version":3,"id":"` + id + `","timestamp":"2026-11-10T09:00:00.000Z","cwd":"` + cwd + `","titleSource":"auto"}` + "\n")
	content.WriteString(`{"type":"message","id":"a1b2c3d4","parentId":null,"timestamp":"2026-11-10T09:00:01.000Z","message":{"role":"user","content":"hello"}}` + "\n")

	path := filepath.Join(folder, timestamp+"_"+id+".jsonl")
	if err := os.WriteFile(path, []byte(content.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modified, modified); err != nil {
		t.Fatal(err)
	}

	return path
}

// defaultOmpRoot is where omp keeps the session folders when nothing
// moves them.
func defaultOmpRoot(home string) string {
	return filepath.Join(home, ".omp", "agent", "sessions")
}

func TestOmpSessionFolder(t *testing.T) {
	tests := []struct {
		name      string
		cwd       string
		home      string
		tempRoot  string
		want      string
		wantScope string
	}{
		{"under home", "/Users/x/src/proj", "/Users/x", "/private/tmp", "-src-proj", "home"},
		{"home itself", "/Users/x", "/Users/x", "/private/tmp", "-", "home"},
		{"colon in a name", "/Users/x/a:b", "/Users/x", "/private/tmp", "-a-b", "home"},
		{"under the temp root", "/private/var/folders/ab/T/build/out", "/Users/x", "/private/var/folders/ab/T", "-tmp-build-out", "tmp"},
		{"the temp root itself", "/private/tmp", "/Users/x", "/private/tmp", "-tmp", "tmp"},
		// The temp root is checked first, for a temp root inside home.
		{"temp root inside home", "/Users/x/AppData/Temp/a", "/Users/x", "/Users/x/AppData/Temp", "-tmp-a", "tmp"},
		{"outside both", "/srv/app.io", "/Users/x", "/private/tmp", "--srv-app.io--", "abs"},
		{"root", "/", "/Users/x", "/private/tmp", "----", "abs"},
		// A home that only shares a prefix with cwd does not hold it.
		{"sibling of home", "/Users/xy/proj", "/Users/x", "/private/tmp", "--Users-xy-proj--", "abs"},
		// omp takes any relative path starting with ".." as outside, so a
		// folder named "..cache" in home gets the absolute form.
		{"dot-dot folder in home", "/Users/x/..cache/proj", "/Users/x", "/private/tmp", "--Users-x-..cache-proj--", "abs"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			folder, scope := ompSessionFolder(test.cwd, test.home, test.tempRoot)
			if folder != test.want || scope != test.wantScope {
				t.Errorf("ompSessionFolder(%q, %q, %q) = %q, %q; want %q, %q",
					test.cwd, test.home, test.tempRoot, folder, scope, test.want, test.wantScope)
			}
		})
	}
}

func TestOmpHashedSessionFolder(t *testing.T) {
	// The digests are `printf '%s' <path> | shasum -a 256`.
	tests := []struct {
		cwd   string
		scope string
		want  string
	}{
		{"/Users/x/proj", "home", "home-proj-25cf7293a3713b733a65527700cf8e7877f7399faa40b6e0e524bdc17b0e3092"},
		{"/srv/my app", "abs", "abs-my-app-8945ea3e2e67b4f6c1b54d15e0fc096eb85fb2c54a21fceae81a079a1458563a"},
		{"/", "abs", "abs-project-8a5edab282632443219e051e4ade2d1d5bbc671c781051bf1437897cbdfea0f1"},
	}
	for _, test := range tests {
		if got := ompHashedSessionFolder(test.cwd, test.scope); got != test.want {
			t.Errorf("ompHashedSessionFolder(%q, %q) = %q, want %q", test.cwd, test.scope, got, test.want)
		}
	}
}

func TestOmpTitleSlotFixtureIsFixedWidth(t *testing.T) {
	if got := len(ompTitleSlot(t, ompTestTitle)); got != 256 {
		t.Fatalf("title slot is %d bytes, want 256", got)
	}
}

func TestDetectOmp_FindsSessionUnderHome(t *testing.T) {
	home, _ := isolateOmp(t)
	cwd := makeOmpProject(t, filepath.Join(home, "src", "proj"))
	writeOmpSession(t, filepath.Join(defaultOmpRoot(home), "-src-proj"), "2026-11-10T09-00-00-000Z",
		ompTestSessionID, cwd, true, time.Now())

	session := detectOmp(cwd, "")

	want := Session{Tool: "omp", CWD: cwd, Command: "omp --resume " + ompTestSessionID}
	if session == nil || *session != want {
		t.Errorf("detectOmp = %+v, want %+v", session, want)
	}
}

func TestDetectOmp_FindsSessionOutsideHome(t *testing.T) {
	home, _ := isolateOmp(t)

	// A third temporary directory is neither under HOME nor under TMPDIR.
	cwd := makeOmpProject(t, filepath.Join(resolvedPath(t.TempDir()), "proj"))
	folder := "--" + strings.ReplaceAll(strings.TrimPrefix(cwd, "/"), "/", "-") + "--"
	writeOmpSession(t, filepath.Join(defaultOmpRoot(home), folder), "2026-11-10T09-00-00-000Z",
		ompTestSessionID, cwd, true, time.Now())

	session := detectOmp(cwd, "")

	if session == nil || session.Command != "omp --resume "+ompTestSessionID {
		t.Errorf("detectOmp = %+v, want the session in %s", session, folder)
	}
}

func TestDetectOmp_FindsSessionUnderTempRoot(t *testing.T) {
	home, tempRoot := isolateOmp(t)
	cwd := makeOmpProject(t, filepath.Join(tempRoot, "scratch", "proj"))
	writeOmpSession(t, filepath.Join(defaultOmpRoot(home), "-tmp-scratch-proj"), "2026-11-10T09-00-00-000Z",
		ompTestSessionID, cwd, true, time.Now())

	session := detectOmp(cwd, "")

	if session == nil || session.Command != "omp --resume "+ompTestSessionID {
		t.Errorf("detectOmp = %+v, want the session in -tmp-scratch-proj", session)
	}
}

func TestDetectOmp_ReadsLegacyFileWithoutTitleSlot(t *testing.T) {
	home, _ := isolateOmp(t)
	cwd := makeOmpProject(t, filepath.Join(home, "proj"))
	writeOmpSession(t, filepath.Join(defaultOmpRoot(home), "-proj"), "2026-11-10T09-00-00-000Z",
		ompTestSessionID, cwd, false, time.Now())

	session := detectOmp(cwd, "")

	if session == nil || session.Command != "omp --resume "+ompTestSessionID {
		t.Errorf("detectOmp = %+v, want the legacy session resumed", session)
	}
}

func TestDetectOmp_NewestOfTwo(t *testing.T) {
	home, _ := isolateOmp(t)
	cwd := makeOmpProject(t, filepath.Join(home, "proj"))
	folder := filepath.Join(defaultOmpRoot(home), "-proj")
	now := time.Now()

	// The session created first was used last: omp's findMostRecentSession
	// goes by modification time, not by the timestamp in the name.
	writeOmpSession(t, folder, "2026-11-09T08-00-00-000Z", "019c625b-0000-7000-8000-000000000001", cwd, true, now)
	writeOmpSession(t, folder, "2026-11-10T08-00-00-000Z", "019c625b-0000-7000-8000-000000000002", cwd, true, now.Add(-time.Hour))

	session := detectOmp(cwd, "")

	if session == nil || session.Command != "omp --resume 019c625b-0000-7000-8000-000000000001" {
		t.Errorf("detectOmp = %+v, want the most recently written session", session)
	}
}

func TestDetectOmp_HeaderCWDMismatchIgnored(t *testing.T) {
	home, _ := isolateOmp(t)
	cwd := makeOmpProject(t, filepath.Join(home, "a", "b"))

	// ~/a/b and ~/a-b share the folder -a-b; the header is the only thing
	// that says whose session it is.
	writeOmpSession(t, filepath.Join(defaultOmpRoot(home), "-a-b"), "2026-11-10T09-00-00-000Z",
		ompTestSessionID, filepath.Join(home, "a-b"), true, time.Now())

	session := detectOmp(cwd, "")

	want := Session{Tool: "omp", CWD: cwd, Command: "omp"}
	if session == nil || *session != want {
		t.Errorf("detectOmp = %+v, want %+v", session, want)
	}
}

func TestDetectOmp_HeaderWithLinkedSpelling(t *testing.T) {
	home, _ := isolateOmp(t)
	cwd := makeOmpProject(t, filepath.Join(home, "proj"))
	link := filepath.Join(home, "link")
	if err := os.Symlink(cwd, link); err != nil {
		t.Fatal(err)
	}

	// omp files the session under the real directory, but the header keeps
	// the spelling it was started from.
	writeOmpSession(t, filepath.Join(defaultOmpRoot(home), "-proj"), "2026-11-10T09-00-00-000Z",
		ompTestSessionID, link, true, time.Now())

	session := detectOmp(cwd, "")

	if session == nil || session.Command != "omp --resume "+ompTestSessionID {
		t.Errorf("detectOmp = %+v, want the session recorded through the link", session)
	}
}

func TestDetectOmp_SkipsFilesWithoutHeader(t *testing.T) {
	home, _ := isolateOmp(t)
	cwd := makeOmpProject(t, filepath.Join(home, "proj"))
	folder := filepath.Join(defaultOmpRoot(home), "-proj")
	writeOmpSession(t, folder, "2026-11-10T08-00-00-000Z", ompTestSessionID, cwd, true, time.Now().Add(-time.Hour))

	// Newer files omp could not open: a title slot followed by a message,
	// and a header with an id crex would not type into a shell.
	broken := filepath.Join(folder, "2026-11-10T09-00-00-000Z_broken.jsonl")
	brokenContent := ompTitleSlot(t, ompTestTitle) + `{"type":"message","id":"x","cwd":"` + cwd + `"}` + "\n"
	if err := os.WriteFile(broken, []byte(brokenContent), 0o644); err != nil {
		t.Fatal(err)
	}
	writeOmpSession(t, folder, "2026-11-10T10-00-00-000Z", "bad;id", cwd, true, time.Now())

	session := detectOmp(cwd, "")

	if session == nil || session.Command != "omp --resume "+ompTestSessionID {
		t.Errorf("detectOmp = %+v, want the one valid session", session)
	}
}

func TestDetectOmp_HashedFolderFromOlderRelease(t *testing.T) {
	home, _ := isolateOmp(t)
	cwd := makeOmpProject(t, filepath.Join(home, "proj"))
	writeOmpSession(t, filepath.Join(defaultOmpRoot(home), ompHashedSessionFolder(cwd, "home")),
		"2026-11-10T09-00-00-000Z", ompTestSessionID, cwd, true, time.Now())

	session := detectOmp(cwd, "")

	if session == nil || session.Command != "omp --resume "+ompTestSessionID {
		t.Errorf("detectOmp = %+v, want the session left in the hashed folder", session)
	}
}

func TestDetectOmp_SessionDirFromProcessEnvironment(t *testing.T) {
	home, _ := isolateOmp(t)
	cwd := makeOmpProject(t, filepath.Join(home, "proj"))
	sessionDir := filepath.Join(resolvedPath(t.TempDir()), "omp sessions")
	stubProcessEnv(t, map[string]map[string]string{"42": {piSessionDirVariable: sessionDir}})

	// A session dir holds the files of every project side by side.
	writeOmpSession(t, sessionDir, "2026-11-10T10-00-00-000Z", "019c625b-0000-7000-8000-0000000000c1", filepath.Join(home, "other"), true, time.Now())
	writeOmpSession(t, sessionDir, "2026-11-10T09-00-00-000Z", "019c625b-0000-7000-8000-0000000000c2", cwd, true, time.Now().Add(-time.Minute))

	session := detectOmp(cwd, "42")

	want := "PI_CODING_AGENT_SESSION_DIR='" + sessionDir + "' omp --resume 019c625b-0000-7000-8000-0000000000c2"
	if session == nil || session.Command != want {
		t.Errorf("detectOmp = %+v, want Command %q", session, want)
	}
}

func TestDetectOmp_ProfileIgnoresAgentDir(t *testing.T) {
	home, _ := isolateOmp(t)
	cwd := makeOmpProject(t, filepath.Join(home, "proj"))

	// OMP_PROFILE wins over PI_PROFILE, and a named profile ignores
	// PI_CODING_AGENT_DIR (docs/config-usage.md, "Profiles").
	stubProcessEnv(t, map[string]map[string]string{"42": {
		ompProfileVariable:       "work",
		ompLegacyProfileVariable: "old",
		piAgentDirVariable:       filepath.Join(home, "ignored"),
	}})
	writeOmpSession(t, filepath.Join(home, ".omp", "profiles", "work", "agent", "sessions", "-proj"),
		"2026-11-10T09-00-00-000Z", ompTestSessionID, cwd, true, time.Now())

	session := detectOmp(cwd, "42")

	want := "OMP_PROFILE=work omp --resume " + ompTestSessionID
	if session == nil || session.Command != want {
		t.Errorf("detectOmp = %+v, want Command %q", session, want)
	}
}

func TestDetectOmp_AgentDirFromCrexEnvironment(t *testing.T) {
	home, _ := isolateOmp(t)
	cwd := makeOmpProject(t, filepath.Join(home, "proj"))
	agentDir := filepath.Join(resolvedPath(t.TempDir()), "omp-agent")
	t.Setenv(piAgentDirVariable, agentDir)
	writeOmpSession(t, filepath.Join(agentDir, "sessions", "-proj"), "2026-11-10T09-00-00-000Z",
		ompTestSessionID, cwd, true, time.Now())

	session := detectOmp(cwd, "")

	want := "PI_CODING_AGENT_DIR=" + agentDir + " omp --resume " + ompTestSessionID
	if session == nil || session.Command != want {
		t.Errorf("detectOmp = %+v, want Command %q", session, want)
	}
}

func TestDetectOmp_DataHomeWhenItExists(t *testing.T) {
	home, _ := isolateOmp(t)
	cwd := makeOmpProject(t, filepath.Join(home, "proj"))
	dataHome := filepath.Join(home, ".local", "share")
	t.Setenv(ompDataHomeVariable, dataHome)

	// XDG drops the agent folder: ~/.omp/agent/sessions becomes
	// $XDG_DATA_HOME/omp/sessions, once `omp config init-xdg` created it.
	writeOmpSession(t, filepath.Join(dataHome, "omp", "sessions", "-proj"), "2026-11-10T09-00-00-000Z",
		ompTestSessionID, cwd, true, time.Now())

	session := detectOmp(cwd, "")

	want := "XDG_DATA_HOME=" + dataHome + " omp --resume " + ompTestSessionID
	if session == nil || session.Command != want {
		t.Errorf("detectOmp = %+v, want Command %q", session, want)
	}
}

func TestDetectOmp_NoSessionsRelaunchesOmp(t *testing.T) {
	home, _ := isolateOmp(t)
	cwd := makeOmpProject(t, filepath.Join(home, "proj"))

	session := detectOmp(cwd, "")

	want := Session{Tool: "omp", CWD: cwd, Command: "omp"}
	if session == nil || *session != want {
		t.Errorf("detectOmp = %+v, want %+v", session, want)
	}
}
