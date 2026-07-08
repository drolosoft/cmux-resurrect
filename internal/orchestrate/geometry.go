package orchestrate

import (
	"math"
	"sort"

	"github.com/drolosoft/cmux-resurrect/internal/client"
)

// PaneCreation describes one step of recreating a workspace's pane layout.
// Steps are in creation order: the first entry is the workspace's initial
// pane (no split); each later entry is created by splitting an existing pane
// — the currently focused one when FocusTarget is -1, otherwise the pane at
// live index FocusTarget.
type PaneCreation struct {
	PaneIndex   int     // cmux pane index (visual order) of the pane this step creates
	Direction   string  // "" for the initial pane; else "right" or "down"
	Ratio       float64 // fraction of the split region the new pane occupies (0.0–1.0)
	FocusTarget int     // live pane index to focus before splitting; -1 if none needed
}

// InferCreationOrder analyzes pane pixel geometry and returns the sequence of
// splits that recreates the layout. Returns nil for 0 or 1 pane, or when the
// geometry can't be reconstructed (zero/overlapping frames).
//
// cmux indexes panes by visual position (left-to-right, then top-to-bottom),
// not creation order — e.g. a full-height right pane has the highest index in
// a "left column split in two + right pane" layout, yet it must be created
// FIRST (splitting the left column before the right pane exists changes what
// region each split divides). So the saved pane order must be a valid
// creation order derived from the split tree, not the index order (GitHub #8
// follow-up: restored layouts came back with panes in the wrong positions).
func InferCreationOrder(panes []client.PaneListPane) []PaneCreation {
	if len(panes) <= 1 {
		return nil
	}

	rects := make([]paneRect, len(panes))
	for i, p := range panes {
		rects[i] = paneRect{
			index: p.Index,
			x:     p.PixelFrame.X,
			y:     p.PixelFrame.Y,
			w:     p.PixelFrame.Width,
			h:     p.PixelFrame.Height,
		}
	}

	tree := buildBSP(rects)
	if countLeaves(tree) != len(rects) {
		return nil // BSP reconstruction failed (degenerate frames)
	}

	st := &creationState{
		origin: make(map[int]paneRect, len(rects)),
		live:   make(map[int]paneRect, len(rects)),
	}
	for _, r := range rects {
		st.origin[r.index] = r
	}

	root := st.repOf(tree)
	st.live[root] = boundingBox(rects)
	st.steps = append(st.steps, PaneCreation{PaneIndex: root, FocusTarget: -1})
	st.flatten(tree)
	return st.steps
}

// creationState tracks the simulated workspace while flattening the BSP tree
// into a creation sequence.
type creationState struct {
	origin map[int]paneRect // final rect per pane index (for representative selection)
	live   map[int]paneRect // region each already-created pane currently occupies
	steps  []PaneCreation
}

// flatten walks the BSP tree pre-order. At each internal node the right/bottom
// child's representative pane is created by splitting the left/top child's
// representative — which currently occupies the node's whole region.
func (st *creationState) flatten(node *bspNode) {
	if node.paneIndex >= 0 {
		return
	}
	leftRep := st.repOf(node.left)
	rightRep := st.repOf(node.right)
	region := st.live[leftRep]

	dir := "right"
	leftR, rightR := region, region
	if node.axis == "vertical" {
		leftR.w = region.w * (1 - node.ratio)
		rightR.x = region.x + leftR.w
		rightR.w = region.w * node.ratio
	} else {
		dir = "down"
		leftR.h = region.h * (1 - node.ratio)
		rightR.y = region.y + leftR.h
		rightR.h = region.h * node.ratio
	}

	// Always emit an explicit focus target — the live index of the pane being
	// split — computed BEFORE this split adds rightRep. cmux keeps focus on
	// the pane it split (not the new one) and `new-split` with no --surface
	// splits whatever is focused, so restore must refocus leftRep before every
	// split. Relying on implicit focus mirrored aside layouts (GitHub #8).
	focusTarget := st.liveIndexOf(leftRep)

	st.live[leftRep] = leftR
	st.live[rightRep] = rightR
	st.steps = append(st.steps, PaneCreation{
		PaneIndex:   rightRep,
		Direction:   dir,
		Ratio:       node.ratio,
		FocusTarget: focusTarget,
	})

	st.flatten(node.left)
	st.flatten(node.right)
}

// repOf returns the subtree's representative pane: the leaf occupying the
// region's top-left corner in the final layout. Splits keep the original
// pane on the left/top side, so this pane holds the whole region until the
// subtree is subdivided.
func (st *creationState) repOf(n *bspNode) int {
	if n.paneIndex >= 0 {
		return n.paneIndex
	}
	l, r := st.repOf(n.left), st.repOf(n.right)
	lr, rr := st.origin[l], st.origin[r]
	if lr.x < rr.x || (lr.x == rr.x && lr.y <= rr.y) {
		return l
	}
	return r
}

// liveIndexOf returns a pane's index in cmux's live visual ordering
// (left-to-right, then top-to-bottom) among the panes created so far.
func (st *creationState) liveIndexOf(idx int) int {
	target := st.live[idx]
	rank := 0
	for other, r := range st.live {
		if other == idx {
			continue
		}
		if r.x < target.x || (r.x == target.x && r.y < target.y) {
			rank++
		}
	}
	return rank
}

// countLeaves returns the number of leaf panes in the BSP tree.
func countLeaves(n *bspNode) int {
	if n.paneIndex >= 0 {
		return 1
	}
	return countLeaves(n.left) + countLeaves(n.right)
}

// boundingBox returns the rect enclosing all pane rects.
func boundingBox(rects []paneRect) paneRect {
	bb := rects[0]
	maxX, maxY := bb.x+bb.w, bb.y+bb.h
	for _, r := range rects[1:] {
		if r.x < bb.x {
			bb.x = r.x
		}
		if r.y < bb.y {
			bb.y = r.y
		}
		if r.x+r.w > maxX {
			maxX = r.x + r.w
		}
		if r.y+r.h > maxY {
			maxY = r.y + r.h
		}
	}
	bb.w = maxX - bb.x
	bb.h = maxY - bb.y
	return bb
}

// paneRect is an internal representation of a pane's geometry.
type paneRect struct {
	index int
	x, y  float64
	w, h  float64
}

// bspNode is a node in a binary space partition tree.
// Leaf nodes have paneIndex >= 0 and nil children.
// Internal nodes have axis set and two children.
type bspNode struct {
	// Leaf fields.
	paneIndex int // -1 for internal nodes

	// Internal fields.
	axis  string   // "vertical" or "horizontal"
	ratio float64  // fraction of total extent that the right/bottom side occupies
	left  *bspNode // left or top subtree
	right *bspNode // right or bottom subtree
}

const edgeTolerance = 2.0

// buildBSP recursively builds a BSP tree from a set of pane rects.
func buildBSP(rects []paneRect) *bspNode {
	if len(rects) == 1 {
		return &bspNode{paneIndex: rects[0].index}
	}

	// Compute bounding box.
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, r := range rects {
		if r.x < minX {
			minX = r.x
		}
		if r.y < minY {
			minY = r.y
		}
		if r.x+r.w > maxX {
			maxX = r.x + r.w
		}
		if r.y+r.h > maxY {
			maxY = r.y + r.h
		}
	}
	totalW := maxX - minX
	totalH := maxY - minY

	// Try vertical split: collect candidate x-edges.
	if split, left, right, ok := tryVerticalSplit(rects, minX, totalW); ok {
		ratio := (maxX - split) / totalW
		return &bspNode{
			paneIndex: -1,
			axis:      "vertical",
			ratio:     ratio,
			left:      buildBSP(left),
			right:     buildBSP(right),
		}
	}

	// Try horizontal split.
	if split, top, bottom, ok := tryHorizontalSplit(rects, minY, totalH); ok {
		ratio := (maxY - split) / totalH
		return &bspNode{
			paneIndex: -1,
			axis:      "horizontal",
			ratio:     ratio,
			left:      buildBSP(top),
			right:     buildBSP(bottom),
		}
	}

	// Fallback: should not happen with valid layouts. Return first as leaf.
	return &bspNode{paneIndex: rects[0].index}
}

// tryVerticalSplit finds a vertical split line that cleanly divides rects into left/right.
func tryVerticalSplit(rects []paneRect, minX, totalW float64) (float64, []paneRect, []paneRect, bool) {
	// Collect candidate edges: right edges of rects (x+w).
	candidates := collectEdges(rects, func(r paneRect) float64 { return r.x + r.w })

	for _, edge := range candidates {
		// Skip edges at the boundary.
		if math.Abs(edge-minX) < edgeTolerance || math.Abs(edge-(minX+totalW)) < edgeTolerance {
			continue
		}

		var left, right []paneRect
		valid := true
		for _, r := range rects {
			rRight := r.x + r.w
			switch {
			case rRight <= edge+edgeTolerance && r.x >= minX-edgeTolerance:
				// Rect is entirely to the left of the split.
				left = append(left, r)
			case r.x >= edge-edgeTolerance:
				// Rect is entirely to the right of the split.
				right = append(right, r)
			default:
				valid = false
			}
			if !valid {
				break
			}
		}
		if valid && len(left) > 0 && len(right) > 0 {
			return edge, left, right, true
		}
	}
	return 0, nil, nil, false
}

// tryHorizontalSplit finds a horizontal split line that cleanly divides rects into top/bottom.
func tryHorizontalSplit(rects []paneRect, minY, totalH float64) (float64, []paneRect, []paneRect, bool) {
	candidates := collectEdges(rects, func(r paneRect) float64 { return r.y + r.h })

	for _, edge := range candidates {
		if math.Abs(edge-minY) < edgeTolerance || math.Abs(edge-(minY+totalH)) < edgeTolerance {
			continue
		}

		var top, bottom []paneRect
		valid := true
		for _, r := range rects {
			rBottom := r.y + r.h
			switch {
			case rBottom <= edge+edgeTolerance && r.y >= minY-edgeTolerance:
				top = append(top, r)
			case r.y >= edge-edgeTolerance:
				bottom = append(bottom, r)
			default:
				valid = false
			}
			if !valid {
				break
			}
		}
		if valid && len(top) > 0 && len(bottom) > 0 {
			return edge, top, bottom, true
		}
	}
	return 0, nil, nil, false
}

// collectEdges returns unique sorted edge values from rects using the given extractor.
func collectEdges(rects []paneRect, extract func(paneRect) float64) []float64 {
	seen := make(map[float64]bool)
	var edges []float64
	for _, r := range rects {
		e := extract(r)
		if !seen[e] {
			seen[e] = true
			edges = append(edges, e)
		}
	}
	sort.Float64s(edges)
	return edges
}
