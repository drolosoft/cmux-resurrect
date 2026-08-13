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
)

// tabCWDs returns the live working directory of every surface in a workspace,
// in tree order.
func tabCWDs(t *testing.T, wsRef string) []string {
	t.Helper()
	cwds, _ := terminals(t)
	ws := wsByRef(t, wsRef)
	if ws == nil {
		return nil
	}
	var out []string
	for _, r := range surfaceRefs(ws) {
		out = append(out, cwds[r])
	}
	return out
}

// allSurfaces flattens a workspace's surfaces in tree order.
func allSurfaces(ws *cmuxWorkspace) []cmuxSurface {
	var out []cmuxSurface
	for _, p := range ws.Panes {
		out = append(out, p.Surfaces...)
	}
	return out
}

// focusSurface selects a tab by UUID, which is what makes cmux render it and
// spawn its shell. Refs are not accepted by this RPC.
func focusSurface(t *testing.T, wsID, surfaceID string) {
	t.Helper()
	if wsID == "" || surfaceID == "" {
		return
	}
	_, _ = cmuxRun(t, "rpc", "surface.focus",
		fmt.Sprintf(`{"workspace_id":%q,"surface_id":%q}`, wsID, surfaceID))
	time.Sleep(300 * time.Millisecond)
}

// buildTabWorkspace creates a workspace whose single pane holds three TABS,
// each cd'd to its own folder — the shape from GitHub #8.
func buildTabWorkspace(t *testing.T, title string) (wsRef string, want []string) {
	t.Helper()
	docs := filepath.Join(homeDir, "Documents")
	downloads := filepath.Join(homeDir, "Downloads")
	desktop := filepath.Join(homeDir, "Desktop")

	refs := newWorkspaces(t, 1, func() {
		if _, err := cmuxRun(t, "new-workspace", "--cwd", docs); err != nil {
			t.Fatalf("new-workspace: %v", err)
		}
	})
	wsRef = refs[0]
	// Register cleanup immediately: a Fatal further down would otherwise leave
	// the workspace behind and make the next run fail its precondition.
	t.Cleanup(func() { closeWorkspace(t, wsRef) })
	_, _ = cmuxRun(t, "select-workspace", "--workspace", wsRef)
	if _, err := cmuxRun(t, "rename-workspace", "--workspace", wsRef, title); err != nil {
		t.Fatalf("rename-workspace: %v", err)
	}

	if ws := wsByRef(t, wsRef); ws == nil || len(ws.Panes) == 0 {
		t.Fatal("workspace has no pane")
	}
	for i := 0; i < 2; i++ {
		if _, err := cmuxRun(t, "new-surface", "--pane", "0", "--workspace", wsRef); err != nil {
			t.Fatalf("new-surface %d: %v", i, err)
		}
		time.Sleep(time.Second)
	}

	// cmux spawns a tab's shell only when the tab is first RENDERED, so a tab
	// nobody looked at has no shell to cd. Select each one to bring it up —
	// this is also the exact behavior the persisted-directory fallback exists
	// for (GitHub #8).
	tries := 0
	ok := waitFor(90*time.Second, time.Second, func() bool {
		w := wsByRef(t, wsRef)
		if w == nil || len(surfaceRefs(w)) != 3 {
			return false
		}
		if tries%5 == 0 {
			_, _ = cmuxRun(t, "select-workspace", "--workspace", wsRef)
			for _, s := range allSurfaces(w) {
				focusSurface(t, w.ID, s.ID)
			}
		}
		tries++
		_, ready := terminals(t)
		for _, r := range surfaceRefs(w) {
			if !ready[r] {
				return false
			}
		}
		return true
	})
	if !ok {
		t.Fatalf("three tab shells never became ready: %v", tabCWDs(t, wsRef))
	}

	// cd tabs 2 and 3; tab 1 stays at the workspace folder.
	refsNow := surfaceRefs(wsByRef(t, wsRef))
	for i, dir := range []string{downloads, desktop} {
		_, _ = cmuxRun(t, "send", "--workspace", wsRef, "--surface", refsNow[i+1],
			fmt.Sprintf("cd '%s'\n", dir))
	}
	want = []string{docs, downloads, desktop}
	waitFor(30*time.Second, time.Second, func() bool {
		return multisetEqual(tabCWDs(t, wsRef), want)
	})
	if got := tabCWDs(t, wsRef); !multisetEqual(got, want) {
		t.Fatalf("harness setup: tab cds never landed, got %v want %v", got, want)
	}
	return wsRef, want
}

// TestCmux_TabsKeepDistinctDirectories is the GitHub #8 regression guard: a
// workspace whose TABS sit in different folders must save and restore with
// each tab in its own folder, not collapsed onto the first tab's path.
func TestCmux_TabsKeepDistinctDirectories(t *testing.T) {
	requireCmux(t)
	const title = "crex-audit-tabs"
	if wsByTitle(t, title) != nil {
		t.Fatalf("a workspace titled %q is already open in cmux — close it and rerun", title)
	}

	wsRef, want := buildTabWorkspace(t, title) // registers its own cleanup
	t.Logf("original tab cwds: %v", tabCWDs(t, wsRef))

	layouts := t.TempDir()
	env := crexEnv("CREX_BACKEND=cmux")
	if _, err := runCrex(t, layouts, env, "save", "audit-tabs"); err != nil {
		t.Fatalf("save: %v", err)
	}

	// The saved TOML must carry three DISTINCT tab directories.
	data, err := os.ReadFile(filepath.Join(layouts, "audit-tabs.toml"))
	if err != nil {
		t.Fatal(err)
	}
	block := workspaceBlock(string(data), title)
	if block == "" {
		t.Fatalf("workspace %q missing from saved layout:\n%s", title, data)
	}
	for _, dir := range want {
		if !strings.Contains(block, dir) {
			t.Errorf("saved layout lost tab directory %s:\n%s", dir, block)
		}
	}

	// Restore into a fresh workspace and require the same three folders.
	closeWorkspace(t, wsRef)
	time.Sleep(2 * time.Second)

	restored := newWorkspaces(t, 1, func() {
		if _, err := runCrex(t, layouts, env, "restore", "audit-tabs", title, "--mode", "add"); err != nil {
			t.Fatalf("restore: %v", err)
		}
	})
	t.Cleanup(func() {
		for _, ref := range restored {
			closeWorkspace(t, ref)
		}
	})

	newRef := restored[0]
	waitFor(60*time.Second, time.Second, func() bool {
		return multisetEqual(tabCWDs(t, newRef), want)
	})
	if got := tabCWDs(t, newRef); !multisetEqual(got, want) {
		t.Fatalf("restored tab cwds wrong.\nwant %v\ngot  %v", want, got)
	}
}

// workspaceBlock returns the TOML text of one workspace section by title.
func workspaceBlock(toml, title string) string {
	i := strings.Index(toml, "title = '"+title+"'")
	if i < 0 {
		i = strings.Index(toml, `title = "`+title+`"`)
	}
	if i < 0 {
		return ""
	}
	rest := toml[i:]
	if j := strings.Index(rest, "\n[[workspace]]"); j > 0 {
		return rest[:j]
	}
	return rest
}

// TestCmux_SurfaceDirectoriesProviderMatchesLive checks the persisted-directory
// provider against the running app: every tab's directory must match what the
// live shells report. This is the source crex falls back to for tabs whose
// shell cmux hasn't spawned yet (GitHub #8).
func TestCmux_SurfaceDirectoriesProviderMatchesLive(t *testing.T) {
	requireCmux(t)
	const title = "crex-audit-dirs"
	if wsByTitle(t, title) != nil {
		t.Fatalf("a workspace titled %q is already open in cmux — close it and rerun", title)
	}

	wsRef, want := buildTabWorkspace(t, title) // registers its own cleanup

	c := &client.CLIClient{Binary: cmuxPath(), Timeout: 10 * time.Second}

	var dirs map[string]string
	ok := waitFor(60*time.Second, 2*time.Second, func() bool {
		m, err := c.SurfaceDirectories()
		if err != nil {
			return false
		}
		dirs = m
		ws := wsByRef(t, wsRef)
		if ws == nil {
			return false
		}
		var got []string
		for _, r := range surfaceRefs(ws) {
			got = append(got, dirs[r])
		}
		return multisetEqual(got, want)
	})

	ws := wsByRef(t, wsRef)
	var got []string
	for _, r := range surfaceRefs(ws) {
		got = append(got, dirs[r])
	}
	if !ok {
		t.Fatalf("SurfaceDirectories did not match the live tabs.\nwant %v\ngot  %v", want, got)
	}
	t.Logf("persisted dirs match live tabs: %v", got)
}

// TestCmux_TabCommandsActuallyRun closes the loop on the per-tab AI resume fix
// (GitHub #8): saving the command is useless if it never reaches the tab. cmux
// spawns a background tab's shell lazily, so this drives a real restore of a
// layout whose three tabs each carry a distinct command and requires all three
// to have executed. Markers are files in a temp dir — no AI CLI needed.
func TestCmux_TabCommandsActuallyRun(t *testing.T) {
	requireCmux(t)
	const title = "crex-audit-tabcmd"
	if wsByTitle(t, title) != nil {
		t.Fatalf("a workspace titled %q is already open in cmux — close it and rerun", title)
	}

	markers := t.TempDir()
	layouts := t.TempDir()
	marker := func(n int) string { return filepath.Join(markers, fmt.Sprintf("tab%d", n)) }

	layoutTOML := fmt.Sprintf(`name = "audit-tabcmd"
version = 1

[[workspace]]
title = %q
cwd = %q
index = 0

  [[workspace.pane]]
  type = "terminal"
  cwd = %q
  command = "touch %s"
  focus = true
  index = 0

    [[workspace.pane.surface]]
    type = "terminal"
    cwd = %q
    command = "touch %s"

    [[workspace.pane.surface]]
    type = "terminal"
    cwd = %q
    command = "touch %s"
`, title, homeDir, homeDir, marker(0),
		filepath.Join(homeDir, "Downloads"), marker(1),
		filepath.Join(homeDir, "Desktop"), marker(2))
	if err := os.WriteFile(filepath.Join(layouts, "audit-tabcmd.toml"), []byte(layoutTOML), 0o644); err != nil {
		t.Fatal(err)
	}

	created := newWorkspaces(t, 1, func() {
		if _, err := runCrex(t, layouts, crexEnv("CREX_BACKEND=cmux"),
			"restore", "audit-tabcmd", title, "--mode", "add"); err != nil {
			t.Fatalf("restore: %v", err)
		}
	})
	t.Cleanup(func() {
		for _, ref := range created {
			closeWorkspace(t, ref)
		}
	})

	// Nudge each tab into rendering: a shell that never spawns never runs its
	// command, which is precisely the failure mode under test.
	tries := 0
	ok := waitFor(90*time.Second, time.Second, func() bool {
		if tries%5 == 0 {
			if w := wsByRef(t, created[0]); w != nil {
				_, _ = cmuxRun(t, "select-workspace", "--workspace", w.Ref)
				for _, s := range allSurfaces(w) {
					focusSurface(t, w.ID, s.ID)
				}
			}
		}
		tries++
		for n := 0; n < 3; n++ {
			if _, err := os.Stat(marker(n)); err != nil {
				return false
			}
		}
		return true
	})
	if !ok {
		var missing []string
		for n := 0; n < 3; n++ {
			if _, err := os.Stat(marker(n)); err != nil {
				missing = append(missing, fmt.Sprintf("tab%d", n))
			}
		}
		t.Fatalf("tab commands never ran: %v (a saved per-tab resume command that never executes is a silent no-op)", missing)
	}
}

// TestGhostty_PaneTabsBecomeSplits: a cmux layout can hold sub-tabs inside a
// pane, which Ghostty has no equivalent for. They used to be dropped outright —
// the shell, its folder and its AI session vanished from the restored layout.
// They must come back as splits instead, each in its own folder.
func TestGhostty_PaneTabsBecomeSplits(t *testing.T) {
	requireGhostty(t)
	layouts := t.TempDir()
	docs := filepath.Join(homeDir, "Documents")
	downloads := filepath.Join(homeDir, "Downloads")
	desktop := filepath.Join(homeDir, "Desktop")

	// One pane holding three tabs, exactly what cmux saves.
	layoutTOML := fmt.Sprintf(`name = "audit-ghtabs"
version = 1

[[workspace]]
title = "crex-audit-ghtabs"
cwd = %q
index = 0

  [[workspace.pane]]
  type = "terminal"
  cwd = %q
  focus = true
  index = 0

    [[workspace.pane.surface]]
    type = "terminal"
    cwd = %q

    [[workspace.pane.surface]]
    type = "terminal"
    cwd = %q
`, docs, docs, downloads, desktop)
	if err := os.WriteFile(filepath.Join(layouts, "audit-ghtabs.toml"), []byte(layoutTOML), 0o644); err != nil {
		t.Fatal(err)
	}

	baseTabs := tabCount(t)
	before := zshPids(t)
	t.Cleanup(func() { closeTabsFrom(t, baseTabs+1) })

	out, err := runCrex(t, layouts, crexEnv("CREX_BACKEND=ghostty"),
		"restore", "audit-ghtabs", "--mode", "add")
	if err != nil {
		t.Fatalf("restore on Ghostty: %v\n%s", err, out)
	}

	// All three shells must exist, each in its own folder — nothing dropped.
	want := []string{docs, downloads, desktop}
	got := waitShellCWDs(t, before, want)
	if !multisetEqual(got, want) {
		t.Fatalf("pane tabs were not restored as splits.\nwant %v\ngot  %v", want, got)
	}
	if n := terminalsInTab(t, baseTabs+1); n != 3 {
		t.Errorf("restored tab holds %d terminals, want 3 (pane + its two tabs as splits)", n)
	}
}

// windowRefs returns every cmux window ref.
func windowRefs(t *testing.T) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	o, err := cmuxRun(t, "tree", "--all", "--json")
	if err != nil {
		return out
	}
	var tr struct {
		Windows []struct{ Ref string } `json:"windows"`
	}
	if json.Unmarshal([]byte(o), &tr) != nil {
		return out
	}
	for _, w := range tr.Windows {
		out[w.Ref] = true
	}
	return out
}

// windowWorkspace returns the first workspace ref and terminal surface ref of a
// window, using the all-windows tree (the scoped one omits unfocused windows).
func windowWorkspace(t *testing.T, windowRef string) (wsRef, surfRef string) {
	t.Helper()
	o, err := cmuxRun(t, "tree", "--all", "--json")
	if err != nil {
		return "", ""
	}
	var tr struct {
		Windows []struct {
			Ref        string          `json:"ref"`
			Workspaces []cmuxWorkspace `json:"workspaces"`
		} `json:"windows"`
	}
	if json.Unmarshal([]byte(o), &tr) != nil {
		return "", ""
	}
	for _, w := range tr.Windows {
		if w.Ref != windowRef || len(w.Workspaces) == 0 {
			continue
		}
		ws := w.Workspaces[0]
		for _, p := range ws.Panes {
			for _, s := range p.Surfaces {
				if s.Type == "terminal" {
					return ws.Ref, s.Ref
				}
			}
		}
	}
	return "", ""
}

// TestCmux_SaveCapturesTheCallersWindow guards a bug that silently stored the
// wrong session: `cmux tree --json` returns the FOCUSED window, not the caller's,
// so `crex save` run in a background window wrote the frontmost window's
// workspaces instead of the ones under the user's cursor.
//
// The caller cannot be faked from the environment (cmux derives it from the
// connection), so this drives a real save from inside a second window's shell.
func TestCmux_SaveCapturesTheCallersWindow(t *testing.T) {
	requireCmux(t)
	const title = "crex-audit-otherwin"

	before := windowRefs(t)
	if _, err := cmuxRun(t, "new-window"); err != nil {
		t.Fatalf("new-window: %v", err)
	}
	var newWin string
	waitFor(30*time.Second, time.Second, func() bool {
		for ref := range windowRefs(t) {
			if !before[ref] {
				newWin = ref
				return true
			}
		}
		return false
	})
	if newWin == "" {
		t.Fatal("second cmux window never appeared")
	}
	// `close-window` answers OK without closing anything, so tear the window
	// down by closing its workspaces: cmux disposes of a window once its last
	// workspace goes. Without this the audit leaks a window per run.
	t.Cleanup(func() { closeWindowByWorkspaces(t, newWin) })

	var wsRef, surfRef string
	waitFor(30*time.Second, time.Second, func() bool {
		wsRef, surfRef = windowWorkspace(t, newWin)
		return wsRef != "" && surfRef != ""
	})
	if wsRef == "" {
		t.Fatalf("no terminal surface found in %s", newWin)
	}
	if _, err := cmuxRun(t, "rename-workspace", "--workspace", wsRef, title); err != nil {
		t.Fatalf("rename-workspace: %v", err)
	}

	// Hand focus BACK to another window: the whole point is that the caller's
	// window is not the frontmost one.
	var refocused string
	for ref := range before {
		refocused = ref
		_, _ = cmuxRun(t, "focus-window", "--window", ref)
		break
	}
	_ = refocused
	time.Sleep(2 * time.Second)

	// Wait for the new window's shell, then run the save from inside it.
	waitFor(60*time.Second, time.Second, func() bool {
		_, ready := terminals(t)
		return ready[surfRef]
	})
	layouts := t.TempDir()
	out := filepath.Join(layouts, "audit-otherwin.toml")
	_, _ = cmuxRun(t, "send", "--surface", surfRef,
		fmt.Sprintf("%s --layouts-dir %s --config %s save audit-otherwin >/dev/null 2>&1\n",
			crexBin, layouts, crexConf))

	if !waitFor(60*time.Second, time.Second, func() bool {
		_, err := os.Stat(out)
		return err == nil
	}) {
		t.Fatal("the save never produced a layout — could not drive crex inside the second window")
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), title) {
		t.Fatalf("save from %s did not capture that window (looking for %q):\n%s", newWin, title, data)
	}
	// And it must not have captured the frontmost window instead.
	if n := strings.Count(string(data), "[[workspace]]"); n != 1 {
		t.Errorf("captured %d workspaces, want only the caller window's 1:\n%s", n, data)
	}
}

// closeWindowByWorkspaces disposes of a cmux window by closing every workspace
// in it. `close-window` reports success without doing anything, and a window
// disappears on its own once its last workspace is gone. Pinned workspaces are
// unpinned first — cmux refuses to close those.
func closeWindowByWorkspaces(t *testing.T, windowRef string) {
	t.Helper()
	for i := 0; i < 10; i++ {
		refs := workspaceRefsIn(t, windowRef)
		if len(refs) == 0 {
			return
		}
		for _, ref := range refs {
			_, _ = cmuxRun(t, "workspace-action", "--action", "unpin", "--workspace", ref)
			_, _ = cmuxRun(t, "close-workspace", "--workspace", ref)
		}
		time.Sleep(time.Second)
	}
}

// workspaceRefsIn lists the workspace refs of one window.
func workspaceRefsIn(t *testing.T, windowRef string) []string {
	t.Helper()
	out, err := cmuxRun(t, "tree", "--all", "--json")
	if err != nil {
		return nil
	}
	var tr struct {
		Windows []struct {
			Ref        string `json:"ref"`
			Workspaces []struct {
				Ref string `json:"ref"`
			} `json:"workspaces"`
		} `json:"windows"`
	}
	if json.Unmarshal([]byte(out), &tr) != nil {
		return nil
	}
	for _, w := range tr.Windows {
		if w.Ref != windowRef {
			continue
		}
		refs := make([]string, 0, len(w.Workspaces))
		for _, ws := range w.Workspaces {
			refs = append(refs, ws.Ref)
		}
		return refs
	}
	return nil
}
