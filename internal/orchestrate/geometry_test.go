package orchestrate

import (
	"math"
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/client"
)

// pf is a shorthand constructor for client.PixelFrame.
func pf(x, y, w, h float64) client.PixelFrame {
	return client.PixelFrame{X: x, Y: y, Width: w, Height: h}
}

// plp is a shorthand constructor for client.PaneListPane.
func plp(index int, frame client.PixelFrame) client.PaneListPane {
	return client.PaneListPane{Index: index, PixelFrame: frame}
}

// assertOrder asserts the creation sequence visits panes in the given order.
func assertOrder(t *testing.T, steps []PaneCreation, want ...int) {
	t.Helper()
	if len(steps) != len(want) {
		t.Fatalf("creation steps = %d, want %d", len(steps), len(want))
	}
	for i, w := range want {
		if steps[i].PaneIndex != w {
			t.Errorf("step %d: pane = %d, want %d (full order: %+v)", i, steps[i].PaneIndex, w, steps)
		}
	}
}

// assertStep asserts direction and focus target of creation step i.
func assertStep(t *testing.T, steps []PaneCreation, i int, wantDir string, wantFocus int) {
	t.Helper()
	if i >= len(steps) {
		t.Fatalf("step %d out of range (%d steps)", i, len(steps))
	}
	s := steps[i]
	if s.Direction != wantDir {
		t.Errorf("step %d (pane %d): direction = %q, want %q", i, s.PaneIndex, s.Direction, wantDir)
	}
	if s.FocusTarget != wantFocus {
		t.Errorf("step %d (pane %d): focus_target = %d, want %d", i, s.PaneIndex, s.FocusTarget, wantFocus)
	}
}

// assertRatioApprox asserts that step i's ratio is within ±0.05 of wantRatio.
func assertRatioApprox(t *testing.T, steps []PaneCreation, i int, wantRatio float64) {
	t.Helper()
	if math.Abs(steps[i].Ratio-wantRatio) > 0.05 {
		t.Errorf("step %d (pane %d): ratio = %.4f, want ≈%.4f (±0.05)", i, steps[i].PaneIndex, steps[i].Ratio, wantRatio)
	}
}

func TestInferCreationOrder_SinglePane(t *testing.T) {
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 1000, 800)),
	}
	if steps := InferCreationOrder(panes); steps != nil {
		t.Errorf("expected nil for single pane, got %+v", steps)
	}
}

func TestInferCreationOrder_ZeroFrames(t *testing.T) {
	// Never-rendered workspaces report zero pixel frames — inference must
	// bail (caller keeps the default right-chain) instead of guessing.
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 0, 0)),
		plp(1, pf(0, 0, 0, 0)),
	}
	if steps := InferCreationOrder(panes); steps != nil {
		t.Errorf("expected nil for zero frames, got %+v", steps)
	}
}

func TestInferCreationOrder_Cols(t *testing.T) {
	// Two columns: P0 left, P1 right
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 500, 800)),
		plp(1, pf(500, 0, 500, 800)),
	}
	steps := InferCreationOrder(panes)
	assertOrder(t, steps, 0, 1)
	assertStep(t, steps, 1, "right", -1)
}

func TestInferCreationOrder_Rows(t *testing.T) {
	// Two rows: P0 top, P1 bottom
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 1000, 400)),
		plp(1, pf(0, 400, 1000, 400)),
	}
	steps := InferCreationOrder(panes)
	assertOrder(t, steps, 0, 1)
	assertStep(t, steps, 1, "down", -1)
}

func TestInferCreationOrder_Triple(t *testing.T) {
	// Three equal columns
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 333, 800)),
		plp(1, pf(333, 0, 333, 800)),
		plp(2, pf(666, 0, 334, 800)),
	}
	steps := InferCreationOrder(panes)
	assertOrder(t, steps, 0, 1, 2)
	assertStep(t, steps, 1, "right", -1)
	assertStep(t, steps, 2, "right", -1)
}

func TestInferCreationOrder_Aside(t *testing.T) {
	// P0 full-height left, P1 top-right, P2 bottom-right
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 500, 800)),
		plp(1, pf(500, 0, 500, 400)),
		plp(2, pf(500, 400, 500, 400)),
	}
	steps := InferCreationOrder(panes)
	assertOrder(t, steps, 0, 1, 2)
	assertStep(t, steps, 1, "right", -1)
	assertStep(t, steps, 2, "down", -1)
}

func TestInferCreationOrder_MirroredAside(t *testing.T) {
	// P0 top-left, P1 bottom-left, P2 full-height RIGHT — cmux's visual
	// indexing puts the full-height pane last, but it must be created
	// FIRST: splitting the left column while P0 still spans the full width
	// yields a full-width bottom strip instead (GitHub #8 follow-up, the
	// "restored layout not respected" report).
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 500, 400)),
		plp(1, pf(0, 400, 500, 400)),
		plp(2, pf(500, 0, 500, 800)),
	}
	steps := InferCreationOrder(panes)
	assertOrder(t, steps, 0, 2, 1)
	assertStep(t, steps, 1, "right", -1) // P2: split P0 right while it spans full height
	assertStep(t, steps, 2, "down", 0)   // P1: refocus P0 (live index 0), split down
}

func TestInferCreationOrder_Shelf(t *testing.T) {
	// P0 full-width top, P1 bottom-left, P2 bottom-right
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 1000, 400)),
		plp(1, pf(0, 400, 500, 400)),
		plp(2, pf(500, 400, 500, 400)),
	}
	steps := InferCreationOrder(panes)
	assertOrder(t, steps, 0, 1, 2)
	assertStep(t, steps, 1, "down", -1)
	assertStep(t, steps, 2, "right", -1)
}

func TestInferCreationOrder_Quad(t *testing.T) {
	// 2x2 grid with cmux's visual (column-major) indexing:
	// P0 top-left, P1 bottom-left, P2 top-right, P3 bottom-right.
	// Build columns first, then split each column; focus targets are LIVE
	// pane indexes at each step, not final ones.
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 500, 400)),
		plp(1, pf(0, 400, 500, 400)),
		plp(2, pf(500, 0, 500, 400)),
		plp(3, pf(500, 400, 500, 400)),
	}
	steps := InferCreationOrder(panes)
	assertOrder(t, steps, 0, 2, 1, 3)
	assertStep(t, steps, 1, "right", -1) // P2: split P0 right → two columns
	assertStep(t, steps, 2, "down", 0)   // P1: refocus P0 (live 0), split down
	assertStep(t, steps, 3, "down", 2)   // P3: refocus P2 (live 2: after P0, P1), split down
}

func TestInferCreationOrder_Dashboard(t *testing.T) {
	// P0 full-width top, P1-P3 three equal columns on the bottom
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 1000, 400)),
		plp(1, pf(0, 400, 333, 400)),
		plp(2, pf(333, 400, 333, 400)),
		plp(3, pf(666, 400, 334, 400)),
	}
	steps := InferCreationOrder(panes)
	assertOrder(t, steps, 0, 1, 2, 3)
	assertStep(t, steps, 1, "down", -1)
	assertStep(t, steps, 2, "right", -1)
	assertStep(t, steps, 3, "right", -1)
}

func TestInferCreationOrder_IDE(t *testing.T) {
	// P0 left sidebar, P1 top-right editor, P2 bottom-mid terminal, P3 bottom-right terminal
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 300, 800)),
		plp(1, pf(300, 0, 700, 400)),
		plp(2, pf(300, 400, 350, 400)),
		plp(3, pf(650, 400, 350, 400)),
	}
	steps := InferCreationOrder(panes)
	assertOrder(t, steps, 0, 1, 2, 3)
	assertStep(t, steps, 1, "right", -1)
	assertStep(t, steps, 2, "down", -1)
	assertStep(t, steps, 3, "right", -1)
}

func TestInferCreationOrder_Ratio(t *testing.T) {
	// Aside with 70/30 split
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 700, 800)),
		plp(1, pf(700, 0, 300, 400)),
		plp(2, pf(700, 400, 300, 400)),
	}
	steps := InferCreationOrder(panes)
	assertOrder(t, steps, 0, 1, 2)
	assertStep(t, steps, 1, "right", -1)
	assertRatioApprox(t, steps, 1, 0.30)
	assertStep(t, steps, 2, "down", -1)
	assertRatioApprox(t, steps, 2, 0.50)
}
