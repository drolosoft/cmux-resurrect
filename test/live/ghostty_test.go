//go:build live

package live

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

// ---------------------------------------------------------------------------
// Ghostty ground-truth helpers. The AppleScript `working directory` property
// depends on OSC 7, which not every shell setup emits — so the harness
// verifies cwds via lsof on the shells' real PIDs instead.
// ---------------------------------------------------------------------------

func osa(t *testing.T, script string) string {
	t.Helper()
	out, _ := exec.Command("osascript", "-e", script).Output()
	return strings.TrimSpace(string(out))
}

func ghosttyAppRunning(t *testing.T) bool {
	return osa(t, `tell application "System Events" to (name of processes) contains "Ghostty"`) == "true"
}

func requireGhostty(t *testing.T) {
	t.Helper()
	if !ghosttyAppRunning(t) {
		_ = exec.Command("open", "-a", "Ghostty").Run()
		if !waitFor(15*time.Second, time.Second, func() bool { return ghosttyAppRunning(t) }) {
			backendMissing(t, "Ghostty", "install/launch the Ghostty app and retry")
		}
	}
	if osa(t, `tell application "Ghostty" to count of windows`) == "0" {
		osa(t, `tell application "Ghostty" to make new window`)
		waitFor(10*time.Second, 500*time.Millisecond, func() bool { return tabCount(t) > 0 })
	}
}

func tabCount(t *testing.T) int {
	n, _ := strconv.Atoi(osa(t, `tell application "Ghostty" to count of tabs of front window`))
	return n
}

func terminalsInTab(t *testing.T, tab int) int {
	n, _ := strconv.Atoi(osa(t, fmt.Sprintf(`tell application "Ghostty" to count of terminals of tab %d of front window`, tab)))
	return n
}

func zshPids(t *testing.T) map[string]bool {
	pids := map[string]bool{}
	for _, pid := range strings.Fields(sh(t, `ps -axo pid,comm | awk '$2~/zsh$/{print $1}'`)) {
		pids[pid] = true
	}
	return pids
}

func lsofCWD(t *testing.T, pid string) string {
	out := sh(t, fmt.Sprintf(`lsof -a -p %s -d cwd -Fn 2>/dev/null | grep '^n' | sed 's/^n//'`, pid))
	return canonPath(out)
}

// waitShellCWDs polls the real cwds (lsof) of the shells that appeared since
// `before` until they match the wanted multiset.
func waitShellCWDs(t *testing.T, before map[string]bool, want []string) []string {
	t.Helper()
	var got []string
	waitFor(90*time.Second, time.Second, func() bool {
		got = got[:0]
		for pid := range zshPids(t) {
			if !before[pid] {
				got = append(got, lsofCWD(t, pid))
			}
		}
		return multisetEqual(got, want)
	})
	return got
}

// closeTabsFrom closes front-window tabs from the highest index down to
// (and including) idxFrom — the harness's cleanup for tabs it created.
func closeTabsFrom(t *testing.T, idxFrom int) {
	t.Helper()
	for i := 0; tabCount(t) >= idxFrom && i < 20; i++ {
		osa(t, fmt.Sprintf(`tell application "Ghostty" to close tab (a reference to tab %d of front window)`, tabCount(t)))
		time.Sleep(800 * time.Millisecond)
	}
}

// writeLayout persists a fixture layout into the harness layouts dir.
func writeLayout(t *testing.T, dir string, layout *model.Layout) {
	t.Helper()
	store, err := persist.NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(layout.Name, layout); err != nil {
		t.Fatal(err)
	}
}

func loadLayout(t *testing.T, dir, name string) *model.Layout {
	t.Helper()
	store, err := persist.NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	layout, err := store.Load(name)
	if err != nil {
		t.Fatalf("loading saved-back layout %q: %v", name, err)
	}
	return layout
}

func singlePaneWorkspace(title, cwd string, index int) model.Workspace {
	return model.Workspace{
		Title: title, CWD: cwd, Index: index,
		Panes: []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}},
	}
}

// ---------------------------------------------------------------------------
// Matrix: Ghostty
// ---------------------------------------------------------------------------

// TestGhostty_DetectionSurvivesDeadCmuxEnv reproduces the leaked-env failure:
// a Ghostty shell inheriting CMUX_* vars from a cmux that is no longer
// reachable. crex must fall back to the live Ghostty instead of dying with
// "backend not reachable".
func TestGhostty_DetectionSurvivesDeadCmuxEnv(t *testing.T) {
	requireGhostty(t)
	layouts := t.TempDir()
	docs := filepath.Join(homeDir, "Documents")
	writeLayout(t, layouts, &model.Layout{
		Name: "audit-det", Version: 1, SavedAt: time.Now(),
		Workspaces: []model.Workspace{singlePaneWorkspace("audit-det", docs, 0)},
	})

	baseTabs := tabCount(t)
	before := zshPids(t)
	env := crexEnv(
		"CMUX_SOCKET_PATH=/tmp/crex-audit-dead.sock", // dead on purpose
		"CMUX_WORKSPACE_ID=workspace:999",
	)
	out, err := runCrex(t, layouts, env, "restore", "audit-det", "--mode", "add")
	t.Cleanup(func() { closeTabsFrom(t, baseTabs+1) })
	if err != nil {
		t.Fatalf("restore with dead CMUX_* env must fall back to Ghostty, got error: %v\n%s", err, out)
	}
	got := waitShellCWDs(t, before, []string{docs})
	if !multisetEqual(got, []string{docs}) {
		t.Fatalf("restored shell cwd wrong: want [%s], got %v", docs, got)
	}
}

// TestGhostty_DemoQuad restores the bundled demo layout on Ghostty and
// asserts the home tab plus a 4-terminal files tab, each shell in its own
// folder (verified via lsof, not the OSC7-fed AppleScript property).
func TestGhostty_DemoQuad(t *testing.T) {
	requireGhostty(t)
	// `--mode add` skips workspaces whose titles are already open.
	tabNames := osa(t, `tell application "Ghostty"
  set outp to ""
  repeat with tt in tabs of front window
    set outp to outp & (name of tt) & linefeed
  end repeat
  return outp
end tell`)
	for _, title := range []string{"🏠 home", "📁 files"} {
		if strings.Contains(tabNames, title) {
			t.Fatalf("a tab titled %q is already open in Ghostty — close it and rerun the audit", title)
		}
	}
	layouts := t.TempDir()
	installDemo(t, layouts)

	baseTabs := tabCount(t)
	before := zshPids(t)
	if _, err := runCrex(t, layouts, crexEnv("CREX_BACKEND=ghostty"), "restore", "demo", "--mode", "add"); err != nil {
		t.Fatalf("restore demo: %v", err)
	}
	t.Cleanup(func() { closeTabsFrom(t, baseTabs+1) })

	if !waitFor(30*time.Second, time.Second, func() bool { return tabCount(t) == baseTabs+2 }) {
		t.Fatalf("expected %d tabs after demo restore, got %d", baseTabs+2, tabCount(t))
	}
	if n := terminalsInTab(t, baseTabs+2); n != 4 {
		t.Fatalf("📁 files tab should have 4 terminals, has %d", n)
	}
	if n := terminalsInTab(t, baseTabs+1); n != 1 {
		t.Fatalf("🏠 home tab should have 1 terminal, has %d", n)
	}
	want := append(demoQuadCWDs(), homeDir) // quad + the home tab's shell
	got := waitShellCWDs(t, before, want)
	if !multisetEqual(got, want) {
		t.Fatalf("per-shell cwds wrong.\nwant %v\ngot  %v", want, got)
	}

	// Placement: Ghostty enumerates a tab's terminals in split-tree (DFS)
	// order, so a correctly built demo quad — H(V(Documents,~),
	// V(Downloads,Desktop)) — reports exactly this sequence. Catches splits
	// landing on the wrong pane (panes in the wrong corner) which the cwd
	// multiset can't see. Needs OSC 7 (`working directory`); skipped when
	// the shells don't report it.
	order := tabWorkingDirs(t, baseTabs+2)
	allReported := len(order) == 4
	for _, c := range order {
		if c == "" {
			allReported = false
		}
	}
	if allReported {
		wantOrder := []string{
			filepath.Join(homeDir, "Documents"),
			homeDir,
			filepath.Join(homeDir, "Downloads"),
			filepath.Join(homeDir, "Desktop"),
		}
		for i := range wantOrder {
			if canonPath(order[i]) != wantOrder[i] {
				t.Fatalf("quad placement wrong (split landed on the wrong pane).\nwant DFS order %v\ngot            %v", wantOrder, order)
			}
		}
	} else {
		t.Logf("OSC 7 not reported by shells; skipping placement-order assertion (got %v)", order)
	}
}

// tabWorkingDirs returns each terminal's `working directory` (OSC 7-fed) in
// Ghostty's enumeration order for the given tab.
func tabWorkingDirs(t *testing.T, tab int) []string {
	t.Helper()
	out := osa(t, fmt.Sprintf(`tell application "Ghostty"
  set outp to ""
  repeat with tt in terminals of tab %d of front window
    set outp to outp & (working directory of tt) & linefeed
  end repeat
  return outp
end tell`, tab))
	var dirs []string
	for _, line := range strings.Split(out, "\n") {
		dirs = append(dirs, strings.TrimSpace(line))
	}
	// osascript's trailing linefeed produces one empty tail entry — drop it.
	if n := len(dirs); n > 0 && dirs[n-1] == "" {
		dirs = dirs[:n-1]
	}
	return dirs
}

// TestGhostty_TabsPerTabCWDRoundtrip is the original issue-8 scenario on
// Ghostty: tabs at different paths must restore to their own cwds AND a
// save-back must capture each tab's cwd again.
func TestGhostty_TabsPerTabCWDRoundtrip(t *testing.T) {
	requireGhostty(t)
	layouts := t.TempDir()
	dirs := []string{
		filepath.Join(homeDir, "Documents"),
		filepath.Join(homeDir, "Downloads"),
		filepath.Join(homeDir, "Desktop"),
	}
	layout := &model.Layout{Name: "audit-gtabs", Version: 1, SavedAt: time.Now()}
	for i, d := range dirs {
		layout.Workspaces = append(layout.Workspaces, singlePaneWorkspace(fmt.Sprintf("audit-gtabs-%d", i), d, i))
	}
	writeLayout(t, layouts, layout)

	baseTabs := tabCount(t)
	before := zshPids(t)
	env := crexEnv("CREX_BACKEND=ghostty")
	if _, err := runCrex(t, layouts, env, "restore", "audit-gtabs", "--mode", "add"); err != nil {
		t.Fatalf("restore: %v", err)
	}
	t.Cleanup(func() { closeTabsFrom(t, baseTabs+1) })
	got := waitShellCWDs(t, before, dirs)
	if !multisetEqual(got, dirs) {
		t.Fatalf("per-tab cwds wrong: want %v, got %v", dirs, got)
	}

	// Save back and require each tab's cwd to be captured again.
	if _, err := runCrex(t, layouts, env, "save", "audit-gtabs-back"); err != nil {
		t.Fatalf("save-back: %v", err)
	}
	saved := loadLayout(t, layouts, "audit-gtabs-back")
	captured := map[string]int{}
	for _, ws := range saved.Workspaces {
		captured[canonPath(expandFixtureHome(ws.CWD))]++
	}
	for _, d := range dirs {
		if captured[d] == 0 {
			t.Fatalf("save-back lost per-tab cwd %s; captured %v", d, captured)
		}
	}
}

// TestGhostty_SplitsPerPaneCWDRoundtrip: splits inside one tab, each pane at
// its own path — restore must route every cd to the right split (Ghostty
// re-indexes terminals on split insertion) and save-back must keep them.
func TestGhostty_SplitsPerPaneCWDRoundtrip(t *testing.T) {
	requireGhostty(t)
	layouts := t.TempDir()
	docs := filepath.Join(homeDir, "Documents")
	downloads := filepath.Join(homeDir, "Downloads")
	writeLayout(t, layouts, &model.Layout{
		Name: "audit-gaside", Version: 1, SavedAt: time.Now(),
		Workspaces: []model.Workspace{{
			Title: "audit-gaside", CWD: homeDir, Index: 0,
			Panes: []model.Pane{
				{Type: "terminal", Focus: true, FocusTarget: -1},
				{Type: "terminal", Split: "right", CWD: docs, Index: 1, FocusTarget: 0},
				{Type: "terminal", Split: "down", CWD: downloads, Index: 2, FocusTarget: 1},
			},
		}},
	})

	baseTabs := tabCount(t)
	before := zshPids(t)
	env := crexEnv("CREX_BACKEND=ghostty")
	if _, err := runCrex(t, layouts, env, "restore", "audit-gaside", "--mode", "add"); err != nil {
		t.Fatalf("restore: %v", err)
	}
	t.Cleanup(func() { closeTabsFrom(t, baseTabs+1) })

	if n := terminalsInTab(t, baseTabs+1); n != 3 {
		t.Fatalf("split tab should have 3 terminals, has %d", n)
	}
	want := []string{homeDir, docs, downloads}
	got := waitShellCWDs(t, before, want)
	if !multisetEqual(got, want) {
		t.Fatalf("per-split cwds wrong (misrouted cd?): want %v, got %v", want, got)
	}

	if _, err := runCrex(t, layouts, env, "save", "audit-gaside-back"); err != nil {
		t.Fatalf("save-back: %v", err)
	}
	saved := loadLayout(t, layouts, "audit-gaside-back")
	var target *model.Workspace
	for i := range saved.Workspaces {
		if len(saved.Workspaces[i].Panes) == 3 {
			target = &saved.Workspaces[i]
			break
		}
	}
	if target == nil {
		t.Fatal("save-back: no 3-pane workspace found in saved layout")
	}
	paneCWDs := map[string]bool{}
	for _, p := range target.Panes {
		if p.CWD != "" {
			paneCWDs[canonPath(expandFixtureHome(p.CWD))] = true
		}
	}
	for _, d := range []string{docs, downloads} {
		if !paneCWDs[d] {
			t.Fatalf("save-back lost per-split cwd %s; captured %v", d, paneCWDs)
		}
	}
}

// expandFixtureHome expands a leading ~ the same way crex does, so save-back
// assertions tolerate layouts stored with either form.
func expandFixtureHome(p string) string {
	if p == "~" {
		return homeDir
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(homeDir, p[2:])
	}
	return p
}
