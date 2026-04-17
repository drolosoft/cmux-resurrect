# Prompt 1: Backend Abstraction Layer

**Branch:** `feat/multi-backend`
**Repo:** `/Users/txeo/Git/drolosoft/cmux-resurrect`
**Prerequisite:** None (start here)

---

## Goal

Extract the `CmuxClient` interface into a backend-agnostic `Backend` interface so crex can support multiple terminal backends (cmux today, Ghostty next). This is a pure refactor — zero behavior changes, all existing tests must pass.

## Context

crex currently hardcodes cmux as its backend. The `CmuxClient` interface in `internal/client/client.go` (41 lines) already defines a clean 12-method contract. The implementation lives in `internal/client/cli.go` (214 lines) as `CLIClient`, which shells out to the `cmux` CLI binary.

Every orchestrator (`Restorer`, `Importer`, `Exporter`, `Saver`, `TemplateUser`) references `client.CmuxClient` as their `Client` field. These are in `internal/orchestrate/*.go`.

The `cmd/` layer instantiates `CLIClient` via a `newClient()` helper.

## What To Do

### Step 1: Rename the interface (in-place, no new packages yet)

In `internal/client/client.go`:
- Rename `CmuxClient` interface to `Backend`
- Add a type alias: `type CmuxClient = Backend` for temporary backward compat
- Update the doc comment: "Backend abstracts interaction with a terminal multiplexer/emulator."

### Step 2: Update all references

In `internal/orchestrate/*.go` (restore.go, import.go, export.go, save.go, template_use.go):
- Change `Client client.CmuxClient` to `Client client.Backend`
- This is a find-and-replace: `client.CmuxClient` → `client.Backend`

In `cmd/` files that reference the client:
- The `newClient()` function returns `*client.CLIClient` which satisfies `client.Backend` — no change needed there
- But if any type assertions reference `CmuxClient`, update them

### Step 3: Add backend detection

Create `internal/client/detect.go`:

```go
package client

import (
    "os"
    "os/exec"
)

// DetectedBackend identifies which terminal backend is available.
type DetectedBackend string

const (
    BackendCmux    DetectedBackend = "cmux"
    BackendGhostty DetectedBackend = "ghostty"
    BackendUnknown DetectedBackend = "unknown"
)

// Detect returns which terminal backend is available.
// Priority: cmux (if CMUX_SOCKET_PATH is set) > Ghostty (if app is running) > unknown.
func Detect() DetectedBackend {
    if os.Getenv("CMUX_SOCKET_PATH") != "" || os.Getenv("CMUX_WORKSPACE_ID") != "" {
        return BackendCmux
    }
    // Check if Ghostty is running (macOS only).
    if err := exec.Command("pgrep", "-x", "Ghostty").Run(); err == nil {
        return BackendGhostty
    }
    return BackendUnknown
}
```

### Step 4: Add `--backend` flag to root command

In `cmd/root.go`:
- Add a `--backend` persistent flag: `auto`, `cmux`, `ghostty`
- Default: `auto` (uses `Detect()`)
- When `auto`: detect at runtime
- When `cmux`: use `CLIClient` (existing)
- When `ghostty`: return error "Ghostty backend not yet implemented" (placeholder for Prompt 2)

Update `newClient()` to respect the flag:

```go
func newClient() client.Backend {
    switch backendFlag {
    case "cmux":
        return client.NewCLI()
    case "ghostty":
        // Placeholder — will be implemented in Prompt 2
        fmt.Fprintln(os.Stderr, "Ghostty backend not yet implemented")
        os.Exit(1)
        return nil
    default: // "auto"
        detected := client.Detect()
        switch detected {
        case client.BackendGhostty:
            fmt.Fprintln(os.Stderr, "Ghostty backend not yet implemented")
            os.Exit(1)
            return nil
        default:
            return client.NewCLI()
        }
    }
}
```

### Step 5: Remove the backward compat alias

Once all references are updated, remove `type CmuxClient = Backend` from client.go.

### Step 6: Run all tests

```sh
go test ./... -count=1
go vet ./...
```

All existing tests must pass with zero changes to test code (except the type name if tests reference `CmuxClient` directly).

## Files to Modify

| File | Change |
|------|--------|
| `internal/client/client.go` | Rename interface to `Backend` |
| `internal/client/detect.go` | NEW — backend detection |
| `internal/orchestrate/restore.go` | `CmuxClient` → `Backend` |
| `internal/orchestrate/import.go` | `CmuxClient` → `Backend` |
| `internal/orchestrate/export.go` | `CmuxClient` → `Backend` |
| `internal/orchestrate/save.go` | `CmuxClient` → `Backend` |
| `internal/orchestrate/template_use.go` | `CmuxClient` → `Backend` |
| `cmd/root.go` | Add `--backend` flag, update `newClient()` |

## Commit

One commit: "refactor: extract Backend interface from CmuxClient, add --backend flag and auto-detection"

## Success Criteria

- `go test ./... -count=1` passes (all existing tests)
- `go vet ./...` clean
- `crex version` works
- `crex --backend cmux save test` works (same as before)
- `crex --backend ghostty` prints "not yet implemented" gracefully
- `crex --backend auto` detects cmux when running inside cmux
