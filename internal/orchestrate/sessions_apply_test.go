package orchestrate

import (
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/detect"
	"github.com/drolosoft/cmux-resurrect/internal/model"
)

func TestApplyDetectedSessions_ReorderedPanesMatchByIndex(t *testing.T) {
	// applySplitGeometry reorders ws.Panes into creation order, so a pane's
	// array position no longer equals its cmux Index. The surface-title map
	// is keyed by Index — looking it up by array position attaches the AI
	// resume command to the WRONG pane (2026-07-11 audit, finding 2).
	//
	// Aside: visual index 2 (full-height right pane, running claude) sits at
	// creation position 1 after the reorder.
	layout := &model.Layout{
		Workspaces: []model.Workspace{{
			Title: "work",
			CWD:   "/home/u/proj",
			Panes: []model.Pane{
				{Type: "terminal", Index: 0},
				{Type: "terminal", Index: 2, Split: "right"}, // claude pane, creation pos 1
				{Type: "terminal", Index: 1, Split: "down"},
			},
		}},
	}
	tree := []client.TreeWorkspace{{
		Title: "work",
		Panes: []client.TreePane{
			{Index: 0, Surfaces: []client.TreeSurface{{Title: "zsh"}}},
			{Index: 1, Surfaces: []client.TreeSurface{{Title: "zsh"}}},
			{Index: 2, Surfaces: []client.TreeSurface{{Title: "✳ claude — refactor"}}},
		},
	}}
	detected := detect.DetectedSessions{
		ByCWD: map[string][]detect.Session{
			"/home/u/proj": {{Tool: "claude", CWD: "/home/u/proj", Command: "claude --resume abc123"}},
		},
		ByTool: map[string][]detect.Session{
			"claude": {{Tool: "claude", CWD: "/home/u/proj", Command: "claude --resume abc123"}},
		},
	}

	applyDetectedSessions(layout, tree, detected)

	panes := layout.Workspaces[0].Panes
	if got := panes[1].Command; got != "claude --resume abc123" {
		t.Errorf("pane Index 2 (creation pos 1) command = %q, want the claude resume command", got)
	}
	for _, i := range []int{0, 2} {
		if panes[i].Command != "" {
			t.Errorf("pane at pos %d (Index %d) should have no command, got %q", i, panes[i].Index, panes[i].Command)
		}
	}
}

// TestApplyDetectedSessions_ExtraTabsGetTheirOwnSession reproduces the
// workflow from GitHub #8: one workspace, one pane, several TABS, each tab a
// different git worktree of the same project with its own Claude session.
// Detection used to be pane-only, so tabs 2..N came back as bare shells.
func TestApplyDetectedSessions_ExtraTabsGetTheirOwnSession(t *testing.T) {
	layout := &model.Layout{
		Workspaces: []model.Workspace{{
			Title: "proj",
			CWD:   "/home/u/proj/main",
			Panes: []model.Pane{{
				Type: "terminal", Index: 0, CWD: "/home/u/proj/main",
				Surfaces: []model.Surface{
					{Type: "terminal", CWD: "/home/u/proj/feature-a"},
					{Type: "terminal", CWD: "/home/u/proj/feature-b"},
				},
			}},
		}},
	}
	tree := []client.TreeWorkspace{{
		Title: "proj",
		Panes: []client.TreePane{{
			Index: 0,
			Surfaces: []client.TreeSurface{
				{Title: "✳ claude — main"},
				{Title: "✳ claude — feature A"},
				{Title: "✳ claude — feature B"},
			},
		}},
	}}
	mk := func(cwd, id string) detect.Session {
		return detect.Session{Tool: "claude", CWD: cwd, Command: "claude --resume " + id}
	}
	detected := detect.DetectedSessions{
		ByCWD: map[string][]detect.Session{
			"/home/u/proj/main":      {mk("/home/u/proj/main", "aaa")},
			"/home/u/proj/feature-a": {mk("/home/u/proj/feature-a", "bbb")},
			"/home/u/proj/feature-b": {mk("/home/u/proj/feature-b", "ccc")},
		},
		ByTool: map[string][]detect.Session{
			"claude": {
				mk("/home/u/proj/main", "aaa"),
				mk("/home/u/proj/feature-a", "bbb"),
				mk("/home/u/proj/feature-b", "ccc"),
			},
		},
	}

	applyDetectedSessions(layout, tree, detected)

	pane := layout.Workspaces[0].Panes[0]
	if pane.Command != "claude --resume aaa" {
		t.Errorf("pane command = %q, want claude --resume aaa", pane.Command)
	}
	if got := pane.Surfaces[0].Command; got != "claude --resume bbb" {
		t.Errorf("tab 1 command = %q, want claude --resume bbb (its own worktree session)", got)
	}
	if got := pane.Surfaces[1].Command; got != "claude --resume ccc" {
		t.Errorf("tab 2 command = %q, want claude --resume ccc (its own worktree session)", got)
	}
}

// TestApplyDetectedSessions_NoSessionStealingAcrossTabs guards the inverse
// risk: with only ONE running session, exactly one target may claim it.
func TestApplyDetectedSessions_NoSessionStealingAcrossTabs(t *testing.T) {
	layout := &model.Layout{
		Workspaces: []model.Workspace{{
			Title: "proj", CWD: "/home/u/proj",
			Panes: []model.Pane{{
				Type: "terminal", Index: 0, CWD: "/home/u/proj",
				Surfaces: []model.Surface{
					{Type: "terminal", CWD: "/home/u/other"},
					{Type: "terminal", CWD: "/home/u/third"},
				},
			}},
		}},
	}
	tree := []client.TreeWorkspace{{
		Title: "proj",
		Panes: []client.TreePane{{
			Index: 0,
			Surfaces: []client.TreeSurface{
				{Title: "✳ claude"}, {Title: "✳ claude"}, {Title: "✳ claude"},
			},
		}},
	}}
	one := detect.Session{Tool: "claude", CWD: "/home/u/proj", Command: "claude --resume only"}
	detected := detect.DetectedSessions{
		ByCWD:  map[string][]detect.Session{"/home/u/proj": {one}},
		ByTool: map[string][]detect.Session{"claude": {one}},
	}

	applyDetectedSessions(layout, tree, detected)

	pane := layout.Workspaces[0].Panes[0]
	assigned := 0
	for _, c := range []string{pane.Command, pane.Surfaces[0].Command, pane.Surfaces[1].Command} {
		if c == one.Command {
			assigned++
		} else if c != "" {
			t.Errorf("unexpected command %q", c)
		}
	}
	if assigned != 1 {
		t.Errorf("session assigned %d times, want exactly 1", assigned)
	}
}

func TestClearAutoDetectedCommands_ClearsExtraTabs(t *testing.T) {
	layout := &model.Layout{
		Workspaces: []model.Workspace{{
			Title: "proj",
			Panes: []model.Pane{{
				Type: "terminal", Command: "claude --resume stale-pane",
				Surfaces: []model.Surface{
					{Type: "terminal", Command: "claude --resume stale-tab"},
					{Type: "terminal", Command: "npm run dev"}, // user command: must survive
				},
			}},
		}},
	}

	clearAutoDetectedCommands(layout)

	pane := layout.Workspaces[0].Panes[0]
	if pane.Command != "" {
		t.Errorf("pane resume command not cleared: %q", pane.Command)
	}
	if pane.Surfaces[0].Command != "" {
		t.Errorf("tab resume command not cleared: %q — stale ids would survive a re-save", pane.Surfaces[0].Command)
	}
	if pane.Surfaces[1].Command != "npm run dev" {
		t.Errorf("user command on tab was wrongly cleared: %q", pane.Surfaces[1].Command)
	}
}
