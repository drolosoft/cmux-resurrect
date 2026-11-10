package detect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The fixtures follow MoonshotAI/kimi-code at main on 2026-11-10 and its docs
// (docs/en/guides/sessions.md, docs/en/configuration/data-locations.md):
// sessions live in <home>/sessions/<workDirKey>/<sessionId>/, where <home> is
// $KIMI_CODE_HOME or ~/.kimi-code (resolveKimiHome in
// packages/agent-core-v2/src/app/bootstrap/bootstrap.ts). Each session folder
// holds state.json and agents/main/wire.jsonl; sub-agents write their own
// wire.jsonl under agents/<agentId>/ inside the same folder, so they never
// show up as sessions of their own. Session ids are session_<uuid>
// (createSessionId in workspace/sessionLifecycle/sessionLifecycleService.ts).

// kimiSessionID is shaped like the ids Kimi Code gives its sessions.
const kimiSessionID = "session_3f6c1e2a-9b4d-4c1e-8a7f-2d5b6c7e8f90"

// kimiSession describes one session folder for writeKimiSession.
type kimiSession struct {
	key       string    // the workDirKey folder it sits in
	id        string    // the session folder name
	cwd       string    // the cwd recorded in state.json
	updatedAt time.Time // state.json updatedAt; zero leaves the field out
	archived  bool
	modified  time.Time // modification time of wire.jsonl and state.json
}

// writeKimiSession creates <home>/sessions/<key>/<id>/ with a state.json in
// the shape SessionMetadata writes it (sessionMetadataService.ts: id,
// version, cwd, createdAt and updatedAt in epoch milliseconds, archived,
// title) and an agents/main/wire.jsonl. The modification times are set
// explicitly so "newest" does not depend on how fast the test runs.
func writeKimiSession(t *testing.T, home string, session kimiSession) {
	t.Helper()
	sessionDir := filepath.Join(home, "sessions", session.key, session.id)
	wireDir := filepath.Join(sessionDir, "agents", "main")
	if err := os.MkdirAll(wireDir, 0o755); err != nil {
		t.Fatal(err)
	}

	state := map[string]any{
		"id":            session.id,
		"version":       1,
		"cwd":           session.cwd,
		"createdAt":     session.modified.Add(-time.Hour).UnixMilli(),
		"archived":      session.archived,
		"title":         "fixture",
		"titleKind":     "generated",
		"isCustomTitle": false,
		"agents":        map[string]any{},
		"custom":        map[string]any{},
	}
	if !session.updatedAt.IsZero() {
		state["updatedAt"] = session.updatedAt.UnixMilli()
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}

	statePath := filepath.Join(sessionDir, "state.json")
	wirePath := filepath.Join(wireDir, "wire.jsonl")
	writeKimiFile(t, statePath, data, session.modified)
	writeKimiFile(t, wirePath, []byte(`{"type":"turn_begin"}`+"\n"), session.modified)
}

// writeKimiFile writes a file and sets its modification time.
func writeKimiFile(t *testing.T, path string, data []byte, modified time.Time) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modified, modified); err != nil {
		t.Fatal(err)
	}
}

// kimiHome points HOME at a fresh directory with no process environment and
// returns the default data root, ~/.kimi-code.
func kimiHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubProcessEnv(t, nil)
	return filepath.Join(home, ".kimi-code")
}

// The expected keys were produced by running encodeWorkDirKey, copied
// verbatim from packages/agent-core-v2/src/_base/utils/workdir-slug.ts, under
// Node 23, and every hash was checked again with
// `printf '%s' <path> | shasum -a 256`, so the Go port is tested against Kimi
// and not against itself.
func TestKimiWorkDirKey(t *testing.T) {
	tests := []struct {
		cwd  string
		want string
	}{
		{"/src/app", "wd_app_c0167fc91cc6"},
		// Trailing slashes are dropped before hashing.
		{"/src/app/", "wd_app_c0167fc91cc6"},
		// Runs of other characters become one "-", trimmed at both ends.
		{"/Users/me/My Project (v2)!", "wd_my-project-v2_8570de993af4"},
		// Cut to 40 characters.
		{"/src/A_Very.Long Folder Name That Goes Past The Forty Character Limit", "wd_a_very.long-folder-name-that-goes-past-t_7f6df701bcb3"},
		// The cut leaves a "-" at the end, which is trimmed again.
		{"/src/" + strings.Repeat("a", 39) + " bbbb", "wd_" + strings.Repeat("a", 39) + "_903ce5e45af2"},
		{"/src/café", "wd_caf_af36d8f05057"},
		// JavaScript lowercases İ to i plus a combining dot, which becomes "-".
		{"/src/İstanbul", "wd_i-stanbul_ba215c49a666"},
		// A slug of "..", or none at all, is called workspace.
		{"/src/..", "wd_workspace_9cfa712dcb47"},
		{"/", "wd_workspace_e3b0c44298fc"},
	}
	for _, test := range tests {
		if got := kimiWorkDirKey(test.cwd); got != test.want {
			t.Errorf("kimiWorkDirKey(%q)\n got %q\nwant %q", test.cwd, got, test.want)
		}
	}
}

func TestDetectKimi_ResumesSession(t *testing.T) {
	home := kimiHome(t)
	now := time.Now()
	writeKimiSession(t, home, kimiSession{key: "wd_app_c0167fc91cc6", id: kimiSessionID, cwd: "/src/app", updatedAt: now, modified: now})

	s := detectKimi("/src/app", "")
	want := Session{Tool: "kimi", CWD: "/src/app", Command: "kimi --session " + kimiSessionID}
	if s == nil || *s != want {
		t.Fatalf("got %+v, want %+v", s, want)
	}
}

func TestDetectKimi_NewestOfTwoByUpdatedAt(t *testing.T) {
	home := kimiHome(t)
	now := time.Now()
	// updatedAt is what `kimi --continue` sorts by, so it wins over the
	// file times, which here say the opposite.
	writeKimiSession(t, home, kimiSession{key: "wd_app_c0167fc91cc6", id: "session_older", cwd: "/src/app", updatedAt: now.Add(-time.Hour), modified: now})
	writeKimiSession(t, home, kimiSession{key: "wd_app_c0167fc91cc6", id: "session_newer", cwd: "/src/app", updatedAt: now, modified: now.Add(-time.Hour)})

	s := detectKimi("/src/app", "")
	if s == nil || s.Command != "kimi --session session_newer" {
		t.Fatalf("got %+v, want kimi --session session_newer", s)
	}
}

func TestDetectKimi_WithoutUpdatedAtUsesWireTime(t *testing.T) {
	home := kimiHome(t)
	now := time.Now()
	writeKimiSession(t, home, kimiSession{key: "wd_app_c0167fc91cc6", id: "session_older", cwd: "/src/app", modified: now.Add(-time.Hour)})
	writeKimiSession(t, home, kimiSession{key: "wd_app_c0167fc91cc6", id: "session_newer", cwd: "/src/app", modified: now})

	s := detectKimi("/src/app", "")
	if s == nil || s.Command != "kimi --session session_newer" {
		t.Fatalf("got %+v, want kimi --session session_newer", s)
	}
}

func TestDetectKimi_OtherDirectoryIgnored(t *testing.T) {
	home := kimiHome(t)
	now := time.Now()
	writeKimiSession(t, home, kimiSession{key: "wd_app_c0167fc91cc6", id: "session_mine", cwd: "/src/app", updatedAt: now.Add(-time.Hour), modified: now})

	// A newer session of another directory, in its own folder.
	writeKimiSession(t, home, kimiSession{key: "wd_my-project-v2_8570de993af4", id: "session_other", cwd: "/Users/me/My Project (v2)!", updatedAt: now, modified: now})

	// A newer session in this directory's folder that recorded another cwd:
	// `kimi --session` refuses one "created under a different directory".
	writeKimiSession(t, home, kimiSession{key: "wd_app_c0167fc91cc6", id: "session_moved", cwd: "/elsewhere/app", updatedAt: now, modified: now})

	s := detectKimi("/src/app", "")
	if s == nil || s.Command != "kimi --session session_mine" {
		t.Fatalf("got %+v, want kimi --session session_mine", s)
	}
}

func TestDetectKimi_ArchivedSkipped(t *testing.T) {
	home := kimiHome(t)
	now := time.Now()
	writeKimiSession(t, home, kimiSession{key: "wd_app_c0167fc91cc6", id: "session_active", cwd: "/src/app", updatedAt: now.Add(-time.Hour), modified: now})
	writeKimiSession(t, home, kimiSession{key: "wd_app_c0167fc91cc6", id: "session_archived", cwd: "/src/app", updatedAt: now, archived: true, modified: now})

	s := detectKimi("/src/app", "")
	if s == nil || s.Command != "kimi --session session_active" {
		t.Fatalf("got %+v, want kimi --session session_active", s)
	}
}

func TestDetectKimi_SluggedAndCutBasename(t *testing.T) {
	home := kimiHome(t)
	now := time.Now()
	cwd := "/src/A_Very.Long Folder Name That Goes Past The Forty Character Limit"
	writeKimiSession(t, home, kimiSession{key: "wd_a_very.long-folder-name-that-goes-past-t_7f6df701bcb3", id: kimiSessionID, cwd: cwd, updatedAt: now, modified: now})

	s := detectKimi(cwd, "")
	if s == nil || s.Command != "kimi --session "+kimiSessionID {
		t.Fatalf("got %+v, want kimi --session %s", s, kimiSessionID)
	}
}

func TestDetectKimi_LegacyStateLocation(t *testing.T) {
	home := kimiHome(t)
	now := time.Now()

	// Older Kimi Code builds kept state.json in a session-meta folder, with
	// workDir for the cwd and ISO dates; Kimi still reads it
	// (legacySessionMetaScopeOf and normalizeSessionMeta).
	sessionDir := filepath.Join(home, "sessions", "wd_app_c0167fc91cc6", "session_legacy")
	if err := os.MkdirAll(filepath.Join(sessionDir, "session-meta"), 0o755); err != nil {
		t.Fatal(err)
	}
	state := `{"id":"session_legacy","workDir":"/src/app","updatedAt":"` + now.UTC().Format(time.RFC3339Nano) + `"}`
	writeKimiFile(t, filepath.Join(sessionDir, "session-meta", "state.json"), []byte(state), now.Add(-2*time.Hour))

	// An older session in the current layout must lose to it.
	writeKimiSession(t, home, kimiSession{key: "wd_app_c0167fc91cc6", id: "session_current", cwd: "/src/app", updatedAt: now.Add(-time.Hour), modified: now})

	s := detectKimi("/src/app", "")
	if s == nil || s.Command != "kimi --session session_legacy" {
		t.Fatalf("got %+v, want kimi --session session_legacy", s)
	}
}

func TestDetectKimi_HomeFromProcessEnv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	kimiDir := filepath.Join(home, "kimi work")
	now := time.Now()
	writeKimiSession(t, kimiDir, kimiSession{key: "wd_app_c0167fc91cc6", id: kimiSessionID, cwd: "/src/app", updatedAt: now, modified: now})
	stubProcessEnv(t, map[string]map[string]string{"42": {"KIMI_CODE_HOME": kimiDir}})

	s := detectKimi("/src/app", "42")
	want := "KIMI_CODE_HOME='" + kimiDir + "' kimi --session " + kimiSessionID
	if s == nil || s.Command != want {
		t.Fatalf("got %+v, want command %q", s, want)
	}
}

func TestDetectKimi_DefaultHomeSpelledOutNeedsNoPrefix(t *testing.T) {
	home := kimiHome(t)
	now := time.Now()
	writeKimiSession(t, home, kimiSession{key: "wd_app_c0167fc91cc6", id: kimiSessionID, cwd: "/src/app", updatedAt: now, modified: now})
	stubProcessEnv(t, map[string]map[string]string{"42": {"KIMI_CODE_HOME": home + "/"}})

	s := detectKimi("/src/app", "42")
	if s == nil || s.Command != "kimi --session "+kimiSessionID {
		t.Fatalf("got %+v, want kimi --session %s", s, kimiSessionID)
	}
}

func TestDetectKimi_NoSessionRelaunchesBare(t *testing.T) {
	home := kimiHome(t)
	bare := Session{Tool: "kimi", CWD: "/src/app", Command: "kimi"}

	// No ~/.kimi-code at all, only the store of the archived Kimi CLI, which
	// ran as the same binary but resumes differently (MoonshotAI/kimi-cli).
	legacyStore := filepath.Join(filepath.Dir(home), ".kimi", "sessions")
	if err := os.MkdirAll(legacyStore, 0o755); err != nil {
		t.Fatal(err)
	}
	if s := detectKimi("/src/app", ""); s == nil || *s != bare {
		t.Errorf("no store: got %+v, want %+v", s, bare)
	}

	// A folder whose only sessions are another directory's, one without a
	// readable state.json, and one whose name is not a safe session id.
	now := time.Now()
	writeKimiSession(t, home, kimiSession{key: "wd_app_c0167fc91cc6", id: "session_other", cwd: "/src/app/other", updatedAt: now, modified: now})
	writeKimiSession(t, home, kimiSession{key: "wd_app_c0167fc91cc6", id: "x;rm -rf ~", cwd: "/src/app", updatedAt: now, modified: now})
	if err := os.MkdirAll(filepath.Join(home, "sessions", "wd_app_c0167fc91cc6", "session_empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if s := detectKimi("/src/app", ""); s == nil || *s != bare {
		t.Errorf("no matching session: got %+v, want %+v", s, bare)
	}
}
