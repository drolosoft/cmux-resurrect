//go:build live

package live

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/orchestrate"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

// ---------------------------------------------------------------------------
// Browser profile helpers (GitHub #9)
// ---------------------------------------------------------------------------

type liveProfile struct {
	ID      string `json:"id"`
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	Default bool   `json:"built_in_default"`
}

func browserProfiles(t *testing.T) []liveProfile {
	t.Helper()
	out, err := cmuxRun(t, "rpc", "browser.profiles.list")
	if err != nil {
		t.Fatalf("browser.profiles.list: %v", err)
	}
	var resp struct {
		Profiles []liveProfile `json:"profiles"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("parse browser.profiles.list: %v", err)
	}
	return resp.Profiles
}

func profileBySlug(t *testing.T, slug string) *liveProfile {
	for _, p := range browserProfiles(t) {
		if p.Slug == slug {
			return &p
		}
	}
	return nil
}

func deleteProfile(t *testing.T, slug string) {
	_, _ = cmuxRun(t, "browser", "profiles", "delete", slug)
}

// TestCmux_BrowserProfileRestoreEnsures restores a layout referencing a
// browser profile that does not exist on this machine and requires crex to
// create the (empty) profile bucket before the pane lands. It then re-saves
// over the same layout name and requires the profile field to survive — on
// cmux ≤0.64.20 the pane itself comes up on the default profile (the
// --profile flag ships in the next cmux release), so survival is the merge
// path's job; on newer cmux the live capture reports it directly.
func TestCmux_BrowserProfileRestoreEnsures(t *testing.T) {
	requireCmux(t)
	const slug = "crex-audit-missing"
	const title = "crex-audit-profile-ws"

	if wsByTitle(t, title) != nil {
		t.Fatalf("a workspace titled %q is already open in cmux — close it and rerun the audit", title)
	}
	deleteProfile(t, slug) // stale run leftovers
	if profileBySlug(t, slug) != nil {
		t.Fatalf("profile %q still exists after delete — remove it in cmux and rerun", slug)
	}
	t.Cleanup(func() { deleteProfile(t, slug) })

	layouts := t.TempDir()
	layoutTOML := fmt.Sprintf(`name = "audit-profile"
version = 1

[[workspace]]
title = %q
cwd = %q
index = 0

  [[workspace.pane]]
  type = "terminal"
  focus = true
  index = 0

  [[workspace.pane]]
  type = "browser"
  split = "right"
  url = "https://example.com"
  profile = %q
  index = 1
`, title, homeDir, slug)
	if err := os.WriteFile(filepath.Join(layouts, "audit-profile.toml"), []byte(layoutTOML), 0o644); err != nil {
		t.Fatal(err)
	}

	env := crexEnv("CREX_BACKEND=cmux")
	created := newWorkspaces(t, 1, func() {
		if _, err := runCrex(t, layouts, env, "restore", "audit-profile", "--mode", "add"); err != nil {
			t.Fatalf("restore: %v", err)
		}
	})
	t.Cleanup(func() {
		for _, ref := range created {
			closeWorkspace(t, ref)
		}
	})

	// The missing profile bucket must now exist (created by crex, contents empty).
	if profileBySlug(t, slug) == nil {
		t.Fatalf("profile %q was not created during restore", slug)
	}

	// The workspace must carry a real browser surface with the URL.
	ok := waitFor(30*time.Second, time.Second, func() bool {
		ws := wsByTitle(t, title)
		if ws == nil {
			return false
		}
		for _, p := range ws.Panes {
			for _, s := range p.Surfaces {
				if s.Type == "browser" {
					return true
				}
			}
		}
		return false
	})
	if !ok {
		t.Fatal("restored workspace has no browser surface")
	}

	// Re-save over the same name: the profile field must survive.
	if _, err := runCrex(t, layouts, env, "save", "audit-profile"); err != nil {
		t.Fatalf("re-save: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(layouts, "audit-profile.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !containsProfileField(string(data), slug) {
		t.Fatalf("re-saved layout lost the browser profile:\n%s", data)
	}
}

// TestCmux_BrowserProfileCapturePipeline exercises the real capture path
// end-to-end against the live app: real profile list, real tree UUIDs, and a
// session file in the exact on-disk format — with one substitution. cmux's
// UI is the only way to assign a non-default profile to a surface (no API),
// so the test copies the app's real session file and rewrites just the target
// panel's profileID, exactly what the file looks like after the user picks a
// profile in the dropdown.
func TestCmux_BrowserProfileCapturePipeline(t *testing.T) {
	requireCmux(t)
	const slug = "crex-audit-capture"
	const title = "crex-audit-capture-ws"

	if wsByTitle(t, title) != nil {
		t.Fatalf("a workspace titled %q is already open in cmux — close it and rerun the audit", title)
	}
	deleteProfile(t, slug)
	if _, err := cmuxRun(t, "browser", "profiles", "add", slug); err != nil {
		t.Fatalf("create profile: %v", err)
	}
	t.Cleanup(func() { deleteProfile(t, slug) })
	prof := profileBySlug(t, slug)
	if prof == nil {
		t.Fatalf("profile %q missing after add", slug)
	}

	// Workspace with a real browser pane.
	refs := newWorkspaces(t, 1, func() {
		if _, err := cmuxRun(t, "new-workspace", "--cwd", homeDir); err != nil {
			t.Fatalf("new-workspace: %v", err)
		}
	})
	wsRef := refs[0]
	t.Cleanup(func() { closeWorkspace(t, wsRef) })
	if _, err := cmuxRun(t, "rename-workspace", "--workspace", wsRef, title); err != nil {
		t.Fatalf("rename-workspace: %v", err)
	}
	if _, err := cmuxRun(t, "new-pane", "--type", "browser", "--url", "https://example.com", "--workspace", wsRef); err != nil {
		t.Fatalf("new-pane browser: %v", err)
	}

	// Resolve the browser surface's UUID from the live tree.
	var surfaceID string
	waitFor(30*time.Second, time.Second, func() bool {
		ws := wsByRef(t, wsRef)
		if ws == nil {
			return false
		}
		for _, p := range ws.Panes {
			for _, s := range p.Surfaces {
				if s.Type == "browser" && s.ID != "" {
					surfaceID = s.ID
					return true
				}
			}
		}
		return false
	})
	if surfaceID == "" {
		t.Fatal("browser surface never appeared in tree")
	}

	// Wait for cmux to persist the panel into its session file.
	realDir := filepath.Join(homeDir, "Library", "Application Support", "cmux")
	var sessionPath string
	var raw []byte
	found := waitFor(60*time.Second, 2*time.Second, func() bool {
		matches, _ := filepath.Glob(filepath.Join(realDir, "session-*.json"))
		for _, m := range matches {
			if strings.HasSuffix(m, "-previous.json") {
				continue
			}
			data, err := os.ReadFile(m)
			if err == nil && strings.Contains(string(data), surfaceID) {
				sessionPath, raw = m, data
				return true
			}
		}
		return false
	})
	if !found {
		t.Fatalf("browser panel %s never appeared in cmux session files", surfaceID)
	}
	t.Logf("panel found in %s", filepath.Base(sessionPath))

	// Doctor a copy: assign our profile to the panel — byte-for-byte the
	// state the file reaches when the user selects the profile in the UI.
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse real session file: %v", err)
	}
	patched := false
	var patch func(v any)
	patch = func(v any) {
		switch node := v.(type) {
		case map[string]any:
			if id, _ := node["id"].(string); id == surfaceID {
				if browser, ok := node["browser"].(map[string]any); ok {
					browser["profileID"] = prof.ID
					patched = true
				}
			}
			for _, c := range node {
				patch(c)
			}
		case []any:
			for _, c := range node {
				patch(c)
			}
		}
	}
	patch(doc)
	if !patched {
		t.Fatalf("panel %s has no browser entry in session file", surfaceID)
	}
	doctored, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	sessionDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sessionDir, "session-audit.json"), doctored, 0o644); err != nil {
		t.Fatal(err)
	}

	// Real client, real cmux — only the session dir is redirected.
	c := &client.CLIClient{Binary: cmuxPath(), Timeout: 10 * time.Second, SessionDir: sessionDir}
	profiles, err := c.SurfaceProfiles()
	if err != nil {
		t.Fatalf("SurfaceProfiles: %v", err)
	}
	var gotSlug string
	for _, s := range profiles {
		if s == slug {
			gotSlug = s
		}
	}
	if gotSlug != slug {
		t.Fatalf("SurfaceProfiles = %v, want an entry with %q", profiles, slug)
	}

	// Full save through the orchestrator: the TOML must carry the profile.
	layouts := t.TempDir()
	store, err := persist.NewFileStore(layouts)
	if err != nil {
		t.Fatal(err)
	}
	saver := &orchestrate.Saver{Client: c, Store: store}
	if _, err := saver.Save("audit-capture", ""); err != nil {
		t.Fatalf("save: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(layouts, "audit-capture.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !containsProfileField(string(data), slug) {
		t.Fatalf("saved layout missing captured profile %q:\n%s", slug, data)
	}
}

// containsProfileField matches `profile = "<slug>"` in either TOML quote style.
func containsProfileField(toml, slug string) bool {
	return strings.Contains(toml, "profile = '"+slug+"'") ||
		strings.Contains(toml, `profile = "`+slug+`"`)
}

// TestGhostty_ProfileBearingLayoutIsHarmless restores a layout whose browser
// pane carries a cmux profile on Ghostty. Ghostty has no browser panes (and
// no profiles): the pane must degrade to a plain terminal split exactly as
// before the feature — the profile field ignored, the restore clean.
func TestGhostty_ProfileBearingLayoutIsHarmless(t *testing.T) {
	requireGhostty(t)
	layouts := t.TempDir()
	const title = "crex-audit-ghostty-profile"

	writeLayout(t, layouts, &model.Layout{
		Name: "ghostty-profile", Version: 1,
		Workspaces: []model.Workspace{{
			Title: title, CWD: homeDir, Index: 0,
			Panes: []model.Pane{
				{Type: "terminal", Focus: true, FocusTarget: -1},
				// No URL on purpose: the URL fallback (`open` in the system
				// browser) is pre-existing behavior and would leave a tab in
				// the user's web browser; the profile field is what's under test.
				{Type: "browser", Split: "right", Profile: "work-admin", FocusTarget: -1},
			},
		}},
	})

	baseTabs := tabCount(t)
	t.Cleanup(func() { closeTabsFrom(t, baseTabs+1) })

	out, err := runCrex(t, layouts, crexEnv("CREX_BACKEND=ghostty"), "restore", "ghostty-profile", "--mode", "add")
	if err != nil {
		t.Fatalf("restore on Ghostty with profile-bearing layout: %v\n%s", err, out)
	}

	ok := waitFor(30*time.Second, time.Second, func() bool {
		return tabCount(t) == baseTabs+1 && terminalsInTab(t, baseTabs+1) == 2
	})
	if !ok {
		t.Fatalf("expected 1 new tab with 2 terminals, tabs=%d terminals=%d",
			tabCount(t)-baseTabs, terminalsInTab(t, baseTabs+1))
	}
}
