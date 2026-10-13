//go:build live

package live

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// liveGroup is one sidebar group as the assertions compare it: member titles
// instead of refs, because refs renumber between the saved window and the
// restored one while titles survive.
type liveGroup struct {
	Name      string
	Anchor    string
	Members   []string
	Collapsed bool
	Pinned    bool
	Color     string
	Icon      string
}

// windowGroups reads the groups of one window from cmux itself — the ground
// truth for "did restore rebuild the groups", independent of crex's output.
func windowGroups(t *testing.T, windowRef string) []liveGroup {
	t.Helper()

	titles := workspaceTitlesIn(t, windowRef)
	out, err := cmuxRun(t, "--json", "workspace-group", "list", "--window", windowRef)
	if err != nil {
		t.Fatalf("workspace-group list: %v", err)
	}
	var resp struct {
		Groups []struct {
			Name      string   `json:"name"`
			Anchor    string   `json:"anchor_workspace_ref"`
			Members   []string `json:"member_workspace_refs"`
			Collapsed bool     `json:"is_collapsed"`
			Pinned    bool     `json:"is_pinned"`
			Color     *string  `json:"custom_color"`
			Icon      *string  `json:"icon_symbol"`
		} `json:"groups"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("parse workspace-group list: %v\n%s", err, out)
	}

	groups := make([]liveGroup, 0, len(resp.Groups))
	for _, g := range resp.Groups {
		group := liveGroup{
			Name:      g.Name,
			Anchor:    titles[g.Anchor],
			Collapsed: g.Collapsed,
			Pinned:    g.Pinned,
		}
		// cmux reports an unset colour or icon as null.
		if g.Color != nil {
			group.Color = *g.Color
		}
		if g.Icon != nil {
			group.Icon = *g.Icon
		}
		for _, ref := range g.Members {
			group.Members = append(group.Members, titles[ref])
		}
		groups = append(groups, group)
	}

	return groups
}

// workspaceTitlesIn maps every workspace ref of one window to its title.
func workspaceTitlesIn(t *testing.T, windowRef string) map[string]string {
	t.Helper()

	out, err := cmuxRun(t, "--json", "list-workspaces", "--window", windowRef)
	if err != nil {
		t.Fatalf("list-workspaces: %v", err)
	}
	var resp struct {
		Workspaces []struct {
			Ref   string `json:"ref"`
			Title string `json:"title"`
		} `json:"workspaces"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("parse list-workspaces: %v\n%s", err, out)
	}

	titles := make(map[string]string, len(resp.Workspaces))
	for _, w := range resp.Workspaces {
		titles[w.Ref] = w.Title
	}

	return titles
}

// openAuditWindow opens a fresh cmux window, registers its teardown at once,
// and returns its ref and the terminal surface of its first workspace once
// that shell can take input.
func openAuditWindow(t *testing.T) (windowRef, surfaceRef string) {
	t.Helper()

	before := windowRefs(t)
	if _, err := cmuxRun(t, "new-window"); err != nil {
		t.Fatalf("new-window: %v", err)
	}
	waitFor(30*time.Second, time.Second, func() bool {
		for ref := range windowRefs(t) {
			if !before[ref] {
				windowRef = ref
				return true
			}
		}
		return false
	})
	if windowRef == "" {
		t.Fatal("new cmux window never appeared")
	}
	// Registered before any further Fatal so a failed run never leaks the window.
	t.Cleanup(func() { closeWindowByWorkspaces(t, windowRef) })

	waitFor(30*time.Second, time.Second, func() bool {
		_, surfaceRef = windowWorkspace(t, windowRef)
		return surfaceRef != ""
	})
	if surfaceRef == "" {
		t.Fatalf("no terminal surface found in %s", windowRef)
	}
	waitFor(60*time.Second, time.Second, func() bool {
		_, ready := terminals(t)
		return ready[surfaceRef]
	})

	return windowRef, surfaceRef
}

// runInWindow types a script into a window's shell and waits for its marker
// file. Group commands act on the CALLER's window and the caller cannot be
// faked, so every step that must land in the audit window runs from inside it.
func runInWindow(t *testing.T, surfaceRef, script string, timeout time.Duration) {
	t.Helper()

	dir := t.TempDir()
	marker := filepath.Join(dir, "done")
	path := filepath.Join(dir, "step.sh")
	body := "#!/bin/bash\nexport CMUX_QUIET=1\n" + script + "\ntouch " + marker + "\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	_, _ = cmuxRun(t, "send", "--surface", surfaceRef, path+" >"+filepath.Join(dir, "log")+" 2>&1\n")
	if !waitFor(timeout, time.Second, func() bool {
		_, err := os.Stat(marker)
		return err == nil
	}) {
		log, _ := os.ReadFile(filepath.Join(dir, "log"))
		t.Fatalf("step in the audit window never finished:\n%s\n--- log ---\n%s", script, log)
	}
}

// TestCmux_GroupsSurviveSaveAndRestore builds two sidebar groups in a fresh
// window (emoji names, a split member, one group collapsed with colour and
// icon, the other pinned), saves that window, restores the layout into a
// second fresh window, and requires cmux to report the same groups there:
// names, anchors, members in order and state. Before workspace-group support
// the groups came back flat and the anchors as stray "Group N" workspaces.
func TestCmux_GroupsSurviveSaveAndRestore(t *testing.T) {
	requireCmux(t)
	const prefix = "crex-audit-grp"
	for _, title := range []string{prefix + "-anchor-a", prefix + "-anchor-b"} {
		if wsByTitle(t, title) != nil {
			t.Fatalf("a workspace titled %q is already open in cmux — close it and rerun", title)
		}
	}

	sourceWindow, sourceSurface := openAuditWindow(t)

	// cmux's `group create --from` always adds a fresh anchor, so the saved
	// anchor is wrapped the same way restore does it: create, set-anchor,
	// close the extra one.
	split := `{"direction":"horizontal","split":0.5,"children":[{"pane":{"surfaces":[{"type":"terminal","cwd":"/tmp"}]}},{"pane":{"surfaces":[{"type":"terminal","cwd":"/usr"}]}}]}`
	setup := fmt.Sprintf(`
ref() { awk '{print $2}'; }
wrap() {
  out=$(cmux --json workspace-group create --name "$1" --from "$2")
  g=$(echo "$out" | python3 -c "import json,sys;print(json.load(sys.stdin)['group']['ref'])")
  extra=$(echo "$out" | python3 -c "import json,sys;print(json.load(sys.stdin)['group']['anchor_workspace_ref'])")
  cmux workspace-group set-anchor --group "$g" --workspace "$2" >/dev/null
  cmux close-workspace --workspace "$extra" >/dev/null
  echo "$g"
}
A=$(cmux new-workspace --name %[1]s-anchor-a --cwd /tmp | ref)
GA=$(wrap "🧪 %[1]s a" "$A")
cmux new-workspace --name %[1]s-a1 --layout '%[2]s' --group "$GA" --group-placement end >/dev/null
cmux new-workspace --name %[1]s-a2 --cwd /usr --group "$GA" --group-placement end >/dev/null
B=$(cmux new-workspace --name %[1]s-anchor-b --cwd /usr | ref)
GB=$(wrap "🔥 %[1]s b" "$B")
cmux new-workspace --name %[1]s-b1 --cwd /tmp --group "$GB" --group-placement end >/dev/null
cmux new-workspace --name %[1]s-loose --cwd /tmp >/dev/null
cmux workspace-group collapse "$GA" >/dev/null
cmux workspace-group set-color "$GA" --hex '#FF8800' >/dev/null
cmux workspace-group set-icon "$GA" --symbol flame >/dev/null
cmux workspace-group pin "$GB" >/dev/null
`, prefix, split)
	runInWindow(t, sourceSurface, setup, 60*time.Second)

	want := windowGroups(t, sourceWindow)
	if len(want) != 2 {
		t.Fatalf("setup built %d groups, want 2: %+v", len(want), want)
	}

	layouts := t.TempDir()
	runInWindow(t, sourceSurface,
		fmt.Sprintf("%s --layouts-dir %s --config %s save audit-groups", crexBin, layouts, crexConf),
		90*time.Second)
	data, err := os.ReadFile(filepath.Join(layouts, "audit-groups.toml"))
	if err != nil {
		t.Fatalf("save produced no layout: %v", err)
	}
	if n := strings.Count(string(data), "[[group]]"); n != 2 {
		t.Fatalf("saved layout has %d [[group]] tables, want 2:\n%s", n, data)
	}

	targetWindow, targetSurface := openAuditWindow(t)
	runInWindow(t, targetSurface,
		fmt.Sprintf("%s --layouts-dir %s --config %s restore audit-groups --mode replace", crexBin, layouts, crexConf),
		180*time.Second)

	got := windowGroups(t, targetWindow)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("restored groups differ from the saved window\nwant: %+v\ngot:  %+v", want, got)
	}

	// No anchor may come back outside its group: that was the visible bug.
	grouped := make(map[string]bool)
	for _, group := range got {
		for _, member := range group.Members {
			grouped[member] = true
		}
	}
	for _, title := range workspaceTitlesIn(t, targetWindow) {
		if strings.Contains(title, "-anchor-") && !grouped[title] {
			t.Errorf("anchor %q was restored as a loose workspace", title)
		}
	}
}
