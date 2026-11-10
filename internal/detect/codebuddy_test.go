package detect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeCodeBuddyTranscript creates a CodeBuddy Code transcript whose lines
// record cwd, and dates it at modTime so tests can order sessions.
//
// The layout and the line shape come from the @tencent-ai/codebuddy-code
// 2.182.0 bundle (dist/codebuddy.js): SessionStore.getSessionFilePath writes
// <config>/projects/<compressPath(cwd)>/<sessionId>.jsonl, and
// transformItemForSaveAsync stamps cwd on every history item except the
// "summary" and "topic" ones. https://www.codebuddy.ai/docs/cli/codebuddy-dir
// describes projects/ as the per-project session records.
func writeCodeBuddyTranscript(t *testing.T, projectDir, sessionID, cwd string, modTime time.Time) {
	t.Helper()

	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// A topic line carries no cwd, so the detector has to look past it.
	lines := []string{
		`{"type":"topic","id":"t1","topic":"Fix the login form"}`,
		`{"id":"m1","type":"message","role":"user","content":[{"type":"input_text","text":"hello"}],"timestamp":1762790400000,"cwd":"` + cwd + `"}`,
	}
	path := filepath.Join(projectDir, sessionID+".jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatal(err)
	}
}

// codeBuddyProjects returns the default projects directory under a HOME.
func codeBuddyProjects(home string) string {
	return filepath.Join(home, ".codebuddy", "projects")
}

func TestDetectCodeBuddy_FindsSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	writeCodeBuddyTranscript(t, filepath.Join(codeBuddyProjects(home), "src-app"), "6f1c2a9e-0b7d-4c1e-9a55-2d0f3b8e7c41", "/src/app", time.Now())

	s := detectCodeBuddy("/src/app", "")

	want := &Session{Tool: "codebuddy", CWD: "/src/app", Command: "codebuddy --resume 6f1c2a9e-0b7d-4c1e-9a55-2d0f3b8e7c41"}
	if s == nil || *s != *want {
		t.Fatalf("got %+v, want %+v", s, want)
	}
}

func TestDetectCodeBuddy_NewestOfTwo(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	projectDir := filepath.Join(codeBuddyProjects(home), "src-app")
	now := time.Now()
	writeCodeBuddyTranscript(t, projectDir, "older", "/src/app", now.Add(-time.Hour))
	writeCodeBuddyTranscript(t, projectDir, "newer", "/src/app", now)

	s := detectCodeBuddy("/src/app", "")

	if s == nil || s.Command != "codebuddy --resume newer" {
		t.Fatalf("got %+v, want codebuddy --resume newer", s)
	}
}

func TestDetectCodeBuddy_IgnoresOtherDirectories(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	now := time.Now()

	// Another project, more recent, in its own folder.
	writeCodeBuddyTranscript(t, filepath.Join(codeBuddyProjects(home), "src-other"), "other", "/src/other", now)

	// "/src/my-app" and "/src/my/app" share the folder "src-my-app". The
	// newer transcript belongs to the other path and must be passed over.
	shared := filepath.Join(codeBuddyProjects(home), "src-my-app")
	writeCodeBuddyTranscript(t, shared, "mine", "/src/my-app", now.Add(-2*time.Hour))
	writeCodeBuddyTranscript(t, shared, "nested", "/src/my/app", now.Add(-time.Hour))

	s := detectCodeBuddy("/src/my-app", "")

	if s == nil || s.Command != "codebuddy --resume mine" {
		t.Fatalf("got %+v, want codebuddy --resume mine", s)
	}
}

func TestDetectCodeBuddy_SkipsSubagentTranscripts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	projectDir := filepath.Join(codeBuddyProjects(home), "src-app")
	now := time.Now()
	writeCodeBuddyTranscript(t, projectDir, "main", "/src/app", now.Add(-time.Hour))

	// Sub-agent sessions are named agent-<id>. With a parent they live in
	// <parent>/subagents/, without one next to the parent's transcript
	// (SessionStore.getSessionFilePath in the 2.182.0 bundle). Neither is
	// something to resume.
	writeCodeBuddyTranscript(t, projectDir, "agent-loose", "/src/app", now)
	writeCodeBuddyTranscript(t, filepath.Join(projectDir, "main", "subagents"), "agent-child", "/src/app", now)

	s := detectCodeBuddy("/src/app", "")

	if s == nil || s.Command != "codebuddy --resume main" {
		t.Fatalf("got %+v, want codebuddy --resume main", s)
	}
}

func TestDetectCodeBuddy_CustomConfigDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configDir := filepath.Join(home, ".codebuddy-work")
	writeCodeBuddyTranscript(t, filepath.Join(configDir, "projects", "src-app"), "work-session", "/src/app", time.Now())

	// resolveCliHomeDir in the bundle takes CODEBUDDY_CONFIG_DIR as the
	// whole base, so projects/ hangs straight off it.
	stubProcessEnv(t, map[string]map[string]string{"42": {"CODEBUDDY_CONFIG_DIR": configDir}})

	s := detectCodeBuddy("/src/app", "42")

	want := "CODEBUDDY_CONFIG_DIR=" + configDir + " codebuddy --resume work-session"
	if s == nil || s.Command != want {
		t.Fatalf("got %+v, want %q", s, want)
	}
}

func TestDetectCodeBuddy_ResolvesSymlinkedCwd(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)

	// CodeBuddy names the folder after realpath(cwd) and records the same
	// path in the lines, while crex may hold the symlinked one.
	realDir := filepath.Join(t.TempDir(), "real")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	realDir, err := filepath.EvalSymlinks(realDir)
	if err != nil {
		t.Fatal(err)
	}
	linkDir := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Fatal(err)
	}
	writeCodeBuddyTranscript(t, filepath.Join(codeBuddyProjects(home), codeBuddyProjectName(realDir)), "linked", realDir, time.Now())

	s := detectCodeBuddy(linkDir, "")

	if s == nil || s.Command != "codebuddy --resume linked" || s.CWD != linkDir {
		t.Fatalf("got %+v, want codebuddy --resume linked in %s", s, linkDir)
	}
}

func TestDetectCodeBuddy_NoSessionRelaunchesBare(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)

	s := detectCodeBuddy("/src/app", "")

	want := &Session{Tool: "codebuddy", CWD: "/src/app", Command: "codebuddy"}
	if s == nil || *s != *want {
		t.Fatalf("got %+v, want %+v", s, want)
	}
}

func TestCodeBuddyProjectName(t *testing.T) {
	// Expected values were produced by running compressWorkspacePathName,
	// copied verbatim from the 2.182.0 bundle, under node.
	tests := []struct {
		cwd  string
		want string
	}{
		{"/Users/me/src/app.io", "Users-me-src-app.io"},
		{"/a//b/", "a-b"},
		{`/src/my_app:x\y`, "src-my_app-x-y"},
		{
			"/src/" + strings.Repeat("deep/", 60) + "app",
			"src-" + strings.Repeat("deep-", 35) + "d-3a99kb",
		},
		{
			// The 180-byte cut lands just after a "-", so the hash is
			// joined with a second one.
			"/src/" + strings.Repeat("ñandú/", 40) + "app",
			"src-" + strings.Repeat("ñandú-", 22) + "-1r1o9kb",
		},
	}
	for _, tt := range tests {
		if got := codeBuddyProjectName(tt.cwd); got != tt.want {
			t.Errorf("codeBuddyProjectName(%q) = %q, want %q", tt.cwd, got, tt.want)
		}
	}
}
