package detect

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The fixtures follow Mistral Vibe's own code (github.com/mistralai/mistral-vibe,
// release 3.1.1 and tag v2.26.1):
//   - vibe/utils/vibe_home.py, get_vibe_home: $VIBE_HOME, default ~/.vibe.
//   - vibe/core/paths/_vibe_home.py: the sessions live in
//     <vibe home>/logs/session (SESSION_LOG_DIR).
//   - vibe/core/config/models.py, SessionLoggingConfig, and
//     vibe/core/config/layers/environment.py: save_dir can be moved from the
//     environment with VIBE_SESSION_LOGGING__SAVE_DIR.
//   - Vibe 2.x, vibe/core/session/session_logger.py at v2.26.1: one folder
//     per session, session_<YYYYMMDD_HHMMSS>_<first 8 of the id>, holding
//     meta.json ({"session_id", "environment": {"working_directory"},
//     "origin_directory", ...}) and messages.jsonl.
//   - vibe/core/session/session_loader.py, latest_session: the newest session
//     is the one whose messages.jsonl changed last.
//   - Vibe 3.x, harness/.../vibe/_storage.py, UnifiedSessionStore: a session
//     is <save dir>/unified/<id>/ with a CURRENT pointer, the documents of each
//     generation in generations/<generation>/ (runtime-state.json carries
//     session_metadata.cwd and identity.kind) and its recovery journal in
//     journal/<snapshot_sequence + 1>.jsonl.
//   - vibe/cli-rust/src/cli.rs and docs.mistral.ai/vibe/code/cli/work-with-cli:
//     `vibe --resume SESSION_ID` resumes one session and works from anywhere.

// vibeTestCWD is the project every Vibe test runs in.
const vibeTestCWD = "/Users/dev/projects/shop"

// isolateVibe points HOME at an empty temporary directory, clears the Vibe
// variables the developer running the tests may have exported and makes the
// process environment unreadable, so only the test decides where Vibe keeps
// its sessions. It returns the session folder Vibe uses by default.
func isolateVibe(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(vibeHomeVariable, "")
	t.Setenv(vibeSaveDirVariable, "")
	stubProcessEnv(t, nil)

	return filepath.Join(home, ".vibe", "logs", "session")
}

// writeVibeLegacySession writes a Vibe 2.x session folder: meta.json with
// the given contents and a messages.jsonl whose modification time is the
// one Vibe sorts sessions by.
func writeVibeLegacySession(t *testing.T, sessionDir, folderName, metadata string, modified time.Time) {
	t.Helper()

	folder := filepath.Join(sessionDir, folderName)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "meta.json"), []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}

	messagesPath := filepath.Join(folder, "messages.jsonl")
	message := `{"role":"user","content":"Fix the login form"}` + "\n"
	if err := os.WriteFile(messagesPath, []byte(message), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(messagesPath, modified, modified); err != nil {
		t.Fatal(err)
	}
}

// vibeLegacyMetadata returns a meta.json body for a session opened in cwd.
func vibeLegacyMetadata(sessionID, cwd string) string {
	return `{"session_id":"` + sessionID + `","start_time":"2026-09-20T10:00:00",` +
		`"environment":{"working_directory":"` + cwd + `"},` +
		`"origin_directory":"` + cwd + `","title":null}`
}

// vibeLegacyFolder names a session folder the way Vibe 2.x does.
func vibeLegacyFolder(sessionID string) string {
	return "session_20260920_100000_" + sessionID[:8]
}

// writeVibeUnifiedSession writes a Vibe 3.x session store under
// <sessionDir>/unified/<id>/ with the given runtime-state.json contents, and
// gives its CURRENT pointer and recovery journal the modification time that
// tells the newest session apart.
func writeVibeUnifiedSession(t *testing.T, sessionDir, sessionID, runtimeState string, modified time.Time) {
	t.Helper()

	sessionRoot := filepath.Join(sessionDir, "unified", sessionID)
	generation := "0000000000000003"
	for _, folder := range []string{"generations/" + generation, "journal"} {
		if err := os.MkdirAll(filepath.Join(sessionRoot, folder), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	pointer := fmt.Sprintf(`{"generation":"%s","manifest_sha256":"%064d","session_id":"%s",`+
		`"snapshot_sequence":3,"store_format":"mistral.vibe.unified-session-store/v1",`+
		`"store_format_minor":10}`, generation, 0, sessionID)
	files := map[string]string{
		"CURRENT": pointer + "\n",
		"generations/" + generation + "/runtime-state.json": runtimeState + "\n",
		"journal/0000000000000004.jsonl":                    "",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(sessionRoot, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for _, name := range []string{"CURRENT", "journal/0000000000000004.jsonl"} {
		if err := os.Chtimes(filepath.Join(sessionRoot, name), modified, modified); err != nil {
			t.Fatal(err)
		}
	}
}

// vibeRuntimeState returns a runtime-state.json body for a session of the
// given kind ("root", "fork" or "subagent") opened in cwd.
func vibeRuntimeState(sessionID, cwd, kind string) string {
	return `{"identity":{"depth":0,"kind":"` + kind + `","parent_session_id":null,` +
		`"root_session_id":"` + sessionID + `","session_id":"` + sessionID + `"},` +
		`"runtime_state_version":3,"session_id":"` + sessionID + `",` +
		`"session_metadata":{"active_model":"devstral-2","cwd":"` + cwd + `",` +
		`"origin_cwd":null,"parent_session_id":null,"root_session_id":"` + sessionID + `"},` +
		`"snapshot_sequence":3,"storage_lifetime":"persistent"}`
}

func TestDetectVibe_FindsUnifiedSession(t *testing.T) {
	sessionDir := isolateVibe(t)
	sessionID := "019a6f2e-4b1c-7d3e-9f80-a1b2c3d4e5f6"
	writeVibeUnifiedSession(t, sessionDir, sessionID, vibeRuntimeState(sessionID, vibeTestCWD, "root"), time.Now())

	session := detectVibe(vibeTestCWD, "")

	want := Session{Tool: "vibe", CWD: vibeTestCWD, Command: "vibe --resume " + sessionID}
	if session == nil || *session != want {
		t.Errorf("detectVibe = %+v, want %+v", session, want)
	}
}

func TestDetectVibe_FindsLegacySession(t *testing.T) {
	sessionDir := isolateVibe(t)
	sessionID := "3f2b8c1e-9d4a-4e7b-a1c2-5d6e7f809a1b"
	writeVibeLegacySession(t, sessionDir, vibeLegacyFolder(sessionID), vibeLegacyMetadata(sessionID, vibeTestCWD), time.Now())

	session := detectVibe(vibeTestCWD, "")

	want := Session{Tool: "vibe", CWD: vibeTestCWD, Command: "vibe --resume " + sessionID}
	if session == nil || *session != want {
		t.Errorf("detectVibe = %+v, want %+v", session, want)
	}
}

func TestDetectVibe_PicksNewestOfTheDirectory(t *testing.T) {
	sessionDir := isolateVibe(t)
	now := time.Now()
	// The newer session sorts first by name, so the folder order cannot
	// pick it by accident.
	newer := "11111111-1111-7111-8111-111111111111"
	older := "22222222-2222-7222-8222-222222222222"
	writeVibeUnifiedSession(t, sessionDir, newer, vibeRuntimeState(newer, vibeTestCWD, "root"), now)
	writeVibeUnifiedSession(t, sessionDir, older, vibeRuntimeState(older, vibeTestCWD, "root"), now.Add(-time.Hour))

	session := detectVibe(vibeTestCWD, "")

	if session == nil || session.Command != "vibe --resume "+newer {
		t.Errorf("detectVibe = %+v, want the session %s", session, newer)
	}
}

func TestDetectVibe_NewestAcrossBothStores(t *testing.T) {
	sessionDir := isolateVibe(t)
	now := time.Now()
	legacy := "3f2b8c1e-9d4a-4e7b-a1c2-5d6e7f809a1b"
	unified := "019a6f2e-4b1c-7d3e-9f80-a1b2c3d4e5f6"
	writeVibeLegacySession(t, sessionDir, vibeLegacyFolder(legacy), vibeLegacyMetadata(legacy, vibeTestCWD), now)
	writeVibeUnifiedSession(t, sessionDir, unified, vibeRuntimeState(unified, vibeTestCWD, "root"), now.Add(-time.Hour))

	session := detectVibe(vibeTestCWD, "")

	if session == nil || session.Command != "vibe --resume "+legacy {
		t.Errorf("detectVibe = %+v, want the legacy session %s", session, legacy)
	}
}

func TestDetectVibe_IgnoresOtherDirectories(t *testing.T) {
	sessionDir := isolateVibe(t)
	now := time.Now()
	ours := "11111111-1111-7111-8111-111111111111"
	theirs := "22222222-2222-7222-8222-222222222222"
	theirsLegacy := "33333333-3333-4333-8333-333333333333"
	writeVibeUnifiedSession(t, sessionDir, ours, vibeRuntimeState(ours, vibeTestCWD, "root"), now.Add(-time.Hour))
	writeVibeUnifiedSession(t, sessionDir, theirs, vibeRuntimeState(theirs, "/Users/dev/projects/blog", "root"), now)
	writeVibeLegacySession(t, sessionDir, vibeLegacyFolder(theirsLegacy), vibeLegacyMetadata(theirsLegacy, "/Users/dev/projects/blog"), now)

	session := detectVibe(vibeTestCWD, "")

	if session == nil || session.Command != "vibe --resume "+ours {
		t.Errorf("detectVibe = %+v, want the session %s", session, ours)
	}
}

func TestDetectVibe_FindsMovedSessionFromItsOrigin(t *testing.T) {
	sessionDir := isolateVibe(t)
	sessionID := "3f2b8c1e-9d4a-4e7b-a1c2-5d6e7f809a1b"

	// A session moved into a worktree keeps the folder it began in as
	// origin_directory, and Vibe still offers it there (_session_reaches in
	// session_loader.py).
	metadata := `{"session_id":"` + sessionID + `",` +
		`"environment":{"working_directory":"/Users/dev/.vibe/worktrees/shop-fix"},` +
		`"origin_directory":"` + vibeTestCWD + `"}`
	writeVibeLegacySession(t, sessionDir, vibeLegacyFolder(sessionID), metadata, time.Now())

	session := detectVibe(vibeTestCWD, "")

	if session == nil || session.Command != "vibe --resume "+sessionID {
		t.Errorf("detectVibe = %+v, want the session %s", session, sessionID)
	}
}

func TestDetectVibe_SkipsMalformedSessions(t *testing.T) {
	sessionDir := isolateVibe(t)
	now := time.Now()
	good := "11111111-1111-7111-8111-111111111111"
	brokenMeta := "22222222-2222-4222-8222-222222222222"
	brokenState := "33333333-3333-7333-8333-333333333333"
	writeVibeUnifiedSession(t, sessionDir, good, vibeRuntimeState(good, vibeTestCWD, "root"), now.Add(-time.Hour))
	writeVibeLegacySession(t, sessionDir, vibeLegacyFolder(brokenMeta), `{"session_id":"`+brokenMeta+`","environment":{"working_dir`, now)
	writeVibeUnifiedSession(t, sessionDir, brokenState, `{"session_id":"`+brokenState, now)

	session := detectVibe(vibeTestCWD, "")

	if session == nil || session.Command != "vibe --resume "+good {
		t.Errorf("detectVibe = %+v, want the session %s", session, good)
	}
}

func TestDetectVibe_SkipsSubagentSessions(t *testing.T) {
	sessionDir := isolateVibe(t)
	now := time.Now()
	root := "11111111-1111-7111-8111-111111111111"
	subagent := "22222222-2222-7222-8222-222222222222"
	writeVibeUnifiedSession(t, sessionDir, root, vibeRuntimeState(root, vibeTestCWD, "root"), now.Add(-time.Hour))
	writeVibeUnifiedSession(t, sessionDir, subagent, vibeRuntimeState(subagent, vibeTestCWD, "subagent"), now)

	session := detectVibe(vibeTestCWD, "")

	if session == nil || session.Command != "vibe --resume "+root {
		t.Errorf("detectVibe = %+v, want the root session %s", session, root)
	}
}

func TestDetectVibe_RejectsUnsafeSessionIDs(t *testing.T) {
	sessionDir := isolateVibe(t)
	unsafe := "abc;rm -rf ~"
	writeVibeLegacySession(t, sessionDir, "session_20260920_100000_abc;rm -r", vibeLegacyMetadata(unsafe, vibeTestCWD), time.Now())

	session := detectVibe(vibeTestCWD, "")

	if session == nil || session.Command != "vibe" {
		t.Errorf("detectVibe = %+v, want the bare command", session)
	}
}

func TestDetectVibe_NoSessionGivesBareCommand(t *testing.T) {
	isolateVibe(t)

	session := detectVibe(vibeTestCWD, "")

	want := Session{Tool: "vibe", CWD: vibeTestCWD, Command: "vibe"}
	if session == nil || *session != want {
		t.Errorf("detectVibe = %+v, want %+v", session, want)
	}
}

func TestDetectVibe_HonoursVibeHome(t *testing.T) {
	isolateVibe(t)
	vibeHome := filepath.Join(t.TempDir(), "vibe home")
	sessionID := "019a6f2e-4b1c-7d3e-9f80-a1b2c3d4e5f6"
	sessionDir := filepath.Join(vibeHome, "logs", "session")
	writeVibeUnifiedSession(t, sessionDir, sessionID, vibeRuntimeState(sessionID, vibeTestCWD, "root"), time.Now())
	stubProcessEnv(t, map[string]map[string]string{"4242": {vibeHomeVariable: vibeHome}})

	session := detectVibe(vibeTestCWD, "4242")

	want := "VIBE_HOME='" + vibeHome + "' vibe --resume " + sessionID
	if session == nil || session.Command != want {
		t.Errorf("detectVibe = %+v, want command %q", session, want)
	}
}

func TestDetectVibe_DefaultVibeHomeNeedsNoPrefix(t *testing.T) {
	defaultDir := isolateVibe(t)
	home := os.Getenv("HOME")
	sessionID := "019a6f2e-4b1c-7d3e-9f80-a1b2c3d4e5f6"
	writeVibeUnifiedSession(t, defaultDir, sessionID, vibeRuntimeState(sessionID, vibeTestCWD, "root"), time.Now())
	stubProcessEnv(t, map[string]map[string]string{"4242": {vibeHomeVariable: "~/.vibe/"}})

	session := detectVibe(vibeTestCWD, "4242")

	if session == nil || session.Command != "vibe --resume "+sessionID {
		t.Errorf("detectVibe = %+v, want no prefix for %s", session, filepath.Join(home, ".vibe"))
	}
}

func TestDetectVibe_HonoursSaveDir(t *testing.T) {
	isolateVibe(t)
	saveDir := filepath.Join(t.TempDir(), "sessions")
	sessionID := "3f2b8c1e-9d4a-4e7b-a1c2-5d6e7f809a1b"
	writeVibeLegacySession(t, saveDir, vibeLegacyFolder(sessionID), vibeLegacyMetadata(sessionID, vibeTestCWD), time.Now())
	stubProcessEnv(t, map[string]map[string]string{"4242": {vibeSaveDirVariable: saveDir}})

	session := detectVibe(vibeTestCWD, "4242")

	want := vibeSaveDirVariable + "=" + saveDir + " vibe --resume " + sessionID
	if session == nil || session.Command != want {
		t.Errorf("detectVibe = %+v, want command %q", session, want)
	}
}

func TestDetectVibe_RelativeVibeHomeGivesBareCommand(t *testing.T) {
	defaultDir := isolateVibe(t)
	sessionID := "019a6f2e-4b1c-7d3e-9f80-a1b2c3d4e5f6"
	writeVibeUnifiedSession(t, defaultDir, sessionID, vibeRuntimeState(sessionID, vibeTestCWD, "root"), time.Now())

	// Vibe resolves a relative home against the directory it started in,
	// which crex cannot know for sure, so it does not guess.
	stubProcessEnv(t, map[string]map[string]string{"4242": {vibeHomeVariable: "vibe-home"}})

	session := detectVibe(vibeTestCWD, "4242")

	if session == nil || session.Command != "vibe" {
		t.Errorf("detectVibe = %+v, want the bare command", session)
	}
}
