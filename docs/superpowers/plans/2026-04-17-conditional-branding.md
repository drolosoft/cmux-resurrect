# Conditional Branding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove all cmux-resurrect / cmux references from user-facing output when running in Ghostty. Show "crex (cmux-resurrect)" as a legacy title only for cmux users.

**Architecture:** A single `cmd/branding.go` file caches the detected backend once at package init and exposes three helpers: `appTitle()`, `appTagline()`, `isCmuxBranding()`. Subcommand descriptions that currently say "cmux" are made universally neutral (no conditional logic needed — "Save current layout" is accurate for both backends). Only the banner, tagline, root Long description, and quick-start hint need conditional rendering.

**Tech Stack:** Go, Cobra CLI, lipgloss (styling), existing `client.Detect()`.

---

### Task 1: Add branding helpers

**Files:**
- Create: `cmd/branding.go`
- Test: `cmd/branding_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package cmd

import (
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/client"
)

func TestAppTitle_Cmux(t *testing.T) {
	cachedBackend = client.BackendCmux
	defer func() { cachedBackend = client.Detect() }()

	got := appTitle()
	if got != "crex (cmux-resurrect)" {
		t.Errorf("appTitle() = %q, want %q", got, "crex (cmux-resurrect)")
	}
}

func TestAppTitle_Ghostty(t *testing.T) {
	cachedBackend = client.BackendGhostty
	defer func() { cachedBackend = client.Detect() }()

	got := appTitle()
	if got != "crex" {
		t.Errorf("appTitle() = %q, want %q", got, "crex")
	}
}

func TestAppTitle_Unknown(t *testing.T) {
	cachedBackend = client.BackendUnknown
	defer func() { cachedBackend = client.Detect() }()

	got := appTitle()
	if got != "crex" {
		t.Errorf("appTitle() = %q, want %q", got, "crex")
	}
}

func TestAppTagline_Cmux(t *testing.T) {
	cachedBackend = client.BackendCmux
	defer func() { cachedBackend = client.Detect() }()

	got := appTagline()
	if got != "Terminal workspace manager for cmux and Ghostty \u2014 your sessions, resurrected." {
		t.Errorf("appTagline() = %q", got)
	}
}

func TestAppTagline_Ghostty(t *testing.T) {
	cachedBackend = client.BackendGhostty
	defer func() { cachedBackend = client.Detect() }()

	got := appTagline()
	if got != "Terminal workspace manager for Ghostty \u2014 your sessions, resurrected." {
		t.Errorf("appTagline() = %q", got)
	}
}

func TestIsCmuxBranding(t *testing.T) {
	cachedBackend = client.BackendCmux
	defer func() { cachedBackend = client.Detect() }()

	if !isCmuxBranding() {
		t.Error("isCmuxBranding() should be true for cmux")
	}

	cachedBackend = client.BackendGhostty
	if isCmuxBranding() {
		t.Error("isCmuxBranding() should be false for Ghostty")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/ -run TestAppTitle -v && go test ./cmd/ -run TestAppTagline -v && go test ./cmd/ -run TestIsCmuxBranding -v`
Expected: FAIL — functions not defined.

- [ ] **Step 3: Implement branding helpers**

```go
package cmd

import "github.com/drolosoft/cmux-resurrect/internal/client"

// cachedBackend stores the detected backend, evaluated once at package init.
// Exported to tests via direct assignment for deterministic assertions.
var cachedBackend = client.Detect()

// appTitle returns the application title appropriate for the active backend.
// cmux users see the legacy "crex (cmux-resurrect)"; everyone else sees "crex".
func appTitle() string {
	if cachedBackend == client.BackendCmux {
		return "crex (cmux-resurrect)"
	}
	return "crex"
}

// appTagline returns the tagline appropriate for the active backend.
func appTagline() string {
	if cachedBackend == client.BackendCmux {
		return "Terminal workspace manager for cmux and Ghostty \u2014 your sessions, resurrected."
	}
	return "Terminal workspace manager for Ghostty \u2014 your sessions, resurrected."
}

// isCmuxBranding returns true when cmux-specific branding should be shown.
func isCmuxBranding() bool {
	return cachedBackend == client.BackendCmux
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/ -run "TestAppTitle|TestAppTagline|TestIsCmuxBranding" -v`
Expected: PASS (6 tests).

- [ ] **Step 5: Commit**

```bash
git add cmd/branding.go cmd/branding_test.go
git commit -m "feat: add conditional branding helpers based on detected backend"
```

---

### Task 2: Conditional banner and styled help

**Files:**
- Modify: `cmd/style.go:43-66` (banner function)
- Modify: `cmd/style.go:70-108` (styledHelp function)

- [ ] **Step 1: Write the failing tests**

```go
// Add to cmd/branding_test.go

func TestBanner_Cmux_ShowsFullName(t *testing.T) {
	cachedBackend = client.BackendCmux
	defer func() { cachedBackend = client.Detect() }()

	out := banner()
	if !strings.Contains(out, "cmux") {
		t.Error("cmux banner should contain 'cmux'")
	}
	if !strings.Contains(out, "resurrect") {
		t.Error("cmux banner should contain 'resurrect'")
	}
}

func TestBanner_Ghostty_NoCmux(t *testing.T) {
	cachedBackend = client.BackendGhostty
	defer func() { cachedBackend = client.Detect() }()

	out := banner()
	if strings.Contains(out, "cmux") {
		t.Error("Ghostty banner should not contain 'cmux'")
	}
}

func TestStyledHelp_Cmux_ShowsLegacyHint(t *testing.T) {
	cachedBackend = client.BackendCmux
	defer func() { cachedBackend = client.Detect() }()

	out := styledHelp()
	if !strings.Contains(out, "cmux-resurrect") {
		t.Error("cmux help should show legacy name hint")
	}
}

func TestStyledHelp_Ghostty_NoLegacyHint(t *testing.T) {
	cachedBackend = client.BackendGhostty
	defer func() { cachedBackend = client.Detect() }()

	out := styledHelp()
	if strings.Contains(out, "cmux-resurrect") {
		t.Error("Ghostty help should not mention cmux-resurrect")
	}
}
```

Note: add `"strings"` to the imports in `branding_test.go`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/ -run "TestBanner|TestStyledHelp" -v`
Expected: FAIL — banner still hardcoded, styled help still shows cmux-resurrect.

- [ ] **Step 3: Update banner() to use conditional art and tagline**

In `cmd/style.go`, replace the `banner()` function. For cmux: keep the current cmux-resurrect ASCII art. For Ghostty/unknown: use a crex-only ASCII art without the cmux prefix. Use `appTagline()` for the tagline line.

cmux banner (existing art, keep as-is):
```
                                                                        _
  ___ _ __ ___  _   ___  __     _ __ ___  ___ _   _ _ __ _ __ ___  ___| |_
 / __| '_ ` _ \| | | \ \/ /____| '__/ _ \/ __| | | | '__| '__/ _ \/ __| __|
| (__| | | | | | |_| |>  <_____| | |  __/\__ \ |_| | |  | | |  __/ (__| |_
 \___|_| |_| |_|\__,_/_/\_\    |_|  \___||___/\__,_|_|  |_|  \___|\___|\__|
```

Ghostty/unknown banner (crex only):
```
                     
  ___ _ __ _____  __
 / __| '__/ _ \ \/ /
| (__| | |  __/>  < 
 \___|_|  \___/_/\_\
```

Replace the tagline line:
```go
b.WriteString(tagStyle.Render("  " + appTagline()))
```

- [ ] **Step 4: Update styledHelp() to conditionally show the legacy name hint**

In `cmd/style.go`, wrap the quick-start hint block (lines 89-96) with `if isCmuxBranding()`:

```go
b.WriteString("\n")
b.WriteString(dimStyle.Render("  Quick start:"))
b.WriteString("\n")
if isCmuxBranding() {
	fmt.Fprintf(&b, "    %s%s%s%s%s\n",
		dimStyle.Render("("),
		greenStyle.Render("crex"),
		dimStyle.Render(" is the short name for "),
		greenStyle.Render("cmux-resurrect"),
		dimStyle.Render(")"))
	b.WriteString("\n")
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./cmd/ -run "TestBanner|TestStyledHelp" -v`
Expected: PASS (4 tests).

- [ ] **Step 6: Commit**

```bash
git add cmd/style.go cmd/branding_test.go
git commit -m "feat: conditional banner and help text based on detected backend"
```

---

### Task 3: Update root command Long description

**Files:**
- Modify: `cmd/root.go:20-28`

- [ ] **Step 1: Write the failing test**

```go
// Add to cmd/branding_test.go

func TestRootLongDescription_Ghostty(t *testing.T) {
	cachedBackend = client.BackendGhostty
	defer func() { cachedBackend = client.Detect() }()

	// Re-initialize the root command Long description.
	updateRootLong()

	if strings.Contains(rootCmd.Long, "cmux-resurrect") {
		t.Error("Ghostty root Long should not mention cmux-resurrect")
	}
}

func TestRootLongDescription_Cmux(t *testing.T) {
	cachedBackend = client.BackendCmux
	defer func() { cachedBackend = client.Detect() }()

	updateRootLong()

	if !strings.Contains(rootCmd.Long, "cmux-resurrect") {
		t.Error("cmux root Long should mention cmux-resurrect")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/ -run TestRootLongDescription -v`
Expected: FAIL — `updateRootLong` not defined.

- [ ] **Step 3: Add updateRootLong() and call it from init**

In `cmd/root.go`, add a function and call it from the `init()`:

```go
func updateRootLong() {
	if isCmuxBranding() {
		rootCmd.Long = "crex (cmux-resurrect) saves, restores, and templates your terminal workspaces.\nWorks with cmux and Ghostty. Inspired by tmux-resurrect."
	} else {
		rootCmd.Long = "crex saves, restores, and templates your terminal workspaces.\nWorks with Ghostty. Inspired by tmux-resurrect."
	}
}
```

Call `updateRootLong()` at the end of `init()` in `cmd/root.go`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/ -run TestRootLongDescription -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/root.go cmd/branding_test.go
git commit -m "feat: conditional root command Long description"
```

---

### Task 4: Neutralize subcommand descriptions

Remove "cmux" from all subcommand Short/Long descriptions, making them universally accurate for both backends. No conditional logic needed — just neutral language.

**Files:**
- Modify: `cmd/save.go:15-16`
- Modify: `cmd/restore.go:18-19`
- Modify: `cmd/watch.go:16-17` and line 60
- Modify: `cmd/export_to_md.go:14-16`
- Modify: `cmd/import_from_md.go:16-18`
- Modify: `cmd/template_use.go:24-25`
- Modify: `internal/persist/store.go:85`

- [ ] **Step 1: Write the failing test**

```go
// Add to cmd/branding_test.go

func TestSubcommandDescriptions_NoCmuxMention(t *testing.T) {
	// Subcommand descriptions should be backend-neutral.
	cmds := []*cobra.Command{
		saveCmd, restoreCmd, watchCmd,
		exportToMDCmd, importFromMDCmd, templateUseCmd,
	}

	for _, c := range cmds {
		if strings.Contains(strings.ToLower(c.Short), "cmux") {
			t.Errorf("%s Short contains 'cmux': %q", c.Name(), c.Short)
		}
		if strings.Contains(strings.ToLower(c.Long), "cmux") {
			t.Errorf("%s Long contains 'cmux': %q", c.Name(), c.Long)
		}
	}
}
```

Note: add `"github.com/spf13/cobra"` to imports in `branding_test.go`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run TestSubcommandDescriptions -v`
Expected: FAIL — multiple commands still reference "cmux".

- [ ] **Step 3: Update all subcommand descriptions**

`cmd/save.go`:
```go
Short: "Save current layout",
Long:  "Captures all workspaces, splits, CWDs, and pinned state from the running terminal.",
```

`cmd/restore.go`:
```go
Short: "Restore a saved layout",
```
(Long description already doesn't mention cmux.)

`cmd/watch.go`:
```go
Long: "Watches terminal state and saves it periodically. Deduplicates via content hash.",
```
And line 60:
```go
fmt.Fprintf(os.Stderr, "Watching terminal state, saving as %q every %s\n", name, interval)
```

`cmd/export_to_md.go`:
```go
Short: "Export live state to a Workspace Blueprint",
Long:  "Captures current workspaces and writes them to a Workspace Blueprint (.md) with default templates.",
```

`cmd/import_from_md.go`:
```go
Short: "Create workspaces from a Workspace Blueprint",
Long:  "Reads a Workspace Blueprint (.md), resolves templates, and creates any workspaces that don't already exist.",
```

`cmd/template_use.go`:
```go
Short: "Create a workspace from a gallery template",
Long:  "Creates a new workspace using a gallery template's layout and commands.\n\nThe first argument is the template name (e.g., cols, claude, ide).\nThe optional second argument is the working directory (defaults to \".\").",
```

`internal/persist/store.go` line 85:
```go
header := fmt.Sprintf("# crex layout: %s\n# Saved at: %s\n\n",
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/ -run TestSubcommandDescriptions -v`
Expected: PASS.

- [ ] **Step 5: Run the full test suite**

Run: `go test ./... -v`
Expected: All packages PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/save.go cmd/restore.go cmd/watch.go cmd/export_to_md.go cmd/import_from_md.go cmd/template_use.go internal/persist/store.go cmd/branding_test.go
git commit -m "refactor: neutralize subcommand descriptions — remove cmux-specific language"
```

---

### Task 5: Verify end-to-end and update goreleaser

**Files:**
- Modify: `.goreleaser.yml:61`

- [ ] **Step 1: Build and verify Ghostty branding (manual check)**

```bash
go build -o crex ./cmd/crex
./crex
```

When not inside cmux (no `CMUX_SOCKET_PATH` or `CMUX_WORKSPACE_ID`): banner should show "crex" art, tagline should mention Ghostty only, no "cmux-resurrect" anywhere.

```bash
CMUX_SOCKET_PATH=/tmp/fake.sock ./crex
```

With cmux env: banner should show full cmux-resurrect art, tagline mentions both, quick-start shows "(crex is the short name for cmux-resurrect)".

- [ ] **Step 2: Update goreleaser description**

`.goreleaser.yml` line 61, change description to:
```yaml
description: "Terminal workspace manager — save, restore, and template your workspaces"
```

This is build-time/package-manager text — keep it neutral since it's seen by all users.

- [ ] **Step 3: Run full test suite one last time**

Run: `go test ./...`
Expected: All packages PASS.

- [ ] **Step 4: Commit**

```bash
git add .goreleaser.yml
git commit -m "chore: neutralize goreleaser description for multi-backend"
```
