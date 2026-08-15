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

// TestCmux_RestoredPanesLandOnTheirProfiles is the end-to-end proof that a
// restored browser pane actually OPENS on its assigned profile — the one check
// that could not run until cmux ≥0.64.21 shipped profile targeting.
//
// Verified live on 0.64.22 while writing this: `new-pane --profile` honors the
// profile, but `workspace create --layout` silently ignores a surface's
// "profile" key and opens the pane on the last-used profile. So the atomic
// path can NEVER be used for profile-bearing layouts; this test would fail
// again the day someone re-enables it. Assignment is proven by localStorage
// isolation, which is per-profile in WebKit — no reliance on cmux's session
// file. Skips (never fails) on cmux that ignores --profile entirely.
func TestCmux_RestoredPanesLandOnTheirProfiles(t *testing.T) {
	requireCmux(t)
	const title = "crex-audit-e2e-profiles"
	if wsByTitle(t, title) != nil {
		t.Fatalf("a workspace titled %q is already open in cmux — close it and rerun", title)
	}
	profiles := []string{"crex-audit-p1", "crex-audit-p2", "crex-audit-p3"}
	for _, p := range profiles {
		deleteProfile(t, p)
	}
	t.Cleanup(func() {
		for _, p := range profiles {
			deleteProfile(t, p)
		}
	})
	// p1 and p2 pre-exist; p3 must be auto-created by the restore.
	for _, p := range profiles[:2] {
		if _, err := cmuxRun(t, "browser", "profiles", "add", p); err != nil {
			t.Fatalf("create profile %s: %v", p, err)
		}
	}

	layouts := t.TempDir()
	layoutTOML := fmt.Sprintf(`name = "audit-e2e-profiles"
version = 1

[[workspace]]
title = %q
cwd = %q
index = 0

  [[workspace.pane]]
  type = "terminal"
  focus = true

  [[workspace.pane]]
  type = "browser"
  split = "right"
  url = "https://example.net"
  profile = %q

  [[workspace.pane]]
  type = "browser"
  split = "down"
  url = "https://example.net"
  profile = %q

  [[workspace.pane]]
  type = "browser"
  split = "down"
  url = "https://example.net"
  profile = %q
`, title, homeDir, profiles[0], profiles[1], profiles[2])
	if err := os.WriteFile(filepath.Join(layouts, "audit-e2e-profiles.toml"), []byte(layoutTOML), 0o644); err != nil {
		t.Fatal(err)
	}

	created := newWorkspaces(t, 1, func() {
		if _, err := runCrex(t, layouts, crexEnv("CREX_BACKEND=cmux"),
			"restore", "audit-e2e-profiles", "--mode", "add"); err != nil {
			t.Fatalf("restore: %v", err)
		}
	})
	t.Cleanup(func() {
		for _, ref := range created {
			closeWorkspace(t, ref)
		}
	})
	if profileBySlug(t, profiles[2]) == nil {
		t.Fatalf("profile %q was not auto-created during restore", profiles[2])
	}

	// Wait for the three browser surfaces to exist and load.
	var browsers []string
	waitFor(60*time.Second, time.Second, func() bool {
		ws := wsByRef(t, created[0])
		if ws == nil {
			return false
		}
		browsers = browsers[:0]
		for _, s := range allSurfaces(ws) {
			if s.Type == "browser" {
				browsers = append(browsers, s.Ref)
			}
		}
		return len(browsers) == 3
	})
	if len(browsers) != 3 {
		t.Fatalf("expected 3 browser surfaces, got %v", browsers)
	}
	time.Sleep(4 * time.Second)

	// Seed a distinct marker in each surface, then require that NO surface can
	// read another's marker: three isolated storages ⇒ three profiles.
	evalOK := func(ref, js string) string {
		out, _ := cmuxRun(t, "browser", "--surface", ref, "eval", js)
		return strings.TrimSpace(out)
	}
	for i, ref := range browsers {
		evalOK(ref, fmt.Sprintf("localStorage.setItem('crexE2E','P%d'); 'ok'", i))
	}
	time.Sleep(time.Second)
	got := map[string]string{}
	for _, ref := range browsers {
		got[ref] = evalOK(ref, "localStorage.getItem('crexE2E')")
	}
	distinct := map[string]bool{}
	for ref, v := range got {
		if v == "" || v == "null" {
			t.Fatalf("surface %s lost its own marker (%v) — eval not working, cannot judge", ref, got)
		}
		distinct[v] = true
	}
	switch len(distinct) {
	case 3:
		t.Logf("each restored pane sits on its own profile: %v", got)
	case 1:
		// Every pane shares one storage: either cmux ignores --profile (older
		// than 0.64.21) or the atomic path swallowed the profiles again.
		out, _ := cmuxRun(t, "version")
		if strings.Contains(out, "0.64.20") || strings.Contains(out, "0.63") {
			t.Skipf("cmux %s ignores --profile; profile assignment cannot be verified here", strings.TrimSpace(out))
		}
		t.Fatalf("all three restored panes share ONE profile on cmux %s — profiles were dropped on restore: %v", strings.TrimSpace(out), got)
	default:
		t.Fatalf("expected 3 isolated profiles, got %d: %v", len(distinct), got)
	}
}
