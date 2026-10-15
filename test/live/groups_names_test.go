//go:build live

package live

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/model"
)

// defaultAnchorTitle matches the title cmux gives the anchor of a group made
// from the sidebar ("Group 7"). It must never reach crex's output or a
// restored anchor: users know their groups by name, not by that number.
var defaultAnchorTitle = regexp.MustCompile(`Group \d+`)

// groupShell is the shell prelude the group scenarios share. wrap turns a
// workspace into the anchor of a new group the way restore does it (cmux's
// `create --from` always adds a fresh anchor, which is closed again) and
// prints the group ref.
const groupShell = `
ref() { awk '{print $2}'; }
wrap() {
  out=$(cmux --json workspace-group create --name "$1" --from "$2")
  g=$(echo "$out" | python3 -c "import json,sys;print(json.load(sys.stdin)['group']['ref'])")
  extra=$(echo "$out" | python3 -c "import json,sys;print(json.load(sys.stdin)['group']['anchor_workspace_ref'])")
  cmux workspace-group set-anchor --group "$g" --workspace "$2" >/dev/null
  cmux close-workspace --workspace "$extra" >/dev/null
  echo "$g"
}
`

// refuseLeftovers stops a scenario when a workspace with one of its titles is
// already open anywhere, so a crashed run is never adopted or closed by
// mistake.
func refuseLeftovers(t *testing.T, titles ...string) {
	t.Helper()

	for _, title := range titles {
		if wsByTitle(t, title) != nil {
			t.Fatalf("a workspace titled %q is already open in cmux — close it and rerun", title)
		}
	}
}

// requireNoDefaultAnchorTitle fails when crex printed an anchor as "Group N".
func requireNoDefaultAnchorTitle(t *testing.T, step, output string) {
	t.Helper()

	if match := defaultAnchorTitle.FindString(output); match != "" {
		t.Errorf("%s printed %q; anchors must show their group's name:\n%s", step, match, output)
	}
}

// requireGroupNames fails when crex's output leaves out a group's name.
func requireGroupNames(t *testing.T, step, output string, names ...string) {
	t.Helper()

	for _, name := range names {
		if !strings.Contains(output, name) {
			t.Errorf("%s output does not name group %q:\n%s", step, name, output)
		}
	}
}

// savedCommands returns every `command` value of a saved layout file, in
// file order.
func savedCommands(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved layout: %v", err)
	}

	var commands []string
	for _, line := range strings.Split(string(data), "\n") {
		value, found := strings.CutPrefix(strings.TrimSpace(line), "command = ")
		if found {
			commands = append(commands, strings.Trim(value, `'"`))
		}
	}

	return commands
}

// workspaceHasTTY reports whether cmux shows a terminal device behind any
// surface of a workspace: what tells save a real workspace from a ghost.
func workspaceHasTTY(t *testing.T, workspaceRef string) bool {
	t.Helper()

	out, err := cmuxRun(t, "--json", "tree", "--all")
	if err != nil {
		return false
	}
	var resp struct {
		Windows []struct {
			Workspaces []struct {
				Ref   string `json:"ref"`
				Panes []struct {
					Surfaces []struct {
						TTY string `json:"tty"`
					} `json:"surfaces"`
				} `json:"panes"`
			} `json:"workspaces"`
		} `json:"windows"`
	}
	if json.Unmarshal([]byte(out), &resp) != nil {
		return false
	}

	for _, window := range resp.Windows {
		for _, ws := range window.Workspaces {
			if ws.Ref != workspaceRef {
				continue
			}
			for _, pane := range ws.Panes {
				for _, surface := range pane.Surfaces {
					if surface.TTY != "" {
						return true
					}
				}
			}
		}
	}

	return false
}

// TestCmux_GroupAnchorsKeepTheirGroupName builds two groups whose anchors
// carry cmux's own "Group N" title, as groups made from the sidebar do, then
// saves and restores them into a fresh window. crex's save and restore output
// must name both groups and never print "Group N", the saved layout must not
// keep that title, and the restored anchors must be titled after their group.
func TestCmux_GroupAnchorsKeepTheirGroupName(t *testing.T) {
	requireCmux(t)
	const prefix = "crex-audit-gname"
	nameA := "🧪 " + prefix + " a"
	nameB := "🔥 " + prefix + " b"
	refuseLeftovers(t, "Group 901", "Group 902", nameA, nameB)

	sourceWindow, sourceSurface := openAuditWindow(t)

	// The anchors are renamed to "Group 90x" after wrapping: that is the
	// title cmux leaves on the anchor of a group made from the sidebar.
	setup := groupShell + fmt.Sprintf(`
A=$(cmux new-workspace --name %[1]s-a-anchor --cwd /tmp | ref)
GA=$(wrap %[2]q "$A")
cmux rename-workspace --workspace "$A" "Group 901" >/dev/null
cmux new-workspace --name %[1]s-a1 --cwd /usr --group "$GA" --group-placement end >/dev/null
B=$(cmux new-workspace --name %[1]s-b-anchor --cwd /usr | ref)
GB=$(wrap %[3]q "$B")
cmux rename-workspace --workspace "$B" "Group 902" >/dev/null
cmux new-workspace --name %[1]s-b1 --cwd /tmp --group "$GB" --group-placement end >/dev/null
cmux new-workspace --name %[1]s-loose --cwd /tmp >/dev/null
cmux workspace-group collapse "$GA" >/dev/null
cmux workspace-group pin "$GB" >/dev/null
`, prefix, nameA, nameB)
	runInWindow(t, sourceSurface, setup, 60*time.Second)

	source := windowGroups(t, sourceWindow)
	if len(source) != 2 || source[0].Anchor == source[0].Name {
		t.Fatalf("setup did not leave two groups with a default anchor title: %+v", source)
	}

	layouts := t.TempDir()
	saveOut := runInWindow(t, sourceSurface,
		fmt.Sprintf("%s --layouts-dir %s --config %s save audit-gname", crexBin, layouts, crexConf),
		90*time.Second)
	requireGroupNames(t, "save", saveOut, nameA, nameB)
	requireNoDefaultAnchorTitle(t, "save", saveOut)

	data, err := os.ReadFile(filepath.Join(layouts, "audit-gname.toml"))
	if err != nil {
		t.Fatalf("save produced no layout: %v", err)
	}
	if match := defaultAnchorTitle.FindString(string(data)); match != "" {
		t.Errorf("saved layout keeps the anchor title %q:\n%s", match, data)
	}

	targetWindow, targetSurface := openAuditWindow(t)
	restoreOut := runInWindow(t, targetSurface,
		fmt.Sprintf("%s --layouts-dir %s --config %s restore audit-gname --mode replace", crexBin, layouts, crexConf),
		180*time.Second)
	requireGroupNames(t, "restore", restoreOut, nameA, nameB)
	requireNoDefaultAnchorTitle(t, "restore", restoreOut)

	// The restored window must hold the same groups, with each anchor now
	// titled after its group instead of "Group 90x".
	restored := windowGroups(t, targetWindow)
	if len(restored) != len(source) {
		t.Fatalf("restored %d groups, want %d: %+v", len(restored), len(source), restored)
	}
	for index, want := range source {
		got := restored[index]
		if got.Name != want.Name || got.Collapsed != want.Collapsed || got.Pinned != want.Pinned {
			t.Errorf("group %d: got %+v, want %+v", index, got, want)
		}
		if got.Anchor != want.Name {
			t.Errorf("group %q: restored anchor is titled %q, want the group name", want.Name, got.Anchor)
		}
		if len(got.Members) != len(want.Members) {
			t.Errorf("group %q: restored members %v, want %d like %v", want.Name, got.Members, len(want.Members), want.Members)
		}
	}
}

// TestCmux_GroupKeepsMembersThatShareATitle puts two live workspaces with the
// same title in one group, as two shells in the same folder do. Save must keep
// both and restore must bring both back inside the group: dedupe used to keep
// only one of them, and that tab never came back.
func TestCmux_GroupKeepsMembersThatShareATitle(t *testing.T) {
	requireCmux(t)
	const prefix = "crex-audit-twin"
	groupName := "🎮 " + prefix
	twin := prefix + "-shell"
	refuseLeftovers(t, prefix+"-anchor", twin, groupName)

	_, sourceSurface := openAuditWindow(t)

	setup := groupShell + fmt.Sprintf(`
A=$(cmux new-workspace --name %[1]s-anchor --cwd /tmp | ref)
G=$(wrap %[2]q "$A")
echo "TWIN $(cmux new-workspace --name %[3]s --cwd /tmp --group "$G" --group-placement end | ref)"
echo "TWIN $(cmux new-workspace --name %[3]s --cwd /tmp --group "$G" --group-placement end | ref)"
`, prefix, groupName, twin)
	setupOut := runInWindow(t, sourceSurface, setup, 60*time.Second)

	// Both twins are shown once so cmux spawns their shells: a real
	// workspace has a terminal behind it, which is what save keeps.
	var twinRefs []string
	for _, line := range strings.Split(setupOut, "\n") {
		if ref, found := strings.CutPrefix(line, "TWIN "); found {
			twinRefs = append(twinRefs, strings.TrimSpace(ref))
		}
	}
	if len(twinRefs) != 2 {
		t.Fatalf("setup did not create two twins:\n%s", setupOut)
	}
	for _, ref := range twinRefs {
		_, _ = cmuxRun(t, "select-workspace", "--workspace", ref)
		if !waitFor(30*time.Second, time.Second, func() bool { return workspaceHasTTY(t, ref) }) {
			t.Fatalf("twin %s never got a terminal", ref)
		}
	}

	layouts := t.TempDir()
	runInWindow(t, sourceSurface,
		fmt.Sprintf("%s --layouts-dir %s --config %s save audit-twin", crexBin, layouts, crexConf),
		90*time.Second)
	data, err := os.ReadFile(filepath.Join(layouts, "audit-twin.toml"))
	if err != nil {
		t.Fatalf("save produced no layout: %v", err)
	}
	if count := strings.Count(string(data), "title = '"+twin+"'"); count != 2 {
		t.Fatalf("saved layout holds %d workspaces titled %q, want 2:\n%s", count, twin, data)
	}

	targetWindow, targetSurface := openAuditWindow(t)
	runInWindow(t, targetSurface,
		fmt.Sprintf("%s --layouts-dir %s --config %s restore audit-twin --mode replace", crexBin, layouts, crexConf),
		180*time.Second)

	restored := windowGroups(t, targetWindow)
	if len(restored) != 1 {
		t.Fatalf("restored %d groups, want 1: %+v", len(restored), restored)
	}
	twins := 0
	for _, member := range restored[0].Members {
		if member == twin {
			twins++
		}
	}
	if twins != 2 {
		t.Errorf("restored group holds %d workspaces titled %q, want 2: %v", twins, twin, restored[0].Members)
	}
}

// TestCmux_SaveFromCrexPaneStoresNoCrexCommand types `crex save` straight
// into a shell, as a user does, so crex is that pane's foreground process
// while it saves. The layout must not store any crex command: restoring it
// would type `crex save` again and overwrite the layout halfway through.
func TestCmux_SaveFromCrexPaneStoresNoCrexCommand(t *testing.T) {
	requireCmux(t)

	_, sourceSurface := openAuditWindow(t)

	layouts := t.TempDir()
	marker := filepath.Join(t.TempDir(), "saved")
	command := fmt.Sprintf("%s --layouts-dir %s --config %s save audit-self >/dev/null 2>&1; touch %s\n",
		crexBin, layouts, crexConf, marker)
	_, _ = cmuxRun(t, "send", "--surface", sourceSurface, command)
	if !waitFor(90*time.Second, time.Second, func() bool {
		_, err := os.Stat(marker)
		return err == nil
	}) {
		t.Fatal("crex save typed into the audit window never finished")
	}

	path := filepath.Join(layouts, "audit-self.toml")
	for _, saved := range savedCommands(t, path) {
		program := strings.Fields(saved)
		if len(program) == 0 {
			continue
		}
		if filepath.Base(program[0]) == "crex" || program[0] == crexBin {
			data, _ := os.ReadFile(path)
			t.Fatalf("layout stores crex's own command %q:\n%s", saved, data)
		}
	}
}

// TestCmux_OldLayoutWithDefaultAnchorTitleRestoresGroupName restores a layout
// written by v1.28.0, whose anchor kept cmux's "Group 2" title and whose pane
// remembered the `crex save now` it was saved from. The anchor must come back
// titled after its group and crex must never type that command.
func TestCmux_OldLayoutWithDefaultAnchorTitleRestoresGroupName(t *testing.T) {
	requireCmux(t)
	const prefix = "crex-audit-legacy"
	groupName := "🗿 " + prefix
	member := prefix + "-member"
	refuseLeftovers(t, groupName, member)

	layouts := t.TempDir()
	anchor := singlePaneWorkspace("Group 2", "/tmp", 0)
	anchor.Group = groupName
	anchor.GroupAnchor = true
	anchor.Panes[0].Command = "crex save now"
	memberWorkspace := singlePaneWorkspace(member, "/usr", 1)
	memberWorkspace.Group = groupName
	writeLayout(t, layouts, &model.Layout{
		Name: "audit-legacy", Version: 1, SavedAt: time.Now(),
		Groups:     []model.Group{{Name: groupName}},
		Workspaces: []model.Workspace{anchor, memberWorkspace},
	})

	targetWindow, targetSurface := openAuditWindow(t)
	restoreOut := runInWindow(t, targetSurface,
		fmt.Sprintf("%s --layouts-dir %s --config %s restore audit-legacy --mode add", crexBin, layouts, crexConf),
		180*time.Second)
	requireNoDefaultAnchorTitle(t, "restore", restoreOut)

	restored := windowGroups(t, targetWindow)
	if len(restored) != 1 || restored[0].Name != groupName {
		t.Fatalf("restored groups %+v, want one named %q", restored, groupName)
	}
	if restored[0].Anchor != groupName {
		t.Errorf("restored anchor is titled %q, want %q", restored[0].Anchor, groupName)
	}
	for _, title := range workspaceTitlesIn(t, targetWindow) {
		if title == "Group 2" {
			t.Errorf("a restored workspace is still titled %q", title)
		}
	}

	// The old command must not have been typed into the restored anchor.
	anchorWorkspace := wsByTitle(t, groupName)
	if anchorWorkspace == nil {
		t.Fatalf("no workspace titled %q after restore", groupName)
	}
	for _, surface := range surfaceRefs(anchorWorkspace) {
		screen, _ := cmuxRun(t, "read-screen", "--surface", surface)
		if strings.Contains(screen, "crex save now") {
			t.Errorf("restore typed the old crex command into %s:\n%s", surface, screen)
		}
	}
}
