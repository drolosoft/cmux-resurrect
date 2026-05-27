package orchestrate

import (
	"math"
	"sort"

	"github.com/drolosoft/cmux-resurrect/internal/client"
)

// PaneSplitInfo describes the inferred split direction, ratio, and focus target.
type PaneSplitInfo struct {
	PaneIndex   int
	Direction   string  // "right", "left", "down", "up"
	Ratio       float64 // fraction the new pane occupies (0.0–1.0)
	FocusTarget int     // pane index to focus before splitting, -1 if not needed
}

// InferSplitDirections analyzes pane pixel geometry and returns split info
// for each pane after the first. Returns nil for 0 or 1 pane.
func InferSplitDirections(panes []client.PaneListPane) []PaneSplitInfo {
	if len(panes) <= 1 {
		return nil
	}

	// Sort by index.
	sorted := make([]client.PaneListPane, len(panes))
	copy(sorted, panes)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Index < sorted[j].Index })

	// Convert to internal rects.
	rects := make([]paneRect, len(sorted))
	for i, p := range sorted {
		rects[i] = paneRect{
			index: p.Index,
			x:     p.PixelFrame.X,
			y:     p.PixelFrame.Y,
			w:     p.PixelFrame.Width,
			h:     p.PixelFrame.Height,
		}
	}

	// Build BSP tree.
	tree := buildBSP(rects)

	// Walk panes in index order and extract split info.
	var splits []PaneSplitInfo
	for i := 1; i < len(sorted); i++ {
		idx := sorted[i].Index
		prevIdx := sorted[i-1].Index
		info := extractSplitInfo(tree, idx, prevIdx)
		splits = append(splits, info)
	}
	return splits
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

// minIndex returns the smallest pane index in this subtree.
func (n *bspNode) minIndex() int {
	if n.paneIndex >= 0 {
		return n.paneIndex
	}
	l := n.left.minIndex()
	r := n.right.minIndex()
	if l < r {
		return l
	}
	return r
}

// containsIndex checks if a pane index exists in this subtree.
func (n *bspNode) containsIndex(idx int) bool {
	if n.paneIndex >= 0 {
		return n.paneIndex == idx
	}
	return n.left.containsIndex(idx) || n.right.containsIndex(idx)
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
			if rRight <= edge+edgeTolerance && r.x >= minX-edgeTolerance {
				// Rect is entirely to the left of the split.
				left = append(left, r)
			} else if r.x >= edge-edgeTolerance {
				// Rect is entirely to the right of the split.
				right = append(right, r)
			} else {
				valid = false
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
			if rBottom <= edge+edgeTolerance && r.y >= minY-edgeTolerance {
				top = append(top, r)
			} else if r.y >= edge-edgeTolerance {
				bottom = append(bottom, r)
			} else {
				valid = false
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

// pathToLeaf returns the path of (node, side) pairs from root to the leaf
// containing paneIdx. Side is "left" or "right" indicating which child was taken.
func pathToLeaf(node *bspNode, paneIdx int) []pathEntry {
	if node.paneIndex >= 0 {
		if node.paneIndex == paneIdx {
			return []pathEntry{}
		}
		return nil
	}
	if node.left.containsIndex(paneIdx) {
		sub := pathToLeaf(node.left, paneIdx)
		if sub != nil {
			return append([]pathEntry{{node: node, side: "left"}}, sub...)
		}
	}
	if node.right.containsIndex(paneIdx) {
		sub := pathToLeaf(node.right, paneIdx)
		if sub != nil {
			return append([]pathEntry{{node: node, side: "right"}}, sub...)
		}
	}
	return nil
}

type pathEntry struct {
	node *bspNode
	side string // "left" or "right" — which child contains the target pane
}

// extractSplitInfo derives PaneSplitInfo for a pane from the BSP tree.
//
// For pane P(i), walk up the BSP path from the leaf. At each ancestor,
// look at the sibling subtree. The sibling's min-index pane that is < P(i)
// is the focus target — the pane that was split to create P(i).
// The ancestor node's axis + which side P(i) is on gives the direction.
func extractSplitInfo(root *bspNode, paneIdx, prevIdx int) PaneSplitInfo {
	path := pathToLeaf(root, paneIdx)
	if len(path) == 0 {
		return PaneSplitInfo{PaneIndex: paneIdx, Direction: "right", Ratio: 0.5, FocusTarget: -1}
	}

	// Walk the path from leaf (deepest) to root (shallowest).
	// Find the deepest ancestor whose sibling subtree contains a pane with index < paneIdx.
	for i := len(path) - 1; i >= 0; i-- {
		entry := path[i]
		var sibling *bspNode
		if entry.side == "left" {
			sibling = entry.node.right
		} else {
			sibling = entry.node.left
		}

		sibMin := sibling.minIndex()
		if sibMin < paneIdx {
			// This ancestor's sibling contains a pane created before paneIdx.
			// The focus target is the min-index pane in the sibling subtree.
			dir := inferDirection(entry.node.axis, entry.side)

			ratio := entry.node.ratio
			if entry.side == "left" {
				ratio = 1.0 - entry.node.ratio
			}

			focusTarget := -1
			if sibMin != prevIdx {
				focusTarget = sibMin
			}

			return PaneSplitInfo{
				PaneIndex:   paneIdx,
				Direction:   dir,
				Ratio:       ratio,
				FocusTarget: focusTarget,
			}
		}
	}

	// Fallback: use the shallowest ancestor.
	entry := path[0]
	dir := inferDirection(entry.node.axis, entry.side)
	ratio := entry.node.ratio
	if entry.side == "left" {
		ratio = 1.0 - entry.node.ratio
	}
	return PaneSplitInfo{
		PaneIndex:   paneIdx,
		Direction:   dir,
		Ratio:       ratio,
		FocusTarget: -1,
	}
}

// inferDirection maps BSP axis + side to a split direction.
func inferDirection(axis, side string) string {
	switch {
	case axis == "vertical" && side == "right":
		return "right"
	case axis == "vertical" && side == "left":
		return "left"
	case axis == "horizontal" && side == "right":
		return "down"
	case axis == "horizontal" && side == "left":
		return "up"
	default:
		return "right"
	}
}
