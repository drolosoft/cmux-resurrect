# crex pop Enhanced Picker — Centered, Fuzzy, Drill-In

**Date:** 2026-06-04
**Status:** Approved
**Scope:** Rewrite PopModel as a centered floating picker with fzf-style search and workspace drill-in

## Problem

The current `crex pop` picker is a flat full-screen list with basic substring
filtering. It doesn't feel like a quick launcher — it feels like a CLI command.
The competitor gtab has a focused, fast TUI. We need the picker to feel like
Spotlight: centered, instant, and smart.

## Solution

Rewrite `internal/tui/pop.go` as a centered floating window with fuzzy search,
match highlighting, and two-level drill-in (layouts → workspaces). Borrow
navigation patterns from the existing BrowseModel.

## Visual Design

### List Mode (default)

```
                                                    
          ╭──────────────────────────────────────╮
          │  crex > mor_                         │
          │                                      │
          │  LAYOUTS                             │
          │  ▸ [1]  morning    3 tabs  2h ago  → │
          │    [2]  afternoon  2 tabs  1d ago  → │
          │                                      │
          │  TEMPLATES                           │
          │    [3]  ⧉ ide      editor+git+term   │
          │    [4]  🤖 claude   AI pair-prog      │
          │                                      │
          │  ↵ launch  tab drill  1-9 jump  esc  │
          ╰──────────────────────────────────────╯
                                                    
```

### Drill Mode (Tab/→ on a layout)

```
          ╭──────────────────────────────────────╮
          │  crex > morning > _                  │
          │                                      │
          │  WORKSPACES in morning               │
          │  ▸ [1]  🚀 webapp  2 panes  npm|term │
          │    [2]  ⚙️ api     2 panes  go|test  │
          │    [3]  📓 docs    1 pane   shell    │
          │                                      │
          │                                      │
          │                                      │
          │  ↵ restore  esc back  1-9 jump       │
          ╰──────────────────────────────────────╯
```

## Key Interactions

### List Mode

| Key | Action |
|-----|--------|
| Any printable | Append to filter, fuzzy re-match, reset cursor |
| Backspace | Delete last filter char |
| ↑/↓ | Move cursor |
| 1-9 | Instant select by number |
| Enter | Launch selected (restore layout / use template) |
| Tab/→ | Drill into layout's workspaces (layouts only) |
| Esc | Quit without action |

### Drill Mode

| Key | Action |
|-----|--------|
| Any printable | Filter workspaces |
| Backspace | Delete filter char |
| ↑/↓ | Move cursor |
| 1-9 | Instant select workspace |
| Enter | Restore selected workspace only |
| Esc/Tab/← | Back to list mode |

### Navigation Flow

```
List mode ──Tab/→ on layout──→ Drill mode
    │                              │
    │ Enter (layout)               │ Enter (workspace)
    ↓                              ↓
Restore full layout        Restore single workspace
    │                              │
    │ Enter (template)             │ Esc/Tab/←
    ↓                              ↓
Apply template             Back to list mode
```

## Centered Floating Window

The picker renders as a bordered box centered on the terminal using
`lipgloss.Place()`. The box uses rounded borders (`lipgloss.RoundedBorder()`)
with the project's orange accent color.

**Dimensions:**
- Width: `min(termWidth - 4, 60)`, minimum 40
- Height: `min(termHeight - 4, 22)`, minimum 12
- Inner content: width - 6 (border + padding), height - 4

**Background:** `lipgloss.Place` with whitespace chars set to `" "` (clean
background). No dimming overlay — just centered box on blank space.

## Fuzzy Search

### New dependency: `github.com/sahilm/fuzzy`

Replaces the current substring filter with proper fuzzy matching:

- Contiguous matches rank higher than scattered
- Prefix matches rank higher
- Results auto-sorted by match quality (best first)
- Returns `MatchedIndexes` per result for character-level highlighting

### Match Highlighting

Matched characters render in bold + the project's orange accent color +
underline. Non-matched characters use the normal style.

```
  Filter: "cla"
  Result: [cla]ude  ← "cla" highlighted in orange+bold+underline
```

### PopItem Source Interface

```go
type fuzzySource []PopItem

func (s fuzzySource) String(i int) string {
    return s[i].Name + " " + s[i].Meta
}

func (s fuzzySource) Len() int {
    return len(s)
}
```

## Drill-In Architecture

### Model state

Single model with `viewMode` enum — no nested models. Follows the same
pattern as the existing `BrowseModel` in `shell_browse.go`.

```go
type viewMode int

const (
    modeList  viewMode = iota
    modeDrill
)
```

### New type: DrillItem

```go
type DrillItem struct {
    LayoutName  string
    Index       int    // workspace index
    Title       string
    CWD         string
    PaneCount   int
    PaneSummary string // "nvim | lazygit | shell"
    Pinned      bool
}
```

### Drill entry/exit

`enterDrill(layoutName)` calls `loadLayout(name)` (injected function),
builds `[]DrillItem` from workspaces, resets filter and cursor.

`exitDrill()` restores list mode, clears drill state, re-applies list filter.

### Layout loader injection

`NewPopModel` accepts a `loadLayout func(string) (*model.Layout, error)`
parameter. This keeps the TUI package decoupled from persist.Store.

## Result Type

The caller needs to distinguish three outcomes:

```go
type PopResult struct {
    Kind           string // "layout", "template", "workspace"
    Name           string // layout or template name
    WorkspaceTitle string // only for Kind=="workspace"
}
```

- `Kind == "layout"` → restore full layout (existing behavior)
- `Kind == "template"` → apply template (existing behavior)
- `Kind == "workspace"` → restore single workspace from layout (new)

## Scrolling

When the item list exceeds the available height, the list scrolls to keep
the cursor visible. An `offset` field tracks the scroll position. The
header and footer are pinned (never scroll).

**Visible lines calculation:**
```
listHeight = innerHeight - headerLines(2) - footerLines(2)
```

## Existing Infrastructure Reuse

| Component | Reuse |
|-----------|-------|
| `tui.Item` / `ItemsFromLayouts` | NOT used — PopItem is simpler, purpose-built |
| `BrowseModel` drill-in pattern | YES — same viewMode enum + enter/exit pattern |
| `shell_styles.go` colors | YES — same AdaptiveColor palette |
| `gallery.List/Get` | YES — for template items |
| `persist.Store.Load` | YES — injected as loadLayout for drill-in |

## Testing Strategy

### Unit tests (pop_test.go) — rewrite

**Fuzzy filter tests:**
- Empty filter shows all items
- Filter "mor" matches "morning" but not "afternoon"
- Filter "cde" fuzzy-matches "claude" (scattered match)
- Filter "zzz" returns empty
- Match positions are captured for highlighting

**Navigation tests:**
- Cursor up/down with bounds clamping
- Number keys 1-9 select correct item
- Number out of range returns nil
- Cursor resets to 0 after filter change

**Drill-in tests:**
- Tab on layout enters drill mode with correct workspaces
- Tab on template does nothing (stays in list)
- Esc in drill mode returns to list
- Enter in drill mode returns workspace result
- Filter works within drill mode

**View rendering tests:**
- Contains section headers (LAYOUTS, TEMPLATES, WORKSPACES)
- Contains footer hints (different per mode)
- Centered box has border characters (╭, ╮, ╰, ╯ for rounded)
- Breadcrumb shows "layout > " in drill mode

**Scroll tests:**
- Cursor at bottom triggers scroll
- Cursor at top after scroll adjusts offset

### Integration tests (cmd/pop.go)

- `crex pop --last` still works
- `crex pop <name>` direct launch still works
- `crex pop <template> <path>` still works

## Files Changed

| File | Type | Description |
|------|------|-------------|
| `internal/tui/pop.go` | rewrite | Centered floating picker with fuzzy + drill-in |
| `internal/tui/pop_test.go` | rewrite | Comprehensive tests for new behavior |
| `cmd/pop.go` | modify | Pass loadLayout to NewPopModel, handle workspace result |
| `go.mod` / `go.sum` | modify | Add `github.com/sahilm/fuzzy` |

## What We Don't Build (YAGNI)

- No dimming overlay / transparent background effect
- No animation on open/close
- No preview panel (Phase 2 — separate spec)
- No multi-select
- No custom keybinding configuration for the picker itself
- No mouse support

## Breaking Changes: NONE

PopItem, PopModel constructor, and Chosen() signatures change, but these
are internal types only used by cmd/pop.go. No public API affected.
