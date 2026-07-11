//go:build live

package live

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// cmux ground-truth helpers: the harness talks to the cmux CLI directly and
// asserts on pane pixel frames and debug.terminals, never on crex's output.
// ---------------------------------------------------------------------------

func cmuxPath() string {
	if p, err := exec.LookPath("cmux"); err == nil {
		return p
	}
	return "/Applications/cmux.app/Contents/Resources/bin/cmux"
}

func cmuxRun(t *testing.T, args ...string) (string, error) {
	t.Helper()
	out, err := exec.Command(cmuxPath(), args...).Output()
	return string(out), err
}

type cmuxSurface struct {
	Ref string `json:"ref"`
}

type cmuxPane struct {
	Surfaces []cmuxSurface `json:"surfaces"`
}

type cmuxWorkspace struct {
	Ref   string     `json:"ref"`
	ID    string     `json:"id"`
	Title string     `json:"title"`
	Panes []cmuxPane `json:"panes"`
}

type cmuxTree struct {
	Windows []struct {
		Workspaces []cmuxWorkspace `json:"workspaces"`
	} `json:"windows"`
}

func tree(t *testing.T) cmuxTree {
	t.Helper()
	out, err := cmuxRun(t, "tree", "--json", "--id-format", "both")
	if err != nil {
		t.Fatalf("cmux tree: %v", err)
	}
	var tr cmuxTree
	if err := json.Unmarshal([]byte(out), &tr); err != nil {
		t.Fatalf("cmux tree JSON: %v", err)
	}
	return tr
}

func cmuxAlive() bool {
	err := exec.Command(cmuxPath(), "tree", "--json").Run()
	return err == nil
}

func wsRefs(t *testing.T) map[string]bool {
	refs := map[string]bool{}
	for _, w := range tree(t).Windows {
		for _, ws := range w.Workspaces {
			refs[ws.Ref] = true
		}
	}
	return refs
}

func wsByRef(t *testing.T, ref string) *cmuxWorkspace {
	for _, w := range tree(t).Windows {
		for _, ws := range w.Workspaces {
			if ws.Ref == ref {
				return &ws
			}
		}
	}
	return nil
}

func wsByTitle(t *testing.T, title string) *cmuxWorkspace {
	for _, w := range tree(t).Windows {
		for _, ws := range w.Workspaces {
			if ws.Title == title {
				return &ws
			}
		}
	}
	return nil
}

func surfaceRefs(ws *cmuxWorkspace) []string {
	var refs []string
	for _, p := range ws.Panes {
		for _, s := range p.Surfaces {
			refs = append(refs, s.Ref)
		}
	}
	return refs
}

// terminals returns surface_ref → current_directory and the ready set.
func terminals(t *testing.T) (map[string]string, map[string]bool) {
	t.Helper()
	out, err := cmuxRun(t, "rpc", "debug.terminals")
	if err != nil {
		t.Fatalf("cmux rpc debug.terminals: %v", err)
	}
	var d struct {
		Terminals []struct {
			SurfaceRef          string  `json:"surface_ref"`
			CurrentDirectory    string  `json:"current_directory"`
			RuntimeSurfaceReady bool    `json:"runtime_surface_ready"`
			TTY                 *string `json:"tty"`
		} `json:"terminals"`
	}
	if err := json.Unmarshal([]byte(out), &d); err != nil {
		t.Fatalf("debug.terminals JSON: %v", err)
	}
	// cmux ≥0.64 flips runtime_surface_ready at render, before the shell
	// accepts input, but reports live shells' ttys — mirror the product's
	// gate: when ttys are reported, a surface without one isn't ready.
	ttysReported := false
	for _, term := range d.Terminals {
		if term.TTY != nil && *term.TTY != "" {
			ttysReported = true
			break
		}
	}
	cwds, ready := map[string]string{}, map[string]bool{}
	for _, term := range d.Terminals {
		cwds[term.SurfaceRef] = term.CurrentDirectory
		if term.RuntimeSurfaceReady && (!ttysReported || (term.TTY != nil && *term.TTY != "")) {
			ready[term.SurfaceRef] = true
		}
	}
	return cwds, ready
}

type paneGeom struct {
	X, Y, W, H int
	CWD        string
}

func (g paneGeom) shape() string { return fmt.Sprintf("%d,%d,%d,%d", g.X, g.Y, g.W, g.H) }
func (g paneGeom) String() string {
	return fmt.Sprintf("(%d,%d %dx%d %s)", g.X, g.Y, g.W, g.H, g.CWD)
}

// paneGeoms returns each pane's pixel frame plus its first surface's cwd.
func paneGeoms(t *testing.T, wsRef string) []paneGeom {
	t.Helper()
	ws := wsByRef(t, wsRef)
	if ws == nil {
		t.Fatalf("workspace %s not in tree", wsRef)
	}
	arg, _ := json.Marshal(map[string]string{"workspace_id": ws.ID})
	out, err := cmuxRun(t, "rpc", "pane.list", string(arg))
	if err != nil {
		t.Fatalf("cmux rpc pane.list: %v", err)
	}
	var d struct {
		Panes []struct {
			SurfaceRefs []string `json:"surface_refs"`
			PixelFrame  struct {
				X, Y, Width, Height float64
			} `json:"pixel_frame"`
		} `json:"panes"`
	}
	if err := json.Unmarshal([]byte(out), &d); err != nil {
		t.Fatalf("pane.list JSON: %v", err)
	}
	cwds, _ := terminals(t)
	var geoms []paneGeom
	for _, p := range d.Panes {
		cwd := ""
		if len(p.SurfaceRefs) > 0 {
			cwd = cwds[p.SurfaceRefs[0]]
		}
		geoms = append(geoms, paneGeom{
			X: int(p.PixelFrame.X + 0.5), Y: int(p.PixelFrame.Y + 0.5),
			W: int(p.PixelFrame.Width + 0.5), H: int(p.PixelFrame.Height + 0.5),
			CWD: cwd,
		})
	}
	return geoms
}

func geomCWDs(geoms []paneGeom) []string {
	var cwds []string
	for _, g := range geoms {
		cwds = append(cwds, g.CWD)
	}
	return cwds
}

func geomPlacements(geoms []paneGeom) []string {
	var out []string
	for _, g := range geoms {
		out = append(out, g.shape()+"|"+g.CWD)
	}
	return out
}

func geomShapes(geoms []paneGeom) []string {
	var out []string
	for _, g := range geoms {
		out = append(out, g.shape())
	}
	return out
}

func closeWorkspace(t *testing.T, ref string) {
	t.Helper()
	_, _ = cmuxRun(t, "close-workspace", "--workspace", ref)
}

// newWorkspaces runs fn and returns the refs of workspaces that appeared.
func newWorkspaces(t *testing.T, wantCount int, fn func()) []string {
	t.Helper()
	before := wsRefs(t)
	fn()
	var created []string
	ok := waitFor(30*time.Second, 500*time.Millisecond, func() bool {
		created = created[:0]
		for ref := range wsRefs(t) {
			if !before[ref] {
				created = append(created, ref)
			}
		}
		return len(created) >= wantCount
	})
	if !ok {
		t.Fatalf("expected %d new workspaces, got %d", wantCount, len(created))
	}
	return created
}

// waitPaneCWDs waits until the workspace's pane cwd multiset equals want.
func waitPaneCWDs(t *testing.T, wsRef string, want []string) []paneGeom {
	t.Helper()
	var geoms []paneGeom
	waitFor(60*time.Second, time.Second, func() bool {
		geoms = paneGeoms(t, wsRef)
		return multisetEqual(geomCWDs(geoms), want)
	})
	return geoms
}

func requireCmux(t *testing.T) {
	t.Helper()
	if !cmuxAlive() {
		backendMissing(t, "cmux", "open the cmux app and retry")
	}
}

// ---------------------------------------------------------------------------
// Matrix: cmux
// ---------------------------------------------------------------------------

// TestCmux_DemoQuad restores the bundled demo layout and asserts the 📁 files
// workspace comes back as a true 2×2 grid with each pane in its own folder.
func TestCmux_DemoQuad(t *testing.T) {
	requireCmux(t)
	layouts := t.TempDir()
	installDemo(t, layouts)

	created := newWorkspaces(t, 2, func() {
		if _, err := runCrex(t, layouts, crexEnv("CREX_BACKEND=cmux"), "restore", "demo", "--mode", "add"); err != nil {
			t.Fatalf("restore demo: %v", err)
		}
	})
	t.Cleanup(func() {
		for _, ref := range created {
			closeWorkspace(t, ref)
		}
	})

	files := wsByTitle(t, "📁 files")
	if files == nil {
		t.Fatal("restored '📁 files' workspace not found in tree")
	}
	home := wsByTitle(t, "🏠 home")
	if home == nil {
		t.Fatal("restored '🏠 home' workspace not found in tree")
	}
	if n := len(home.Panes); n != 1 {
		t.Fatalf("🏠 home should have 1 pane, has %d", n)
	}

	geoms := waitPaneCWDs(t, files.Ref, demoQuadCWDs())
	if len(geoms) != 4 {
		t.Fatalf("📁 files should have 4 panes, has %d: %v", len(geoms), geoms)
	}
	if !multisetEqual(geomCWDs(geoms), demoQuadCWDs()) {
		t.Fatalf("per-pane cwds wrong.\nwant %v\ngot  %v", demoQuadCWDs(), geoms)
	}
	xs, ys := map[int]bool{}, map[int]bool{}
	for _, g := range geoms {
		xs[g.X], ys[g.Y] = true, true
	}
	if len(xs) != 2 || len(ys) != 2 {
		t.Fatalf("not a 2×2 grid (distinct x=%d, y=%d): %v", len(xs), len(ys), geoms)
	}
}

// TestCmux_SaveRestoreResaveAside builds an aside layout by hand (full-height
// left pane + split right column), saves it, RE-SAVES over the same name (a
// hard product precondition), closes it, restores it, and requires the pixel
// shape and per-pane cwd placement to match the original exactly.
func TestCmux_SaveRestoreResaveAside(t *testing.T) {
	requireCmux(t)
	layouts := t.TempDir()
	docs := filepath.Join(homeDir, "Documents")
	downloads := filepath.Join(homeDir, "Downloads")

	// Build: home full-height left, Documents top-right, Downloads bottom-right.
	refs := newWorkspaces(t, 1, func() {
		if _, err := cmuxRun(t, "new-workspace", "--cwd", homeDir); err != nil {
			t.Fatalf("new-workspace: %v", err)
		}
	})
	wsRef := refs[0]
	closed := false
	closeOnce := func(ref string) {
		if !closed {
			closeWorkspace(t, ref)
		}
	}
	t.Cleanup(func() { closeOnce(wsRef) })
	_, _ = cmuxRun(t, "select-workspace", "--workspace", wsRef)

	surfHome := surfaceRefs(wsByRef(t, wsRef))[0]
	if _, err := cmuxRun(t, "new-split", "right", "--workspace", wsRef, "--surface", surfHome); err != nil {
		t.Fatalf("new-split right: %v", err)
	}
	var surfDocs string
	waitFor(15*time.Second, 500*time.Millisecond, func() bool {
		for _, s := range surfaceRefs(wsByRef(t, wsRef)) {
			if s != surfHome {
				surfDocs = s
				return true
			}
		}
		return false
	})
	if _, err := cmuxRun(t, "new-split", "down", "--workspace", wsRef, "--surface", surfDocs); err != nil {
		t.Fatalf("new-split down: %v", err)
	}
	var surfDownloads string
	waitFor(15*time.Second, 500*time.Millisecond, func() bool {
		for _, s := range surfaceRefs(wsByRef(t, wsRef)) {
			if s != surfHome && s != surfDocs {
				surfDownloads = s
				return true
			}
		}
		return false
	})

	waitFor(45*time.Second, 500*time.Millisecond, func() bool {
		_, ready := terminals(t)
		return ready[surfHome] && ready[surfDocs] && ready[surfDownloads]
	})
	_, _ = cmuxRun(t, "send", "--workspace", wsRef, "--surface", surfDocs, fmt.Sprintf("cd '%s'\\n", docs))
	_, _ = cmuxRun(t, "send", "--workspace", wsRef, "--surface", surfDownloads, fmt.Sprintf("cd '%s'\\n", downloads))
	orig := waitPaneCWDs(t, wsRef, []string{homeDir, docs, downloads})
	if !multisetEqual(geomCWDs(orig), []string{homeDir, docs, downloads}) {
		t.Fatalf("harness setup: cds never landed, got %v", orig)
	}
	// Give the workspace a distinctive title: `--mode add` skips layouts
	// whose titles are already open, and a default home-ish title can
	// collide with the user's real workspaces.
	title := "crex-audit-aside"
	if _, err := cmuxRun(t, "rename-workspace", "--workspace", wsRef, title); err != nil {
		t.Fatalf("rename-workspace: %v", err)
	}
	t.Logf("original: %v", orig)

	// Save, then re-save over the same name.
	env := crexEnv("CREX_BACKEND=cmux")
	if _, err := runCrex(t, layouts, env, "save", "audit-aside"); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := runCrex(t, layouts, env, "save", "audit-aside"); err != nil {
		t.Fatalf("re-save over existing name: %v", err)
	}
	if _, err := os.Stat(filepath.Join(layouts, "audit-aside.toml")); err != nil {
		t.Fatalf("layout file missing after save: %v", err)
	}

	// Close and restore.
	closeWorkspace(t, wsRef)
	closed = true
	time.Sleep(2 * time.Second)
	restoredAll := newWorkspaces(t, 1, func() {
		if _, err := runCrex(t, layouts, env, "restore", "audit-aside", "--mode", "add"); err != nil {
			t.Fatalf("restore: %v", err)
		}
	})
	t.Cleanup(func() {
		for _, ref := range restoredAll {
			closeWorkspace(t, ref)
		}
	})
	var newRef string
	for _, ref := range restoredAll {
		if ws := wsByRef(t, ref); ws != nil && ws.Title == title {
			newRef = ref
		}
	}
	if newRef == "" {
		newRef = restoredAll[0]
	}
	_, _ = cmuxRun(t, "select-workspace", "--workspace", newRef)

	rest := waitPaneCWDs(t, newRef, geomCWDs(orig))
	t.Logf("restored: %v", rest)
	if !multisetEqual(geomShapes(orig), geomShapes(rest)) {
		t.Fatalf("pixel shape not preserved after re-save+restore.\norig %v\ngot  %v", orig, rest)
	}
	if !multisetEqual(geomPlacements(orig), geomPlacements(rest)) {
		t.Fatalf("cwd placement not preserved after re-save+restore.\norig %v\ngot  %v", orig, rest)
	}
}
