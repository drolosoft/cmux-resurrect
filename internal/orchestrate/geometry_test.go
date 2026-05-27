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

// assertSplit asserts that the split for paneIndex has the expected direction and focus target.
func assertSplit(t *testing.T, splits []PaneSplitInfo, paneIndex int, wantDir string, wantFocus int) {
	t.Helper()
	for _, s := range splits {
		if s.PaneIndex == paneIndex {
			if s.Direction != wantDir {
				t.Errorf("pane %d: direction = %q, want %q", paneIndex, s.Direction, wantDir)
			}
			if s.FocusTarget != wantFocus {
				t.Errorf("pane %d: focus_target = %d, want %d", paneIndex, s.FocusTarget, wantFocus)
			}
			return
		}
	}
	t.Errorf("pane %d: not found in splits", paneIndex)
}

// assertRatioApprox asserts that the ratio for paneIndex is within ±0.05 of wantRatio.
func assertRatioApprox(t *testing.T, splits []PaneSplitInfo, paneIndex int, wantRatio float64) {
	t.Helper()
	for _, s := range splits {
		if s.PaneIndex == paneIndex {
			if math.Abs(s.Ratio-wantRatio) > 0.05 {
				t.Errorf("pane %d: ratio = %.4f, want ≈%.4f (±0.05)", paneIndex, s.Ratio, wantRatio)
			}
			return
		}
	}
	t.Errorf("pane %d: not found in splits", paneIndex)
}

func TestInferSplitDirections_SinglePane(t *testing.T) {
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 1000, 800)),
	}
	splits := InferSplitDirections(panes)
	if len(splits) != 0 {
		t.Errorf("expected empty splits for single pane, got %d", len(splits))
	}
}

func TestInferSplitDirections_Cols(t *testing.T) {
	// Two columns: P0 left, P1 right
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 500, 800)),
		plp(1, pf(500, 0, 500, 800)),
	}
	splits := InferSplitDirections(panes)
	if len(splits) != 1 {
		t.Fatalf("expected 1 split, got %d", len(splits))
	}
	assertSplit(t, splits, 1, "right", -1)
}

func TestInferSplitDirections_Rows(t *testing.T) {
	// Two rows: P0 top, P1 bottom
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 1000, 400)),
		plp(1, pf(0, 400, 1000, 400)),
	}
	splits := InferSplitDirections(panes)
	if len(splits) != 1 {
		t.Fatalf("expected 1 split, got %d", len(splits))
	}
	assertSplit(t, splits, 1, "down", -1)
}

func TestInferSplitDirections_Triple(t *testing.T) {
	// Three equal columns
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 333, 800)),
		plp(1, pf(333, 0, 333, 800)),
		plp(2, pf(666, 0, 334, 800)),
	}
	splits := InferSplitDirections(panes)
	if len(splits) != 2 {
		t.Fatalf("expected 2 splits, got %d", len(splits))
	}
	assertSplit(t, splits, 1, "right", -1)
	assertSplit(t, splits, 2, "right", -1)
}

func TestInferSplitDirections_Aside(t *testing.T) {
	// P0 full-height left, P1 top-right, P2 bottom-right
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 500, 800)),
		plp(1, pf(500, 0, 500, 400)),
		plp(2, pf(500, 400, 500, 400)),
	}
	splits := InferSplitDirections(panes)
	if len(splits) != 2 {
		t.Fatalf("expected 2 splits, got %d", len(splits))
	}
	assertSplit(t, splits, 1, "right", -1)
	assertSplit(t, splits, 2, "down", -1)
}

func TestInferSplitDirections_Shelf(t *testing.T) {
	// P0 full-width top, P1 bottom-left, P2 bottom-right
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 1000, 400)),
		plp(1, pf(0, 400, 500, 400)),
		plp(2, pf(500, 400, 500, 400)),
	}
	splits := InferSplitDirections(panes)
	if len(splits) != 2 {
		t.Fatalf("expected 2 splits, got %d", len(splits))
	}
	assertSplit(t, splits, 1, "down", -1)
	assertSplit(t, splits, 2, "right", -1)
}

func TestInferSplitDirections_Quad(t *testing.T) {
	// 2x2 grid:
	// P0 top-left, P1 top-right, P2 bottom-left, P3 bottom-right
	// P2 must focus P0 (not P1), P3 must focus P1 (not P2)
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 500, 400)),
		plp(1, pf(500, 0, 500, 400)),
		plp(2, pf(0, 400, 500, 400)),
		plp(3, pf(500, 400, 500, 400)),
	}
	splits := InferSplitDirections(panes)
	if len(splits) != 3 {
		t.Fatalf("expected 3 splits, got %d", len(splits))
	}
	assertSplit(t, splits, 1, "right", -1)
	assertSplit(t, splits, 2, "down", 0)
	assertSplit(t, splits, 3, "down", 1)
}

func TestInferSplitDirections_Dashboard(t *testing.T) {
	// P0 full-width top, P1-P3 three equal columns on the bottom
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 1000, 400)),
		plp(1, pf(0, 400, 333, 400)),
		plp(2, pf(333, 400, 333, 400)),
		plp(3, pf(666, 400, 334, 400)),
	}
	splits := InferSplitDirections(panes)
	if len(splits) != 3 {
		t.Fatalf("expected 3 splits, got %d", len(splits))
	}
	assertSplit(t, splits, 1, "down", -1)
	assertSplit(t, splits, 2, "right", -1)
	assertSplit(t, splits, 3, "right", -1)
}

func TestInferSplitDirections_IDE(t *testing.T) {
	// P0 left sidebar, P1 top-right editor, P2 bottom-mid terminal, P3 bottom-right terminal
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 300, 800)),
		plp(1, pf(300, 0, 700, 400)),
		plp(2, pf(300, 400, 350, 400)),
		plp(3, pf(650, 400, 350, 400)),
	}
	splits := InferSplitDirections(panes)
	if len(splits) != 3 {
		t.Fatalf("expected 3 splits, got %d", len(splits))
	}
	assertSplit(t, splits, 1, "right", -1)
	assertSplit(t, splits, 2, "down", -1)
	assertSplit(t, splits, 3, "right", -1)
}

func TestInferSplitDirections_Ratio(t *testing.T) {
	// Aside with 70/30 split
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 700, 800)),
		plp(1, pf(700, 0, 300, 400)),
		plp(2, pf(700, 400, 300, 400)),
	}
	splits := InferSplitDirections(panes)
	if len(splits) != 2 {
		t.Fatalf("expected 2 splits, got %d", len(splits))
	}
	assertSplit(t, splits, 1, "right", -1)
	assertRatioApprox(t, splits, 1, 0.30)
	assertSplit(t, splits, 2, "down", -1)
	assertRatioApprox(t, splits, 2, 0.50)
}
