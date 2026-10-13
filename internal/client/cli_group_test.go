package client

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// groupShim is a fake cmux binary that records every invocation and answers
// the few commands whose output the group code reads.
type groupShim struct {
	binary  string
	callLog string
}

// argSeparator splits the arguments of one invocation in the call log. It is
// the ASCII unit separator because group names carry spaces and emoji, so
// neither a space nor a tab could tell one argument from the next.
const argSeparator = "\x1f"

// writeGroupShim installs a fake cmux that logs each argument exactly as it
// arrived, prints the real 0.64.22 JSON for `workspace-group list` and
// `create`, and makes `list-workspaces` report one extra workspace once
// `new-workspace` has run, so createWorkspace can discover the new ref. When
// failGroups is set every `workspace-group` call exits non-zero.
func writeGroupShim(t *testing.T, failGroups bool) groupShim {
	t.Helper()

	dir := t.TempDir()
	listFixture, err := filepath.Abs("../../testdata/responses/workspace-group-list.json")
	if err != nil {
		t.Fatal(err)
	}
	createFixture, err := filepath.Abs("../../testdata/responses/workspace-group-create.json")
	if err != nil {
		t.Fatal(err)
	}

	shim := groupShim{
		binary:  filepath.Join(dir, "cmux"),
		callLog: filepath.Join(dir, "calls.log"),
	}
	createdMarker := filepath.Join(dir, "created")

	// The failure switch is decided here rather than inside the script so
	// the script stays a plain list of canned answers.
	failExit := "0"
	if failGroups {
		failExit = "1"
	}

	script := fmt.Sprintf(`#!/bin/sh
for arg in "$@"; do printf '%%s\037' "$arg" >> %[1]q; done
printf '\n' >> %[1]q
case "$*" in
  *"workspace-group"*)
    if [ %[5]q = "1" ]; then echo "boom" >&2; exit 1; fi
    case "$*" in
      *"workspace-group list"*) cat %[2]q ;;
      *"workspace-group create"*) cat %[3]q ;;
    esac
    ;;
  "list-workspaces")
    echo "  workspace:1  0 existing"
    if [ -f %[4]q ]; then echo "  workspace:2  1 created"; fi
    ;;
  new-workspace*)
    touch %[4]q
    ;;
esac
exit 0
`, shim.callLog, listFixture, createFixture, createdMarker, failExit)

	if err := os.WriteFile(shim.binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return shim
}

// client returns a CLIClient wired to the shim.
func (shim groupShim) client() *CLIClient {
	return &CLIClient{Binary: shim.binary, Timeout: 5 * time.Second}
}

// invocations returns every recorded call as its exact argument list, in the
// order cmux received them.
func (shim groupShim) invocations(t *testing.T) [][]string {
	t.Helper()

	data, err := os.ReadFile(shim.callLog)
	if err != nil {
		t.Fatalf("read call log: %v", err)
	}

	var result [][]string
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		arguments := strings.Split(strings.TrimSuffix(line, argSeparator), argSeparator)
		result = append(result, arguments)
	}
	return result
}

// onlyInvocation asserts that exactly one call reached cmux and returns it.
func (shim groupShim) onlyInvocation(t *testing.T) []string {
	t.Helper()

	all := shim.invocations(t)
	if len(all) != 1 {
		t.Fatalf("got %d cmux invocations, want 1: %q", len(all), all)
	}
	return all[0]
}

func TestListGroups_CallerWindow(t *testing.T) {
	shim := writeGroupShim(t, false)

	groups, err := shim.client().ListGroups("")
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}

	// --json has to precede the subcommand: that is the form the live probe
	// verified, and without --window cmux answers for the caller's window.
	want := []string{"--json", "workspace-group", "list"}
	if got := shim.onlyInvocation(t); !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %q, want %q", got, want)
	}
	if len(groups) != 2 || groups[0].Name != "🌋 Dos grupo" {
		t.Errorf("groups = %+v, want the two fixture groups", groups)
	}
}

func TestListGroups_ExplicitWindow(t *testing.T) {
	shim := writeGroupShim(t, false)

	if _, err := shim.client().ListGroups("window:4"); err != nil {
		t.Fatalf("ListGroups: %v", err)
	}

	want := []string{"--json", "workspace-group", "list", "--window", "window:4"}
	if got := shim.onlyInvocation(t); !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %q, want %q", got, want)
	}
}

func TestCreateGroup_PassesNameAndMembers(t *testing.T) {
	shim := writeGroupShim(t, false)

	group, err := shim.client().CreateGroup("🌋 Dos grupo", []string{"workspace:74", "workspace:80"})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}

	// The emoji name must reach cmux as one argument, untouched.
	want := []string{"--json", "workspace-group", "create", "--name", "🌋 Dos grupo", "--from", "workspace:74,workspace:80"}
	if got := shim.onlyInvocation(t); !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %q, want %q", got, want)
	}

	// cmux always makes a new anchor; the caller learns it from AnchorRef.
	if group.Ref != "workspace_group:16" || group.AnchorRef != "workspace:75" {
		t.Errorf("group = %+v, want ref workspace_group:16 with anchor workspace:75", group)
	}
}

func TestCreateGroup_WithoutMembers(t *testing.T) {
	shim := writeGroupShim(t, false)

	if _, err := shim.client().CreateGroup("Solo", nil); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}

	want := []string{"--json", "workspace-group", "create", "--name", "Solo"}
	if got := shim.onlyInvocation(t); !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %q, want %q", got, want)
	}
}

// Every mutating call is a fire-and-forget command; the table pins the exact
// flag shape each one takes, as printed by `cmux workspace-group --help`:
// some take the group positionally, others only through --group.
func TestGroupMutations_Argv(t *testing.T) {
	cases := []struct {
		name string
		call func(manager GroupManager) error
		want []string
	}{
		{
			name: "SetGroupAnchor",
			call: func(manager GroupManager) error {
				return manager.SetGroupAnchor("workspace_group:16", "workspace:74")
			},
			want: []string{"--json", "workspace-group", "set-anchor", "--group", "workspace_group:16", "--workspace", "workspace:74"},
		},
		{
			name: "AddToGroup",
			call: func(manager GroupManager) error {
				return manager.AddToGroup("workspace_group:16", "workspace:79")
			},
			want: []string{"--json", "workspace-group", "add", "--group", "workspace_group:16", "--workspace", "workspace:79"},
		},
		{
			name: "RenameGroup",
			call: func(manager GroupManager) error {
				return manager.RenameGroup("workspace_group:16", "🎮 Prueba uno")
			},
			want: []string{"--json", "workspace-group", "rename", "workspace_group:16", "--name", "🎮 Prueba uno"},
		},
		{
			name: "CollapseGroup",
			call: func(manager GroupManager) error {
				return manager.CollapseGroup("workspace_group:16")
			},
			want: []string{"--json", "workspace-group", "collapse", "workspace_group:16"},
		},
		{
			name: "PinGroup",
			call: func(manager GroupManager) error {
				return manager.PinGroup("workspace_group:16")
			},
			want: []string{"--json", "workspace-group", "pin", "workspace_group:16"},
		},
		{
			name: "SetGroupColor",
			call: func(manager GroupManager) error {
				return manager.SetGroupColor("workspace_group:16", "#FF8800")
			},
			want: []string{"--json", "workspace-group", "set-color", "workspace_group:16", "--hex", "#FF8800"},
		},
		{
			name: "SetGroupIcon",
			call: func(manager GroupManager) error {
				return manager.SetGroupIcon("workspace_group:16", "flame")
			},
			want: []string{"--json", "workspace-group", "set-icon", "workspace_group:16", "--symbol", "flame"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			shim := writeGroupShim(t, false)

			if err := testCase.call(shim.client()); err != nil {
				t.Fatalf("%s: %v", testCase.name, err)
			}
			if got := shim.onlyInvocation(t); !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("argv = %q, want %q", got, testCase.want)
			}
		})
	}
}

// A failing cmux must surface as an error on every group call; restore
// decides what to do with it, the client never swallows it.
func TestGroupCalls_PropagateFailure(t *testing.T) {
	shim := writeGroupShim(t, true)
	manager := shim.client()

	if _, err := manager.ListGroups(""); err == nil {
		t.Error("ListGroups: expected error")
	}
	if _, err := manager.CreateGroup("Solo", nil); err == nil {
		t.Error("CreateGroup: expected error")
	}
	if err := manager.SetGroupAnchor("workspace_group:16", "workspace:74"); err == nil {
		t.Error("SetGroupAnchor: expected error")
	}
	if err := manager.PinGroup("workspace_group:16"); err == nil {
		t.Error("PinGroup: expected error")
	}
}

// newWorkspaceCall returns the recorded new-workspace invocation, skipping the
// list-workspaces snapshots createWorkspace takes around it.
func newWorkspaceCall(t *testing.T, shim groupShim) []string {
	t.Helper()

	for _, arguments := range shim.invocations(t) {
		if len(arguments) > 0 && arguments[0] == "new-workspace" {
			return arguments
		}
	}
	t.Fatal("new-workspace was never called")
	return nil
}

func TestNewWorkspace_InGroupAppendsAtEnd(t *testing.T) {
	shim := writeGroupShim(t, false)

	ref, err := shim.client().NewWorkspace(NewWorkspaceOpts{CWD: "/tmp", Group: "workspace_group:16"})
	if err != nil {
		t.Fatalf("NewWorkspace: %v", err)
	}
	if ref != "workspace:2" {
		t.Errorf("ref = %q, want workspace:2", ref)
	}

	// Placement "end" keeps the saved member order: cmux's default
	// (afterCurrent) would insert each member next to the focused one.
	want := []string{"new-workspace", "--cwd", "/tmp", "--group", "workspace_group:16", "--group-placement", "end"}
	if got := newWorkspaceCall(t, shim); !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %q, want %q", got, want)
	}
}

func TestNewWorkspaceLayout_InGroupAppendsAtEnd(t *testing.T) {
	shim := writeGroupShim(t, false)
	layoutJSON := `{"pane":{"surfaces":[{"type":"terminal"}]}}`

	if _, err := shim.client().NewWorkspaceLayout(NewWorkspaceOpts{CWD: "/tmp", Group: "workspace_group:16"}, layoutJSON); err != nil {
		t.Fatalf("NewWorkspaceLayout: %v", err)
	}

	want := []string{"new-workspace", "--cwd", "/tmp", "--layout", layoutJSON, "--group", "workspace_group:16", "--group-placement", "end"}
	if got := newWorkspaceCall(t, shim); !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %q, want %q", got, want)
	}
}

func TestNewWorkspace_WithoutGroupHasNoGroupFlags(t *testing.T) {
	shim := writeGroupShim(t, false)

	if _, err := shim.client().NewWorkspace(NewWorkspaceOpts{CWD: "/tmp"}); err != nil {
		t.Fatalf("NewWorkspace: %v", err)
	}

	want := []string{"new-workspace", "--cwd", "/tmp"}
	if got := newWorkspaceCall(t, shim); !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %q, want %q", got, want)
	}
}

// Restore finds group support by type assertion, so the answer has to be yes
// for cmux and no for Ghostty, which has no sidebar groups at all.
func TestGroupManager_OnlyCmux(t *testing.T) {
	var cmuxBackend Backend = NewCLIClient()
	if _, ok := cmuxBackend.(GroupManager); !ok {
		t.Error("CLIClient must implement GroupManager")
	}

	var ghosttyBackend Backend = NewGhosttyClient()
	if _, ok := ghosttyBackend.(GroupManager); ok {
		t.Error("GhosttyClient must not implement GroupManager")
	}
}
