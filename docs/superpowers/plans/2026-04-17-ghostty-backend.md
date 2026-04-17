# Ghostty Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement `GhosttyClient` satisfying `client.Backend` (12 methods) via macOS AppleScript (`osascript`).

**Architecture:** Single file `internal/client/ghostty.go` mirrors the `CLIClient` pattern in `cli.go`. Each method builds an AppleScript string and runs it via `osascript`. Ref format: `tab:N` for workspaces (1-based), `terminal:N` for surfaces (1-based). Orchestrators pass `pane:N` (0-based) for FocusPane — the client converts to 1-based.

**Tech Stack:** Go 1.26, `os/exec` (osascript), no new dependencies.

**AppleScript Reference (verified against Ghostty 1.3.1 sdef):**

| Operation | Correct Syntax |
|-----------|---------------|
| Ping | `tell application "System Events" to (name of processes) contains "Ghostty"` |
| New tab | `tell application "Ghostty" to new tab in front window` |
| New tab + CWD | `set cfg to new surface configuration from {initial working directory:"/path"}` then `new tab in front window with configuration cfg` |
| Select tab | `select tab (a reference to tab N of window 1)` |
| Rename tab | `perform action "set_tab_title:NAME" on terminal 1 of tab N of window 1` |
| Split | `split terminal T of tab N of window 1 direction right` |
| Focus | `focus terminal T of tab N of window 1` |
| Input text | `input text "cmd" to terminal T of tab N of window 1` |
| Execute cmd | `input text "cmd" to terminal T of ...` then `send key "enter" to terminal T of ...` |
| Close tab | `close tab (a reference to tab N of window 1)` |
| Tab count | `count of tabs of window 1` |
| Terminal count | `count of terminals of tab N of window 1` |
| Tab name | `name of tab N of window 1` |
| Tab selected | `selected of tab N of window 1` |
| Terminal CWD | `working directory of terminal T of tab N of window 1` |
| Focused terminal | `focused terminal of tab N of window 1` |

---

### Task 1: Scaffolding — Struct, Constructor, osascript Runner, Ping, PinWorkspace

**Files:**
- Create: `internal/client/ghostty.go`

- [ ] **Step 1: Create the file with struct, constructor, runner, Ping, and PinWorkspace**

```go
package client

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// GhosttyClient implements Backend for Ghostty (macOS only, requires 1.3+).
//
// Limitations vs cmux backend:
//   - PinWorkspace is a no-op (Ghostty has no pin concept)
//   - SidebarState returns no git info (not exposed by Ghostty API)
//   - Tree enumeration is slower (no single JSON snapshot, must loop via AppleScript)
//   - Split sizing cannot be controlled (always equal splits)
//   - AppleScript API is preview — breaking changes expected in Ghostty 1.4
//   - macOS only until Ghostty ships D-Bus support on Linux
type GhosttyClient struct {
	Timeout time.Duration
}

// NewGhosttyClient creates a GhosttyClient with sensible defaults.
func NewGhosttyClient() *GhosttyClient {
	return &GhosttyClient{
		Timeout: 10 * time.Second,
	}
}

// runScript executes a single-line AppleScript via osascript.
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

// runScriptLines executes a multi-line AppleScript (each line as a separate -e arg).
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

func (g *GhosttyClient) Ping() error {
	out, err := g.runScript(`tell application "System Events" to (name of processes) contains "Ghostty"`)
	if err != nil {
		return fmt.Errorf("ghostty ping: %w", err)
	}
	if out != "true" {
		return fmt.Errorf("ghostty is not running")
	}
	return nil
}

func (g *GhosttyClient) PinWorkspace(ref string) error {
	return nil // Ghostty does not support pinning tabs
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`

Expected: Compile error — `GhosttyClient` doesn't implement `Backend` yet (missing 10 methods). That's expected; we'll add stubs in subsequent tasks.

- [ ] **Step 3: Add stub methods to satisfy Backend interface**

Add these stubs at the bottom of `ghostty.go` so the project compiles while we implement each method properly in later tasks:

```go
func (g *GhosttyClient) Tree() (*TreeResponse, error) {
	return nil, fmt.Errorf("ghostty: Tree not yet implemented")
}

func (g *GhosttyClient) SidebarState(workspaceRef string) (*SidebarState, error) {
	return nil, fmt.Errorf("ghostty: SidebarState not yet implemented")
}

func (g *GhosttyClient) ListWorkspaces() ([]WorkspaceInfo, error) {
	return nil, fmt.Errorf("ghostty: ListWorkspaces not yet implemented")
}

func (g *GhosttyClient) NewWorkspace(opts NewWorkspaceOpts) (string, error) {
	return "", fmt.Errorf("ghostty: NewWorkspace not yet implemented")
}

func (g *GhosttyClient) RenameWorkspace(ref, title string) error {
	return fmt.Errorf("ghostty: RenameWorkspace not yet implemented")
}

func (g *GhosttyClient) SelectWorkspace(ref string) error {
	return fmt.Errorf("ghostty: SelectWorkspace not yet implemented")
}

func (g *GhosttyClient) NewSplit(direction, workspaceRef string) (string, error) {
	return "", fmt.Errorf("ghostty: NewSplit not yet implemented")
}

func (g *GhosttyClient) FocusPane(paneRef, workspaceRef string) error {
	return fmt.Errorf("ghostty: FocusPane not yet implemented")
}

func (g *GhosttyClient) Send(workspaceRef, surfaceRef, text string) error {
	return fmt.Errorf("ghostty: Send not yet implemented")
}

func (g *GhosttyClient) CloseWorkspace(ref string) error {
	return fmt.Errorf("ghostty: CloseWorkspace not yet implemented")
}
```

- [ ] **Step 4: Verify it compiles and tests pass**

Run: `go build ./... && go test ./... -count=1`

Expected: Compiles and all tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/client/ghostty.go
git commit -m "feat(ghostty): add GhosttyClient scaffolding with Ping and stubs"
```

---

### Task 2: Ref-Parsing Helpers + Unit Tests

**Files:**
- Create: `internal/client/ghostty_test.go`
- Modify: `internal/client/ghostty.go`

- [ ] **Step 1: Write tests for ref helpers**

Create `internal/client/ghostty_test.go`:

```go
package client

import "testing"

func TestParseTabIndex(t *testing.T) {
	tests := []struct {
		ref     string
		want    int
		wantErr bool
	}{
		{"tab:1", 1, false},
		{"tab:5", 5, false},
		{"tab:0", 0, false},
		{"invalid", 0, true},
		{"tab:", 0, true},
		{"tab:abc", 0, true},
	}
	for _, tt := range tests {
		got, err := parseTabIndex(tt.ref)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseTabIndex(%q) error = %v, wantErr %v", tt.ref, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("parseTabIndex(%q) = %d, want %d", tt.ref, got, tt.want)
		}
	}
}

func TestParseTerminalIndex(t *testing.T) {
	tests := []struct {
		ref     string
		want    int
		wantErr bool
	}{
		// terminal refs are already 1-based — pass through.
		{"terminal:1", 1, false},
		{"terminal:3", 3, false},
		// pane refs are 0-based — convert to 1-based.
		{"pane:0", 1, false},
		{"pane:1", 2, false},
		{"pane:2", 3, false},
		// errors
		{"invalid", 0, true},
		{"pane:", 0, true},
		{"pane:abc", 0, true},
	}
	for _, tt := range tests {
		got, err := parseTerminalIndex(tt.ref)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseTerminalIndex(%q) error = %v, wantErr %v", tt.ref, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("parseTerminalIndex(%q) = %d, want %d", tt.ref, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run tests — expect FAIL**

Run: `go test ./internal/client/ -run TestParse -v`

Expected: FAIL — `parseTabIndex` and `parseTerminalIndex` not defined.

- [ ] **Step 3: Implement ref helpers in ghostty.go**

Add to `internal/client/ghostty.go`:

```go
import "strconv"
```

(add `strconv` to existing imports)

```go
// parseTabIndex extracts the 1-based tab index from a ref like "tab:3".
func parseTabIndex(ref string) (int, error) {
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) != 2 || parts[1] == "" {
		return 0, fmt.Errorf("invalid tab ref: %s", ref)
	}
	return strconv.Atoi(parts[1])
}

// parseTerminalIndex extracts the 1-based terminal index from refs.
// "terminal:N" refs are already 1-based (pass through).
// "pane:N" refs are 0-based (cmux convention) — adds 1 for AppleScript.
func parseTerminalIndex(ref string) (int, error) {
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) != 2 || parts[1] == "" {
		return 0, fmt.Errorf("invalid terminal ref: %s", ref)
	}
	idx, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}
	if parts[0] == "pane" {
		return idx + 1, nil
	}
	return idx, nil
}
```

- [ ] **Step 4: Run tests — expect PASS**

Run: `go test ./internal/client/ -run TestParse -v`

Expected: All tests pass.

- [ ] **Step 5: Run full suite**

Run: `go test ./... -count=1`

Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/client/ghostty.go internal/client/ghostty_test.go
git commit -m "feat(ghostty): add ref-parsing helpers with unit tests"
```

---

### Task 3: Simple Tab Operations — ListWorkspaces, SelectWorkspace, CloseWorkspace

**Files:**
- Modify: `internal/client/ghostty.go`

- [ ] **Step 1: Replace ListWorkspaces stub**

Replace the `ListWorkspaces` stub with:

```go
func (g *GhosttyClient) ListWorkspaces() ([]WorkspaceInfo, error) {
	out, err := g.runScriptLines(
		`tell application "Ghostty"`,
		`  set tabCount to count of tabs of front window`,
		`  set output to ""`,
		`  repeat with t from 1 to tabCount`,
		`    set tabName to name of tab t of front window`,
		`    set isSel to selected of tab t of front window`,
		`    set output to output & "tab:" & t & "|" & tabName & "|" & isSel & linefeed`,
		`  end repeat`,
		`  return output`,
		`end tell`,
	)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}

	var workspaces []WorkspaceInfo
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			continue
		}
		workspaces = append(workspaces, WorkspaceInfo{
			Ref:      parts[0],
			Title:    parts[1],
			Selected: parts[2] == "true",
		})
	}
	return workspaces, nil
}
```

- [ ] **Step 2: Replace SelectWorkspace stub**

```go
func (g *GhosttyClient) SelectWorkspace(ref string) error {
	tabIdx, err := parseTabIndex(ref)
	if err != nil {
		return err
	}
	_, err = g.runScript(fmt.Sprintf(
		`tell application "Ghostty" to select tab (a reference to tab %d of front window)`,
		tabIdx,
	))
	return err
}
```

- [ ] **Step 3: Replace CloseWorkspace stub**

```go
func (g *GhosttyClient) CloseWorkspace(ref string) error {
	tabIdx, err := parseTabIndex(ref)
	if err != nil {
		return err
	}
	_, err = g.runScript(fmt.Sprintf(
		`tell application "Ghostty" to close tab (a reference to tab %d of front window)`,
		tabIdx,
	))
	return err
}
```

- [ ] **Step 4: Verify it compiles and tests pass**

Run: `go build ./... && go test ./... -count=1`

Expected: All pass.

- [ ] **Step 5: Commit**

```bash
git add internal/client/ghostty.go
git commit -m "feat(ghostty): implement ListWorkspaces, SelectWorkspace, CloseWorkspace"
```

---

### Task 4: SidebarState + Git Info

**Files:**
- Modify: `internal/client/ghostty.go`

- [ ] **Step 1: Replace SidebarState stub**

```go
func (g *GhosttyClient) SidebarState(workspaceRef string) (*SidebarState, error) {
	tabIdx, err := parseTabIndex(workspaceRef)
	if err != nil {
		return nil, err
	}

	// Get CWD of the focused terminal in the tab.
	cwd, err := g.runScriptLines(
		`tell application "Ghostty"`,
		fmt.Sprintf(`  set focTerm to focused terminal of tab %d of front window`, tabIdx),
		fmt.Sprintf(`  set cwd to working directory of focTerm`, ),
		`  return cwd`,
		`end tell`,
	)
	if err != nil {
		return nil, fmt.Errorf("sidebar state: %w", err)
	}

	state := &SidebarState{
		CWD:        cwd,
		FocusedCWD: cwd,
	}

	// Git info: shell out to git (not available from Ghostty API).
	if cwd != "" {
		if branch, err := g.gitBranch(cwd); err == nil {
			state.GitBranch = branch
		}
		state.GitDirty = g.gitDirty(cwd)
	}

	return state, nil
}

func (g *GhosttyClient) gitBranch(cwd string) (string, error) {
	cmd := exec.Command("git", "-C", cwd, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (g *GhosttyClient) gitDirty(cwd string) bool {
	cmd := exec.Command("git", "-C", cwd, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}
```

- [ ] **Step 2: Verify it compiles and tests pass**

Run: `go build ./... && go test ./... -count=1`

Expected: All pass.

- [ ] **Step 3: Commit**

```bash
git add internal/client/ghostty.go
git commit -m "feat(ghostty): implement SidebarState with git info via shell"
```

---

### Task 5: NewWorkspace + RenameWorkspace

**Files:**
- Modify: `internal/client/ghostty.go`

- [ ] **Step 1: Replace NewWorkspace stub**

```go
func (g *GhosttyClient) NewWorkspace(opts NewWorkspaceOpts) (string, error) {
	// Snapshot tab count before creation.
	beforeOut, err := g.runScript(`tell application "Ghostty" to count of tabs of front window`)
	if err != nil {
		return "", fmt.Errorf("count tabs: %w", err)
	}
	beforeCount, _ := strconv.Atoi(beforeOut)

	// Create a new tab, optionally with a working directory.
	if opts.CWD != "" {
		_, err = g.runScriptLines(
			`tell application "Ghostty"`,
			fmt.Sprintf(`  set cfg to new surface configuration from {initial working directory:"%s"}`, opts.CWD),
			`  new tab in front window with configuration cfg`,
			`end tell`,
		)
	} else {
		_, err = g.runScript(`tell application "Ghostty" to new tab in front window`)
	}
	if err != nil {
		return "", fmt.Errorf("new tab: %w", err)
	}

	// Poll for the new tab to appear.
	var ref string
	deadline := time.Now().Add(NewWorkspaceDeadline)
	for time.Now().Before(deadline) {
		afterOut, err := g.runScript(`tell application "Ghostty" to count of tabs of front window`)
		if err != nil {
			time.Sleep(PollInterval)
			continue
		}
		afterCount, _ := strconv.Atoi(afterOut)
		if afterCount > beforeCount {
			ref = fmt.Sprintf("tab:%d", afterCount)
			break
		}
		time.Sleep(PollInterval)
	}
	if ref == "" {
		return "", fmt.Errorf("new tab created but could not determine ref")
	}

	// If a startup command was specified, send it via initial input isn't reliable
	// post-creation, so use Send once the shell is ready.
	if opts.Command != "" {
		// Wait for shell readiness (working directory becomes non-empty).
		g.waitForShellReady(ref)
		_ = g.Send(ref, "", opts.Command+"\\n")
	}

	return ref, nil
}

// waitForShellReady polls until the first terminal in the tab has a non-empty working directory.
func (g *GhosttyClient) waitForShellReady(workspaceRef string) {
	tabIdx, err := parseTabIndex(workspaceRef)
	if err != nil {
		return
	}
	deadline := time.Now().Add(NewWorkspaceDeadline)
	for time.Now().Before(deadline) {
		cwd, err := g.runScript(fmt.Sprintf(
			`tell application "Ghostty" to working directory of terminal 1 of tab %d of front window`,
			tabIdx,
		))
		if err == nil && cwd != "" {
			return
		}
		time.Sleep(PollInterval)
	}
}
```

- [ ] **Step 2: Replace RenameWorkspace stub**

```go
func (g *GhosttyClient) RenameWorkspace(ref, title string) error {
	tabIdx, err := parseTabIndex(ref)
	if err != nil {
		return err
	}
	// Ghostty's set_tab_title requires targeting a terminal in the tab.
	_, err = g.runScript(fmt.Sprintf(
		`tell application "Ghostty" to perform action "set_tab_title:%s" on terminal 1 of tab %d of front window`,
		title, tabIdx,
	))
	return err
}
```

- [ ] **Step 3: Verify it compiles and tests pass**

Run: `go build ./... && go test ./... -count=1`

Expected: All pass.

- [ ] **Step 4: Commit**

```bash
git add internal/client/ghostty.go
git commit -m "feat(ghostty): implement NewWorkspace and RenameWorkspace"
```

---

### Task 6: NewSplit + FocusPane

**Files:**
- Modify: `internal/client/ghostty.go`

- [ ] **Step 1: Replace NewSplit stub**

```go
func (g *GhosttyClient) NewSplit(direction, workspaceRef string) (string, error) {
	tabIdx, err := parseTabIndex(workspaceRef)
	if err != nil {
		return "", fmt.Errorf("parse workspace ref: %w", err)
	}

	// Map cmux direction names to Ghostty.
	// cmux uses "right" and "down"; Ghostty uses same names.
	if direction == "" {
		direction = "right"
	}

	// Snapshot terminal count before split.
	beforeOut, err := g.runScript(fmt.Sprintf(
		`tell application "Ghostty" to count of terminals of tab %d of front window`,
		tabIdx,
	))
	if err != nil {
		return "", fmt.Errorf("count terminals: %w", err)
	}
	beforeCount, _ := strconv.Atoi(beforeOut)

	// Split the focused terminal in this tab.
	// Must select the tab first, then split the focused terminal.
	_, err = g.runScriptLines(
		`tell application "Ghostty"`,
		fmt.Sprintf(`  set focTerm to focused terminal of tab %d of front window`, tabIdx),
		fmt.Sprintf(`  split focTerm direction %s`, direction),
		`end tell`,
	)
	if err != nil {
		return "", fmt.Errorf("split: %w", err)
	}

	// Poll for the new terminal to appear.
	deadline := time.Now().Add(NewSplitDeadline)
	for time.Now().Before(deadline) {
		time.Sleep(PollInterval)
		afterOut, err := g.runScript(fmt.Sprintf(
			`tell application "Ghostty" to count of terminals of tab %d of front window`,
			tabIdx,
		))
		if err != nil {
			continue
		}
		afterCount, _ := strconv.Atoi(afterOut)
		if afterCount > beforeCount {
			ref := fmt.Sprintf("terminal:%d", afterCount)
			return ref, nil
		}
	}
	return "", fmt.Errorf("split created but could not determine new terminal ref")
}
```

- [ ] **Step 2: Replace FocusPane stub**

```go
func (g *GhosttyClient) FocusPane(paneRef, workspaceRef string) error {
	tabIdx, err := parseTabIndex(workspaceRef)
	if err != nil {
		return err
	}
	termIdx, err := parseTerminalIndex(paneRef)
	if err != nil {
		return err
	}
	_, err = g.runScript(fmt.Sprintf(
		`tell application "Ghostty" to focus terminal %d of tab %d of front window`,
		termIdx, tabIdx,
	))
	return err
}
```

- [ ] **Step 3: Verify it compiles and tests pass**

Run: `go build ./... && go test ./... -count=1`

Expected: All pass.

- [ ] **Step 4: Commit**

```bash
git add internal/client/ghostty.go
git commit -m "feat(ghostty): implement NewSplit and FocusPane"
```

---

### Task 7: Send — Text Input with Enter Key

**Files:**
- Modify: `internal/client/ghostty.go`

- [ ] **Step 1: Replace Send stub**

The orchestrators append `"\\n"` (literal backslash-n) to commands before calling Send. The cmux CLI interprets this as a newline. For Ghostty, we use `input text` followed by `send key "enter"`.

```go
func (g *GhosttyClient) Send(workspaceRef, surfaceRef, text string) error {
	tabIdx, err := parseTabIndex(workspaceRef)
	if err != nil {
		return err
	}

	// Default to terminal 1 if no surface ref.
	termIdx := 1
	if surfaceRef != "" {
		termIdx, err = parseTerminalIndex(surfaceRef)
		if err != nil {
			return err
		}
	}

	target := fmt.Sprintf("terminal %d of tab %d of front window", termIdx, tabIdx)

	// Strip trailing literal "\n" — orchestrators append "\\n" for cmux.
	// For Ghostty, we send the text then a separate enter keypress.
	needsEnter := false
	if strings.HasSuffix(text, "\\n") {
		text = strings.TrimSuffix(text, "\\n")
		needsEnter = true
	}

	// Send the text.
	if text != "" {
		_, err = g.runScript(fmt.Sprintf(
			`tell application "Ghostty" to input text %q to %s`,
			text, target,
		))
		if err != nil {
			return fmt.Errorf("input text: %w", err)
		}
	}

	// Press enter if the command had a trailing \n.
	if needsEnter {
		_, err = g.runScript(fmt.Sprintf(
			`tell application "Ghostty" to send key "enter" to %s`,
			target,
		))
		if err != nil {
			return fmt.Errorf("send enter: %w", err)
		}
	}

	return nil
}
```

- [ ] **Step 2: Verify it compiles and tests pass**

Run: `go build ./... && go test ./... -count=1`

Expected: All pass.

- [ ] **Step 3: Commit**

```bash
git add internal/client/ghostty.go
git commit -m "feat(ghostty): implement Send with input text + send key enter"
```

---

### Task 8: Tree — Full Enumeration

**Files:**
- Modify: `internal/client/ghostty.go`

- [ ] **Step 1: Replace Tree stub**

This is the most complex method. Builds a single AppleScript that collects all data into a delimited string, then parses it in Go.

```go
func (g *GhosttyClient) Tree() (*TreeResponse, error) {
	// Single AppleScript that enumerates all windows/tabs/terminals.
	out, err := g.runScriptLines(
		`tell application "Ghostty"`,
		`  set output to ""`,
		`  set winCount to count of windows`,
		`  repeat with w from 1 to winCount`,
		`    set winID to id of window w`,
		`    set tabCount to count of tabs of window w`,
		`    set output to output & "WIN|" & winID & "|" & tabCount & linefeed`,
		`    repeat with t from 1 to tabCount`,
		`      set tabName to name of tab t of window w`,
		`      set isSel to selected of tab t of window w`,
		`      set termCount to count of terminals of tab t of window w`,
		`      set output to output & "TAB|" & t & "|" & tabName & "|" & isSel & "|" & termCount & linefeed`,
		`      repeat with term from 1 to termCount`,
		`        set termCWD to working directory of terminal term of tab t of window w`,
		`        set output to output & "TERM|" & term & "|" & termCWD & linefeed`,
		`      end repeat`,
		`    end repeat`,
		`  end repeat`,
		`  return output`,
		`end tell`,
	)
	if err != nil {
		return nil, fmt.Errorf("tree: %w", err)
	}

	resp := &TreeResponse{}
	var currentWindow *TreeWindow
	var currentWorkspace *TreeWorkspace

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 5)
		if len(parts) < 2 {
			continue
		}

		switch parts[0] {
		case "WIN":
			if currentWorkspace != nil && currentWindow != nil {
				currentWindow.Workspaces = append(currentWindow.Workspaces, *currentWorkspace)
				currentWorkspace = nil
			}
			if currentWindow != nil {
				resp.Windows = append(resp.Windows, *currentWindow)
			}
			tabCount, _ := strconv.Atoi(parts[2])
			currentWindow = &TreeWindow{
				Ref:            parts[1],
				Index:          len(resp.Windows),
				Active:         true,
				Visible:        true,
				Current:        len(resp.Windows) == 0,
				WorkspaceCount: tabCount,
			}
			currentWorkspace = nil

		case "TAB":
			if currentWorkspace != nil && currentWindow != nil {
				currentWindow.Workspaces = append(currentWindow.Workspaces, *currentWorkspace)
			}
			tabIdx, _ := strconv.Atoi(parts[1])
			tabName := parts[2]
			isSel := parts[3] == "true"
			ref := fmt.Sprintf("tab:%d", tabIdx)
			currentWorkspace = &TreeWorkspace{
				Ref:      ref,
				Title:    tabName,
				Index:    tabIdx - 1, // 0-based for model
				Pinned:   false,      // Ghostty has no pin concept
				Active:   isSel,
				Selected: isSel,
			}
			if isSel && currentWindow != nil {
				currentWindow.SelectedWorkspaceRef = ref
			}

		case "TERM":
			if currentWorkspace == nil {
				continue
			}
			termIdx, _ := strconv.Atoi(parts[1])
			termCWD := ""
			if len(parts) > 2 {
				termCWD = parts[2]
			}
			paneRef := fmt.Sprintf("pane:%d", termIdx-1) // 0-based for model
			surfaceRef := fmt.Sprintf("terminal:%d", termIdx)
			pane := TreePane{
				Ref:                paneRef,
				Index:              termIdx - 1,
				Active:             termIdx == 1,
				Focused:            termIdx == 1,
				SurfaceCount:       1,
				SelectedSurfaceRef: surfaceRef,
				SurfaceRefs:        []string{surfaceRef},
				Surfaces: []TreeSurface{
					{
						Ref:            surfaceRef,
						PaneRef:        paneRef,
						Type:           "terminal",
						Title:          termCWD,
						Index:          termIdx - 1,
						IndexInPane:    0,
						Active:         termIdx == 1,
						Focused:        termIdx == 1,
						Selected:       termIdx == 1,
						SelectedInPane: true,
					},
				},
			}
			currentWorkspace.Panes = append(currentWorkspace.Panes, pane)
		}
	}

	// Flush remaining workspace and window.
	if currentWorkspace != nil && currentWindow != nil {
		currentWindow.Workspaces = append(currentWindow.Workspaces, *currentWorkspace)
	}
	if currentWindow != nil {
		resp.Windows = append(resp.Windows, *currentWindow)
	}

	// Set Caller to the selected tab's first terminal in the first window.
	if len(resp.Windows) > 0 {
		for _, ws := range resp.Windows[0].Workspaces {
			if ws.Selected && len(ws.Panes) > 0 {
				resp.Caller = &CallerInfo{
					WorkspaceRef: ws.Ref,
					PaneRef:      ws.Panes[0].Ref,
					WindowRef:    resp.Windows[0].Ref,
					SurfaceRef:   ws.Panes[0].SurfaceRefs[0],
					SurfaceType:  "terminal",
				}
				resp.Active = resp.Caller
				break
			}
		}
	}

	return resp, nil
}
```

- [ ] **Step 2: Verify it compiles and tests pass**

Run: `go build ./... && go test ./... -count=1`

Expected: All pass.

- [ ] **Step 3: Commit**

```bash
git add internal/client/ghostty.go
git commit -m "feat(ghostty): implement Tree via AppleScript enumeration"
```

---

### Task 9: Wire Into newClient() + Integration Tests

**Files:**
- Modify: `cmd/root.go`
- Create: `internal/client/ghostty_integration_test.go`

- [ ] **Step 1: Update newClient() to return GhosttyClient**

In `cmd/root.go`, replace:

```go
	case client.BackendGhostty:
		fmt.Fprintln(os.Stderr, "Ghostty backend not yet implemented")
		os.Exit(1)
		return nil
```

with:

```go
	case client.BackendGhostty:
		return client.NewGhosttyClient()
```

- [ ] **Step 2: Create integration test file**

Create `internal/client/ghostty_integration_test.go`:

```go
//go:build integration && darwin

package client

import "testing"

func TestGhosttyPing_Integration(t *testing.T) {
	gc := NewGhosttyClient()
	if err := gc.Ping(); err != nil {
		t.Skipf("Ghostty not running: %v", err)
	}
}

func TestGhosttyListWorkspaces_Integration(t *testing.T) {
	gc := NewGhosttyClient()
	if err := gc.Ping(); err != nil {
		t.Skipf("Ghostty not running: %v", err)
	}
	ws, err := gc.ListWorkspaces()
	if err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}
	if len(ws) == 0 {
		t.Fatal("expected at least one workspace (tab)")
	}
	for _, w := range ws {
		if w.Ref == "" || w.Title == "" {
			t.Errorf("workspace with empty ref or title: %+v", w)
		}
	}
}

func TestGhosttyTree_Integration(t *testing.T) {
	gc := NewGhosttyClient()
	if err := gc.Ping(); err != nil {
		t.Skipf("Ghostty not running: %v", err)
	}
	tree, err := gc.Tree()
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}
	if len(tree.Windows) == 0 {
		t.Fatal("expected at least one window")
	}
	if len(tree.Windows[0].Workspaces) == 0 {
		t.Fatal("expected at least one workspace in first window")
	}
}

func TestGhosttyPinWorkspace_Integration(t *testing.T) {
	gc := NewGhosttyClient()
	// PinWorkspace is a no-op — should always succeed.
	if err := gc.PinWorkspace("tab:1"); err != nil {
		t.Fatalf("PinWorkspace should be no-op: %v", err)
	}
}
```

- [ ] **Step 3: Verify it compiles and standard tests pass**

Run: `go build ./... && go test ./... -count=1`

Expected: All pass. Integration tests are skipped (no `integration` build tag).

- [ ] **Step 4: Run vet**

Run: `go vet ./...`

Expected: Clean.

- [ ] **Step 5: Commit**

```bash
git add cmd/root.go internal/client/ghostty_integration_test.go
git commit -m "feat(ghostty): wire GhosttyClient into newClient, add integration tests"
```

---

### Task 10: Final Verification

- [ ] **Step 1: Full test suite**

Run: `go test ./... -count=1`

Expected: All tests pass.

- [ ] **Step 2: Vet**

Run: `go vet ./...`

Expected: Clean.

- [ ] **Step 3: Verify all 12 Backend methods are implemented**

Run: `grep -n "func (g \*GhosttyClient)" internal/client/ghostty.go`

Expected: 12 method implementations (Ping, Tree, SidebarState, ListWorkspaces, NewWorkspace, RenameWorkspace, SelectWorkspace, NewSplit, FocusPane, Send, PinWorkspace, CloseWorkspace) plus helpers.

- [ ] **Step 4: Verify no remaining "not yet implemented" stubs**

Run: `grep "not yet implemented" internal/client/ghostty.go`

Expected: Zero results.
