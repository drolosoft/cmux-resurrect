package orchestrate

import (
	"sort"

	"github.com/drolosoft/cmux-resurrect/internal/model"
)

// resolveSplitTargets replays a workspace's split sequence with rect tracking
// and resolves, for each pane in creation order, WHICH earlier pane it splits.
//
// A pane's FocusTarget is a live VISUAL index (left-to-right, then
// top-to-bottom) at the moment the split happens — the same convention the
// geometry-aware save emits — so the replay keeps normalized rects to rank
// panes visually at every step.
//
// Returns:
//   - targets[i]: creation index of pane i's split target (targets[0] = -1)
//   - visual[i]:  pane i's FINAL visual index in the finished workspace
//   - ok=false when a focus target is unresolvable or a direction is invalid
//
// Both the atomic cmux layout builder and the sequential restore path use
// this: the former to build the split tree, the latter to address each split
// at an explicit surface ref instead of relying on focus + live indexes
// (Ghostty re-indexes terminals when splits are inserted).
func resolveSplitTargets(ws model.Workspace) (targets []int, visual []int, ok bool) {
	n := len(ws.Panes)
	if n == 0 {
		return nil, nil, false
	}

	type rect struct{ x, y, w, h float64 }
	rects := map[int]rect{0: {0, 0, 1, 1}}

	// visualIndexOf ranks created panes by (x, y) — the live visual order.
	visualIndexOf := func(target int) int {
		type pr struct {
			idx int
			r   rect
		}
		all := make([]pr, 0, len(rects))
		for i, r := range rects {
			all = append(all, pr{i, r})
		}
		sort.Slice(all, func(a, b int) bool {
			if all[a].r.x != all[b].r.x {
				return all[a].r.x < all[b].r.x
			}
			return all[a].r.y < all[b].r.y
		})
		for rank, p := range all {
			if p.idx == target {
				return rank
			}
		}
		return -1
	}

	targets = make([]int, n)
	targets[0] = -1

	for i := 1; i < n; i++ {
		p := ws.Panes[i]

		// Explicit live visual index, or the previously created pane.
		target := i - 1
		if p.FocusTarget >= 0 {
			target = -1
			for idx := range rects {
				if visualIndexOf(idx) == p.FocusTarget {
					target = idx
					break
				}
			}
			if target < 0 {
				return nil, nil, false // dangling focus target
			}
		}
		targets[i] = target

		dir := p.Split
		if dir == "" {
			dir = "right"
		}
		newFrac := p.SplitRatio
		if newFrac <= 0 || newFrac >= 1 {
			newFrac = 0.5
		}

		tr := rects[target]
		switch dir {
		case "right":
			rects[target] = rect{tr.x, tr.y, tr.w * (1 - newFrac), tr.h}
			rects[i] = rect{tr.x + tr.w*(1-newFrac), tr.y, tr.w * newFrac, tr.h}
		case "left":
			rects[i] = rect{tr.x, tr.y, tr.w * newFrac, tr.h}
			rects[target] = rect{tr.x + tr.w*newFrac, tr.y, tr.w * (1 - newFrac), tr.h}
		case "down":
			rects[target] = rect{tr.x, tr.y, tr.w, tr.h * (1 - newFrac)}
			rects[i] = rect{tr.x, tr.y + tr.h*(1-newFrac), tr.w, tr.h * newFrac}
		case "up":
			rects[i] = rect{tr.x, tr.y, tr.w, tr.h * newFrac}
			rects[target] = rect{tr.x, tr.y + tr.h*newFrac, tr.w, tr.h * (1 - newFrac)}
		default:
			return nil, nil, false
		}
	}

	visual = make([]int, n)
	for i := 0; i < n; i++ {
		visual[i] = visualIndexOf(i)
	}
	return targets, visual, true
}
