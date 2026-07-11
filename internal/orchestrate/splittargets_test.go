package orchestrate

import (
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/model"
)

func TestResolveSplitTargets_Aside(t *testing.T) {
	// p0, p1 right of p0, p2 down from p1 (focus_target 1 = visual index of
	// p1 at that moment).
	ws := model.Workspace{
		Panes: []model.Pane{
			{Type: "terminal", FocusTarget: -1},
			{Type: "terminal", Split: "right", FocusTarget: 0},
			{Type: "terminal", Split: "down", FocusTarget: 1},
		},
	}
	targets, visual, ok := resolveSplitTargets(ws)
	if !ok {
		t.Fatal("expected resolvable targets")
	}
	if targets[0] != -1 || targets[1] != 0 || targets[2] != 1 {
		t.Errorf("targets = %v, want [-1 0 1]", targets)
	}
	if visual[0] != 0 || visual[1] != 1 || visual[2] != 2 {
		t.Errorf("visual = %v, want [0 1 2]", visual)
	}
}

func TestResolveSplitTargets_QuadLikeUserTabTest(t *testing.T) {
	// The user's quad: p0 repo, p1 right (home), p2 down from VISUAL 0
	// (focus_target absent → 0 → repo), p3 down from VISUAL 2. After p2,
	// visual order is [p0(0,0), p2(0,.5), p1(.5,0)] — visual 2 is p1, so p3
	// must split p1 (creation index 1), NOT p2.
	ws := model.Workspace{
		Panes: []model.Pane{
			{Type: "terminal", FocusTarget: -1},
			{Type: "terminal", Split: "right", FocusTarget: 0},
			{Type: "terminal", Split: "down", FocusTarget: 0},
			{Type: "terminal", Split: "down", FocusTarget: 2},
		},
	}
	targets, visual, ok := resolveSplitTargets(ws)
	if !ok {
		t.Fatal("expected resolvable targets")
	}
	want := []int{-1, 0, 0, 1}
	for i := range want {
		if targets[i] != want[i] {
			t.Fatalf("targets = %v, want %v", targets, want)
		}
	}
	// Final visual: p0 TL(0), p2 BL(1), p1 TR(2), p3 BR(3).
	wantVis := []int{0, 2, 1, 3}
	for i := range wantVis {
		if visual[i] != wantVis[i] {
			t.Fatalf("visual = %v, want %v", visual, wantVis)
		}
	}
}

func TestResolveSplitTargets_DanglingFocusTarget(t *testing.T) {
	ws := model.Workspace{
		Panes: []model.Pane{
			{Type: "terminal", FocusTarget: -1},
			{Type: "terminal", Split: "right", FocusTarget: 7},
		},
	}
	if _, _, ok := resolveSplitTargets(ws); ok {
		t.Fatal("dangling focus target must not resolve")
	}
}

func TestResolveSplitTargets_ImplicitPreviousPane(t *testing.T) {
	// FocusTarget -1 on a split means "the previously created pane".
	ws := model.Workspace{
		Panes: []model.Pane{
			{Type: "terminal", FocusTarget: -1},
			{Type: "terminal", Split: "right", FocusTarget: -1},
			{Type: "terminal", Split: "down", FocusTarget: -1},
		},
	}
	targets, _, ok := resolveSplitTargets(ws)
	if !ok {
		t.Fatal("expected resolvable targets")
	}
	if targets[1] != 0 || targets[2] != 1 {
		t.Errorf("targets = %v, want [-1 0 1]", targets)
	}
}
