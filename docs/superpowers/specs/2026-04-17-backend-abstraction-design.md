# Backend Abstraction Layer — Design Spec

**Date:** 2026-04-17
**Branch:** `feat/multi-backend`
**Type:** Pure refactor — zero behavior changes

---

## Problem

crex hardcodes cmux as its backend. The `CmuxClient` interface in `internal/client/client.go` defines a clean 12-method contract, but every consumer references it by its cmux-specific name. To support Ghostty (and future backends), we need a backend-agnostic interface and transparent auto-detection.

## Design

### 1. Rename Interface

In `internal/client/client.go`:
- Rename `CmuxClient` → `Backend`
- Add temporary alias `type CmuxClient = Backend` during transition
- Remove alias once all references are updated
- Update doc comment: "Backend abstracts interaction with a terminal multiplexer/emulator."

The 12 methods stay exactly the same. No signature changes.

### 2. Update All Consumers

Find-and-replace `client.CmuxClient` → `client.Backend` in:

| File | Field |
|------|-------|
| `internal/orchestrate/restore.go` | `Restorer.Client` |
| `internal/orchestrate/save.go` | `Saver.Client` |
| `internal/orchestrate/export.go` | `Exporter.Client` |
| `internal/orchestrate/import.go` | `Importer.Client` |
| `internal/orchestrate/template_use.go` | `TemplateUser.Client` |
| `internal/orchestrate/watch.go` | `Watcher.Client` |
| `cmd/root.go` | `newClient()` return type |
| `internal/orchestrate/save_test.go` | `mockClient` comment |

### 3. Auto-Detection

New file `internal/client/detect.go`:

```go
type DetectedBackend string

const (
    BackendCmux    DetectedBackend = "cmux"
    BackendGhostty DetectedBackend = "ghostty"
    BackendUnknown DetectedBackend = "unknown"
)

func Detect() DetectedBackend
```

Detection priority:
1. `CMUX_SOCKET_PATH` or `CMUX_WORKSPACE_ID` env var set → `BackendCmux`
2. `pgrep -x Ghostty` succeeds → `BackendGhostty`
3. Otherwise → `BackendUnknown`

### 4. Update `newClient()` in `cmd/root.go`

```go
func newClient() client.Backend {
    detected := client.Detect()
    switch detected {
    case client.BackendGhostty:
        fmt.Fprintln(os.Stderr, "Ghostty backend not yet implemented")
        os.Exit(1)
        return nil
    default:
        return client.NewCLIClient()
    }
}
```

No `--backend` flag. No configuration. The user runs `crex save my-day` and it just works.

## What Doesn't Change

- All 12 interface method signatures
- `CLIClient` implementation (`cli.go`)
- Shared types (`types.go`)
- Parse helpers (`parse.go`)
- Template gallery, Blueprint format, persist layer, config
- All existing test assertions
- User-facing CLI commands and flags
- Binary name, install paths, Homebrew formula

## Files Modified

| File | Change |
|------|--------|
| `internal/client/client.go` | Rename interface, update doc comment |
| `internal/client/detect.go` | **NEW** — `Detect()` function |
| `internal/orchestrate/restore.go` | `CmuxClient` → `Backend` |
| `internal/orchestrate/save.go` | `CmuxClient` → `Backend` |
| `internal/orchestrate/export.go` | `CmuxClient` → `Backend` |
| `internal/orchestrate/import.go` | `CmuxClient` → `Backend` |
| `internal/orchestrate/template_use.go` | `CmuxClient` → `Backend` |
| `internal/orchestrate/watch.go` | `CmuxClient` → `Backend` |
| `internal/orchestrate/save_test.go` | Update mock comment |
| `cmd/root.go` | `newClient()` return type + auto-detection |

## Success Criteria

- `go test ./... -count=1` passes with zero test code changes (except mock comment)
- `go vet ./...` clean
- `crex save test` works exactly as before inside cmux
- `crex version` works
- Running outside both cmux and Ghostty falls back to cmux (existing behavior)
