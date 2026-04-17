# Ghostty Backend Implementation Prompt

**Give this prompt to a fresh Claude Code session on branch `feat/multi-backend`.**

---

## Context

crex (`cmux-resurrect`) is a Go CLI that saves/restores terminal workspaces. It currently only supports **cmux** (Ghostty's built-in multiplexer) via CLI exec. We need a second backend that drives **Ghostty directly via AppleScript**, bypassing cmux entirely. This lets crex work with stock Ghostty — no cmux binary required.

The codebase already has a clean `CmuxClient` interface. Your job is to implement `GhosttyClient` satisfying that same interface, wire it up via config, and test it.

**Repo:** `/Users/txeo/Git/drolosoft/cmux-resurrect`
**Branch:** `feat/multi-backend` (already checked out)
**Module:** `github.com/drolosoft/cmux-resurrect`
**Go version:** 1.26

---

## Architecture — What Already Exists

### The interface (`internal/client/client.go`)

```go
type CmuxClient interface {
    Ping() error
    Tree() (*TreeResponse, error)
    SidebarState(workspaceRef string) (*SidebarState, error)
    ListWorkspaces() ([]WorkspaceInfo, error)
    NewWorkspace(opts NewWorkspaceOpts) (string, error)
    RenameWorkspace(ref, title string) error
    SelectWorkspace(ref string) error
    NewSplit(direction, workspaceRef string) (string, error)
    FocusPane(paneRef, workspaceRef string) error
    Send(workspaceRef, surfaceRef, text string) error
    PinWorkspace(ref string) error
    CloseWorkspace(ref string) error
}
```

### The existing CLI backend (`internal/client/cli.go`)

`CLIClient` implements the interface by shelling out to the `cmux` binary. Key patterns to replicate:
- `NewWorkspace()` uses snapshot-and-diff: captures refs before creation, polls `ListWorkspaces()` until a new ref appears
- `NewSplit()` uses the same snapshot-and-diff against `Tree()` to find the new surface ref
- `Send()` appends `\n` to execute commands (the orchestration layer adds `\\n` to the text)

### Data types (`internal/client/types.go`)

The `TreeResponse` hierarchy: `TreeResponse` → `TreeWindow[]` → `TreeWorkspace[]` → `TreePane[]` → `TreeSurface[]`. Each level has `Ref`, `Index`, `Active`/`Selected`/`Focused` fields. The Ghostty backend must build this same structure from AppleScript queries.

Also: `SidebarState` (CWD, git info), `WorkspaceInfo` (ref, title, selected), `NewWorkspaceOpts` (CWD, command).

### Orchestration (`internal/orchestrate/`)

`restore.go` and `template_use.go` consume the `CmuxClient` interface. They handle:
- Fixed delays between operations (`internal/orchestrate/timing.go`)
- Workspace creation → select → split → send command → rename → pin

The orchestration layer does NOT need changes. Your `GhosttyClient` just needs to satisfy the interface correctly.

### Factory (`cmd/root.go`)

Currently hardcoded:
```go
func newClient() client.CmuxClient {
    return client.NewCLIClient()
}
```

Needs to become backend-aware (see Task 4 below).

### Config (`internal/config/config.go`)

TOML-based at `~/.config/crex/config.toml`. Currently has: `layouts_dir`, `workspace_file`, `watch_interval`, `max_autosaves`. Needs a `backend` field.

---

## Ghostty AppleScript API — Validated Results

All commands below were **tested and verified** on Ghostty 1.3.1. The sdef lives at `Ghostty.app/Contents/Resources/Ghostty.sdef`. Read `docs/multi-backend/ghostty-validation-tests.md` for full test results.

### Correct Syntax (many docs/examples online are WRONG)

| Operation | Correct AppleScript |
|-----------|-------------------|
| **Ping** | `tell application "System Events" to (name of processes) contains "Ghostty"` |
| **Count windows** | `tell application "Ghostty" to count of windows` |
| **Count tabs** | `tell application "Ghostty" to count of tabs of window 1` |
| **Tab name** | `tell application "Ghostty" to name of tab N of window 1` |
| **Tab selected** | `tell application "Ghostty" to selected of tab N of window 1` |
| **Terminal count** | `tell application "Ghostty" to count of terminals of tab N of window 1` |
| **Working directory** | `tell application "Ghostty" to working directory of terminal N of tab M of window 1` |
| **Terminal ID** | `tell application "Ghostty" to id of terminal N of tab M of window 1` — returns UUID |
| **Tab ID** | `tell application "Ghostty" to id of tab N of window 1` — returns `tab-HEXADDR` |
| **Window ID** | `tell application "Ghostty" to id of window 1` — returns `tab-group-HEXADDR` |
| **New tab** | `tell application "Ghostty" to new tab in front window` |
| **New tab + CWD** | `set cfg to new surface configuration from {initial working directory:"/path"}` then `new tab in front window with configuration cfg` |
| **New tab + CWD + command** | `new surface configuration from {initial working directory:"/path", initial input:"cmd\n"}` — Ghostty handles timing internally |
| **Select tab** | `tell application "Ghostty" to select tab (a reference to tab N of window 1)` |
| **Rename tab** | `tell application "Ghostty" to perform action "set_tab_title:NAME" on terminal 1 of tab N of window 1` — returns `true` |
| **Split** | `tell application "Ghostty" to split terminal N of tab M of window 1 direction right` — returns terminal specifier |
| **Split with config** | `split terminal N of tab M of window 1 direction right with configuration cfg` |
| **Focus terminal** | `tell application "Ghostty" to focus terminal N of tab M of window 1` |
| **Focused terminal** | `tell application "Ghostty" to focused terminal of tab N of window 1` — returns terminal ref |
| **Send text (no execute)** | `tell application "Ghostty" to input text "cmd" to terminal N of tab M of window 1` |
| **Execute command** | `input text "cmd" to terminal ...` then `send key "enter" to terminal ...` — two separate calls |
| **Send Ctrl+C** | `send key "c" modifiers "control" to terminal N of tab M of window 1` |
| **Close tab** | `tell application "Ghostty" to close tab (a reference to tab N of window 1)` — no confirmation dialog |
| **Close terminal** | `tell application "Ghostty" to close terminal N of tab M of window 1` |

### Critical Behaviors

1. **CWD updates after `cd`** — `working directory` returns empty until the shell starts, then tracks CWD changes reliably.

2. **Shell readiness detection** — After `new tab` or `split`, poll `working directory` until non-empty. Do NOT use a fixed delay. Pattern:
   ```
   empty → empty → ... → CWD populated (shell ready)
   ```
   On a fast Mac this takes ~500ms. On slow hardware it takes longer. Polling adapts automatically.

3. **`initial input` on surface configuration** — For commands known at creation time, use `initial input` in the surface config. Ghostty queues the input and delivers it when the shell is ready. **This eliminates the readiness-polling problem for `template use` and `restore`.** Example: `new surface configuration from {initial working directory:"/tmp", initial input:"npm run dev\n"}`.

4. **Tab titles are sticky** — `set_tab_title` is NOT overwritten by the shell prompt. No delay-before-rename needed (unlike cmux where `DelayBeforeRename` exists).

5. **`input text` does NOT execute** — It pastes text but does not press Enter. To execute: `input text "cmd" to terminal ...` followed by `send key "enter" to terminal ...`. Embedded `& return`, `& linefeed`, and literal newlines all FAIL.

6. **No confirmation dialogs** — `close tab` and `close terminal` close immediately.

7. **No Accessibility permissions needed** — Ghostty's native AppleScript suite works without extra grants.

8. **ID formats** — Terminal: UUID (`F0D23D26-BA0D-40B8-9637-94701BFB8E34`), Tab: `tab-HEXADDR`, Window: `tab-group-HEXADDR`.

---

## Implementation Tasks

### Task 1: AppleScript executor helper (`internal/client/applescript.go`)

Create a low-level helper that all Ghostty methods will use:

```go
// runAppleScript executes an AppleScript snippet via osascript and returns trimmed stdout.
func runAppleScript(ctx context.Context, script string) (string, error)
```

- Use `exec.CommandContext(ctx, "osascript", "-e", script)`
- Trim whitespace from output
- Wrap errors with the script snippet (truncated) for debuggability
- Keep it simple — no template engine, just string formatting in each caller

### Task 2: `GhosttyClient` struct (`internal/client/ghostty.go`)

```go
type GhosttyClient struct {
    Timeout time.Duration
}

func NewGhosttyClient() *GhosttyClient {
    return &GhosttyClient{Timeout: 10 * time.Second}
}
```

Implement every `CmuxClient` method. Mapping:

| CmuxClient method | Ghostty AppleScript approach |
|---|---|
| `Ping()` | Check `System Events` for Ghostty process |
| `Tree()` | Enumerate windows → tabs → terminals. Build `TreeResponse`. Tab = workspace, terminal = pane/surface. Use window 1 only (single-window model). |
| `SidebarState(ref)` | `ref` is a tab ID. Find the tab, read `working directory` of terminal 1. Git info: exec `git -C <cwd> rev-parse --abbrev-ref HEAD` + `git -C <cwd> status --porcelain` (same as what cmux does internally). |
| `ListWorkspaces()` | Enumerate tabs of window 1. Tab ID = ref, tab name = title, tab selected = selected. |
| `NewWorkspace(opts)` | Create surface config with `initial working directory` (and `initial input` if `opts.Command` is set). `new tab in front window with configuration cfg`. Parse returned tab ID from output. |
| `RenameWorkspace(ref, title)` | Find tab by ref, `perform action "set_tab_title:TITLE" on terminal 1 of tab N of window 1`. |
| `SelectWorkspace(ref)` | Find tab by ref, `select tab (a reference to tab N of window 1)`. |
| `NewSplit(direction, ref)` | Find tab by ref. Determine which terminal to split (use focused terminal). `split terminal N of tab M of window 1 direction DIR`. The split command returns a terminal specifier — parse its ID (UUID). **Poll `working directory` of the new terminal until non-empty before returning** (shell readiness). |
| `FocusPane(paneRef, workspaceRef)` | `paneRef` is a terminal ID (UUID). Find its tab index and terminal index, then `focus terminal N of tab M of window 1`. |
| `Send(workspaceRef, surfaceRef, text)` | Find terminal by surfaceRef within tab workspaceRef. If text ends with `\n`, strip it and use `input text` + `send key "enter"`. If text ends with `\\n` (literal backslash-n, which is what the orchestration layer sends), also strip and execute. Otherwise just `input text`. |
| `PinWorkspace(ref)` | **Not supported by Ghostty.** Return nil (no-op). Ghostty has no pin concept. |
| `CloseWorkspace(ref)` | Find tab by ref, `close tab (a reference to tab N of window 1)`. |

#### Ref-finding helper

Many methods need to resolve a tab/terminal ref (ID string) to an index. Write a helper:

```go
// findTabIndex returns the 1-based tab index for a given tab ID in window 1.
func (g *GhosttyClient) findTabIndex(ctx context.Context, tabID string) (int, error)

// findTerminalIndex returns the 1-based terminal index for a given terminal UUID within a tab.
func (g *GhosttyClient) findTerminalIndex(ctx context.Context, termID string, tabIdx int) (int, error)
```

These iterate over tabs/terminals comparing IDs. Cache-free is fine for v1.

#### Tree building

`Tree()` must build the full `TreeResponse` hierarchy. The mapping is:

- **Ghostty window** → `TreeWindow` (use window 1 only)
- **Ghostty tab** → `TreeWorkspace` (tab ID = ref, tab name = title, tab index = index, tab selected = selected)
- **Ghostty terminal** → both `TreePane` and `TreeSurface` (terminal ID = ref for both, index matches)
- `CallerInfo` / `Active`: determine by checking which tab is selected and which terminal is focused

For a single tab, build the full tree with one AppleScript call that enumerates everything in a loop (avoid N+1 queries). Example pattern:

```applescript
tell application "Ghostty"
    set tabCount to count of tabs of window 1
    set output to ""
    repeat with t from 1 to tabCount
        set tabID to id of tab t of window 1
        set tabName to name of tab t of window 1
        set isSel to selected of tab t of window 1
        set termCount to count of terminals of tab t of window 1
        repeat with term from 1 to termCount
            set termID to id of terminal term of tab t of window 1
            set termCWD to working directory of terminal term of tab t of window 1
            set termName to name of terminal term of tab t of window 1
            set output to output & tabID & "||" & tabName & "||" & isSel & "||" & t & "||" & termID & "||" & termCWD & "||" & termName & "||" & term & linefeed
        end repeat
    end repeat
    return output
end tell
```

Parse the delimited output in Go. Use `||` as delimiter (tabs/pipes appear in terminal titles).

### Task 3: Shell readiness polling (`internal/client/ghostty.go`)

After `NewSplit()` creates a terminal, poll until the shell is ready:

```go
func (g *GhosttyClient) waitForShell(ctx context.Context, tabIdx, termIdx int) error {
    deadline := time.Now().Add(g.Timeout)
    for time.Now().Before(deadline) {
        cwd, _ := g.getTerminalCWD(ctx, tabIdx, termIdx)
        if cwd != "" {
            return nil
        }
        time.Sleep(100 * time.Millisecond)
    }
    return fmt.Errorf("terminal %d in tab %d: shell did not start within %v", termIdx, tabIdx, g.Timeout)
}
```

For `NewWorkspace()` this is NOT needed if using `initial input` on the surface config — Ghostty handles timing internally.

### Task 4: Backend selection

**`internal/config/config.go`** — add:
```go
Backend string `toml:"backend"` // "cmux" (default) or "ghostty"
```

**`cmd/root.go`** — update `newClient()`:
```go
func newClient() client.CmuxClient {
    switch cfg.Backend {
    case "ghostty":
        return client.NewGhosttyClient()
    default:
        return client.NewCLIClient()
    }
}
```

Also add a `--backend` persistent flag override:
```go
rootCmd.PersistentFlags().StringVar(&backendFlag, "backend", "", "terminal backend: cmux or ghostty")
```

And in `initConfig()`, if `backendFlag != ""` override `cfg.Backend`.

### Task 5: Timing adjustments

The orchestration layer has fixed delays in `internal/orchestrate/timing.go`. For Ghostty:

- `DelayAfterSplit` can potentially be **reduced or eliminated** because `GhosttyClient.NewSplit()` already polls for shell readiness before returning. But leave the orchestration delays as-is for now — they provide a safety margin.
- `DelayBeforeRename` can be **reduced to 0** for Ghostty since titles don't get overwritten by the shell. But again, leave as-is for v1 — it's a no-op wait, not harmful.
- Do NOT create Ghostty-specific timing constants. The readiness polling in `GhosttyClient` handles it.

### Task 6: Tests (`internal/client/ghostty_test.go`)

Write unit tests that:

1. **Test AppleScript output parsing** — create test helpers that parse the delimited output from `Tree()` enumeration. Feed them known strings and verify the `TreeResponse` structure.

2. **Test ref-finding** — verify `findTabIndex` and `findTerminalIndex` with mocked osascript output.

3. **Test `Send()` text handling** — verify `\n` and `\\n` stripping, verify the two-call pattern (input text + send key).

4. **Test `NewWorkspace()` tab ID parsing** — verify parsing of `tab id tab-HEXADDR of window id tab-group-HEXADDR`.

5. **Integration test** (build-tagged `//go:build ghostty_integration`) — requires Ghostty running. Tests `Ping()`, `NewWorkspace()`, `Tree()`, `CloseWorkspace()` end-to-end.

Follow the existing mock pattern from `internal/orchestrate/save_test.go` — the `mockClient` struct there shows how tests mock the interface.

---

## Concept Mapping: cmux ↔ Ghostty

| cmux concept | Ghostty equivalent | Notes |
|---|---|---|
| Window | Window | 1:1 — crex uses single-window model |
| Workspace | Tab | Tabs ARE workspaces in Ghostty |
| Pane | Terminal (split) | Each split creates a new terminal |
| Surface | Terminal | A terminal IS a surface |
| workspace ref | Tab ID | `tab-HEXADDR` string |
| surface ref | Terminal ID | UUID string |
| `cmux ping` | System Events process check | |
| `cmux tree --json` | Enumerate tabs → terminals via AppleScript | |
| `cmux sidebar-state` | `working directory` + git CLI | |
| `cmux new-workspace` | `new tab in front window` | |
| `cmux new-split` | `split terminal ... direction ...` | |
| `cmux send` | `input text` + `send key "enter"` | |
| `cmux rename-workspace` | `perform action "set_tab_title:..."` | |
| `cmux select-workspace` | `select tab` | |
| `cmux close-workspace` | `close tab` | |
| `cmux pin` | No equivalent | Return nil (no-op) |

---

## File Checklist

Files to **create**:
- `internal/client/applescript.go` — osascript executor
- `internal/client/ghostty.go` — `GhosttyClient` implementing `CmuxClient`
- `internal/client/ghostty_test.go` — unit tests

Files to **modify**:
- `internal/config/config.go` — add `Backend` field
- `cmd/root.go` — update `newClient()` factory + `--backend` flag

Files to **NOT modify**:
- `internal/client/client.go` — the interface stays as-is
- `internal/client/cli.go` — the cmux backend stays as-is
- `internal/orchestrate/*.go` — the orchestration layer stays as-is
- `internal/model/*.go` — data models stay as-is

---

## Quality Bar

- All existing tests must still pass (`go test ./...`)
- New tests must pass
- `go vet ./...` clean
- No new dependencies — only stdlib (`os/exec`, `context`, `strings`, `fmt`, `time`)
- The orchestration layer (`save`, `restore`, `template use`) must work unchanged with the new backend
- `crex --backend ghostty save my-session` and `crex --backend ghostty restore my-session` should work end-to-end if Ghostty is running
