# Geometry-Aware Save & Restore — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Infer correct pane split directions and ratios from cmux pane geometry during save, and apply saved ratios during restore.

**Architecture:** Optional `PaneGeometryProvider` interface (type assertion on Backend) calls `cmux rpc pane.list` to get pixel_frame data. A BSP tree reconstruction algorithm infers split directions and ratios from the rectangles. On restore, optional `PaneResizer` interface applies saved ratios via `cmux resize-pane`. Zero breaking changes — backends without geometry support fall back to the current `split = "right"` default.

**Tech Stack:** Go, cmux CLI/RPC, TOML (pelletier/go-toml/v2)

**Spec:** `docs/superpowers/specs/2026-05-27-geometry-aware-save-restore-design.md`

---

### Task 1: Foundation Types

**Files:**
- Modify: `internal/client/types.go`
- Modify: `internal/client/client.go`
- Modify: `internal/model/layout.go`

- [ ] **Step 1: Add geometry types to `client/types.go`**

Append after the `NewPaneOpts` struct (line 98):

```go
// PaneListResponse is the parsed output of `cmux rpc pane.list`.
type PaneListResponse struct {
	WorkspaceRef   string         `json:"workspace_ref"`
	ContainerFrame ContainerFrame `json:"container_frame"`
	Panes          []PaneListPane `json:"panes"`
}

// ContainerFrame is the workspace container dimensions.
type ContainerFrame struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// PaneListPane is a pane with pixel geometry from pane.list.
type PaneListPane struct {
	Ref        string     `json:"ref"`
	Index      int        `json:"index"`
	Focused    bool       `json:"focused"`
	PixelFrame PixelFrame `json:"pixel_frame"`
}

// PixelFrame is the pixel position and size of a pane.
type PixelFrame struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// ResizePaneOpts for resizing a pane.
type ResizePaneOpts struct {
	PaneRef      string
	WorkspaceRef string
	Direction    string // "L", "R", "U", "D"
	Amount       int    // cells
}
```

- [ ] **Step 2: Add optional interfaces to `client/client.go`**

Append after the `Backend` interface (line 51):

```go
// ErrNotSupported is returned by backends that don't support an optional operation.
var ErrNotSupported = fmt.Errorf("operation not supported by this backend")

// PaneGeometryProvider is optionally implemented by backends that expose
// pane pixel geometry. Used during save to infer split directions and ratios.
// Backends that don't support this are detected via type assertion; save
// falls back to default split directions.
type PaneGeometryProvider interface {
	PaneList(workspaceRef string) (*PaneListResponse, error)
}

// PaneResizer is optionally implemented by backends that support
// programmatic pane resizing. Used during restore to apply saved split ratios.
type PaneResizer interface {
	ResizePane(opts ResizePaneOpts) error
}
```

Add `"fmt"` to the import block (currently has none — create one):

```go
package client

import "fmt"
```

- [ ] **Step 3: Add SplitRatio to `model/layout.go`**

Add the field to the `Pane` struct, after `FocusTarget`:

```go
	FocusTarget int     `toml:"focus_target,omitempty"`
	SplitRatio  float64 `toml:"split_ratio,omitempty"`
```

- [ ] **Step 4: Verify it compiles**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go build ./...`
Expected: clean build, no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/client/types.go internal/client/client.go internal/model/layout.go
git commit -m "feat: add geometry types and optional PaneGeometryProvider interface

PaneListResponse, PixelFrame, PaneResizer, SplitRatio field.
Optional interfaces via type assertion — zero changes to Backend."
```

---

### Task 2: BSP Algorithm + Tests

**Files:**
- Create: `internal/orchestrate/geometry.go`
- Create: `internal/orchestrate/geometry_test.go`

- [ ] **Step 1: Write the test file with all 8 template fixtures**

Create `internal/orchestrate/geometry_test.go`:

```go
package orchestrate

import (
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/client"
)

// pf is a shorthand constructor for PixelFrame test data.
func pf(x, y, w, h float64) client.PixelFrame {
	return client.PixelFrame{X: x, Y: y, Width: w, Height: h}
}

// plp builds a PaneListPane for testing.
func plp(index int, frame client.PixelFrame) client.PaneListPane {
	return client.PaneListPane{Index: index, PixelFrame: frame}
}

func TestInferSplitDirections_SinglePane(t *testing.T) {
	panes := []client.PaneListPane{plp(0, pf(0, 0, 1000, 800))}
	got := InferSplitDirections(panes)
	if len(got) != 0 {
		t.Errorf("single pane should return empty, got %d entries", len(got))
	}
}

func TestInferSplitDirections_Cols(t *testing.T) {
	// Template: cols — two side-by-side columns
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 500, 800)),
		plp(1, pf(500, 0, 500, 800)),
	}
	got := InferSplitDirections(panes)
	assertSplit(t, got, 1, "right", -1)
}

func TestInferSplitDirections_Rows(t *testing.T) {
	// Template: rows — two stacked rows
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 1000, 400)),
		plp(1, pf(0, 400, 1000, 400)),
	}
	got := InferSplitDirections(panes)
	assertSplit(t, got, 1, "down", -1)
}

func TestInferSplitDirections_Triple(t *testing.T) {
	// Template: triple — three columns
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 333, 800)),
		plp(1, pf(333, 0, 333, 800)),
		plp(2, pf(666, 0, 334, 800)),
	}
	got := InferSplitDirections(panes)
	assertSplit(t, got, 1, "right", -1)
	assertSplit(t, got, 2, "right", -1)
}

func TestInferSplitDirections_Aside(t *testing.T) {
	// Template: aside — big left, two stacked right (the porra layout)
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 500, 800)),
		plp(1, pf(500, 0, 500, 400)),
		plp(2, pf(500, 400, 500, 400)),
	}
	got := InferSplitDirections(panes)
	assertSplit(t, got, 1, "right", -1)
	assertSplit(t, got, 2, "down", -1)
}

func TestInferSplitDirections_Shelf(t *testing.T) {
	// Template: shelf — big top, two bottom
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 1000, 400)),
		plp(1, pf(0, 400, 500, 400)),
		plp(2, pf(500, 400, 500, 400)),
	}
	got := InferSplitDirections(panes)
	assertSplit(t, got, 1, "down", -1)
	assertSplit(t, got, 2, "right", -1)
}

func TestInferSplitDirections_Quad(t *testing.T) {
	// Template: quad — 2x2 grid, requires focus_target
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 500, 400)),
		plp(1, pf(500, 0, 500, 400)),
		plp(2, pf(0, 400, 500, 400)),
		plp(3, pf(500, 400, 500, 400)),
	}
	got := InferSplitDirections(panes)
	assertSplit(t, got, 1, "right", -1)
	assertSplit(t, got, 2, "down", 0)
	assertSplit(t, got, 3, "down", 1)
}

func TestInferSplitDirections_Dashboard(t *testing.T) {
	// Template: dashboard — big top, three bottom
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 1000, 400)),
		plp(1, pf(0, 400, 333, 400)),
		plp(2, pf(333, 400, 333, 400)),
		plp(3, pf(666, 400, 334, 400)),
	}
	got := InferSplitDirections(panes)
	assertSplit(t, got, 1, "down", -1)
	assertSplit(t, got, 2, "right", -1)
	assertSplit(t, got, 3, "right", -1)
}

func TestInferSplitDirections_IDE(t *testing.T) {
	// Template: ide — left sidebar, top-right, bottom-right split
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 300, 800)),
		plp(1, pf(300, 0, 700, 400)),
		plp(2, pf(300, 400, 350, 400)),
		plp(3, pf(650, 400, 350, 400)),
	}
	got := InferSplitDirections(panes)
	assertSplit(t, got, 1, "right", -1)
	assertSplit(t, got, 2, "down", -1)
	assertSplit(t, got, 3, "right", -1)
}

func TestInferSplitDirections_Ratio(t *testing.T) {
	// Aside layout with 70/30 horizontal split
	panes := []client.PaneListPane{
		plp(0, pf(0, 0, 700, 800)),
		plp(1, pf(700, 0, 300, 400)),
		plp(2, pf(700, 400, 300, 400)),
	}
	got := InferSplitDirections(panes)
	assertSplit(t, got, 1, "right", -1)
	assertRatioApprox(t, got, 1, 0.30)
	assertSplit(t, got, 2, "down", -1)
	assertRatioApprox(t, got, 2, 0.50)
}

// --- test helpers ---

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

func assertRatioApprox(t *testing.T, splits []PaneSplitInfo, paneIndex int, wantRatio float64) {
	t.Helper()
	for _, s := range splits {
		if s.PaneIndex == paneIndex {
			diff := s.Ratio - wantRatio
			if diff < -0.05 || diff > 0.05 {
				t.Errorf("pane %d: ratio = %.3f, want ~%.3f", paneIndex, s.Ratio, wantRatio)
			}
			return
		}
	}
	t.Errorf("pane %d: not found in splits", paneIndex)
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go test ./internal/orchestrate/ -run TestInferSplit -v -count=1`
Expected: FAIL — `InferSplitDirections` and `PaneSplitInfo` not defined.

- [ ] **Step 3: Implement the BSP algorithm**

Create `internal/orchestrate/geometry.go`:

```go
package orchestrate

import (
	"math"
	"sort"

	"github.com/drolosoft/cmux-resurrect/internal/client"
)

// PaneSplitInfo describes the inferred split direction, ratio, and focus target
// for a single pane (all panes except the first).
type PaneSplitInfo struct {
	PaneIndex   int
	Direction   string  // "right", "left", "down", "up"
	Ratio       float64 // fraction the new pane occupies (0.0–1.0)
	FocusTarget int     // pane index to focus before splitting, -1 if not needed
}

// InferSplitDirections analyzes pane pixel geometry and returns split info
// for each pane after the first. Returns nil for 0 or 1 pane.
// If BSP reconstruction fails, returns nil (caller should fall back to defaults).
func InferSplitDirections(panes []client.PaneListPane) []PaneSplitInfo {
	if len(panes) <= 1 {
		return nil
	}

	// Sort by index.
	sorted := make([]client.PaneListPane, len(panes))
	copy(sorted, panes)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Index < sorted[j].Index })

	// Build BSP tree from pane rectangles.
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

	tree := buildBSP(rects)
	if tree == nil {
		return nil
	}

	// Walk panes in index order and extract split info from the BSP tree.
	var result []PaneSplitInfo
	for i := 1; i < len(sorted); i++ {
		idx := sorted[i].Index
		info := extractPaneInfo(tree, idx, sorted[i-1].Index)
		if info != nil {
			result = append(result, *info)
		}
	}
	return result
}

// --- BSP tree types ---

type paneRect struct {
	index      int
	x, y, w, h float64
}

type bspNode struct {
	// Leaf
	isLeaf    bool
	paneIndex int

	// Internal
	axis  string   // "vertical" (split left/right) or "horizontal" (split up/down)
	ratio float64  // fraction of total that the RIGHT/BOTTOM child occupies
	left  *bspNode // left or top child
	right *bspNode // right or bottom child
}

// minIndex returns the smallest pane index in the subtree.
func (n *bspNode) minIndex() int {
	if n.isLeaf {
		return n.paneIndex
	}
	l := n.left.minIndex()
	r := n.right.minIndex()
	if l < r {
		return l
	}
	return r
}

// findParent returns the parent node of the leaf with the given pane index,
// and which side the leaf is on ("left" or "right").
func (n *bspNode) findParent(paneIndex int) (*bspNode, string) {
	if n.isLeaf {
		return nil, ""
	}
	if containsIndex(n.left, paneIndex) {
		if n.left.isLeaf && n.left.paneIndex == paneIndex {
			return n, "left"
		}
		return n.left.findParent(paneIndex)
	}
	if containsIndex(n.right, paneIndex) {
		if n.right.isLeaf && n.right.paneIndex == paneIndex {
			return n, "right"
		}
		return n.right.findParent(paneIndex)
	}
	return nil, ""
}

func containsIndex(n *bspNode, paneIndex int) bool {
	if n.isLeaf {
		return n.paneIndex == paneIndex
	}
	return containsIndex(n.left, paneIndex) || containsIndex(n.right, paneIndex)
}

// --- BSP construction ---

const tolerance = 2.0 // pixels

func approxEq(a, b float64) bool {
	return math.Abs(a-b) < tolerance
}

// buildBSP reconstructs a BSP tree from non-overlapping pane rectangles.
func buildBSP(rects []paneRect) *bspNode {
	if len(rects) == 0 {
		return nil
	}
	if len(rects) == 1 {
		return &bspNode{isLeaf: true, paneIndex: rects[0].index}
	}

	// Try vertical splits (x-axis): use right edges of panes as candidates.
	xCandidates := make(map[float64]bool)
	for _, r := range rects {
		xCandidates[r.x+r.w] = true
	}
	for x := range xCandidates {
		left, right := splitAtX(rects, x)
		if len(left) > 0 && len(right) > 0 && len(left)+len(right) == len(rects) {
			totalW := containerWidth(rects)
			rightW := containerWidth(right)
			ratio := rightW / totalW
			return &bspNode{
				axis:  "vertical",
				ratio: ratio,
				left:  buildBSP(left),
				right: buildBSP(right),
			}
		}
	}

	// Try horizontal splits (y-axis): use bottom edges of panes as candidates.
	yCandidates := make(map[float64]bool)
	for _, r := range rects {
		yCandidates[r.y+r.h] = true
	}
	for y := range yCandidates {
		top, bottom := splitAtY(rects, y)
		if len(top) > 0 && len(bottom) > 0 && len(top)+len(bottom) == len(rects) {
			totalH := containerHeight(rects)
			bottomH := containerHeight(bottom)
			ratio := bottomH / totalH
			return &bspNode{
				axis:  "horizontal",
				ratio: ratio,
				left:  buildBSP(top),
				right: buildBSP(bottom),
			}
		}
	}

	return nil // can't partition — fall back
}

func splitAtX(rects []paneRect, x float64) (left, right []paneRect) {
	for _, r := range rects {
		if r.x+r.w <= x+tolerance {
			left = append(left, r)
		} else if r.x >= x-tolerance {
			right = append(right, r)
		} else {
			// Pane straddles the split line — invalid split.
			return nil, nil
		}
	}
	return left, right
}

func splitAtY(rects []paneRect, y float64) (top, bottom []paneRect) {
	for _, r := range rects {
		if r.y+r.h <= y+tolerance {
			top = append(top, r)
		} else if r.y >= y-tolerance {
			bottom = append(bottom, r)
		} else {
			return nil, nil
		}
	}
	return top, bottom
}

func containerWidth(rects []paneRect) float64 {
	minX := math.Inf(1)
	maxX := math.Inf(-1)
	for _, r := range rects {
		if r.x < minX {
			minX = r.x
		}
		if r.x+r.w > maxX {
			maxX = r.x + r.w
		}
	}
	return maxX - minX
}

func containerHeight(rects []paneRect) float64 {
	minY := math.Inf(1)
	maxY := math.Inf(-1)
	for _, r := range rects {
		if r.y < minY {
			minY = r.y
		}
		if r.y+r.h > maxY {
			maxY = r.y + r.h
		}
	}
	return maxY - minY
}

// extractPaneInfo finds a pane's split info from the BSP tree.
func extractPaneInfo(tree *bspNode, paneIndex, prevIndex int) *PaneSplitInfo {
	parent, side := tree.findParent(paneIndex)
	if parent == nil {
		return nil
	}

	info := &PaneSplitInfo{
		PaneIndex:   paneIndex,
		FocusTarget: -1,
	}

	// Determine direction from axis and which side this pane is on.
	switch {
	case parent.axis == "vertical" && side == "right":
		info.Direction = "right"
		info.Ratio = parent.ratio
	case parent.axis == "vertical" && side == "left":
		info.Direction = "left"
		info.Ratio = 1 - parent.ratio
	case parent.axis == "horizontal" && side == "right":
		info.Direction = "down"
		info.Ratio = parent.ratio
	case parent.axis == "horizontal" && side == "left":
		info.Direction = "up"
		info.Ratio = 1 - parent.ratio
	}

	// Determine focus_target: sibling subtree's min index.
	var sibling *bspNode
	if side == "left" {
		sibling = parent.right
	} else {
		sibling = parent.left
	}
	siblingMin := sibling.minIndex()
	if siblingMin != prevIndex {
		info.FocusTarget = siblingMin
	}

	return info
}
```

- [ ] **Step 4: Run the tests**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go test ./internal/orchestrate/ -run TestInferSplit -v -count=1`
Expected: ALL PASS (cols, rows, triple, aside, shelf, quad, dashboard, ide, ratio, single pane).

- [ ] **Step 5: Commit**

```bash
git add internal/orchestrate/geometry.go internal/orchestrate/geometry_test.go
git commit -m "feat: BSP tree reconstruction for pane split inference

Reconstructs binary space partition tree from pane pixel_frame
rectangles. Infers split directions, ratios, and focus targets.
Tested against all 8 gallery templates + ratio edge case."
```

---

### Task 3: CLIClient PaneList

**Files:**
- Modify: `internal/client/cli.go`

- [ ] **Step 1: Implement PaneList on CLIClient**

Add at the end of `cli.go` (before the closing — after the `Send` method):

```go
// PaneList returns pane geometry for a workspace. Implements PaneGeometryProvider.
func (c *CLIClient) PaneList(workspaceRef string) (*PaneListResponse, error) {
	// Resolve workspace ref to UUID (pane.list RPC requires workspace_id).
	uuid, err := c.resolveWorkspaceUUID(workspaceRef)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace UUID: %w", err)
	}

	params := fmt.Sprintf(`{"workspace_id": %q}`, uuid)
	out, err := c.run("rpc", "pane.list", params)
	if err != nil {
		return nil, fmt.Errorf("pane.list: %w", err)
	}

	var resp PaneListResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return nil, fmt.Errorf("parse pane.list: %w", err)
	}
	return &resp, nil
}

// resolveWorkspaceUUID maps a workspace ref (e.g. "workspace:16") to its UUID
// by querying the tree with --id-format both.
func (c *CLIClient) resolveWorkspaceUUID(workspaceRef string) (string, error) {
	out, err := c.run("tree", "--json", "--id-format", "both")
	if err != nil {
		return "", err
	}

	// Parse tree with id fields included.
	var raw struct {
		Windows []struct {
			Workspaces []struct {
				ID  string `json:"id"`
				Ref string `json:"ref"`
			} `json:"workspaces"`
		} `json:"windows"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return "", fmt.Errorf("parse tree: %w", err)
	}

	for _, w := range raw.Windows {
		for _, ws := range w.Workspaces {
			if ws.Ref == workspaceRef {
				return ws.ID, nil
			}
		}
	}
	return "", fmt.Errorf("workspace %s not found in tree", workspaceRef)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go build ./...`
Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add internal/client/cli.go
git commit -m "feat: implement PaneList on CLIClient

Calls cmux rpc pane.list with workspace UUID resolution.
Provides pixel_frame geometry for split direction inference."
```

---

### Task 4: CLIClient ResizePane

**Files:**
- Modify: `internal/client/cli.go`

- [ ] **Step 1: Implement ResizePane on CLIClient**

Add after `PaneList`:

```go
// ResizePane resizes a pane in the given direction. Implements PaneResizer.
func (c *CLIClient) ResizePane(opts ResizePaneOpts) error {
	if opts.Amount <= 0 {
		return nil // no-op for zero or negative
	}
	args := []string{"resize-pane", "--pane", opts.PaneRef, "-" + opts.Direction, "--amount", fmt.Sprintf("%d", opts.Amount)}
	if opts.WorkspaceRef != "" {
		args = append(args, "--workspace", opts.WorkspaceRef)
	}
	_, err := c.run(args...)
	return err
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go build ./...`
Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add internal/client/cli.go
git commit -m "feat: implement ResizePane on CLIClient

Calls cmux resize-pane for programmatic split ratio adjustment."
```

---

### Task 5: Ghostty Stubs

**Files:**
- Modify: `internal/client/ghostty.go`

- [ ] **Step 1: Add PaneList and ResizePane stubs**

Add after the `UnpinWorkspace` method (around line 77):

```go
// PaneList is not supported by Ghostty (no geometry API).
// Implements PaneGeometryProvider for interface satisfaction.
func (g *GhosttyClient) PaneList(workspaceRef string) (*PaneListResponse, error) {
	return nil, ErrNotSupported
}

// ResizePane is not supported by Ghostty (always equal splits).
// Implements PaneResizer for interface satisfaction.
func (g *GhosttyClient) ResizePane(opts ResizePaneOpts) error {
	return ErrNotSupported
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go build ./...`
Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add internal/client/ghostty.go
git commit -m "feat: add PaneList and ResizePane stubs for Ghostty

Returns ErrNotSupported — save falls back to default split directions."
```

---

### Task 6: Wire Geometry Into Save

**Files:**
- Modify: `internal/orchestrate/save.go`

- [ ] **Step 1: Add applySplitGeometry function**

Add after the `buildWorkspace` function (after line 161):

```go
// applySplitGeometry uses pane pixel geometry to set correct split directions,
// ratios, and focus targets on the workspace panes. Replaces the default
// "all splits are right" heuristic with BSP tree inference.
func applySplitGeometry(ws *model.Workspace, paneList *client.PaneListResponse) {
	if len(paneList.Panes) <= 1 || len(ws.Panes) <= 1 {
		return
	}

	splits := InferSplitDirections(paneList.Panes)
	if splits == nil {
		return // BSP reconstruction failed, keep defaults
	}

	// Build lookup by pane index.
	byIndex := make(map[int]PaneSplitInfo, len(splits))
	for _, s := range splits {
		byIndex[s.PaneIndex] = s
	}

	for i := range ws.Panes {
		info, ok := byIndex[ws.Panes[i].Index]
		if !ok {
			continue
		}
		ws.Panes[i].Split = info.Direction
		ws.Panes[i].FocusTarget = info.FocusTarget

		// Only store ratio if it's meaningfully different from 0.5 (equal split).
		if info.Ratio > 0 && (info.Ratio < 0.48 || info.Ratio > 0.52) {
			ws.Panes[i].SplitRatio = info.Ratio
		}
	}
}
```

- [ ] **Step 2: Call geometry inference in buildWorkspace**

In `buildWorkspace`, after the pane-building loop and before `return ws, nil` (after line 153), add:

```go
	// Infer split directions from pane pixel geometry when available.
	if gp, ok := s.Client.(client.PaneGeometryProvider); ok {
		if paneList, err := gp.PaneList(tw.Ref); err == nil {
			applySplitGeometry(ws, paneList)
		}
		// Silently fall back to default "right" if PaneList fails.
	}
```

- [ ] **Step 3: Update mergeUserEdits to preserve geometry-detected splits**

In `mergeUserEdits`, the existing logic at line 471-473 only preserves non-"right" splits from the existing file. With geometry inference, the live save now produces correct splits. Update the merge logic to prefer the LIVE (geometry-detected) split over the existing one, but still preserve user-edited splits that override both:

Replace the split merge block (lines 471-473):

```go
			// Preserve user-set split direction.
			if ep.Split != "" && ep.Split != "right" {
				lp.Split = ep.Split
			}
```

With:

```go
			// Preserve user-set split direction only when geometry
			// inference is unavailable (live split == default "right").
			// When geometry detected a real direction, trust it over
			// the existing file — the user may have repositioned panes.
			if lp.Split == "right" && ep.Split != "" && ep.Split != "right" {
				lp.Split = ep.Split
			}
```

- [ ] **Step 4: Update layoutContentChanged to include SplitRatio**

In `layoutContentChanged`, update the pane comparison (line 432) to include SplitRatio:

Replace:
```go
			if pa.Type != pb.Type || pa.Split != pb.Split || pa.Command != pb.Command || pa.URL != pb.URL || pa.Focus != pb.Focus {
```

With:
```go
			if pa.Type != pb.Type || pa.Split != pb.Split || pa.Command != pb.Command || pa.URL != pb.URL || pa.Focus != pb.Focus || pa.SplitRatio != pb.SplitRatio {
```

- [ ] **Step 5: Run existing tests to verify nothing breaks**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go test ./internal/orchestrate/ -v -count=1`
Expected: ALL existing tests still pass. The mockClient doesn't implement PaneGeometryProvider, so the type assertion falls through and old behavior is preserved.

- [ ] **Step 6: Commit**

```bash
git add internal/orchestrate/save.go
git commit -m "feat: wire geometry inference into save

Calls PaneList when backend supports it, runs BSP inference,
sets correct split directions and ratios. Falls back gracefully."
```

---

### Task 7: Fix mergeUserEdits Browser Command Leak

**Files:**
- Modify: `internal/orchestrate/save.go`

- [ ] **Step 1: Write the test**

Add to `internal/orchestrate/save_test.go`:

```go
func TestMergeUserEdits_NoBrowserCommandLeak(t *testing.T) {
	live := &model.Layout{
		Name: "test",
		Workspaces: []model.Workspace{{
			Title: "ws1",
			Panes: []model.Pane{
				{Type: "terminal"},
				{Type: "terminal", Split: "right", Command: "lnav /tmp/app.log"},
				{Type: "browser", Split: "right", URL: "http://localhost:3000"},
			},
		}},
	}
	existing := &model.Layout{
		Name: "test",
		Workspaces: []model.Workspace{{
			Title: "ws1",
			Panes: []model.Pane{
				{Type: "terminal"},
				{Type: "terminal", Split: "right", Command: "lnav /tmp/app.log"},
				{Type: "browser", Split: "right", Command: "lnav /tmp/app.log", URL: "http://localhost:3000"},
			},
		}},
	}

	mergeUserEdits(live, existing)

	// The browser pane must NOT inherit the terminal command.
	if got := live.Workspaces[0].Panes[2].Command; got != "" {
		t.Errorf("browser pane leaked command = %q, want empty", got)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go test ./internal/orchestrate/ -run TestMergeUserEdits_NoBrowserCommandLeak -v -count=1`
Expected: FAIL — browser pane gets the command.

- [ ] **Step 3: Fix mergeUserEdits**

In `mergeUserEdits` (line 477-479 of save.go), replace:

```go
			// Preserve user-set command.
			if ep.Command != "" {
				lp.Command = ep.Command
			}
```

With:

```go
			// Preserve user-set command, but never for browser panes
			// (browser panes don't run shell commands; a stale command
			// from a previous pane at this index would leak through).
			if ep.Command != "" && lp.Type != "browser" {
				lp.Command = ep.Command
			}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go test ./internal/orchestrate/ -run TestMergeUserEdits_NoBrowserCommandLeak -v -count=1`
Expected: PASS.

- [ ] **Step 5: Run all save tests**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go test ./internal/orchestrate/ -run TestSave -v -count=1`
Expected: ALL PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/orchestrate/save.go internal/orchestrate/save_test.go
git commit -m "fix: prevent browser panes from inheriting terminal commands

mergeUserEdits now skips command copy for browser panes, preventing
stale terminal commands from leaking into browser pane entries."
```

---

### Task 8: Save Test With Geometry Mock

**Files:**
- Modify: `internal/orchestrate/save_test.go`

- [ ] **Step 1: Write geometry-aware save test**

Add to `save_test.go`:

```go
// geometryMockClient extends mockClient with PaneGeometryProvider.
type geometryMockClient struct {
	mockClient
	paneListByRef map[string]*client.PaneListResponse
}

func (m *geometryMockClient) PaneList(workspaceRef string) (*client.PaneListResponse, error) {
	resp, ok := m.paneListByRef[workspaceRef]
	if !ok {
		return nil, fmt.Errorf("no geometry for %s", workspaceRef)
	}
	return resp, nil
}

func TestSave_GeometryInfersAsideLayout(t *testing.T) {
	// Tree with one workspace, 3 panes (aside layout).
	treeResp := &client.TreeResponse{
		Windows: []client.TreeWindow{{
			Ref: "window:1",
			Workspaces: []client.TreeWorkspace{{
				Ref:   "workspace:1",
				Title: "dev",
				Index: 0,
				Panes: []client.TreePane{
					{Index: 0, Focused: true, Surfaces: []client.TreeSurface{{Type: "terminal", TTY: ""}}},
					{Index: 1, Surfaces: []client.TreeSurface{{Type: "browser", URL: strPtr("http://localhost:3000")}}},
					{Index: 2, Surfaces: []client.TreeSurface{{Type: "terminal", TTY: ""}}},
				},
			}},
		}},
	}

	gmc := &geometryMockClient{
		mockClient: mockClient{
			treeResp:    treeResp,
			sidebarCWDs: map[string]string{"workspace:1": "/home/user/project"},
		},
		paneListByRef: map[string]*client.PaneListResponse{
			"workspace:1": {
				WorkspaceRef:   "workspace:1",
				ContainerFrame: client.ContainerFrame{Width: 1000, Height: 800},
				Panes: []client.PaneListPane{
					{Index: 0, PixelFrame: client.PixelFrame{X: 0, Y: 0, Width: 500, Height: 800}},
					{Index: 1, PixelFrame: client.PixelFrame{X: 500, Y: 0, Width: 500, Height: 400}},
					{Index: 2, PixelFrame: client.PixelFrame{X: 500, Y: 400, Width: 500, Height: 400}},
				},
			},
		},
	}

	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)
	saver := &Saver{Client: gmc, Store: store}

	layout, err := saver.Save("geo-test", "")
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	ws := layout.Workspaces[0]
	if len(ws.Panes) != 3 {
		t.Fatalf("panes = %d, want 3", len(ws.Panes))
	}

	// Pane 0: no split.
	if ws.Panes[0].Split != "" {
		t.Errorf("pane 0: split = %q, want empty", ws.Panes[0].Split)
	}
	// Pane 1: split right (geometry-detected).
	if ws.Panes[1].Split != "right" {
		t.Errorf("pane 1: split = %q, want right", ws.Panes[1].Split)
	}
	// Pane 2: split down (geometry-detected, NOT the default "right").
	if ws.Panes[2].Split != "down" {
		t.Errorf("pane 2: split = %q, want down", ws.Panes[2].Split)
	}
}

func strPtr(s string) *string { return &s }
```

- [ ] **Step 2: Run the test**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go test ./internal/orchestrate/ -run TestSave_GeometryInfersAsideLayout -v -count=1`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/orchestrate/save_test.go
git commit -m "test: geometry-aware save test for aside layout

Verifies BSP inference produces split=down for stacked-right pane
instead of the old default split=right."
```

---

### Task 9: Wire Ratio Into Restore

**Files:**
- Modify: `internal/orchestrate/restore.go`

- [ ] **Step 1: Add needsResize helper**

Add at the bottom of `restore.go`:

```go
// needsResize returns true if a split ratio requires a resize after creation.
// Splits default to 50/50; ratios within ±2% of 0.5 are treated as equal.
func needsResize(ratio float64) bool {
	return ratio > 0 && (ratio < 0.48 || ratio > 0.52)
}

// resizeAfterSplit adjusts a newly created pane to match the saved split ratio.
// direction is the split direction ("right", "left", "down", "up").
// ratio is the fraction the new pane should occupy.
func resizeAfterSplit(r *Restorer, paneRef, workspaceRef, direction string, ratio float64) {
	resizer, ok := r.Client.(client.PaneResizer)
	if !ok {
		return
	}

	// Delta from 50% to target. Negative means shrink the new pane.
	delta := ratio - 0.5

	// Estimate cells: assume 9px/col, 20px/row as typical defaults.
	// The actual cell size varies, but this gets within 1-2 cells.
	const cellW, cellH = 9.0, 20.0

	var resizeDir string
	var amount int
	switch direction {
	case "right":
		amount = int(1000 * delta / cellW) // rough estimate, workspace ~1000px
		if delta < 0 {
			resizeDir = "L" // shrink: pull left edge left
		} else {
			resizeDir = "R"
		}
	case "left":
		amount = int(1000 * delta / cellW)
		if delta < 0 {
			resizeDir = "R"
		} else {
			resizeDir = "L"
		}
	case "down":
		amount = int(800 * delta / cellH)
		if delta < 0 {
			resizeDir = "U"
		} else {
			resizeDir = "D"
		}
	case "up":
		amount = int(800 * delta / cellH)
		if delta < 0 {
			resizeDir = "D"
		} else {
			resizeDir = "U"
		}
	}

	if amount < 0 {
		amount = -amount
	}
	if amount == 0 {
		return
	}

	_ = resizer.ResizePane(client.ResizePaneOpts{
		PaneRef:      paneRef,
		WorkspaceRef: workspaceRef,
		Direction:    resizeDir,
		Amount:       amount,
	})
}
```

- [ ] **Step 2: Wire resize into the restore pane-creation loop**

In `restoreWorkspace`, after each successful `NewSplit` (around line 263, after `surfaceRef, err := r.Client.NewSplit(...)`), add the resize call. The block currently looks like:

```go
			surfaceRef, err := r.Client.NewSplit(direction, ref)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("  pane %d split: %v", i, err))
				continue
			}
```

Add after it (before the `if pane.Command != ""` block):

```go
			// Apply saved split ratio if available.
			if needsResize(pane.SplitRatio) {
				resizeAfterSplit(r, surfaceRef, ref, direction, pane.SplitRatio)
			}
```

Do the same after the `NewPane` call for browser panes (around line 248). After the successful `NewPane`, add:

```go
			// Apply saved split ratio if available.
			if needsResize(pane.SplitRatio) {
				resizeAfterSplit(r, "", ref, direction, pane.SplitRatio)
			}
```

- [ ] **Step 3: Run existing restore tests**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go test ./internal/orchestrate/ -run TestRestore -v -count=1`
Expected: ALL existing tests pass. SplitRatio is 0 in all existing fixtures, so needsResize returns false and nothing changes.

- [ ] **Step 4: Commit**

```bash
git add internal/orchestrate/restore.go
git commit -m "feat: apply saved split ratios during restore

Uses ResizePane to adjust panes to saved proportions. No-op when
ratio is ~0.5 or backend doesn't implement PaneResizer."
```

---

### Task 10: Backward Compatibility Tests

**Files:**
- Modify: `internal/orchestrate/save_test.go`

- [ ] **Step 1: Test that save without geometry falls back to defaults**

Add to `save_test.go`:

```go
func TestSave_NoGeometry_FallsBackToRight(t *testing.T) {
	// mockClient does NOT implement PaneGeometryProvider.
	// Save should produce the old default behavior: all splits = "right".
	treeResp := &client.TreeResponse{
		Windows: []client.TreeWindow{{
			Ref: "window:1",
			Workspaces: []client.TreeWorkspace{{
				Ref:   "workspace:1",
				Title: "compat",
				Index: 0,
				Panes: []client.TreePane{
					{Index: 0, Focused: true, Surfaces: []client.TreeSurface{{Type: "terminal", TTY: ""}}},
					{Index: 1, Surfaces: []client.TreeSurface{{Type: "terminal", TTY: ""}}},
					{Index: 2, Surfaces: []client.TreeSurface{{Type: "terminal", TTY: ""}}},
				},
			}},
		}},
	}

	mc := &mockClient{
		treeResp:    treeResp,
		sidebarCWDs: map[string]string{"workspace:1": "/tmp"},
	}

	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)
	saver := &Saver{Client: mc, Store: store}

	layout, err := saver.Save("compat-test", "")
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	ws := layout.Workspaces[0]
	// Without geometry, all non-first panes should default to "right".
	for i := 1; i < len(ws.Panes); i++ {
		if ws.Panes[i].Split != "right" {
			t.Errorf("pane %d: split = %q, want right (default)", i, ws.Panes[i].Split)
		}
		if ws.Panes[i].SplitRatio != 0 {
			t.Errorf("pane %d: split_ratio = %f, want 0 (not set)", i, ws.Panes[i].SplitRatio)
		}
	}
}
```

- [ ] **Step 2: Test TOML round-trip with SplitRatio**

Add to `save_test.go`:

```go
func TestSplitRatio_TOMLRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name:    "ratio-test",
		Version: 1,
		SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{{
			Title: "ws1",
			CWD:   "/tmp",
			Panes: []model.Pane{
				{Type: "terminal", Focus: true, FocusTarget: -1},
				{Type: "terminal", Split: "right", SplitRatio: 0.30, Index: 1, FocusTarget: -1},
				{Type: "terminal", Split: "down", Index: 2, FocusTarget: -1},
			},
		}},
	}

	if err := store.Save("ratio-test", layout); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := store.Load("ratio-test")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	pane1 := loaded.Workspaces[0].Panes[1]
	if pane1.SplitRatio != 0.30 {
		t.Errorf("pane 1: split_ratio = %f, want 0.30", pane1.SplitRatio)
	}

	// Pane 2 has zero ratio — should round-trip as 0.
	pane2 := loaded.Workspaces[0].Panes[2]
	if pane2.SplitRatio != 0 {
		t.Errorf("pane 2: split_ratio = %f, want 0", pane2.SplitRatio)
	}
}
```

- [ ] **Step 3: Run all tests**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go test ./internal/orchestrate/ -v -count=1`
Expected: ALL PASS.

- [ ] **Step 4: Run full project test suite**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go test ./... -count=1`
Expected: ALL PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/orchestrate/save_test.go
git commit -m "test: backward compatibility for geometry-aware save

Verifies no-geometry fallback produces old defaults, and SplitRatio
survives TOML round-trip correctly."
```
