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
