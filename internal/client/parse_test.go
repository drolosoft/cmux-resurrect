package client

import (
	"os"
	"testing"
)

func TestParseSidebarState(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/responses/sidebar-state-ioc.txt")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	state, err := ParseSidebarState(string(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if state.CWD != "/home/user/projects/ioc-events" {
		t.Errorf("CWD = %q, want /home/user/projects/ioc-events", state.CWD)
	}
	if state.FocusedCWD != "/home/user/projects/ioc-events" {
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
	raw := `  workspace:1  0 ioc-events
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
