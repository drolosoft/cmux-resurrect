# Backend Abstraction Layer — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rename `CmuxClient` to `Backend`, update all consumers, add transparent auto-detection — pure refactor, zero behavior changes.

**Architecture:** The existing `CmuxClient` interface becomes `Backend`. A new `Detect()` function determines the active terminal at runtime. `newClient()` in `cmd/root.go` calls `Detect()` and returns the appropriate backend (cmux today, Ghostty placeholder).

**Tech Stack:** Go 1.26, no new dependencies.

---

### Task 1: Rename Interface and Add Compat Alias

**Files:**
- Modify: `internal/client/client.go` (all 42 lines)
- Modify: `internal/client/cli.go:12` (comment only)

- [ ] **Step 1: Rename the interface and update doc comment**

In `internal/client/client.go`, replace the entire file:

```go
package client

// Backend abstracts interaction with a terminal multiplexer/emulator.
// Implementations exist for cmux (CLIClient) and Ghostty (planned).
type Backend interface {
	// Ping checks if the backend is running and reachable.
	Ping() error

	// Tree returns the full workspace/pane hierarchy.
	Tree() (*TreeResponse, error)

	// SidebarState returns metadata (CWD, git info) for a workspace.
	SidebarState(workspaceRef string) (*SidebarState, error)

	// ListWorkspaces returns all workspaces with their refs and titles.
	ListWorkspaces() ([]WorkspaceInfo, error)

	// NewWorkspace creates a new workspace, returning its ref.
	NewWorkspace(opts NewWorkspaceOpts) (string, error)

	// RenameWorkspace renames a workspace.
	RenameWorkspace(ref, title string) error

	// SelectWorkspace makes a workspace the active/visible one.
	SelectWorkspace(ref string) error

	// NewSplit creates a new split pane in a workspace, returning the new surface ref.
	NewSplit(direction, workspaceRef string) (string, error)

	// FocusPane focuses a specific pane in a workspace.
	FocusPane(paneRef, workspaceRef string) error

	// Send sends text to a surface in a workspace.
	Send(workspaceRef, surfaceRef, text string) error

	// PinWorkspace pins a workspace in the sidebar.
	PinWorkspace(ref string) error

	// CloseWorkspace closes a workspace.
	CloseWorkspace(ref string) error
}

// CmuxClient is an alias for backward compatibility during the transition.
type CmuxClient = Backend
```

- [ ] **Step 2: Update CLIClient comment in cli.go**

In `internal/client/cli.go`, line 12, change:

```go
// CLIClient implements CmuxClient by exec'ing the cmux binary.
```

to:

```go
// CLIClient implements Backend by exec'ing the cmux binary.
```

- [ ] **Step 3: Run tests to verify the alias keeps everything compiling**

Run: `go test ./... -count=1`

Expected: All tests pass — the `CmuxClient = Backend` alias means every existing reference still compiles.

- [ ] **Step 4: Run vet**

Run: `go vet ./...`

Expected: Clean.

- [ ] **Step 5: Commit**

```bash
git add internal/client/client.go internal/client/cli.go
git commit -m "refactor: rename CmuxClient interface to Backend with compat alias"
```

---

### Task 2: Update All Consumers to Use `Backend`

**Files:**
- Modify: `internal/orchestrate/restore.go:25`
- Modify: `internal/orchestrate/save.go:16`
- Modify: `internal/orchestrate/export.go:15`
- Modify: `internal/orchestrate/import.go:47`
- Modify: `internal/orchestrate/template_use.go:30`
- Modify: `internal/orchestrate/watch.go:18`
- Modify: `internal/orchestrate/save_test.go:12`
- Modify: `cmd/root.go:77`

- [ ] **Step 1: Update Restorer in restore.go**

In `internal/orchestrate/restore.go`, line 25, change:

```go
Client     client.CmuxClient
```

to:

```go
Client     client.Backend
```

- [ ] **Step 2: Update Saver in save.go**

In `internal/orchestrate/save.go`, line 16, change:

```go
Client client.CmuxClient
```

to:

```go
Client client.Backend
```

- [ ] **Step 3: Update Exporter in export.go**

In `internal/orchestrate/export.go`, line 15, change:

```go
Client client.CmuxClient
```

to:

```go
Client client.Backend
```

- [ ] **Step 4: Update Importer in import.go**

In `internal/orchestrate/import.go`, line 47, change:

```go
Client     client.CmuxClient
```

to:

```go
Client     client.Backend
```

- [ ] **Step 5: Update TemplateUser in template_use.go**

In `internal/orchestrate/template_use.go`, line 30, change:

```go
Client     client.CmuxClient
```

to:

```go
Client     client.Backend
```

- [ ] **Step 6: Update Watcher in watch.go**

In `internal/orchestrate/watch.go`, line 18, change:

```go
Client        client.CmuxClient
```

to:

```go
Client        client.Backend
```

- [ ] **Step 7: Update mockClient comment in save_test.go**

In `internal/orchestrate/save_test.go`, line 12, change:

```go
// mockClient implements client.CmuxClient for testing.
```

to:

```go
// mockClient implements client.Backend for testing.
```

- [ ] **Step 8: Update newClient() return type in root.go**

In `cmd/root.go`, line 77, change:

```go
func newClient() client.CmuxClient {
```

to:

```go
func newClient() client.Backend {
```

- [ ] **Step 9: Run tests**

Run: `go test ./... -count=1`

Expected: All tests pass — these are type-compatible changes.

- [ ] **Step 10: Commit**

```bash
git add internal/orchestrate/restore.go internal/orchestrate/save.go internal/orchestrate/export.go internal/orchestrate/import.go internal/orchestrate/template_use.go internal/orchestrate/watch.go internal/orchestrate/save_test.go cmd/root.go
git commit -m "refactor: update all consumers from CmuxClient to Backend"
```

---

### Task 3: Remove the Backward-Compat Alias

**Files:**
- Modify: `internal/client/client.go`

- [ ] **Step 1: Remove the alias**

In `internal/client/client.go`, delete these two lines at the bottom:

```go
// CmuxClient is an alias for backward compatibility during the transition.
type CmuxClient = Backend
```

- [ ] **Step 2: Run tests to confirm no remaining references**

Run: `go build ./...`

Expected: Compiles cleanly. If it fails, there's a missed `CmuxClient` reference — find and update it.

Run: `go test ./... -count=1`

Expected: All tests pass.

- [ ] **Step 3: Verify no stale references**

Run: `grep -rn "CmuxClient" --include="*.go" .`

Expected: Zero results in source code. (May appear in docs or non-Go files — that's fine.)

- [ ] **Step 4: Commit**

```bash
git add internal/client/client.go
git commit -m "refactor: remove CmuxClient backward-compat alias"
```

---

### Task 4: Add Backend Detection

**Files:**
- Create: `internal/client/detect.go`
- Create: `internal/client/detect_test.go`

- [ ] **Step 1: Write the test for Detect()**

Create `internal/client/detect_test.go`:

```go
package client

import (
	"os"
	"testing"
)

func TestDetect_CmuxSocketPath(t *testing.T) {
	t.Setenv("CMUX_SOCKET_PATH", "/tmp/cmux.sock")
	t.Setenv("CMUX_WORKSPACE_ID", "")
	if got := Detect(); got != BackendCmux {
		t.Errorf("Detect() = %q, want %q", got, BackendCmux)
	}
}

func TestDetect_CmuxWorkspaceID(t *testing.T) {
	t.Setenv("CMUX_SOCKET_PATH", "")
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:1")
	if got := Detect(); got != BackendCmux {
		t.Errorf("Detect() = %q, want %q", got, BackendCmux)
	}
}

func TestDetect_NoCmuxEnv(t *testing.T) {
	t.Setenv("CMUX_SOCKET_PATH", "")
	t.Setenv("CMUX_WORKSPACE_ID", "")
	got := Detect()
	// Without cmux env vars, result depends on whether Ghostty is running.
	// In CI/test, Ghostty is not running, so expect Unknown.
	if got == BackendCmux {
		t.Errorf("Detect() = %q without cmux env vars", got)
	}
}

func TestDetect_CmuxTakesPriority(t *testing.T) {
	// Even if Ghostty is running, cmux env vars take priority.
	t.Setenv("CMUX_SOCKET_PATH", "/tmp/cmux.sock")
	if got := Detect(); got != BackendCmux {
		t.Errorf("Detect() = %q, want %q (cmux should take priority)", got, BackendCmux)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/client/ -run TestDetect -v`

Expected: FAIL — `Detect` function not defined.

- [ ] **Step 3: Implement Detect()**

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
// Priority: cmux (if env vars are set) > Ghostty (if app is running) > unknown.
func Detect() DetectedBackend {
	if os.Getenv("CMUX_SOCKET_PATH") != "" || os.Getenv("CMUX_WORKSPACE_ID") != "" {
		return BackendCmux
	}
	if err := exec.Command("pgrep", "-x", "Ghostty").Run(); err == nil {
		return BackendGhostty
	}
	return BackendUnknown
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/client/ -run TestDetect -v`

Expected: All 4 tests pass.

- [ ] **Step 5: Run full test suite**

Run: `go test ./... -count=1`

Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/client/detect.go internal/client/detect_test.go
git commit -m "feat: add backend auto-detection (cmux env vars, Ghostty pgrep)"
```

---

### Task 5: Wire Up Auto-Detection in newClient()

**Files:**
- Modify: `cmd/root.go:77-79`

- [ ] **Step 1: Update newClient() to use Detect()**

In `cmd/root.go`, replace:

```go
func newClient() client.Backend {
	return client.NewCLIClient()
}
```

with:

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

- [ ] **Step 2: Run tests**

Run: `go test ./... -count=1`

Expected: All tests pass. Tests don't call `newClient()` directly — they use `mockClient`.

- [ ] **Step 3: Run vet**

Run: `go vet ./...`

Expected: Clean.

- [ ] **Step 4: Verify crex still works**

Run: `go run ./cmd/crex version`

Expected: Prints version info (falls through to `default` → `NewCLIClient()`).

- [ ] **Step 5: Commit**

```bash
git add cmd/root.go
git commit -m "feat: wire auto-detection into newClient(), Ghostty placeholder"
```

---

### Task 6: Final Verification

- [ ] **Step 1: Full test suite**

Run: `go test ./... -count=1`

Expected: All tests pass.

- [ ] **Step 2: Vet**

Run: `go vet ./...`

Expected: Clean.

- [ ] **Step 3: No stale CmuxClient references in Go source**

Run: `grep -rn "CmuxClient" --include="*.go" .`

Expected: Zero results.

- [ ] **Step 4: Verify Detect constants are exported**

Run: `grep -n "Backend" internal/client/detect.go`

Expected: `BackendCmux`, `BackendGhostty`, `BackendUnknown` all present.
