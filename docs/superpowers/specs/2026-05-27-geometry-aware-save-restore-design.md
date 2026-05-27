# Geometry-Aware Save & Restore

**Date:** 2026-05-27
**Status:** Approved
**Scope:** Save pane split directions and ratios from live geometry

## Problem

When saving a layout, crex defaults all non-first panes to `split = "right"` because
`cmux tree --json` does not expose pane geometry. This produces incorrect layouts on
restore — an "aside" layout (left panel + stacked right) restores as three columns
instead. The user must manually edit the TOML to set correct split directions.

Additionally, split proportions (e.g., 70/30) are lost — restore always creates
50/50 splits regardless of the original sizes.

## Solution

Use `cmux rpc pane.list` which returns `pixel_frame` (x, y, width, height) for every
pane. Reconstruct the Binary Space Partition (BSP) tree from the rectangles to infer
both split directions and ratios.

## Breaking Changes: NONE

| Change | Old crex reads new TOML | New crex reads old TOML |
|--------|------------------------|------------------------|
| `split_ratio` field | Ignored (go-toml/v2 skips unknown fields) | Zero-value (0.0) → equal split |
| `PaneGeometryProvider` interface | N/A (internal) | Optional type assertion — falls back to `split = "right"` |
| `PaneResizer` interface | N/A (internal) | Optional — skipped if unsupported |

New fields use `omitempty` so they don't appear in TOML unless needed.

## Data Model

### New field on `model.Pane`

```go
SplitRatio float64 `toml:"split_ratio,omitempty"`
```

- `0.0` or absent → equal split (0.5). No resize on restore.
- Non-zero → fraction the NEW pane occupies after the split.
- `omitempty` prevents emission when zero, keeping old TOMLs identical.

### New optional interfaces (not added to Backend)

```go
// PaneGeometryProvider is implemented by backends that expose pane pixel geometry.
// Used during save to infer split directions and ratios.
type PaneGeometryProvider interface {
    PaneList(workspaceRef string) (*PaneListResponse, error)
}

// PaneResizer is implemented by backends that support programmatic pane resize.
// Used during restore to apply saved split ratios.
type PaneResizer interface {
    ResizePane(opts ResizePaneOpts) error
}
```

### New types in `client/types.go`

```go
type PaneListResponse struct {
    WorkspaceRef   string         `json:"workspace_ref"`
    ContainerFrame ContainerFrame `json:"container_frame"`
    Panes          []PaneListPane `json:"panes"`
}

type ContainerFrame struct {
    Width  float64 `json:"width"`
    Height float64 `json:"height"`
}

type PaneListPane struct {
    Ref        string     `json:"ref"`
    Index      int        `json:"index"`
    Focused    bool       `json:"focused"`
    PixelFrame PixelFrame `json:"pixel_frame"`
}

type PixelFrame struct {
    X      float64 `json:"x"`
    Y      float64 `json:"y"`
    Width  float64 `json:"width"`
    Height float64 `json:"height"`
}

type ResizePaneOpts struct {
    PaneRef      string
    WorkspaceRef string
    Direction    string // "L", "R", "U", "D"
    Amount       int    // cells
}
```

## Algorithm: BSP Tree Reconstruction

### Core idea

Given N non-overlapping pane rectangles that tile a container, recursively find
the line (vertical or horizontal) that partitions them into two non-empty groups.

### Steps

1. **Base case**: 1 pane → leaf node.
2. **Find split line**: collect all x-right-edges and y-bottom-edges as candidates.
   For each candidate, check if it cleanly divides ALL panes into two groups
   (every pane entirely on one side).
3. **Determine direction**: vertical line → horizontal split (left/right).
   Horizontal line → vertical split (up/down).
4. **Compute ratio**: new_side_size / total_size along the split axis.
5. **Recurse** into each group with its sub-container.

### Mapping BSP tree to pane creation order

Walk panes in index order (0, 1, 2, ...). For each pane after 0:

- Find it in the BSP tree.
- Its parent node's split axis determines the direction:
  - Vertical line, pane on right → `"right"`
  - Vertical line, pane on left → `"left"`
  - Horizontal line, pane on bottom → `"down"`
  - Horizontal line, pane on top → `"up"`
- Its sibling subtree's minimum-index leaf = the pane that must be focused before
  splitting. If that index == i-1 (already focused after last creation), `focus_target = -1`.
  Otherwise, `focus_target = sibling_min_index`.

### Tolerance

Floating-point comparison uses ±2px tolerance for edge alignment.

### Fallback

If BSP reconstruction fails (panes don't tile cleanly), fall back to current
behavior: all non-first panes get `split = "right"`.

## Save Changes (`orchestrate/save.go`)

### `buildWorkspace` updates

After building the pane list from the tree, attempt geometry inference:

```go
if gp, ok := s.Client.(client.PaneGeometryProvider); ok {
    if paneList, err := gp.PaneList(tw.Ref); err == nil {
        applySplitGeometry(ws, paneList)
    }
}
```

`applySplitGeometry` runs the BSP algorithm and sets `Split`, `SplitRatio`,
and `FocusTarget` on each pane.

### `mergeUserEdits` bug fix

Add guard: do not copy commands to browser panes.

```go
// Preserve user-set command (but never for browser panes).
if ep.Command != "" && lp.Type != "browser" {
    lp.Command = ep.Command
}
```

### Split ratio in `mergeUserEdits`

Preserve user-set `SplitRatio` only if geometry is unavailable (same pattern as
split direction).

## Restore Changes (`orchestrate/restore.go`)

After each `NewSplit` / `NewPane`, if `SplitRatio` is set and != 0.5 (±0.02):

```go
if resizer, ok := r.Client.(client.PaneResizer); ok && needsResize(pane.SplitRatio) {
    // Calculate delta from 0.5 to target ratio
    resizer.ResizePane(client.ResizePaneOpts{...})
}
```

The resize amount in cells is computed from the container frame size and
`cell_width_px` / `cell_height_px` (available from pane.list during restore
if needed, or estimated at 9px wide / 20px tall as typical defaults).

## Client Implementations

### CLIClient (cmux backend)

- `PaneList`: calls `cmux rpc pane.list '{"workspace_id": "<uuid>"}'`
  - Needs workspace UUID; obtained from `tree --json --id-format both`
- `ResizePane`: calls `cmux resize-pane --pane <ref> --workspace <ref> -<DIR> --amount <n>`

### GhosttyClient

- `PaneList`: returns `ErrNotSupported` (Ghostty has no pane.list equivalent).
  Save falls back to default `"right"` behavior.
- `ResizePane`: returns `ErrNotSupported`. Restore skips ratio adjustment.

## Testing

### Geometry test cases (from existing templates)

Each template defines ground-truth split directions. Tests simulate `pixel_frame`
rectangles for a 1000x800 container and verify BSP reconstruction matches.

| Template | Panes | Container | Expected |
|----------|-------|-----------|----------|
| `cols` | P0:{0,0,500,800} P1:{500,0,500,800} | 1000x800 | P1: right |
| `rows` | P0:{0,0,1000,400} P1:{0,400,1000,400} | 1000x800 | P1: down |
| `triple` | P0:{0,0,333,800} P1:{333,0,333,800} P2:{666,0,334,800} | 1000x800 | P1: right, P2: right |
| `aside` | P0:{0,0,500,800} P1:{500,0,500,400} P2:{500,400,500,400} | 1000x800 | P1: right, P2: down |
| `shelf` | P0:{0,0,1000,400} P1:{0,400,500,400} P2:{500,400,500,400} | 1000x800 | P1: down, P2: right |
| `quad` | P0:{0,0,500,400} P1:{500,0,500,400} P2:{0,400,500,400} P3:{500,400,500,400} | 1000x800 | P1: right, P2: down @focus=0, P3: down @focus=1 |
| `dashboard` | P0:{0,0,1000,400} P1:{0,400,333,400} P2:{333,400,333,400} P3:{666,400,334,400} | 1000x800 | P1: down, P2: right, P3: right |
| `ide` | P0:{0,0,300,800} P1:{300,0,700,400} P2:{300,400,350,400} P3:{650,400,350,400} | 1000x800 | P1: right, P2: down, P3: right |

### Ratio test case

Aside with 70/30 split: P0:{0,0,700,800} P1:{700,0,300,400} P2:{700,400,300,400}
→ P1: right, ratio=0.30; P2: down, ratio=0.50.

### Integration tests

- Save → inspect TOML → verify split directions match live geometry.
- Save → restore → compare pixel_frame before/after (within tolerance).

### Backward compatibility tests

- Load old TOML (no split_ratio) → restore works identically to today.
- Save with geometry → load with mock old-parser → no parse errors.
- Backend without PaneGeometryProvider → save falls back to "right" default.

## Files Changed

| File | Type | Description |
|------|------|-------------|
| `internal/client/types.go` | modify | Add PaneListResponse, PixelFrame, ResizePaneOpts |
| `internal/client/client.go` | modify | Add PaneGeometryProvider, PaneResizer optional interfaces |
| `internal/client/cli.go` | modify | Implement PaneList, ResizePane on CLIClient |
| `internal/client/ghostty.go` | modify | Return ErrNotSupported for both |
| `internal/model/layout.go` | modify | Add SplitRatio to Pane |
| `internal/orchestrate/geometry.go` | **new** | BSP reconstruction + split inference |
| `internal/orchestrate/geometry_test.go` | **new** | Template-based test fixtures |
| `internal/orchestrate/save.go` | modify | Use geometry inference, fix mergeUserEdits |
| `internal/orchestrate/restore.go` | modify | Apply SplitRatio via ResizePane |
| `internal/orchestrate/save_test.go` | modify | Add PaneList to mock |
| `internal/orchestrate/restore_test.go` | modify | Add PaneList + ResizePane to mocks, test ratio |
| `internal/orchestrate/readiness_test.go` | modify | Add PaneList to mock |
