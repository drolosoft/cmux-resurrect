# Prompt 2: Ghostty Backend Implementation

**Branch:** `feat/multi-backend`
**Repo:** `/Users/txeo/Git/drolosoft/cmux-resurrect`
**Prerequisite:** Prompt 1 (Backend abstraction layer) must be completed first

---

## Goal

Implement a `GhosttyClient` that satisfies the `client.Backend` interface using macOS AppleScript (via `osascript`). This is the Ghostty-specific backend — it shells out to `osascript` to control Ghostty via its AppleScript API (available since Ghostty 1.3, March 2026).

## Context

After Prompt 1, the codebase has:
- A `Backend` interface in `internal/client/client.go` with 12 methods
- A `CLIClient` in `internal/client/cli.go` implementing that interface for cmux
- A `Detect()` function in `internal/client/detect.go` returning `BackendCmux`, `BackendGhostty`, or `BackendUnknown`
- A `--backend` flag in `cmd/root.go` with a placeholder error for `ghostty`
- All orchestrators (`Restorer`, `Saver`, `Exporter`, `Importer`, `Watcher`, `TemplateUser`) reference `client.Backend`

The shared types are in `internal/client/types.go`:

```go
type TreeResponse struct {
    Caller  *CallerInfo  `json:"caller"`
    Active  *CallerInfo  `json:"active"`
    Windows []TreeWindow `json:"windows"`
}

type TreeWindow struct {
    Ref, SelectedWorkspaceRef string
    Index                     int
    Active, Visible, Current  bool
    WorkspaceCount            int
    Workspaces                []TreeWorkspace
}

type TreeWorkspace struct {
    Ref, Title       string
    Index            int
    Pinned, Active, Selected bool
    Panes            []TreePane
}

type TreePane struct {
    Ref, SelectedSurfaceRef string
    Index                   int
    Active, Focused         bool
    SurfaceCount            int
    SurfaceRefs             []string
    Surfaces                []TreeSurface
}

type TreeSurface struct {
    Ref, PaneRef, Type, Title string
    URL                       *string
    Index, IndexInPane        int
    Active, Focused, Selected, SelectedInPane, Here bool
}

type SidebarState struct {
    CWD, FocusedCWD, GitBranch string
    GitDirty                   bool
}

type WorkspaceInfo struct {
    Ref, Title string
    Selected   bool
}

type NewWorkspaceOpts struct {
    CWD, Command string
}
```

## Ghostty AppleScript API Reference

Ghostty 1.3 exposes this AppleScript object hierarchy on macOS:

```
application "Ghostty"
  └─ windows
       └─ tabs
            └─ terminals
```

**Key commands:**
| AppleScript | What it does |
|---|---|
| `tell application "Ghostty" to new window with config "initial-working-directory=..."` | New window |
| `tell application "Ghostty" to new tab with config "initial-working-directory=..."` | New tab in front window |
| `tell application "Ghostty" to split direction with config "..."` | Split (direction: right, left, down, up) |
| `tell application "Ghostty" to input text "..." of terminal N of tab M of window 1` | Send text to terminal |
| `tell application "Ghostty" to focus terminal N of tab M of window 1` | Focus a terminal |
| `tell application "Ghostty" to select tab M of window 1` | Switch active tab |
| `tell application "Ghostty" to close tab M of window 1` | Close a tab |
| `tell application "Ghostty" to perform action "set_tab_title:My Title"` | Rename active tab |

**Key readable properties:**
| Property | Object | Example |
|---|---|---|
| `id` | window, tab, terminal | Unique identifier |
| `name` | tab | Tab title |
| `working directory` | terminal | Current working directory |
| `index` | tab | Tab position (1-based) |
| `selected` | tab | Whether tab is active |
| `count of terminals` | tab | Number of terminals in tab |

**Critical differences from cmux:**
- Ghostty uses **tabs** where cmux uses **workspaces**
- Ghostty uses **terminals** where cmux uses **panes/surfaces**
- Ghostty **does not have a `tree --json` equivalent** — you must enumerate via AppleScript loops
- Ghostty **has no pin concept** — `PinWorkspace` must be a no-op
- Ghostty tab indices are **1-based** in AppleScript
- AppleScript `split` operates on the **currently focused terminal** — you must focus first, then split
- The `perform action "set_tab_title:..."` command only works on the **active tab** — select the tab first

## What To Do

### Step 1: Create the Ghostty client file

Create `internal/client/ghostty.go`:

```go
package client

import (
    "context"
    "fmt"
    "os/exec"
    "strconv"
    "strings"
    "time"
)

// GhosttyClient implements Backend by controlling Ghostty via AppleScript (osascript).
// Requires macOS and Ghostty 1.3+ with AppleScript support.
type GhosttyClient struct {
    Timeout time.Duration
}

// NewGhosttyClient creates a GhosttyClient with sensible defaults.
func NewGhosttyClient() *GhosttyClient {
    return &GhosttyClient{
        Timeout: 10 * time.Second,
    }
}
```

### Step 2: Implement the `osascript` runner

The client needs a helper to execute AppleScript:

```go
func (g *GhosttyClient) runScript(script string) (string, error) {
    ctx, cancel := context.WithTimeout(context.Background(), g.Timeout)
    defer cancel()
    cmd := exec.CommandContext(ctx, "osascript", "-e", script)
    out, err := cmd.CombinedOutput()
    if err != nil {
        return "", fmt.Errorf("osascript: %w\n%s", err, string(out))
    }
    return strings.TrimSpace(string(out)), nil
}

// For multi-line scripts, pass each line as a separate -e argument:
func (g *GhosttyClient) runScriptLines(lines ...string) (string, error) {
    ctx, cancel := context.WithTimeout(context.Background(), g.Timeout)
    defer cancel()
    args := make([]string, 0, len(lines)*2)
    for _, line := range lines {
        args = append(args, "-e", line)
    }
    cmd := exec.CommandContext(ctx, "osascript", args...)
    out, err := cmd.CombinedOutput()
    if err != nil {
        return "", fmt.Errorf("osascript: %w\n%s", err, string(out))
    }
    return strings.TrimSpace(string(out)), nil
}
```

### Step 3: Implement each Backend method

Here is the mapping and implementation guidance for each of the 12 methods:

#### `Ping() error`
Check if Ghostty is running:
```applescript
tell application "System Events" to (name of processes) contains "Ghostty"
```
Return `nil` if "true", error if "false" or if osascript fails.

#### `Tree() (*TreeResponse, error)`
This is the most complex method. Ghostty has no `tree --json`, so you must enumerate:

```applescript
tell application "Ghostty"
    set windowCount to count of windows
    repeat with w from 1 to windowCount
        set tabCount to count of tabs of window w
        repeat with t from 1 to tabCount
            set tabName to name of tab t of window w
            set isSelected to selected of tab t of window w
            set termCount to count of terminals of tab t of window w
            -- for each terminal, get working directory
            repeat with term from 1 to termCount
                set termID to id of terminal term of tab t of window w
                set termCWD to working directory of terminal term of tab t of window w
            end repeat
        end repeat
    end repeat
end tell
```

**Strategy:** Build a single AppleScript that collects all data into a delimited string (e.g., pipe-separated lines), parse it in Go. This avoids N+1 osascript calls.

Map the data to the existing types:
- Ghostty window → `TreeWindow`
- Ghostty tab → `TreeWorkspace` (ref: `"tab:N"`, title: tab name, index: tab index)
- Ghostty terminal → `TreePane` + `TreeSurface` (ref: `"terminal:N"`)
- Set `Pinned: false` always (Ghostty has no pin concept)
- For `Caller`: Use the currently focused terminal in the selected tab

#### `SidebarState(workspaceRef string) (*SidebarState, error)`
Get the working directory from the first terminal in the tab:

```applescript
tell application "Ghostty"
    set cwd to working directory of terminal 1 of tab N of window 1
end tell
```

Parse the tab index from `workspaceRef` (e.g., `"tab:3"` → tab index 3).

Return `&SidebarState{CWD: cwd, FocusedCWD: cwd}`. Git info is not available from Ghostty's API — leave `GitBranch` empty and `GitDirty` false.

#### `ListWorkspaces() ([]WorkspaceInfo, error)`
Enumerate tabs in the front window:

```applescript
tell application "Ghostty"
    set tabCount to count of tabs of window 1
    repeat with t from 1 to tabCount
        set tabName to name of tab t of window 1
        set isSel to selected of tab t of window 1
        -- output: "tab:t|tabName|isSel"
    end repeat
end tell
```

Map to `[]WorkspaceInfo{Ref: "tab:N", Title: name, Selected: isSel}`.

#### `NewWorkspace(opts NewWorkspaceOpts) (string, error)`
Create a new tab:

```applescript
tell application "Ghostty"
    new tab with config "initial-working-directory=/path/to/dir"
end tell
```

**Ref detection:** Snapshot tab count before, create tab, then find the new tab. The new tab becomes the selected tab, so count tabs and use the last one, or diff tab IDs.

If `opts.Command` is set, send it to the new tab's terminal after creation.

Return the ref as `"tab:N"` where N is the new tab's index.

#### `RenameWorkspace(ref, title string) error`
Rename requires selecting the tab first, then using `perform action`:

```applescript
tell application "Ghostty"
    select tab N of window 1
    perform action "set_tab_title:My Title"
end tell
```

Parse tab index from ref. Note: there may be a brief delay needed between select and rename.

#### `SelectWorkspace(ref string) error`
```applescript
tell application "Ghostty" to select tab N of window 1
```

#### `NewSplit(direction, workspaceRef string) (string, error)`
Ghostty's `split` operates on the currently focused terminal. First ensure the right tab is selected, then split:

```applescript
tell application "Ghostty"
    select tab N of window 1
    split right
end tell
```

Valid directions: `right`, `left`, `down`, `up`. The cmux backend uses `right`/`down`; map accordingly.

**Ref detection:** Snapshot terminal count in the tab before split. After split, the new terminal is the newly added one. Return `"terminal:M"`.

Important: `split` creates a split of the **focused** terminal, not the tab. If you need to split a specific terminal (for quad layouts), you must `focus` that terminal first.

#### `FocusPane(paneRef, workspaceRef string) error`
Focus a specific terminal:

```applescript
tell application "Ghostty" to focus terminal M of tab N of window 1
```

Parse terminal index from `paneRef` (e.g., `"pane:0"` → terminal 1, since cmux panes are 0-based but AppleScript is 1-based).

**Important index mapping:** cmux pane refs are 0-based (`pane:0`, `pane:1`). Ghostty terminal indices are 1-based. Add 1 when converting: `pane:0` → `terminal 1`.

#### `Send(workspaceRef, surfaceRef, text string) error`
```applescript
tell application "Ghostty" to input text "command here\n" of terminal M of tab N of window 1
```

If `surfaceRef` is empty, send to terminal 1 of the tab. If set, parse the terminal index from it.

Note: cmux sends `command\n` (literal backslash-n). Check whether Ghostty's `input text` interprets `\n` as a newline or needs an actual return character. You may need to use `input text "command" & return`.

#### `PinWorkspace(ref string) error`
**No-op.** Ghostty has no pin concept. Return `nil` silently.

```go
func (g *GhosttyClient) PinWorkspace(ref string) error {
    return nil // Ghostty does not support pinning tabs
}
```

#### `CloseWorkspace(ref string) error`
```applescript
tell application "Ghostty" to close tab N of window 1
```

### Step 4: Add ref-parsing helpers

Create helpers in the same file to parse refs:

```go
// parseTabIndex extracts the 1-based tab index from a ref like "tab:3".
func parseTabIndex(ref string) (int, error) {
    parts := strings.SplitN(ref, ":", 2)
    if len(parts) != 2 {
        return 0, fmt.Errorf("invalid tab ref: %s", ref)
    }
    return strconv.Atoi(parts[1])
}

// parseTerminalIndex extracts the 1-based terminal index from refs like "terminal:2" or "pane:1".
// cmux uses 0-based pane refs; Ghostty uses 1-based terminal indices.
func parseTerminalIndex(ref string) (int, error) {
    parts := strings.SplitN(ref, ":", 2)
    if len(parts) != 2 {
        return 0, fmt.Errorf("invalid terminal ref: %s", ref)
    }
    idx, err := strconv.Atoi(parts[1])
    if err != nil {
        return 0, err
    }
    // If it's a pane ref (0-based), convert to 1-based for AppleScript.
    if parts[0] == "pane" {
        return idx + 1, nil
    }
    return idx, nil
}
```

### Step 5: Wire up the Ghostty backend in `cmd/root.go`

Replace the placeholder in `newClient()`:

```go
case "ghostty":
    return client.NewGhosttyClient()
```

And in the auto-detect path:

```go
case client.BackendGhostty:
    return client.NewGhosttyClient()
```

### Step 6: Handle the `\n` issue

The cmux `Send()` implementation appends `\\n` to commands (literal backslash-n, which cmux interprets as a newline). Ghostty's `input text` may handle this differently.

**Test this:** Send `"echo hello\n"` via `input text` and check:
1. If Ghostty treats `\n` as a newline → no change needed
2. If Ghostty needs an actual return character → transform `\\n` to `\n` (Go newline) or use `& return` in the AppleScript

The orchestrators (restore.go, import.go, template_use.go) all append `+"\\n"` before calling `Send()`. If Ghostty needs different line-ending handling, do the conversion inside `GhosttyClient.Send()` so orchestrators remain backend-agnostic.

### Step 7: Write tests

Create `internal/client/ghostty_test.go` with unit tests for:
- `parseTabIndex()` — valid and invalid refs
- `parseTerminalIndex()` — both `pane:N` (0-based) and `terminal:N` (1-based) refs
- Script generation helpers (if you extract AppleScript building into testable functions)

For the actual AppleScript integration, create `internal/client/ghostty_integration_test.go` with a build tag:

```go
//go:build integration && darwin

package client

import "testing"

func TestGhosttyPing_Integration(t *testing.T) {
    gc := NewGhosttyClient()
    err := gc.Ping()
    if err != nil {
        t.Skipf("Ghostty not running: %v", err)
    }
}
```

Use `//go:build integration && darwin` so these don't run in CI or on Linux.

### Step 8: Run all tests

```sh
go test ./... -count=1
go vet ./...
```

All existing tests must still pass. The `mockClient` in `save_test.go` already implements all 12 interface methods — it will satisfy `Backend` after Prompt 1's rename.

## Files to Create/Modify

| File | Change |
|------|--------|
| `internal/client/ghostty.go` | NEW — GhosttyClient implementation |
| `internal/client/ghostty_test.go` | NEW — unit tests for ref parsing + script helpers |
| `internal/client/ghostty_integration_test.go` | NEW — integration tests (build-tagged) |
| `cmd/root.go` | Replace ghostty placeholder with `NewGhosttyClient()` |

## Known Limitations to Document

Add a comment block at the top of `ghostty.go`:

```go
// GhosttyClient implements Backend for Ghostty (macOS only, requires 1.3+).
//
// Limitations vs cmux backend:
// - PinWorkspace is a no-op (Ghostty has no pin concept)
// - SidebarState returns no git info (not exposed by Ghostty API)
// - Tree enumeration is slower (no single JSON snapshot, must loop via AppleScript)
// - Split sizing cannot be controlled (always equal splits)
// - AppleScript API is preview — breaking changes expected in Ghostty 1.4
// - macOS only until Ghostty ships D-Bus support on Linux
```

## Commit

One commit: "feat: implement Ghostty backend via AppleScript (macOS)"

## Success Criteria

- `go test ./... -count=1` passes (all existing + new unit tests)
- `go vet ./...` clean
- `crex --backend ghostty version` works (prints version, no crash)
- When Ghostty is running on macOS:
  - `crex --backend ghostty save test` captures tabs as workspaces
  - `crex --backend ghostty restore test` recreates tabs with splits and commands
  - `crex --backend ghostty template use dev` creates a dev workspace in Ghostty
- `crex --backend ghostty template use dev --dry-run` shows Ghostty-appropriate dry-run commands
- `PinWorkspace` silently succeeds (no-op)
- Existing cmux tests still pass unchanged
