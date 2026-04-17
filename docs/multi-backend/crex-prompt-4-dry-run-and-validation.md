# Prompt 4: Backend-Aware Dry-Run and Validation

**Branch:** `feat/multi-backend`
**Repo:** `/Users/txeo/Git/drolosoft/cmux-resurrect`
**Prerequisite:** Prompts 1, 2, and 3 must be completed first

---

## Goal

Make the dry-run output backend-aware so it shows the correct commands for the active backend (cmux CLI commands for cmux, AppleScript snippets for Ghostty). Also fix hardcoded "cmux not reachable" error messages and validate the full end-to-end flow.

## Context

After Prompts 1-3, the codebase has:
- A `Backend` interface with `CLIClient` (cmux) and `GhosttyClient` (Ghostty) implementations
- A `--backend` flag with auto-detection
- All orchestrators using `client.Backend`
- Updated README, CLI help, and banner reflecting multi-backend support (cmux + Ghostty)
- Go module path is still `github.com/drolosoft/cmux-resurrect` (unchanged)

**The problem:** The dry-run code in `restore.go`, `import.go`, and `template_use.go` hardcodes `cmux` CLI commands in the output strings. When a user runs `crex --backend ghostty restore test --dry-run`, they see `cmux new-workspace ...` commands — confusing and wrong.

Similarly, error messages like `"cmux not reachable"` are cmux-specific.

## What To Do

### Step 1: Add a `DryRunPrefix` method to the Backend interface (or use a different approach)

Option A — Add a method to Backend:
```go
// DryRunCommandPrefix returns the command prefix for dry-run output.
// For cmux: "cmux", for Ghostty: "osascript -e 'tell application \"Ghostty\" to"
DryRunCommandPrefix() string
```

Option B (recommended) — Pass the backend name to orchestrators and use it in dry-run formatting:

Add a `BackendName() string` method to the `Backend` interface:
```go
// BackendName returns the human-readable name of the backend ("cmux" or "ghostty").
BackendName() string
```

Then in the orchestrators, replace hardcoded `"cmux "` with the backend name. But since cmux and Ghostty have completely different command syntaxes, the cleaner approach is:

Option C (cleanest) — Add a `DryRunCommands` method to Backend that generates the dry-run command list for a set of operations. This way each backend controls its own dry-run output format.

**Recommended: Option B with a `DryRunFormatter` interface.**

Create `internal/client/dryrun.go`:

```go
package client

import "fmt"

// DryRunFormatter generates human-readable command strings for dry-run mode.
type DryRunFormatter interface {
    FmtNewWorkspace(cwd string) string
    FmtRenameWorkspace(ref, title string) string
    FmtSelectWorkspace(ref string) string
    FmtNewSplit(direction, ref string) string
    FmtFocusPane(paneRef, workspaceRef string) string
    FmtSend(workspaceRef, text string) string
    FmtPinWorkspace(ref string) string
    FmtCloseWorkspace(ref string) string
}

// CmuxDryRun formats dry-run commands as cmux CLI commands.
type CmuxDryRun struct{}

func (CmuxDryRun) FmtNewWorkspace(cwd string) string {
    return fmt.Sprintf("cmux new-workspace --cwd %q", cwd)
}
func (CmuxDryRun) FmtRenameWorkspace(ref, title string) string {
    return fmt.Sprintf("cmux rename-workspace --workspace %s %q", ref, title)
}
func (CmuxDryRun) FmtSelectWorkspace(ref string) string {
    return fmt.Sprintf("cmux select-workspace --workspace %s", ref)
}
func (CmuxDryRun) FmtNewSplit(direction, ref string) string {
    return fmt.Sprintf("cmux new-split %s --workspace %s", direction, ref)
}
func (CmuxDryRun) FmtFocusPane(paneRef, workspaceRef string) string {
    return fmt.Sprintf("cmux focus-pane --pane %s --workspace %s", paneRef, workspaceRef)
}
func (CmuxDryRun) FmtSend(workspaceRef, text string) string {
    return fmt.Sprintf("cmux send --workspace %s %q", workspaceRef, text)
}
func (CmuxDryRun) FmtPinWorkspace(ref string) string {
    return fmt.Sprintf("cmux workspace-action --action pin --workspace %s", ref)
}
func (CmuxDryRun) FmtCloseWorkspace(ref string) string {
    return fmt.Sprintf("cmux close-workspace --workspace %s", ref)
}

// GhosttyDryRun formats dry-run commands as AppleScript snippets.
type GhosttyDryRun struct{}

func (GhosttyDryRun) FmtNewWorkspace(cwd string) string {
    return fmt.Sprintf(`osascript -e 'tell application "Ghostty" to new tab with config "initial-working-directory=%s"'`, cwd)
}
func (GhosttyDryRun) FmtRenameWorkspace(ref, title string) string {
    return fmt.Sprintf(`osascript -e 'tell application "Ghostty" to perform action "set_tab_title:%s"'`, title)
}
func (GhosttyDryRun) FmtSelectWorkspace(ref string) string {
    return fmt.Sprintf(`osascript -e 'tell application "Ghostty" to select tab %s of window 1'`, ref)
}
func (GhosttyDryRun) FmtNewSplit(direction, ref string) string {
    return fmt.Sprintf(`osascript -e 'tell application "Ghostty" to split %s'`, direction)
}
func (GhosttyDryRun) FmtFocusPane(paneRef, workspaceRef string) string {
    return fmt.Sprintf(`osascript -e 'tell application "Ghostty" to focus terminal %s of tab %s of window 1'`, paneRef, workspaceRef)
}
func (GhosttyDryRun) FmtSend(workspaceRef, text string) string {
    return fmt.Sprintf(`osascript -e 'tell application "Ghostty" to input text %q of tab %s of window 1'`, text, workspaceRef)
}
func (GhosttyDryRun) FmtPinWorkspace(ref string) string {
    return "# pin: not supported by Ghostty"
}
func (GhosttyDryRun) FmtCloseWorkspace(ref string) string {
    return fmt.Sprintf(`osascript -e 'tell application "Ghostty" to close tab %s of window 1'`, ref)
}
```

### Step 2: Add `DryRunFormatter()` to the Backend interface

```go
type Backend interface {
    // ... existing 12 methods ...

    // DryRunFormatter returns a formatter for generating dry-run command output.
    DryRunFormatter() DryRunFormatter
}
```

Implement in both clients:
```go
func (c *CLIClient) DryRunFormatter() DryRunFormatter { return CmuxDryRun{} }
func (g *GhosttyClient) DryRunFormatter() DryRunFormatter { return GhosttyDryRun{} }
```

Update the `mockClient` in `save_test.go`:
```go
func (m *mockClient) DryRunFormatter() client.DryRunFormatter { return client.CmuxDryRun{} }
```

### Step 3: Update orchestrators to use DryRunFormatter

In the `Restorer`, `Importer`, and `TemplateUser` structs, use `r.Client.DryRunFormatter()` to get the formatter, then call its methods instead of hardcoded `fmt.Sprintf("cmux ...")` strings.

**`internal/orchestrate/restore.go` — `dryRunWorkspace()`:**

Replace all `fmt.Sprintf("cmux ...")` with formatter calls:
```go
func (r *Restorer) dryRunWorkspace(ws model.Workspace, result *RestoreResult) (string, error) {
    ref := fmt.Sprintf("workspace:new_%d", ws.Index)
    f := r.Client.DryRunFormatter()

    result.Commands = append(result.Commands, "")
    result.Commands = append(result.Commands, fmt.Sprintf("# %s", ws.Title))
    result.Commands = append(result.Commands, f.FmtNewWorkspace(ws.CWD))
    result.Commands = append(result.Commands, f.FmtRenameWorkspace(ref, ws.Title))

    for i, pane := range ws.Panes {
        if i == 0 {
            if pane.Command != "" {
                result.Commands = append(result.Commands, f.FmtSend(ref, pane.Command))
            }
            continue
        }
        if pane.FocusTarget >= 0 {
            result.Commands = append(result.Commands,
                f.FmtFocusPane(fmt.Sprintf("pane:%d", pane.FocusTarget), ref))
        }
        direction := pane.Split
        if direction == "" {
            direction = "right"
        }
        result.Commands = append(result.Commands, f.FmtNewSplit(direction, ref))
        if pane.Command != "" {
            result.Commands = append(result.Commands, f.FmtSend(ref, pane.Command))
        }
    }

    return ref, nil
}
```

Also update the `"cmux select-workspace --workspace <caller>"` line in `Restore()`:
```go
result.Commands = append(result.Commands, r.Client.DryRunFormatter().FmtSelectWorkspace("<caller>"))
```

And the `"# Close all existing workspaces (except caller)"` comment line — that one is fine as-is (it's a comment, not a command).

**`internal/orchestrate/template_use.go` — `dryRun()`:**

Same pattern — replace all `fmt.Sprintf("cmux ...")` with formatter calls.

**`internal/orchestrate/import.go`:**

The import dry-run path uses `ImportEvent` with `ImportCreated` status, not direct command strings. The dry-run preview is rendered by the cmd layer. Check if any cmux-specific strings are in the import dry-run path. If not, no changes needed here. But the actual execution path has `"cmux not reachable"` error messages that should be made generic.

### Step 4: Fix hardcoded error messages

Replace backend-specific error messages:

| File | Current | New |
|------|---------|-----|
| `cmd/template_use.go:85` | `"cmux not reachable: %w"` | `"backend not reachable: %w"` |
| `internal/orchestrate/restore.go:50` | `"cmux not reachable: %w"` | `"backend not reachable: %w"` |
| `cmd/import_from_md.go:36` | `"cmux not reachable: %w"` | `"backend not reachable: %w"` |
| `internal/orchestrate/save.go:28` | `"no windows found in cmux"` | `"no windows found"` |

Search for any other `"cmux` strings in error messages:
```sh
grep -rn '"cmux ' --include="*.go" .
```

### Step 5: Update dry-run test assertions

The test `TestRestore_DryRun` in `restore_test.go` checks for strings like `"new-workspace"`, `"rename-workspace"`, `"new-split"`, `"send"`, `"select-workspace"`. These use `containsStr()` which checks substrings.

Since the `mockClient` returns `CmuxDryRun{}`, the dry-run output will still contain `cmux new-workspace`, so the existing tests should pass. But verify this. If the test assertions are too cmux-specific, make them check for the operation concept rather than the exact command syntax.

### Step 6: Run full test suite

```sh
go test ./... -count=1
go vet ./...
```

## Files to Create/Modify

| File | Change |
|------|--------|
| `internal/client/dryrun.go` | NEW — DryRunFormatter interface + cmux/ghostty implementations |
| `internal/client/client.go` | Add `DryRunFormatter()` to Backend interface |
| `internal/client/cli.go` | Implement `DryRunFormatter()` |
| `internal/client/ghostty.go` | Implement `DryRunFormatter()` |
| `internal/orchestrate/restore.go` | Use formatter for dry-run commands, fix error messages |
| `internal/orchestrate/template_use.go` | Use formatter for dry-run commands |
| `internal/orchestrate/save.go` | Fix error message |
| `internal/orchestrate/save_test.go` | Add `DryRunFormatter()` to mockClient |
| `cmd/template_use.go` | Fix error message |
| `cmd/import_from_md.go` | Fix error message |

## Commit

One commit: "feat: backend-aware dry-run output and generic error messages"

## Success Criteria

- `go test ./... -count=1` passes
- `go vet ./...` clean
- `crex --backend cmux restore test --dry-run` shows `cmux` CLI commands
- `crex --backend ghostty restore test --dry-run` shows `osascript` commands
- `crex --backend ghostty template use dev --dry-run` shows `osascript` commands
- No remaining hardcoded `"cmux "` strings in orchestrator dry-run output
- `grep -rn '"cmux ' --include="*.go" internal/orchestrate/` returns only comments/timing docs, not command strings
