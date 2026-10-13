package client

import (
	"os"
	"reflect"
	"testing"
)

func TestParseSidebarState(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/responses/sidebar-state-api.txt")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	state, err := ParseSidebarState(string(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if state.CWD != "/home/user/projects/api-server" {
		t.Errorf("CWD = %q, want /home/user/projects/api-server", state.CWD)
	}
	if state.FocusedCWD != "/home/user/projects/api-server" {
		t.Errorf("FocusedCWD = %q", state.FocusedCWD)
	}
	if state.GitBranch != "main" {
		t.Errorf("GitBranch = %q, want main", state.GitBranch)
	}
	if !state.GitDirty {
		t.Error("GitDirty = false, want true")
	}
}

func TestParseSidebarState_NoCWD(t *testing.T) {
	raw := "tab=abc\ncolor=none\n"
	_, err := ParseSidebarState(raw)
	if err == nil {
		t.Error("expected error for missing cwd")
	}
}

func TestParseSidebarState_CleanBranch(t *testing.T) {
	raw := "cwd=/tmp/test\ngit_branch=develop\n"
	state, err := ParseSidebarState(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if state.GitBranch != "develop" {
		t.Errorf("GitBranch = %q, want develop", state.GitBranch)
	}
	if state.GitDirty {
		t.Error("GitDirty should be false for clean branch")
	}
}

func TestParseListWorkspaces(t *testing.T) {
	raw := `  workspace:1  0 api-server
  workspace:2  1 Obsidian
  workspace:3  2 LaPorrA
* workspace:6  Claude Code  [selected]`

	ws, err := ParseListWorkspaces(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(ws) != 4 {
		t.Fatalf("got %d workspaces, want 4", len(ws))
	}

	// First workspace
	if ws[0].Ref != "workspace:1" {
		t.Errorf("ws[0].Ref = %q", ws[0].Ref)
	}
	if ws[0].Selected {
		t.Error("ws[0] should not be selected")
	}

	// Last workspace (selected)
	if ws[3].Ref != "workspace:6" {
		t.Errorf("ws[3].Ref = %q", ws[3].Ref)
	}
	if !ws[3].Selected {
		t.Error("ws[3] should be selected")
	}
	if ws[3].Title != "Claude Code" {
		t.Errorf("ws[3].Title = %q, want 'Claude Code'", ws[3].Title)
	}
}

func TestParseListWorkspaces_Empty(t *testing.T) {
	ws, err := ParseListWorkspaces("")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(ws) != 0 {
		t.Errorf("got %d workspaces for empty input", len(ws))
	}
}

// The list fixture is the real `cmux --json workspace-group list` output from
// cmux 0.64.22: one group with every state flag set and one with null color
// and icon, which must come back as empty strings rather than "null".
func TestParseWorkspaceGroups(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/responses/workspace-group-list.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	groups, err := ParseWorkspaceGroups(string(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("got %d groups, want 2", len(groups))
	}

	styled := groups[0]
	want := WorkspaceGroup{
		Ref:        "workspace_group:16",
		Name:       "🌋 Dos grupo",
		AnchorRef:  "workspace:74",
		MemberRefs: []string{"workspace:74", "workspace:76", "workspace:77"},
		Collapsed:  true,
		Pinned:     true,
		Color:      "#FF8800",
		Icon:       "flame",
	}
	if !reflect.DeepEqual(styled, want) {
		t.Errorf("groups[0] = %+v\nwant        %+v", styled, want)
	}

	plain := groups[1]
	if plain.Name != "🎮 Prueba uno" {
		t.Errorf("groups[1].Name = %q", plain.Name)
	}
	if plain.Color != "" || plain.Icon != "" {
		t.Errorf("null color/icon must parse as empty, got color=%q icon=%q", plain.Color, plain.Icon)
	}
	if plain.Collapsed || plain.Pinned {
		t.Errorf("groups[1] flags = collapsed %v pinned %v, want both false", plain.Collapsed, plain.Pinned)
	}

	// Members come in sidebar order with the anchor first, and the save side
	// relies on that order, so it must survive parsing untouched.
	wantMembers := []string{"workspace:72", "workspace:71", "workspace:70", "workspace:69", "workspace:73"}
	if !reflect.DeepEqual(plain.MemberRefs, wantMembers) {
		t.Errorf("groups[1].MemberRefs = %v, want %v", plain.MemberRefs, wantMembers)
	}
}

func TestParseWorkspaceGroups_NoGroups(t *testing.T) {
	groups, err := ParseWorkspaceGroups(`{"groups": [], "window_ref": "window:1"}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(groups) != 0 {
		t.Errorf("got %d groups, want 0", len(groups))
	}
}

func TestParseWorkspaceGroups_InvalidJSON(t *testing.T) {
	if _, err := ParseWorkspaceGroups("not json"); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// The create fixture shows the brand-new anchor cmux makes on every create
// (workspace:75), listed before the workspace that was passed in --from.
func TestParseWorkspaceGroup(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/responses/workspace-group-create.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	group, err := ParseWorkspaceGroup(string(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	want := WorkspaceGroup{
		Ref:        "workspace_group:16",
		Name:       "🌋 Dos grupo",
		AnchorRef:  "workspace:75",
		MemberRefs: []string{"workspace:75", "workspace:74"},
	}
	if !reflect.DeepEqual(group, want) {
		t.Errorf("group = %+v\nwant    %+v", group, want)
	}
}

func TestParseWorkspaceGroup_MissingGroup(t *testing.T) {
	if _, err := ParseWorkspaceGroup(`{"ok": true}`); err == nil {
		t.Error("expected error when the response has no group object")
	}
}
