package detect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The folder names in these fixtures were produced by running Gf, copied
// verbatim from @qoder-ai/qodercli 1.1.86 (bundle/qodercli.js), under Node,
// so the tests check the Go port against Qoder itself and not against itself.
// Gf replaces every non-alphanumeric UTF-16 unit with "-"; a name longer than
// 200 is cut to 200 and gets "-" plus the base-36 djb2-xor hash of the cwd.

// qoderLongCWD is 212 characters long, so its folder name is cut and hashed.
var qoderLongCWD = "/src/" + strings.Repeat("nested-directory/", 12) + "app"

// qoderLongTwinCWD differs from qoderLongCWD only in characters Gf turns into
// "-", so both share the same 200-character prefix and only the hash differs.
var qoderLongTwinCWD = "/src/" + strings.Repeat("nested_directory/", 12) + "app"

// qoderLongPrefix is the first 200 characters of the encoded long paths.
const qoderLongPrefix = "-src-nested-directory-nested-directory-nested-directory-nested-directory-nested-directory-nested-directory-nested-directory-nested-directory-nested-directory-nested-directory-nested-directory-nested-d"

// writeQoderTranscript creates <configDir>/projects/<dirName>/<sessionID>.jsonl
// in the shape Qoder writes it: a metadata line without cwd, then message
// lines that carry sessionId and cwd. The modification time is set
// explicitly so "newest" does not depend on how fast the test runs.
func writeQoderTranscript(t *testing.T, configDir, dirName, sessionID, cwd string, modified time.Time) {
	t.Helper()
	projectDir := filepath.Join(configDir, "projects", dirName)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	lines := []string{
		`{"type":"custom-title","customTitle":"fixture","sessionId":"` + sessionID + `"}`,
		`{"type":"user","sessionId":"` + sessionID + `","cwd":"` + cwd + `","version":"1.1.86","message":{"role":"user","content":"hello"}}`,
	}
	path := filepath.Join(projectDir, sessionID+".jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modified, modified); err != nil {
		t.Fatal(err)
	}
}

// qoderHome points HOME at a fresh directory with no process environment
// and returns the default config dir, ~/.qoder.
func qoderHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	return filepath.Join(home, ".qoder")
}

func TestQoderProjectDirName(t *testing.T) {
	tests := []struct {
		cwd  string
		want string
	}{
		{"/src/app.io", "-src-app-io"},
		{"/src/my_app", "-src-my-app"},
		{qoderLongCWD, qoderLongPrefix + "-5mrmm2"},
		{qoderLongTwinCWD, qoderLongPrefix + "-cz3f3u"},
		// An accented letter is one UTF-16 unit and one "-".
		{"/src/café/" + strings.Repeat("x", 200), "-src-caf--" + strings.Repeat("x", 190) + "-57tyz"},
		// An emoji is two UTF-16 units, so Qoder writes two "-" for it.
		{"/src/🚀/" + strings.Repeat("y", 200), "-src----" + strings.Repeat("y", 192) + "-rj76tn"},
	}
	for _, test := range tests {
		if got := qoderProjectDirName(test.cwd); got != test.want {
			t.Errorf("qoderProjectDirName(%q)\n got %q\nwant %q", test.cwd, got, test.want)
		}
	}
}

func TestDetectQoder_ResumesSession(t *testing.T) {
	configDir := qoderHome(t)
	writeQoderTranscript(t, configDir, "-src-app-io", "3f6c1e2a-9b4d-4c1e-8a7f-2d5b6c7e8f90", "/src/app.io", time.Now())

	s := detectQoder("/src/app.io", "")
	want := Session{Tool: "qoder", CWD: "/src/app.io", Command: "qoder --resume 3f6c1e2a-9b4d-4c1e-8a7f-2d5b6c7e8f90"}
	if s == nil || *s != want {
		t.Fatalf("got %+v, want %+v", s, want)
	}
}

func TestDetectQoder_NewestOfTwo(t *testing.T) {
	configDir := qoderHome(t)
	now := time.Now()
	writeQoderTranscript(t, configDir, "-src-app", "older", "/src/app", now.Add(-time.Hour))
	writeQoderTranscript(t, configDir, "-src-app", "newer", "/src/app", now)

	s := detectQoder("/src/app", "")
	if s == nil || s.Command != "qoder --resume newer" {
		t.Fatalf("got %+v, want qoder --resume newer", s)
	}
}

func TestDetectQoder_LongPathUsesHashedFolder(t *testing.T) {
	configDir := qoderHome(t)
	now := time.Now()
	writeQoderTranscript(t, configDir, qoderLongPrefix+"-5mrmm2", "long-session", qoderLongCWD, now.Add(-time.Hour))
	// The twin's folder shares the prefix and holds a newer session: picking
	// it would mean the hash was ignored.
	writeQoderTranscript(t, configDir, qoderLongPrefix+"-cz3f3u", "twin-session", qoderLongTwinCWD, now)

	s := detectQoder(qoderLongCWD, "")
	if s == nil || s.Command != "qoder --resume long-session" {
		t.Fatalf("got %+v, want qoder --resume long-session", s)
	}
}

func TestDetectQoder_SameFolderToldApartByCWD(t *testing.T) {
	configDir := qoderHome(t)
	now := time.Now()
	// /src/my-app and /src/my_app both encode to -src-my-app, so their
	// sessions share one folder and only the cwd in the transcript says
	// whose each one is.
	writeQoderTranscript(t, configDir, "-src-my-app", "dash-session", "/src/my-app", now.Add(-time.Hour))
	writeQoderTranscript(t, configDir, "-src-my-app", "underscore-session", "/src/my_app", now)

	if s := detectQoder("/src/my-app", ""); s == nil || s.Command != "qoder --resume dash-session" {
		t.Errorf("/src/my-app: got %+v, want qoder --resume dash-session", s)
	}
	if s := detectQoder("/src/my_app", ""); s == nil || s.Command != "qoder --resume underscore-session" {
		t.Errorf("/src/my_app: got %+v, want qoder --resume underscore-session", s)
	}
}

func TestDetectQoder_NoSessionRelaunchesBare(t *testing.T) {
	configDir := qoderHome(t)
	bare := Session{Tool: "qoder", CWD: "/src/app", Command: "qoder"}

	// No projects folder at all.
	if s := detectQoder("/src/app", ""); s == nil || *s != bare {
		t.Errorf("no folder: got %+v, want %+v", s, bare)
	}

	// A folder whose only session belongs to another directory, a transcript
	// without any cwd yet, and a file name that is not a safe session ID.
	writeQoderTranscript(t, configDir, "-src-app", "other-cwd", "/src/app/other", time.Now())
	writeQoderTranscript(t, configDir, "-src-app", "x;rm -rf ~", "/src/app", time.Now())
	empty := filepath.Join(configDir, "projects", "-src-app", "empty.jsonl")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if s := detectQoder("/src/app", ""); s == nil || *s != bare {
		t.Errorf("no matching session: got %+v, want %+v", s, bare)
	}
}

func TestDetectQoder_ConfigDirFromProcessEnv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configDir := filepath.Join(home, "qoder work")
	writeQoderTranscript(t, configDir, "-src-app", "work-session", "/src/app", time.Now())
	stubProcessEnv(t, map[string]map[string]string{"42": {"QODER_CONFIG_DIR": configDir}})

	s := detectQoder("/src/app", "42")
	want := "QODER_CONFIG_DIR='" + configDir + "' qoder --resume work-session"
	if s == nil || s.Command != want {
		t.Fatalf("got %+v, want command %q", s, want)
	}
}

func TestQoderConfigDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	tests := []struct {
		name       string
		env        map[string]string
		wantDir    string
		wantCustom bool
	}{
		{"default", nil, filepath.Join(home, ".qoder"), false},
		{"config dir wins", map[string]string{"QODER_CONFIG_DIR": "/opt/q", "QODER_CLI_HOME": "/ignored"}, "/opt/q", true},
		{"relative config dir is resolved against cwd", map[string]string{"QODER_CONFIG_DIR": "conf"}, "/src/app/conf", true},
		{"cli home", map[string]string{"QODER_CLI_HOME": "/users/work"}, "/users/work/.qoder", true},
		{"dir name", map[string]string{"QODER_CONFIG_DIR_NAME": ".qoder-work"}, filepath.Join(home, ".qoder-work"), true},
		{"unsafe dir name is ignored", map[string]string{"QODER_CONFIG_DIR_NAME": "../escape"}, filepath.Join(home, ".qoder"), false},
		{"default spelled out", map[string]string{"QODER_CONFIG_DIR": filepath.Join(home, ".qoder") + "/"}, filepath.Join(home, ".qoder"), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stubProcessEnv(t, map[string]map[string]string{"7": test.env})
			dir, custom := qoderConfigDir("7", "/src/app")
			if dir != test.wantDir || custom != test.wantCustom {
				t.Errorf("got (%q, %v), want (%q, %v)", dir, custom, test.wantDir, test.wantCustom)
			}
		})
	}
}
