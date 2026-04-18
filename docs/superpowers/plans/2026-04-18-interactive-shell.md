# Interactive Shell & Terminology Fixes — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace crex's flat TUI with an nb-style interactive shell (REPL), ship backend-adaptive terminology, and rename the `workspace` subcommand to `blueprint`.

**Architecture:** The interactive shell is a Bubble Tea program running inline (no AltScreen) with three modes: prompt, browse, and confirm. It reuses all existing orchestrate/client/persist packages — no new business logic. Priority 0 terminology fixes (unitName, blueprint rename, splits wording) ship alongside as prerequisites.

**Tech Stack:** Go, Bubble Tea, Lip Gloss (AdaptiveColor), Bubbles textinput, existing internal packages (client, orchestrate, persist, gallery, mdfile)

---

### Task 1: Add `unitName()` to branding (Priority 0a)

**Files:**
- Modify: `cmd/branding.go`
- Test: `cmd/branding_test.go` (create)

- [ ] **Step 1: Write the failing tests**

Create `cmd/branding_test.go`:

```go
package cmd

import (
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/client"
)

func TestUnitName_Ghostty_Singular(t *testing.T) {
	orig := cachedBackend
	cachedBackend = client.BackendGhostty
	defer func() { cachedBackend = orig }()

	if got := unitName(1); got != "tab" {
		t.Errorf("unitName(1) with Ghostty = %q, want %q", got, "tab")
	}
}

func TestUnitName_Ghostty_Plural(t *testing.T) {
	orig := cachedBackend
	cachedBackend = client.BackendGhostty
	defer func() { cachedBackend = orig }()

	if got := unitName(3); got != "tabs" {
		t.Errorf("unitName(3) with Ghostty = %q, want %q", got, "tabs")
	}
}

func TestUnitName_Cmux_Singular(t *testing.T) {
	orig := cachedBackend
	cachedBackend = client.BackendCmux
	defer func() { cachedBackend = orig }()

	if got := unitName(1); got != "workspace" {
		t.Errorf("unitName(1) with cmux = %q, want %q", got, "workspace")
	}
}

func TestUnitName_Cmux_Plural(t *testing.T) {
	orig := cachedBackend
	cachedBackend = client.BackendCmux
	defer func() { cachedBackend = orig }()

	if got := unitName(3); got != "workspaces" {
		t.Errorf("unitName(3) with cmux = %q, want %q", got, "workspaces")
	}
}

func TestUnitName_Unknown_DefaultsToCmux(t *testing.T) {
	orig := cachedBackend
	cachedBackend = client.BackendUnknown
	defer func() { cachedBackend = orig }()

	if got := unitName(2); got != "workspaces" {
		t.Errorf("unitName(2) with unknown = %q, want %q", got, "workspaces")
	}
}

func TestUnitNameCapitalized(t *testing.T) {
	orig := cachedBackend
	cachedBackend = client.BackendGhostty
	defer func() { cachedBackend = orig }()

	if got := unitNameCap(1); got != "Tab" {
		t.Errorf("unitNameCap(1) with Ghostty = %q, want %q", got, "Tab")
	}
	if got := unitNameCap(3); got != "Tabs" {
		t.Errorf("unitNameCap(3) with Ghostty = %q, want %q", got, "Tabs")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -count=1 -run TestUnitName ./cmd/`
Expected: FAIL — `unitName` undefined

- [ ] **Step 3: Implement `unitName()` and `unitNameCap()`**

Add to `cmd/branding.go` after the existing functions:

```go
// unitName returns the backend-adaptive label for a terminal tab/workspace.
// Ghostty users see "tab(s)", cmux users see "workspace(s)".
func unitName(count int) string {
	if cachedBackend == client.BackendGhostty {
		if count == 1 {
			return "tab"
		}
		return "tabs"
	}
	if count == 1 {
		return "workspace"
	}
	return "workspaces"
}

// unitNameCap returns unitName with the first letter capitalized.
func unitNameCap(count int) string {
	s := unitName(count)
	return strings.ToUpper(s[:1]) + s[1:]
}
```

Add `"strings"` to the imports in `cmd/branding.go`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -count=1 -run TestUnitName ./cmd/`
Expected: PASS (6 tests)

- [ ] **Step 5: Commit**

```bash
git add cmd/branding.go cmd/branding_test.go
git commit -m "feat: add unitName() for backend-adaptive labels (Priority 0a)"
```

---

### Task 2: Apply `unitName()` across all cmd files (Priority 0a continued)

**Files:**
- Modify: `cmd/save.go`
- Modify: `cmd/restore.go`
- Modify: `cmd/list.go`
- Modify: `cmd/show.go`
- Modify: `cmd/import_from_md.go`
- Modify: `cmd/export_to_md.go`
- Modify: `cmd/watch.go`
- Modify: `cmd/tui.go`
- Modify: `cmd/completion_helpers.go`

- [ ] **Step 1: Update `cmd/save.go`**

Line 18 — change Long description:
```go
Long:  "Captures all tabs, pane arrangements, CWDs, and pinned state from the running terminal.",
```

Line 68 — change the summary line:
```go
fmt.Fprintf(os.Stderr, "%s\n",
    greenStyle.Render(fmt.Sprintf("✅ Saved %d %s to %s", len(layout.Workspaces), unitName(len(layout.Workspaces)), store.Path(name))))
```

- [ ] **Step 2: Update `cmd/restore.go`**

Line 19 — change Long description:
```go
Long:  "Recreates tabs, pane arrangements, and sends commands from a saved layout.\n\nYou will be asked whether to replace your current tabs or add to them.\nUse --mode to skip the interactive prompt (useful for scripts).\n\nIf no layout name is given, an interactive picker is shown.",
```

Line 150 — change the commands summary:
```go
greenStyle.Render(fmt.Sprintf("✅ %d commands for %d %s", len(result.Commands)-countBlanks(result.Commands), result.WorkspacesTotal, unitName(result.WorkspacesTotal))))
```

Line 156 — change closed message:
```go
fmt.Fprintf(os.Stderr, "%s\n", dimStyle.Render(fmt.Sprintf("  Closed %d existing %s", result.WorkspacesClosed, unitName(result.WorkspacesClosed))))
```

Line 159 — change restored summary:
```go
greenStyle.Render(fmt.Sprintf("✅ Restored %d/%d %s", result.WorkspacesOK, result.WorkspacesTotal, unitName(result.WorkspacesTotal))))
```

Line 185-186 — change restore mode descriptions:
```go
fmt.Fprintf(os.Stderr, "  %s  %s\n", cyanStyle.Render("[r]"), fmt.Sprintf("Replace — close all current %s, then restore", unitName(2)))
fmt.Fprintf(os.Stderr, "  %s  %s\n", cyanStyle.Render("[a]"), fmt.Sprintf("Add     — keep current %s, add restored ones", unitName(2)))
```

- [ ] **Step 3: Update `cmd/list.go`**

Line 44 — change workspace count:
```go
ws := cyanStyle.Render(fmt.Sprintf("%d %s", m.WorkspaceCount, unitName(m.WorkspaceCount)))
```

- [ ] **Step 4: Update `cmd/show.go`**

Line 52 — change the header line:
```go
fmt.Fprintf(os.Stderr, "   %s\n", dimStyle.Render(fmt.Sprintf("Saved %s · %d %s", saved, len(layout.Workspaces), unitName(len(layout.Workspaces)))))
```

- [ ] **Step 5: Update `cmd/import_from_md.go`**

Line 17 — change Short:
```go
Short: "Create tabs from a Blueprint",
```

Line 18 — change Long:
```go
Long:  "Reads a Blueprint (.md), resolves templates, and creates any tabs that don't already exist.",
```

Line 41 — change empty message:
```go
fmt.Fprintln(os.Stderr, dimStyle.Render("  No enabled entries in Blueprint."))
```

Line 91 — change dry-run summary:
```go
greenStyle.Render(fmt.Sprintf("✅ Would create %d %s", result.Created, unitName(result.Created))))
```

Line 95 — change import summary:
```go
greenStyle.Render(fmt.Sprintf("✅ Import complete: %d created, %d skipped", result.Created, result.Skipped)))
```

- [ ] **Step 6: Update `cmd/export_to_md.go`**

Line 16 — change Short:
```go
Short: "Export live state to a Blueprint",
```

Line 17 — change Long:
```go
Long:  "Captures current tabs and writes them to a Blueprint (.md) with default templates.",
```

Line 39 — change summary:
```go
fmt.Fprintf(os.Stderr, "Exported %d %s to %s\n", len(wf.Projects), unitName(len(wf.Projects)), wsFile)
```

- [ ] **Step 7: Update `cmd/completion_helpers.go`**

Line 34 — change description fallback:
```go
desc = fmt.Sprintf("%d %s", m.WorkspaceCount, unitName(m.WorkspaceCount))
```

- [ ] **Step 8: Update `cmd/tui.go`**

Line 98 — change restored summary:
```go
greenStyle.Render(fmt.Sprintf("Restored %d/%d %s", result.WorkspacesOK, result.WorkspacesTotal, unitName(result.WorkspacesTotal))))
```

Line 143 — change saved summary:
```go
greenStyle.Render(fmt.Sprintf("Saved %d %s to %s", len(layout.Workspaces), unitName(len(layout.Workspaces)), store.Path("default"))))
```

- [ ] **Step 9: Run all tests to verify no regressions**

Run: `go test -count=1 ./...`
Expected: All tests PASS

- [ ] **Step 10: Commit**

```bash
git add cmd/save.go cmd/restore.go cmd/list.go cmd/show.go cmd/import_from_md.go cmd/export_to_md.go cmd/watch.go cmd/tui.go cmd/completion_helpers.go
git commit -m "feat: apply unitName() across all user-facing output (Priority 0a)"
```

---

### Task 3: Rename `workspace` subcommand to `blueprint` (Priority 0b)

**Files:**
- Delete: `cmd/ws.go`, `cmd/ws_add.go`, `cmd/ws_remove.go`, `cmd/ws_list.go`, `cmd/ws_toggle.go`
- Create: `cmd/blueprint.go`, `cmd/blueprint_add.go`, `cmd/blueprint_remove.go`, `cmd/blueprint_list.go`, `cmd/blueprint_toggle.go`
- Modify: `cmd/style.go` (update `styledHelp`)

- [ ] **Step 1: Create `cmd/blueprint.go`**

```go
package cmd

import (
	"github.com/spf13/cobra"
)

var blueprintCmd = &cobra.Command{
	Use:     "blueprint",
	Short:   "Manage entries in the Blueprint",
	Long:    "Add, remove, list, and toggle entries in the Blueprint (.md).",
	Aliases: []string{"bp"},
}

// workspaceLegacyCmd is a hidden alias so existing scripts using "crex workspace" still work.
var workspaceLegacyCmd = &cobra.Command{
	Use:     "workspace",
	Short:   "Manage entries in the Blueprint",
	Long:    "Add, remove, list, and toggle entries in the Blueprint (.md).",
	Hidden:  true,
	Aliases: []string{"ws"},
}

func init() {
	rootCmd.AddCommand(blueprintCmd)
	rootCmd.AddCommand(workspaceLegacyCmd)
}
```

- [ ] **Step 2: Create `cmd/blueprint_add.go`**

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/drolosoft/cmux-resurrect/internal/mdfile"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/spf13/cobra"
)

var (
	bpAddIcon     string
	bpAddTemplate string
	bpAddPin      bool
	bpAddDisabled bool
)

var blueprintAddCmd = &cobra.Command{
	Use:   "add <name> <path>",
	Short: "Add an entry to the Blueprint",
	Args:  cobra.ExactArgs(2),
	RunE:  runBlueprintAdd,
}

func init() {
	blueprintAddCmd.Flags().StringVarP(&bpAddIcon, "icon", "i", "📁", "entry icon emoji")
	blueprintAddCmd.Flags().StringVarP(&bpAddTemplate, "template", "t", "dev", "template name (run 'crex template list' for options)")
	blueprintAddCmd.Flags().BoolVar(&bpAddPin, "pin", true, "pin in sidebar")
	blueprintAddCmd.Flags().BoolVar(&bpAddDisabled, "disabled", false, "add as disabled (unchecked)")
	_ = blueprintAddCmd.RegisterFlagCompletionFunc("template", completeTemplateNames)
	blueprintAddCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			return nil, cobra.ShellCompDirectiveNoFileComp
		case 1:
			return nil, cobra.ShellCompDirectiveFilterDirs
		default:
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}
	blueprintCmd.AddCommand(blueprintAddCmd)
	workspaceLegacyCmd.AddCommand(&cobra.Command{
		Use:  "add <name> <path>",
		Args: cobra.ExactArgs(2),
		RunE: runBlueprintAdd,
		Hidden: true,
	})
}

func runBlueprintAdd(cmd *cobra.Command, args []string) error {
	name := args[0]
	path := args[1]

	p := model.Project{
		Enabled:  !bpAddDisabled,
		Icon:     bpAddIcon,
		Name:     name,
		Template: bpAddTemplate,
		Pin:      bpAddPin,
		Path:     path,
	}

	wsFile := cfg.WorkspaceFile
	if err := mdfile.AddProject(wsFile, p); err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr)
	check := greenStyle.Render("✅")
	if bpAddDisabled {
		check = dimStyle.Render("⬜")
	}
	fmt.Fprintf(os.Stderr, "  %s %s %s  %s  %s\n",
		check,
		p.Icon,
		greenStyle.Render(p.Name),
		cyanStyle.Render("template="+p.Template),
		dimStyle.Render(path))
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "%s\n\n",
		greenStyle.Render("✅ Added to Blueprint"))
	return nil
}
```

- [ ] **Step 3: Create `cmd/blueprint_remove.go`**

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/drolosoft/cmux-resurrect/internal/mdfile"
	"github.com/spf13/cobra"
)

var blueprintRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Short:   "Remove an entry from the Blueprint",
	Aliases: []string{"rm"},
	Args:    cobra.ExactArgs(1),
	RunE:    runBlueprintRemove,
}

func init() {
	blueprintRemoveCmd.ValidArgsFunction = completeBlueprintNames
	blueprintCmd.AddCommand(blueprintRemoveCmd)
	workspaceLegacyCmd.AddCommand(&cobra.Command{
		Use:    "remove <name>",
		Args:   cobra.ExactArgs(1),
		RunE:   runBlueprintRemove,
		Hidden: true,
	})
}

func runBlueprintRemove(cmd *cobra.Command, args []string) error {
	name := args[0]
	wsFile := cfg.WorkspaceFile

	if err := mdfile.RemoveProject(wsFile, name); err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "%s\n\n",
		greenStyle.Render(fmt.Sprintf("✅ Removed %q from Blueprint", name)))
	return nil
}
```

- [ ] **Step 4: Create `cmd/blueprint_list.go`**

```go
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/drolosoft/cmux-resurrect/internal/mdfile"
	"github.com/spf13/cobra"
)

var bpListAll bool

var blueprintListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List entries from the Blueprint",
	Aliases: []string{"ls"},
	Args:    cobra.NoArgs,
	RunE:    runBlueprintList,
}

func init() {
	blueprintListCmd.Flags().BoolVarP(&bpListAll, "all", "a", false, "show disabled entries too")
	blueprintCmd.AddCommand(blueprintListCmd)
	workspaceLegacyCmd.AddCommand(&cobra.Command{
		Use:    "list",
		Args:   cobra.NoArgs,
		RunE:   runBlueprintList,
		Hidden: true,
	})
}

func runBlueprintList(cmd *cobra.Command, args []string) error {
	wsFile := cfg.WorkspaceFile
	wf, err := mdfile.Parse(wsFile)
	if err != nil {
		return fmt.Errorf("read Blueprint: %w", err)
	}

	fmt.Fprintln(os.Stderr, headingStyle.Render("📝 Blueprint"))
	fmt.Fprintln(os.Stderr)

	enabled := 0
	disabled := 0
	shown := 0

	for _, p := range wf.Projects {
		if !bpListAll && !p.Enabled {
			disabled++
			continue
		}

		check := greenStyle.Render("✅")
		if !p.Enabled {
			check = dimStyle.Render("⬜")
			disabled++
		} else {
			enabled++
		}

		name := greenStyle.Render(fmt.Sprintf("%-14s", p.Name))
		tmpl := cyanStyle.Render(fmt.Sprintf("%-10s", p.Template))
		path := dimStyle.Render(p.Path)

		pin := ""
		if p.Pin {
			pin = " 📌"
		}

		icon := p.Icon
		if strings.Contains(icon, "\uFE0F") {
			icon += " "
		}
		fmt.Fprintf(os.Stderr, "  %s %s %s %s %s%s\n", check, icon, name, tmpl, path, pin)
		shown++
	}

	if shown == 0 {
		fmt.Fprintln(os.Stderr, dimStyle.Render("  No Blueprint entries found."))
	}

	fmt.Fprintln(os.Stderr)
	total := enabled + disabled
	if bpListAll && disabled > 0 {
		fmt.Fprintln(os.Stderr, dimStyle.Render(fmt.Sprintf("  %d entries (%d enabled, %d disabled)", total, enabled, disabled)))
	} else {
		fmt.Fprintln(os.Stderr, dimStyle.Render(fmt.Sprintf("  %d entries", shown)))
	}
	return nil
}
```

- [ ] **Step 5: Create `cmd/blueprint_toggle.go`**

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/drolosoft/cmux-resurrect/internal/mdfile"
	"github.com/spf13/cobra"
)

var blueprintToggleCmd = &cobra.Command{
	Use:   "toggle <name>",
	Short: "Toggle an entry between enabled and disabled",
	Args:  cobra.ExactArgs(1),
	RunE:  runBlueprintToggle,
}

func init() {
	blueprintToggleCmd.ValidArgsFunction = completeBlueprintNames
	blueprintCmd.AddCommand(blueprintToggleCmd)
	workspaceLegacyCmd.AddCommand(&cobra.Command{
		Use:    "toggle <name>",
		Args:   cobra.ExactArgs(1),
		RunE:   runBlueprintToggle,
		Hidden: true,
	})
}

func runBlueprintToggle(cmd *cobra.Command, args []string) error {
	name := args[0]
	wsFile := cfg.WorkspaceFile

	newState, err := mdfile.ToggleProject(wsFile, name)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr)
	if newState {
		fmt.Fprintf(os.Stderr, "  %s %s\n", greenStyle.Render("✅"), greenStyle.Render(name+" enabled"))
	} else {
		fmt.Fprintf(os.Stderr, "  %s %s\n", dimStyle.Render("⬜"), dimStyle.Render(name+" disabled"))
	}
	fmt.Fprintln(os.Stderr)
	return nil
}
```

- [ ] **Step 6: Rename completion helper**

In `cmd/completion_helpers.go`, rename the function `completeWorkspaceNames` to `completeBlueprintNames` and update the comment:

```go
// completeBlueprintNames provides dynamic completion of project names
// from the Blueprint.
// Used by: blueprint remove, blueprint toggle.
func completeBlueprintNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
```

- [ ] **Step 7: Delete old ws files**

```bash
git rm cmd/ws.go cmd/ws_add.go cmd/ws_remove.go cmd/ws_list.go cmd/ws_toggle.go
```

- [ ] **Step 8: Update `styledHelp()` in `cmd/style.go`**

Change line 220 in the help output:
```go
helpCmd(&b, "blueprint", "<cmd>", "Manage Blueprint (add|remove|list|toggle)")
```

And update the example:
```go
helpExample(&b, "crex blueprint add notes ~/docs", "add entry to Blueprint")
```

- [ ] **Step 9: Run all tests**

Run: `go test -count=1 ./...`
Expected: All tests PASS. The old ws tests (if any) should have been removed with the ws files.

- [ ] **Step 10: Commit**

```bash
git add cmd/blueprint.go cmd/blueprint_add.go cmd/blueprint_remove.go cmd/blueprint_list.go cmd/blueprint_toggle.go cmd/completion_helpers.go cmd/style.go
git commit -m "feat: rename workspace subcommand to blueprint with bp alias (Priority 0b)"
```

---

### Task 4: Fix "splits" language (Priority 0c)

**Files:**
- Modify: `README.md`
- Modify: `cmd/save.go` (already partly done in Task 2)
- Modify: `cmd/restore.go` (already partly done in Task 2)

- [ ] **Step 1: Update README tagline**

In `README.md`, line 18, change:
```
crex saves your entire layout and brings it back: workspaces, splits, CWDs, pinned state, startup commands, everything.
```
To:
```
crex saves your entire layout and brings it back: all your tabs, pane arrangements, working directories, pinned state, and startup commands.
```

- [ ] **Step 2: Verify save.go and restore.go Long descriptions were updated in Task 2**

The Long descriptions should already say "tabs, pane arrangements" from Task 2. Verify by reading the files.

- [ ] **Step 3: Run all tests**

Run: `go test -count=1 ./...`
Expected: All tests PASS

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: fix splits language to pane arrangements (Priority 0c)"
```

---

### Task 5: Shell styles and help renderer

**Files:**
- Create: `internal/tui/shell_styles.go`
- Create: `internal/tui/shell_help.go`
- Create: `internal/tui/shell_help_test.go`

- [ ] **Step 1: Write failing test for help rendering**

Create `internal/tui/shell_help_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/client"
)

func TestRenderHelp_ContainsAllGroups(t *testing.T) {
	help := renderHelp(client.BackendGhostty)
	groups := []string{"Live", "Layouts", "Templates", "Blueprint", "Shell"}
	for _, g := range groups {
		if !strings.Contains(help, g) {
			t.Errorf("help output missing group %q", g)
		}
	}
}

func TestRenderHelp_ContainsAllCommands(t *testing.T) {
	help := renderHelp(client.BackendGhostty)
	commands := []string{"now", "watch", "ls", "restore", "save", "delete", "templates", "use", "bp add", "bp list", "bp remove", "bp toggle", "help", "exit"}
	for _, cmd := range commands {
		if !strings.Contains(help, cmd) {
			t.Errorf("help output missing command %q", cmd)
		}
	}
}

func TestRenderHelp_ContainsIcons(t *testing.T) {
	help := renderHelp(client.BackendGhostty)
	icons := []string{"🖥", "⏱", "📋", "🔄", "💾", "🗑", "📦", "🚀", "📐", "❓", "👋"}
	for _, icon := range icons {
		if !strings.Contains(help, icon) {
			t.Errorf("help output missing icon %q", icon)
		}
	}
}

func TestRenderHelp_GhosttyShowsTabs(t *testing.T) {
	help := renderHelp(client.BackendGhostty)
	if !strings.Contains(help, "current tabs") || !strings.Contains(help, "Show current tabs") {
		t.Error("Ghostty help should say 'tabs', not 'workspaces'")
	}
}

func TestRenderHelp_CmuxShowsWorkspaces(t *testing.T) {
	help := renderHelp(client.BackendCmux)
	if !strings.Contains(help, "current workspaces") || !strings.Contains(help, "Show current workspaces") {
		t.Error("cmux help should say 'workspaces', not 'tabs'")
	}
}

func TestRenderHelp_ContainsTip(t *testing.T) {
	help := renderHelp(client.BackendGhostty)
	if !strings.Contains(help, "Tip") {
		t.Error("help should contain tip line")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -count=1 -run TestRenderHelp ./internal/tui/`
Expected: FAIL — `renderHelp` undefined

- [ ] **Step 3: Create `internal/tui/shell_styles.go`**

```go
package tui

import "github.com/charmbracelet/lipgloss"

var (
	shellPromptStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#5FFF87", Light: "#1A8A3E"}).Bold(true)
	shellHeadingStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#FFD787", Light: "#B8860B"}).Bold(true)
	shellDimStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#8C8C8C", Light: "#6C6C6C"})
	shellErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#FF6B6B", Light: "#CC3333"})
	shellSuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#5FFF87", Light: "#1A8A3E"})
	shellCyanStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#87D7FF", Light: "#0277BD"})
	shellCursorStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#5FFF87", Light: "#1A8A3E"}).Bold(true)
)
```

- [ ] **Step 4: Create `internal/tui/shell_help.go`**

```go
package tui

import (
	"fmt"
	"strings"

	"github.com/drolosoft/cmux-resurrect/internal/client"
)

type helpEntry struct {
	icon  string
	cmd   string
	args  string
	desc  func(client.DetectedBackend) string
	group string
}

var helpEntries = []helpEntry{
	{"🖥", "now", "", func(b client.DetectedBackend) string { return "Show current " + unitLabel(b, 2) }, "Live"},
	{"⏱", "watch", "start|stop|status", func(b client.DetectedBackend) string { return "Auto-save daemon" }, "Live"},
	{"📋", "ls", "", func(b client.DetectedBackend) string { return "List saved layouts" }, "Layouts"},
	{"🔄", "restore", "<name|#>", func(b client.DetectedBackend) string { return "Restore a saved layout" }, "Layouts"},
	{"💾", "save", "[name]", func(b client.DetectedBackend) string { return "Save current layout" }, "Layouts"},
	{"🗑", "delete", "<name|#>", func(b client.DetectedBackend) string { return "Delete a saved layout" }, "Layouts"},
	{"📦", "templates", "", func(b client.DetectedBackend) string { return "Browse template gallery" }, "Templates"},
	{"🚀", "use", "<template|#>", func(b client.DetectedBackend) string { return "Create " + unitLabel(b, 1) + " from template" }, "Templates"},
	{"📐", "bp add", "<name> <path>", func(b client.DetectedBackend) string { return "Add Blueprint entry" }, "Blueprint"},
	{"📐", "bp list", "", func(b client.DetectedBackend) string { return "List Blueprint entries" }, "Blueprint"},
	{"📐", "bp remove", "<name|#>", func(b client.DetectedBackend) string { return "Remove Blueprint entry" }, "Blueprint"},
	{"📐", "bp toggle", "<name|#>", func(b client.DetectedBackend) string { return "Enable/disable entry" }, "Blueprint"},
	{"❓", "help", "", func(b client.DetectedBackend) string { return "Show this help" }, "Shell"},
	{"👋", "exit", "", func(b client.DetectedBackend) string { return "Exit the shell" }, "Shell"},
}

// unitLabel returns "tab(s)" for Ghostty, "workspace(s)" for cmux.
// This is the shell-internal version (doesn't depend on cmd.cachedBackend).
func unitLabel(b client.DetectedBackend, count int) string {
	if b == client.BackendGhostty {
		if count == 1 {
			return "tab"
		}
		return "tabs"
	}
	if count == 1 {
		return "workspace"
	}
	return "workspaces"
}

// renderHelp builds the full help text with icons, grouped by section.
func renderHelp(backend client.DetectedBackend) string {
	var b strings.Builder

	groupOrder := []string{"Live", "Layouts", "Templates", "Blueprint", "Shell"}

	for _, group := range groupOrder {
		b.WriteString("  ")
		b.WriteString(shellHeadingStyle.Render(group))
		b.WriteString("\n")

		for _, e := range helpEntries {
			if e.group != group {
				continue
			}
			args := ""
			if e.args != "" {
				args = " " + shellDimStyle.Render(e.args)
			}
			desc := shellDimStyle.Render(e.desc(backend))
			cmd := shellSuccessStyle.Render(e.cmd)
			b.WriteString(fmt.Sprintf("  %s  %-28s %s\n", e.icon, cmd+args, desc))
		}
		b.WriteString("\n")
	}

	b.WriteString(shellDimStyle.Render("  Tip: Use # from the last listing, or ↑/↓ to navigate results."))
	b.WriteString("\n")

	return b.String()
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test -count=1 -run TestRenderHelp ./internal/tui/`
Expected: PASS (6 tests)

- [ ] **Step 6: Commit**

```bash
git add internal/tui/shell_styles.go internal/tui/shell_help.go internal/tui/shell_help_test.go
git commit -m "feat: add shell styles and help renderer with icons and adaptive labels"
```

---

### Task 6: Command parser and registry

**Files:**
- Create: `internal/tui/shell_commands.go`
- Create: `internal/tui/shell_commands_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/tui/shell_commands_test.go`:

```go
package tui

import (
	"testing"
)

func TestParseCommand_Simple(t *testing.T) {
	tests := []struct {
		input   string
		wantCmd string
		wantArgs []string
	}{
		{"ls", "ls", nil},
		{"save morning", "save", []string{"morning"}},
		{"restore 2", "restore", []string{"2"}},
		{"delete my-layout", "delete", []string{"my-layout"}},
		{"use claude", "use", []string{"claude"}},
		{"now", "now", nil},
		{"help", "help", nil},
		{"exit", "exit", nil},
		{"templates", "templates", nil},
		{"watch start", "watch", []string{"start"}},
		{"watch stop", "watch", []string{"stop"}},
		{"watch status", "watch", []string{"status"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cmd, args := parseCommand(tt.input)
			if cmd != tt.wantCmd {
				t.Errorf("parseCommand(%q) cmd = %q, want %q", tt.input, cmd, tt.wantCmd)
			}
			if len(args) != len(tt.wantArgs) {
				t.Errorf("parseCommand(%q) args = %v, want %v", tt.input, args, tt.wantArgs)
			}
		})
	}
}

func TestParseCommand_Blueprint(t *testing.T) {
	tests := []struct {
		input   string
		wantCmd string
		wantArgs []string
	}{
		{"bp add api ~/projects/api", "bp add", []string{"api", "~/projects/api"}},
		{"bp list", "bp list", nil},
		{"bp remove api", "bp remove", []string{"api"}},
		{"bp toggle 3", "bp toggle", []string{"3"}},
		{"bp rm api", "bp rm", []string{"api"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cmd, args := parseCommand(tt.input)
			if cmd != tt.wantCmd {
				t.Errorf("parseCommand(%q) cmd = %q, want %q", tt.input, cmd, tt.wantCmd)
			}
			if len(args) != len(tt.wantArgs) {
				t.Errorf("parseCommand(%q) args = %v, want %v", tt.input, args, tt.wantArgs)
				return
			}
			for i, a := range args {
				if a != tt.wantArgs[i] {
					t.Errorf("parseCommand(%q) args[%d] = %q, want %q", tt.input, i, a, tt.wantArgs[i])
				}
			}
		})
	}
}

func TestParseCommand_EmptyAndWhitespace(t *testing.T) {
	cmd, args := parseCommand("")
	if cmd != "" || args != nil {
		t.Errorf("empty input should return empty cmd, got %q %v", cmd, args)
	}

	cmd, args = parseCommand("   ")
	if cmd != "" || args != nil {
		t.Errorf("whitespace input should return empty cmd, got %q %v", cmd, args)
	}
}

func TestResolveNumberRef_Valid(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "morning"},
		{Kind: KindLayout, Name: "afternoon"},
		{Kind: KindLayout, Name: "evening"},
	}

	item, err := resolveNumberRef("2", items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.Name != "afternoon" {
		t.Errorf("resolveNumberRef(2) = %q, want %q", item.Name, "afternoon")
	}
}

func TestResolveNumberRef_OutOfRange(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "morning"},
	}

	_, err := resolveNumberRef("99", items)
	if err == nil {
		t.Error("expected error for out-of-range ref, got nil")
	}
}

func TestResolveNumberRef_NotANumber(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "morning"},
	}

	_, err := resolveNumberRef("abc", items)
	if err == nil {
		t.Error("expected error for non-numeric ref, got nil")
	}
}

func TestResolveNumberRef_EmptyItems(t *testing.T) {
	_, err := resolveNumberRef("1", nil)
	if err == nil {
		t.Error("expected error for empty items, got nil")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -count=1 -run "TestParseCommand|TestResolveNumber" ./internal/tui/`
Expected: FAIL — `parseCommand` and `resolveNumberRef` undefined

- [ ] **Step 3: Implement `shell_commands.go`**

Create `internal/tui/shell_commands.go`:

```go
package tui

import (
	"fmt"
	"strconv"
	"strings"
)

// parseCommand splits user input into a command name and arguments.
// For "bp" subcommands, the command includes the subcommand: "bp add", "bp list", etc.
func parseCommand(input string) (cmd string, args []string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil
	}

	parts := strings.Fields(input)
	if len(parts) == 0 {
		return "", nil
	}

	// Handle "bp" subcommands: "bp add api ~/p" -> cmd="bp add", args=["api", "~/p"]
	if parts[0] == "bp" && len(parts) >= 2 {
		cmd = parts[0] + " " + parts[1]
		if len(parts) > 2 {
			args = parts[2:]
		}
		return cmd, args
	}

	cmd = parts[0]
	if len(parts) > 1 {
		args = parts[1:]
	}
	return cmd, args
}

// resolveNumberRef resolves a "#N" reference against the last listing.
// Numbers are 1-based (displayed as [1], [2], ...).
func resolveNumberRef(ref string, items []Item) (Item, error) {
	n, err := strconv.Atoi(ref)
	if err != nil {
		return Item{}, fmt.Errorf("not a number: %q", ref)
	}
	if len(items) == 0 {
		return Item{}, fmt.Errorf("no items in last listing")
	}
	if n < 1 || n > len(items) {
		return Item{}, fmt.Errorf("no item #%d in last listing", n)
	}
	return items[n-1], nil
}

// resolveNameOrNumber resolves an argument that could be a name or a #N reference.
// If the arg is a valid integer, it resolves against lastItems.
// Otherwise, it returns an Item with the Name set to the argument.
func resolveNameOrNumber(arg string, lastItems []Item) (string, error) {
	// Try as number first
	if n, err := strconv.Atoi(arg); err == nil {
		item, err := resolveNumberRef(strconv.Itoa(n), lastItems)
		if err != nil {
			return "", err
		}
		return item.Name, nil
	}
	// It's a name
	return arg, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -count=1 -run "TestParseCommand|TestResolveNumber" ./internal/tui/`
Expected: PASS (all tests)

- [ ] **Step 5: Commit**

```bash
git add internal/tui/shell_commands.go internal/tui/shell_commands_test.go
git commit -m "feat: add command parser and number reference resolution for shell"
```

---

### Task 7: Browse model

**Files:**
- Create: `internal/tui/shell_browse.go`
- Create: `internal/tui/shell_browse_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/tui/shell_browse_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestBrowseModel_NavigateDown(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "a"},
		{Kind: KindLayout, Name: "b"},
		{Kind: KindLayout, Name: "c"},
	}
	bm := NewBrowseModel(items, "restore")

	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyDown})
	if bm.cursor != 1 {
		t.Errorf("cursor after down = %d, want 1", bm.cursor)
	}
}

func TestBrowseModel_NavigateUp(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "a"},
		{Kind: KindLayout, Name: "b"},
	}
	bm := NewBrowseModel(items, "restore")
	bm.cursor = 1

	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyUp})
	if bm.cursor != 0 {
		t.Errorf("cursor after up = %d, want 0", bm.cursor)
	}
}

func TestBrowseModel_ClampTop(t *testing.T) {
	items := []Item{{Kind: KindLayout, Name: "a"}}
	bm := NewBrowseModel(items, "restore")

	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyUp})
	if bm.cursor != 0 {
		t.Errorf("cursor should clamp at 0, got %d", bm.cursor)
	}
}

func TestBrowseModel_ClampBottom(t *testing.T) {
	items := []Item{{Kind: KindLayout, Name: "a"}}
	bm := NewBrowseModel(items, "restore")

	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyDown})
	if bm.cursor != 0 {
		t.Errorf("cursor should clamp at 0, got %d", bm.cursor)
	}
}

func TestBrowseModel_EnterSelectsItem(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "morning"},
		{Kind: KindLayout, Name: "afternoon"},
	}
	bm := NewBrowseModel(items, "restore")
	bm.cursor = 1

	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !bm.selected {
		t.Error("expected selected=true after Enter")
	}
	if bm.SelectedItem().Name != "afternoon" {
		t.Errorf("selected item = %q, want %q", bm.SelectedItem().Name, "afternoon")
	}
}

func TestBrowseModel_QuitReturnsToPrompt(t *testing.T) {
	items := []Item{{Kind: KindLayout, Name: "a"}}
	bm := NewBrowseModel(items, "restore")

	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if !bm.done {
		t.Error("expected done=true after q")
	}
	if bm.selected {
		t.Error("expected selected=false after q")
	}
}

func TestBrowseModel_LetterExitsBrowse(t *testing.T) {
	items := []Item{{Kind: KindLayout, Name: "a"}}
	bm := NewBrowseModel(items, "restore")

	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if !bm.done {
		t.Error("expected done=true after typing a letter")
	}
	if bm.passthrough != 's' {
		t.Errorf("passthrough = %q, want 's'", bm.passthrough)
	}
}

func TestBrowseModel_FilterNarrows(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "morning"},
		{Kind: KindLayout, Name: "afternoon"},
		{Kind: KindLayout, Name: "evening"},
	}
	bm := NewBrowseModel(items, "restore")

	// Press / to enter filter
	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !bm.filtering {
		t.Error("expected filtering=true after /")
	}

	// Type 'm' — should match "morning"
	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if len(bm.visible) != 1 {
		t.Errorf("after filter 'm': visible = %d, want 1", len(bm.visible))
	}
}

func TestBrowseModel_View_ContainsCursor(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "morning", Description: "test", Workspaces: 2},
	}
	bm := NewBrowseModel(items, "restore")
	view := bm.View()

	if !strings.Contains(view, "▸") {
		t.Error("browse view should contain cursor marker ▸")
	}
	if !strings.Contains(view, "[1]") {
		t.Error("browse view should contain numbered index [1]")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -count=1 -run TestBrowseModel ./internal/tui/`
Expected: FAIL — `NewBrowseModel` undefined

- [ ] **Step 3: Implement `shell_browse.go`**

Create `internal/tui/shell_browse.go`:

```go
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// BrowseModel handles arrow-key navigation on a listing.
type BrowseModel struct {
	items       []Item    // all items
	visible     []Item    // filtered items
	cursor      int
	action      string    // "restore", "use", "toggle" — the Enter action label
	filtering   bool
	filterText  string
	selected    bool      // true when Enter was pressed
	done        bool      // true when user wants to exit browse mode
	passthrough rune      // non-zero if user typed a letter (pass to prompt)
}

// NewBrowseModel creates a browse model from a list of items.
func NewBrowseModel(items []Item, action string) BrowseModel {
	vis := make([]Item, len(items))
	copy(vis, items)
	return BrowseModel{
		items:   items,
		visible: vis,
		action:  action,
	}
}

// SelectedItem returns the currently selected item.
func (bm BrowseModel) SelectedItem() Item {
	if bm.cursor < len(bm.visible) {
		return bm.visible[bm.cursor]
	}
	return Item{}
}

// Update processes key events in browse mode.
func (bm BrowseModel) Update(msg tea.KeyMsg) (BrowseModel, tea.Cmd) {
	if bm.filtering {
		return bm.updateFilter(msg)
	}

	switch msg.Type {
	case tea.KeyDown:
		if bm.cursor < len(bm.visible)-1 {
			bm.cursor++
		}
		return bm, nil

	case tea.KeyUp:
		if bm.cursor > 0 {
			bm.cursor--
		}
		return bm, nil

	case tea.KeyEnter:
		if len(bm.visible) > 0 {
			bm.selected = true
			bm.done = true
		}
		return bm, nil

	case tea.KeyEsc:
		bm.done = true
		return bm, nil

	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			r := msg.Runes[0]
			switch r {
			case 'q':
				bm.done = true
				return bm, nil
			case '/':
				bm.filtering = true
				bm.filterText = ""
				return bm, nil
			default:
				// Any other letter exits browse and passes to prompt
				bm.done = true
				bm.passthrough = r
				return bm, nil
			}
		}
	}
	return bm, nil
}

func (bm BrowseModel) updateFilter(msg tea.KeyMsg) (BrowseModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		bm.filtering = false
		bm.filterText = ""
		bm.visible = make([]Item, len(bm.items))
		copy(bm.visible, bm.items)
		bm.cursor = 0
		return bm, nil

	case tea.KeyEnter:
		bm.filtering = false
		if len(bm.visible) > 0 {
			bm.selected = true
			bm.done = true
		}
		return bm, nil

	case tea.KeyBackspace:
		if len(bm.filterText) > 0 {
			bm.filterText = bm.filterText[:len(bm.filterText)-1]
			bm.applyFilter()
		}
		return bm, nil

	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			bm.filterText += string(msg.Runes[0])
			bm.applyFilter()
		}
		return bm, nil
	}
	return bm, nil
}

func (bm *BrowseModel) applyFilter() {
	if bm.filterText == "" {
		bm.visible = make([]Item, len(bm.items))
		copy(bm.visible, bm.items)
	} else {
		lower := strings.ToLower(bm.filterText)
		bm.visible = nil
		for _, item := range bm.items {
			if strings.Contains(strings.ToLower(item.FilterValue()), lower) {
				bm.visible = append(bm.visible, item)
			}
		}
	}
	bm.cursor = 0
}

// View renders the browse list with cursor and indices.
func (bm BrowseModel) View() string {
	var b strings.Builder

	for i, item := range bm.visible {
		idx := shellDimStyle.Render(fmt.Sprintf("[%d]", i+1))
		name := item.Title()
		desc := item.Desc()

		if i == bm.cursor {
			b.WriteString(fmt.Sprintf("  %s %s %s", shellCursorStyle.Render("▸"), idx, shellSuccessStyle.Render(name)))
		} else {
			b.WriteString(fmt.Sprintf("    %s %s", idx, name))
		}
		if desc != "" {
			b.WriteString("  ")
			b.WriteString(shellDimStyle.Render(desc))
		}
		b.WriteString("\n")
	}

	// Footer
	if bm.filtering {
		b.WriteString(fmt.Sprintf("  / %s", bm.filterText))
		b.WriteString(shellDimStyle.Render("▌"))
		b.WriteString("\n")
	} else {
		hint := fmt.Sprintf("  ↑/↓ select · ↵ %s · / filter · q back", bm.action)
		b.WriteString(shellDimStyle.Render(hint))
		b.WriteString("\n")
	}

	return b.String()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -count=1 -run TestBrowseModel ./internal/tui/`
Expected: PASS (all tests)

- [ ] **Step 5: Commit**

```bash
git add internal/tui/shell_browse.go internal/tui/shell_browse_test.go
git commit -m "feat: add browse model with arrow navigation, filter, and cursor"
```

---

### Task 8: Shell model (main REPL)

**Files:**
- Create: `internal/tui/shell.go`
- Create: `internal/tui/shell_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/tui/shell_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/drolosoft/cmux-resurrect/internal/client"
)

func TestShellModel_InitShowsWelcome(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")
	view := m.View()

	if !strings.Contains(view, "crex") {
		t.Error("initial view should contain 'crex'")
	}
	if !strings.Contains(view, "help") {
		t.Error("initial view should mention 'help'")
	}
}

func TestShellModel_StartsInPromptMode(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")
	if m.mode != modePrompt {
		t.Errorf("expected modePrompt, got %v", m.mode)
	}
}

func TestShellModel_ExitQuits(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")
	m.prompt.SetValue("exit")

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sm := result.(ShellModel)

	if !sm.quitting {
		t.Error("expected quitting=true after 'exit'")
	}
	if cmd == nil {
		t.Error("expected tea.Quit command")
	}
}

func TestShellModel_HelpShowsOutput(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")
	m.prompt.SetValue("help")

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sm := result.(ShellModel)

	view := sm.View()
	if !strings.Contains(view, "Live") {
		t.Error("help output should contain 'Live' group")
	}
	if !strings.Contains(view, "Layouts") {
		t.Error("help output should contain 'Layouts' group")
	}
}

func TestShellModel_UnknownCommand(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")
	m.prompt.SetValue("wat")

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sm := result.(ShellModel)

	if !strings.Contains(sm.output.String(), "Unknown command") {
		t.Error("unknown command should show error message")
	}
}

func TestShellModel_EmptyEnterDoesNothing(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")
	m.prompt.SetValue("")

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sm := result.(ShellModel)

	if sm.quitting {
		t.Error("empty enter should not quit")
	}
	if sm.mode != modePrompt {
		t.Error("should stay in prompt mode")
	}
}

func TestShellModel_HistoryRecordsCommands(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")

	// Execute "help"
	m.prompt.SetValue("help")
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sm := result.(ShellModel)

	if len(sm.history) != 1 {
		t.Errorf("history length = %d, want 1", len(sm.history))
	}
	if sm.history[0] != "help" {
		t.Errorf("history[0] = %q, want %q", sm.history[0], "help")
	}
}

func TestShellModel_CtrlCQuits(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	sm := result.(ShellModel)

	if !sm.quitting {
		t.Error("ctrl+c should quit")
	}
	if cmd == nil {
		t.Error("expected tea.Quit command")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -count=1 -run TestShellModel ./internal/tui/`
Expected: FAIL — `NewShellModel` undefined

- [ ] **Step 3: Implement `shell.go`**

Create `internal/tui/shell.go`:

```go
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

type shellMode int

const (
	modePrompt  shellMode = iota
	modeBrowse
	modeConfirm
)

const maxHistory = 50

// ShellModel is the main Bubble Tea model for the crex interactive shell.
type ShellModel struct {
	mode    shellMode
	prompt  textinput.Model
	browse  BrowseModel
	output  strings.Builder
	lastItems []Item
	history []string
	histIdx int
	backend client.DetectedBackend
	store   persist.Store
	client  client.Backend
	wsFile  string
	quitting bool

	// Confirmation state
	confirmMsg string
	confirmFn  func()
}

// NewShellModel creates the interactive shell model.
func NewShellModel(store persist.Store, cl client.Backend, backend client.DetectedBackend, wsFile string) ShellModel {
	ti := textinput.New()
	ti.Prompt = shellPromptStyle.Render("crex❯") + " "
	ti.Focus()
	ti.CharLimit = 256

	m := ShellModel{
		mode:    modePrompt,
		prompt:  ti,
		backend: backend,
		store:   store,
		client:  cl,
		wsFile:  wsFile,
		histIdx: -1,
	}

	// Welcome message
	m.output.WriteString(shellDimStyle.Render("  crex interactive shell. Type "))
	m.output.WriteString(shellSuccessStyle.Render("help"))
	m.output.WriteString(shellDimStyle.Render(" for commands, "))
	m.output.WriteString(shellSuccessStyle.Render("exit"))
	m.output.WriteString(shellDimStyle.Render(" to quit."))
	m.output.WriteString("\n\n")

	return m
}

// Init is the Bubble Tea init function.
func (m ShellModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles all incoming messages.
func (m ShellModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.mode {
		case modePrompt:
			return m.updatePrompt(msg)
		case modeBrowse:
			return m.updateBrowse(msg)
		case modeConfirm:
			return m.updateConfirm(msg)
		}
	}

	// Pass other messages to the text input
	var cmd tea.Cmd
	m.prompt, cmd = m.prompt.Update(msg)
	return m, cmd
}

func (m ShellModel) updatePrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit

	case tea.KeyUp:
		// History navigation
		if len(m.history) > 0 && m.histIdx < len(m.history)-1 {
			m.histIdx++
			m.prompt.SetValue(m.history[len(m.history)-1-m.histIdx])
			m.prompt.CursorEnd()
		}
		return m, nil

	case tea.KeyDown:
		// History navigation
		if m.histIdx > 0 {
			m.histIdx--
			m.prompt.SetValue(m.history[len(m.history)-1-m.histIdx])
			m.prompt.CursorEnd()
		} else if m.histIdx == 0 {
			m.histIdx = -1
			m.prompt.SetValue("")
		}
		return m, nil

	case tea.KeyEnter:
		input := strings.TrimSpace(m.prompt.Value())
		m.prompt.SetValue("")
		m.histIdx = -1

		if input == "" {
			return m, nil
		}

		// Record in history
		m.history = append(m.history, input)
		if len(m.history) > maxHistory {
			m.history = m.history[len(m.history)-maxHistory:]
		}

		// Echo the command
		m.output.WriteString(shellPromptStyle.Render("crex❯"))
		m.output.WriteString(" ")
		m.output.WriteString(input)
		m.output.WriteString("\n")

		return m.dispatch(input)
	}

	// Pass to text input for line editing
	var cmd tea.Cmd
	m.prompt, cmd = m.prompt.Update(msg)
	return m, cmd
}

func (m ShellModel) updateBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	bm, _ := m.browse.Update(msg)
	m.browse = bm

	if bm.done {
		m.mode = modePrompt
		if bm.selected {
			return m.handleBrowseSelection(bm.SelectedItem())
		}
		if bm.passthrough != 0 {
			m.prompt.SetValue(string(bm.passthrough))
			m.prompt.CursorEnd()
		}
	}
	return m, nil
}

func (m ShellModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && (msg.Runes[0] == 'y' || msg.Runes[0] == 'Y') {
		if m.confirmFn != nil {
			m.confirmFn()
		}
		m.output.WriteString(shellSuccessStyle.Render("  ✓ Done"))
		m.output.WriteString("\n\n")
	} else {
		m.output.WriteString(shellDimStyle.Render("  Cancelled"))
		m.output.WriteString("\n\n")
	}
	m.mode = modePrompt
	m.confirmMsg = ""
	m.confirmFn = nil
	return m, nil
}

func (m ShellModel) handleBrowseSelection(item Item) (tea.Model, tea.Cmd) {
	// Determine action based on browse context
	switch m.browse.action {
	case "restore":
		m.execRestore(item.Name)
	case "use":
		m.execUse(item.Name)
	case "toggle":
		m.execBpToggle(item.Name)
	}
	return m, nil
}

func (m ShellModel) dispatch(input string) (tea.Model, tea.Cmd) {
	cmd, args := parseCommand(input)

	switch cmd {
	case "exit", "quit":
		m.output.WriteString(shellDimStyle.Render("  👋"))
		m.output.WriteString("\n")
		m.quitting = true
		return m, tea.Quit

	case "help", "?":
		m.output.WriteString(renderHelp(m.backend))
		m.output.WriteString("\n")

	case "ls", "list":
		m.execList()

	case "now":
		m.execNow()

	case "save":
		name := "default"
		if len(args) > 0 {
			name = args[0]
		}
		m.execSave(name)

	case "restore":
		if len(args) == 0 {
			m.output.WriteString(shellErrorStyle.Render("  ✗ Usage: restore <name|#>"))
			m.output.WriteString("\n\n")
			break
		}
		resolved, err := resolveNameOrNumber(args[0], m.lastItems)
		if err != nil {
			m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %s", err)))
			m.output.WriteString("\n\n")
			break
		}
		m.execRestore(resolved)

	case "delete":
		if len(args) == 0 {
			m.output.WriteString(shellErrorStyle.Render("  ✗ Usage: delete <name|#>"))
			m.output.WriteString("\n\n")
			break
		}
		resolved, err := resolveNameOrNumber(args[0], m.lastItems)
		if err != nil {
			m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %s", err)))
			m.output.WriteString("\n\n")
			break
		}
		m.execDelete(resolved)

	case "templates":
		m.execTemplates()

	case "use":
		if len(args) == 0 {
			m.output.WriteString(shellErrorStyle.Render("  ✗ Usage: use <template|#>"))
			m.output.WriteString("\n\n")
			break
		}
		resolved, err := resolveNameOrNumber(args[0], m.lastItems)
		if err != nil {
			m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %s", err)))
			m.output.WriteString("\n\n")
			break
		}
		m.execUse(resolved)

	case "watch":
		sub := ""
		if len(args) > 0 {
			sub = args[0]
		}
		m.execWatch(sub)

	case "bp add":
		if len(args) < 2 {
			m.output.WriteString(shellErrorStyle.Render("  ✗ Usage: bp add <name> <path>"))
			m.output.WriteString("\n\n")
			break
		}
		m.execBpAdd(args[0], args[1])

	case "bp list", "bp ls":
		m.execBpList()

	case "bp remove", "bp rm":
		if len(args) == 0 {
			m.output.WriteString(shellErrorStyle.Render("  ✗ Usage: bp remove <name|#>"))
			m.output.WriteString("\n\n")
			break
		}
		resolved, err := resolveNameOrNumber(args[0], m.lastItems)
		if err != nil {
			m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %s", err)))
			m.output.WriteString("\n\n")
			break
		}
		m.execBpRemove(resolved)

	case "bp toggle":
		if len(args) == 0 {
			m.output.WriteString(shellErrorStyle.Render("  ✗ Usage: bp toggle <name|#>"))
			m.output.WriteString("\n\n")
			break
		}
		resolved, err := resolveNameOrNumber(args[0], m.lastItems)
		if err != nil {
			m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %s", err)))
			m.output.WriteString("\n\n")
			break
		}
		m.execBpToggle(resolved)

	default:
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ Unknown command: %s", cmd)))
		m.output.WriteString("\n")
		m.output.WriteString(shellDimStyle.Render("  Type help for available commands."))
		m.output.WriteString("\n\n")
	}

	return m, nil
}

// View renders the full shell output.
func (m ShellModel) View() string {
	if m.quitting {
		return m.output.String()
	}

	var b strings.Builder
	b.WriteString(m.output.String())

	if m.mode == modeBrowse {
		b.WriteString(m.browse.View())
	}

	if m.mode == modeConfirm {
		b.WriteString(m.confirmMsg)
		b.WriteString("\n")
	}

	if m.mode == modePrompt {
		b.WriteString(m.prompt.View())
	}

	return b.String()
}

// --- Exec stubs (implemented in Task 9) ---

func (m *ShellModel) execList()                       {}
func (m *ShellModel) execNow()                        {}
func (m *ShellModel) execSave(name string)            {}
func (m *ShellModel) execRestore(name string)         {}
func (m *ShellModel) execDelete(name string)          {}
func (m *ShellModel) execTemplates()                  {}
func (m *ShellModel) execUse(name string)             {}
func (m *ShellModel) execWatch(sub string)            {}
func (m *ShellModel) execBpAdd(name, path string)     {}
func (m *ShellModel) execBpList()                     {}
func (m *ShellModel) execBpRemove(name string)        {}
func (m *ShellModel) execBpToggle(name string)        {}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -count=1 -run TestShellModel ./internal/tui/`
Expected: PASS (all tests)

- [ ] **Step 5: Commit**

```bash
git add internal/tui/shell.go internal/tui/shell_test.go
git commit -m "feat: add shell model with prompt mode, history, and command dispatch"
```

---

### Task 9: Shell command implementations

**Files:**
- Create: `internal/tui/shell_exec.go`
- Modify: `internal/tui/shell.go` (remove exec stubs)

- [ ] **Step 1: Create `internal/tui/shell_exec.go`**

This file implements all the `exec*` methods on `ShellModel`. Replace the stubs in `shell.go`.

```go
package tui

import (
	"fmt"
	"strings"
	"syscall"
	"os"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/gallery"
	"github.com/drolosoft/cmux-resurrect/internal/mdfile"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/orchestrate"
)

func (m *ShellModel) execNow() {
	if m.client == nil {
		m.output.WriteString(shellErrorStyle.Render("  ✗ No backend available"))
		m.output.WriteString("\n\n")
		return
	}

	tree, err := m.client.Tree()
	if err != nil {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %s", err)))
		m.output.WriteString("\n\n")
		return
	}

	header := "Current " + strings.Title(unitLabel(m.backend, 2))
	m.output.WriteString("  ")
	m.output.WriteString(shellHeadingStyle.Render(header))
	m.output.WriteString("\n")

	for _, win := range tree.Windows {
		for _, ws := range win.Workspaces {
			pin := "   "
			if ws.Pinned {
				pin = "📌 "
			}
			active := ""
			if ws.Active || ws.Selected {
				active = " " + shellHeadingStyle.Render("★")
			}

			cwd := ""
			if m.client != nil {
				if ss, err := m.client.SidebarState(ws.Ref); err == nil && ss.CWD != "" {
					home, _ := os.UserHomeDir()
					cwd = ss.CWD
					if home != "" && strings.HasPrefix(cwd, home) {
						cwd = "~" + cwd[len(home):]
					}
					cwd = "  " + shellDimStyle.Render(cwd)
				}
			}

			m.output.WriteString(fmt.Sprintf("  %s%s%s%s\n",
				pin,
				ws.Title,
				cwd,
				active))
		}
	}

	m.output.WriteString("\n")
	m.output.WriteString(shellDimStyle.Render("  📌 pinned · ★ active"))
	m.output.WriteString("\n\n")
}

func (m *ShellModel) execList() {
	if m.store == nil {
		m.output.WriteString(shellErrorStyle.Render("  ✗ No store configured"))
		m.output.WriteString("\n\n")
		return
	}

	metas, err := m.store.List()
	if err != nil {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %s", err)))
		m.output.WriteString("\n\n")
		return
	}

	if len(metas) == 0 {
		m.output.WriteString(shellDimStyle.Render("  No saved layouts yet."))
		m.output.WriteString("\n")
		m.output.WriteString(shellDimStyle.Render("  Try "))
		m.output.WriteString(shellSuccessStyle.Render("save my-day"))
		m.output.WriteString(shellDimStyle.Render(fmt.Sprintf(" to snapshot your current %s.", unitLabel(m.backend, 2))))
		m.output.WriteString("\n\n")
		return
	}

	items := make([]Item, len(metas))
	for i, meta := range metas {
		items[i] = Item{
			Kind:        KindLayout,
			Name:        meta.Name,
			Description: meta.Description,
			Workspaces:  meta.WorkspaceCount,
		}
	}
	m.lastItems = items

	m.output.WriteString("  ")
	m.output.WriteString(shellHeadingStyle.Render("Saved Layouts"))
	m.output.WriteString("\n")

	m.browse = NewBrowseModel(items, "restore")
	m.mode = modeBrowse
}

func (m *ShellModel) execSave(name string) {
	if m.client == nil || m.store == nil {
		m.output.WriteString(shellErrorStyle.Render("  ✗ No backend or store available"))
		m.output.WriteString("\n\n")
		return
	}

	saver := &orchestrate.Saver{Client: m.client, Store: m.store}
	layout, err := saver.Save(name, "")
	if err != nil {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %s", err)))
		m.output.WriteString("\n\n")
		return
	}

	count := len(layout.Workspaces)
	m.output.WriteString(fmt.Sprintf("  %s Saved %d %s as %s\n\n",
		shellSuccessStyle.Render("✓"),
		count,
		unitLabel(m.backend, count),
		shellSuccessStyle.Render(name)))
}

func (m *ShellModel) execRestore(name string) {
	if m.client == nil || m.store == nil {
		m.output.WriteString(shellErrorStyle.Render("  ✗ No backend or store available"))
		m.output.WriteString("\n\n")
		return
	}

	if !m.store.Exists(name) {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ Layout %s not found", name)))
		m.output.WriteString("\n\n")
		return
	}

	m.output.WriteString(fmt.Sprintf("  %s Restoring %s\n",
		shellSuccessStyle.Render("✓"),
		shellSuccessStyle.Render(name)))

	restorer := &orchestrate.Restorer{
		Client: m.client,
		Store:  m.store,
		OnProgress: func(title string, panes int, err error) {
			if err != nil {
				m.output.WriteString(fmt.Sprintf("  %s  %s: %v\n",
					shellErrorStyle.Render("FAIL"), title, err))
			} else {
				m.output.WriteString(fmt.Sprintf("  %s  %s (%d panes)\n",
					shellSuccessStyle.Render("OK"), title, panes))
			}
		},
	}

	result, err := restorer.Restore(name, false, orchestrate.RestoreModeAdd)
	if err != nil {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %s", err)))
		m.output.WriteString("\n\n")
		return
	}

	count := result.WorkspacesTotal
	m.output.WriteString(fmt.Sprintf("  %s Restored %d/%d %s\n\n",
		shellSuccessStyle.Render("✓"),
		result.WorkspacesOK, count,
		unitLabel(m.backend, count)))
}

func (m *ShellModel) execDelete(name string) {
	if m.store == nil {
		m.output.WriteString(shellErrorStyle.Render("  ✗ No store configured"))
		m.output.WriteString("\n\n")
		return
	}

	if !m.store.Exists(name) {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ Layout %s not found", name)))
		m.output.WriteString("\n\n")
		return
	}

	m.confirmMsg = fmt.Sprintf("  Delete '%s'? [y/N] ", name)
	m.confirmFn = func() {
		if err := m.store.Delete(name); err != nil {
			m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %s", err)))
		}
	}
	m.mode = modeConfirm
}

func (m *ShellModel) execTemplates() {
	templates := gallery.List()
	if len(templates) == 0 {
		m.output.WriteString(shellDimStyle.Render("  No templates available."))
		m.output.WriteString("\n\n")
		return
	}

	items := ItemsFromTemplates(templates)
	m.lastItems = items

	// Group by category
	categories := []string{}
	catItems := map[string][]Item{}
	for _, item := range items {
		cat := item.Category
		if cat == "" {
			cat = "Other"
		}
		if _, ok := catItems[cat]; !ok {
			categories = append(categories, cat)
		}
		catItems[cat] = append(catItems[cat], item)
	}

	for _, cat := range categories {
		m.output.WriteString("  ")
		m.output.WriteString(shellHeadingStyle.Render(strings.Title(cat)))
		m.output.WriteString("\n")
	}

	m.browse = NewBrowseModel(items, "use")
	m.mode = modeBrowse
}

func (m *ShellModel) execUse(name string) {
	tmpl, ok := gallery.Get(name)
	if !ok {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ Template %s not found", name)))
		m.output.WriteString("\n\n")
		return
	}

	if m.client == nil {
		m.output.WriteString(shellErrorStyle.Render("  ✗ No backend available"))
		m.output.WriteString("\n\n")
		return
	}

	panes := gallery.BuildPanes(tmpl)
	user := &orchestrate.TemplateUser{
		Client: m.client,
		OnProgress: func(msg string) {},
	}

	cwd, _ := os.Getwd()
	result, err := user.Use(panes, orchestrate.TemplateUseOpts{
		CWD: cwd,
	}, false)
	if err != nil {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %s", err)))
		m.output.WriteString("\n\n")
		return
	}

	m.output.WriteString(fmt.Sprintf("  %s Created %s from %s\n\n",
		shellSuccessStyle.Render("✓"),
		unitLabel(m.backend, 1),
		shellSuccessStyle.Render(tmpl.Name)))
	_ = result
}

func (m *ShellModel) execWatch(sub string) {
	pidPath := orchestrate.DefaultPIDPath()

	switch sub {
	case "status":
		running, pid := orchestrate.IsDaemonRunning(pidPath)
		if running {
			m.output.WriteString(fmt.Sprintf("  ⏱  Daemon: %s (pid %d)\n\n",
				shellSuccessStyle.Render("running"), pid))
		} else {
			m.output.WriteString(fmt.Sprintf("  ⏱  Daemon: %s\n\n",
				shellErrorStyle.Render("not running")))
		}

	case "stop":
		running, pid := orchestrate.IsDaemonRunning(pidPath)
		if !running {
			m.output.WriteString(shellDimStyle.Render("  Daemon not running"))
			m.output.WriteString("\n\n")
			return
		}
		proc, err := os.FindProcess(pid)
		if err != nil {
			m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %s", err)))
			m.output.WriteString("\n\n")
			return
		}
		if err := proc.Signal(syscall.SIGINT); err != nil {
			m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %s", err)))
			m.output.WriteString("\n\n")
			return
		}
		orchestrate.RemovePIDFile(pidPath)
		m.output.WriteString(fmt.Sprintf("  %s Daemon stopped (pid %d)\n\n",
			shellSuccessStyle.Render("✓"), pid))

	case "start":
		m.output.WriteString(shellDimStyle.Render("  Use 'crex watch --daemon' from the terminal to start the daemon."))
		m.output.WriteString("\n\n")

	default:
		m.output.WriteString(shellErrorStyle.Render("  ✗ Usage: watch start|stop|status"))
		m.output.WriteString("\n\n")
	}
}

func (m *ShellModel) execBpAdd(name, path string) {
	p := model.Project{
		Enabled:  true,
		Icon:     "📁",
		Name:     name,
		Template: "dev",
		Pin:      true,
		Path:     path,
	}

	if err := mdfile.AddProject(m.wsFile, p); err != nil {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %s", err)))
		m.output.WriteString("\n\n")
		return
	}

	m.output.WriteString(fmt.Sprintf("  %s Added %s to Blueprint\n\n",
		shellSuccessStyle.Render("✓"),
		shellSuccessStyle.Render(name)))
}

func (m *ShellModel) execBpList() {
	wf, err := mdfile.Parse(m.wsFile)
	if err != nil {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %s", err)))
		m.output.WriteString("\n\n")
		return
	}

	if len(wf.Projects) == 0 {
		m.output.WriteString(shellDimStyle.Render("  No Blueprint entries."))
		m.output.WriteString("\n")
		m.output.WriteString(shellDimStyle.Render("  Try "))
		m.output.WriteString(shellSuccessStyle.Render("bp add myapp ~/projects/myapp"))
		m.output.WriteString("\n\n")
		return
	}

	items := make([]Item, len(wf.Projects))
	for i, p := range wf.Projects {
		desc := p.Template + " · " + p.Path
		if !p.Enabled {
			desc += " (disabled)"
		}
		items[i] = Item{
			Kind:        KindLayout, // reuse kind for indexing
			Name:        p.Name,
			Description: desc,
			Icon:        p.Icon,
		}
	}
	m.lastItems = items

	m.output.WriteString("  ")
	m.output.WriteString(shellHeadingStyle.Render("Blueprint Entries"))
	m.output.WriteString("\n")

	m.browse = NewBrowseModel(items, "toggle")
	m.mode = modeBrowse
}

func (m *ShellModel) execBpRemove(name string) {
	if err := mdfile.RemoveProject(m.wsFile, name); err != nil {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %s", err)))
		m.output.WriteString("\n\n")
		return
	}

	m.output.WriteString(fmt.Sprintf("  %s Removed %s from Blueprint\n\n",
		shellSuccessStyle.Render("✓"),
		shellSuccessStyle.Render(name)))
}

func (m *ShellModel) execBpToggle(name string) {
	newState, err := mdfile.ToggleProject(m.wsFile, name)
	if err != nil {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %s", err)))
		m.output.WriteString("\n\n")
		return
	}

	if newState {
		m.output.WriteString(fmt.Sprintf("  %s Enabled %s\n\n",
			shellSuccessStyle.Render("✓"),
			shellSuccessStyle.Render(name)))
	} else {
		m.output.WriteString(fmt.Sprintf("  %s Disabled %s\n\n",
			shellDimStyle.Render("✗"),
			shellDimStyle.Render(name)))
	}
}
```

- [ ] **Step 2: Remove exec stubs from `shell.go`**

Delete the stub section at the bottom of `internal/tui/shell.go` (the lines starting with `// --- Exec stubs`).

- [ ] **Step 3: Run all tests**

Run: `go test -count=1 ./internal/tui/`
Expected: All tests PASS (compile check — exec functions reference real packages)

- [ ] **Step 4: Commit**

```bash
git add internal/tui/shell_exec.go internal/tui/shell.go
git commit -m "feat: implement all shell command handlers (now, ls, save, restore, delete, templates, use, watch, bp)"
```

---

### Task 10: Wire up cmd/tui.go and root.go

**Files:**
- Modify: `cmd/tui.go`
- Modify: `cmd/root.go`
- Delete: `internal/tui/model.go`, `internal/tui/view.go`, `internal/tui/keys.go`
- Delete: `internal/tui/model_test.go` (replaced by shell_test.go)

- [ ] **Step 1: Rewrite `cmd/tui.go`**

Replace the entire file:

```go
package cmd

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/drolosoft/cmux-resurrect/internal/tui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Interactive shell",
	Long:  "Launch the crex interactive shell for browsing layouts, templates, and live state.",
	Args:  cobra.NoArgs,
	RunE:  runTUI,
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}

func runTUI(cmd *cobra.Command, args []string) error {
	store, err := newStore()
	if err != nil {
		return fmt.Errorf("init store: %w", err)
	}
	cl := newClient()
	backend := cachedBackend

	m := tui.NewShellModel(store, cl, backend, cfg.WorkspaceFile)
	p := tea.NewProgram(m) // no AltScreen — inline shell
	_, err = p.Run()
	return err
}
```

- [ ] **Step 2: Update root command in `cmd/root.go`**

Change the `RunE` function of `rootCmd` (lines 24-38):

```go
RunE: func(cmd *cobra.Command, args []string) error {
    store, err := newStore()
    if err != nil {
        fmt.Print(banner())
        fmt.Print(styledHelp())
        return nil
    }
    metas, err := store.List()
    if err != nil || len(metas) == 0 {
        // No layouts — check if config exists for setup hint
        if !configExists() {
            fmt.Print(banner())
            fmt.Println()
            fmt.Println(dimStyle.Render("  First time? Run ") + greenStyle.Render("crex setup") + dimStyle.Render(" to get started."))
            fmt.Println()
            return nil
        }
        fmt.Print(banner())
        fmt.Print(styledHelp())
        return nil
    }
    return runTUI(cmd, args)
},
```

Add a helper at the bottom of `cmd/root.go`:

```go
func configExists() bool {
    path := config.DefaultConfigPath()
    _, err := os.Stat(path)
    return err == nil
}
```

- [ ] **Step 3: Delete old TUI files**

```bash
git rm internal/tui/model.go internal/tui/view.go internal/tui/keys.go internal/tui/model_test.go
```

- [ ] **Step 4: Run all tests**

Run: `go test -count=1 ./...`
Expected: All tests PASS. Verify that old model tests are gone and new shell tests pass.

- [ ] **Step 5: Commit**

```bash
git add cmd/tui.go cmd/root.go
git commit -m "feat: wire interactive shell to tui command and root, remove old TUI"
```

---

### Task 11: Update styledHelp and README

**Files:**
- Modify: `cmd/style.go`
- Modify: `README.md`
- Modify: `docs/commands.md`

- [ ] **Step 1: Update `styledHelp()` in `cmd/style.go`**

Replace the `tui` help line:
```go
helpCmd(&b, "tui", "", "Interactive shell")
```

Update the examples section:
```go
helpExample(&b, "crex blueprint add notes ~/docs", "add entry to Blueprint")
```

- [ ] **Step 2: Update README.md**

Update the TUI section to describe the interactive shell. Replace "TUI Launcher" with "Interactive Shell" in feature descriptions. Update the command reference to include `now`, `bp`, etc.

- [ ] **Step 3: Update `docs/commands.md`**

Add interactive shell section. Update `workspace` references to `blueprint`.

- [ ] **Step 4: Run all tests**

Run: `go test -count=1 ./...`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/style.go README.md docs/commands.md
git commit -m "docs: update help, README, and commands for interactive shell and blueprint rename"
```

---

### Task 12: Final integration test

**Files:**
- Modify: `cmd/newcmds_test.go`

- [ ] **Step 1: Update command registration tests**

Update the TUI registration test to verify the new shell description. Add test for blueprint command:

```go
func TestBlueprintCmd_IsRegistered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "blueprint" {
			found = true
			if !slices.Contains(cmd.Aliases, "bp") {
				t.Error("blueprint command should have 'bp' alias")
			}
			break
		}
	}
	if !found {
		t.Error("blueprint command not registered on root")
	}
}

func TestWorkspaceLegacy_IsHidden(t *testing.T) {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "workspace" {
			if !cmd.Hidden {
				t.Error("workspace command should be hidden")
			}
			return
		}
	}
	t.Error("workspace legacy command not registered")
}

func TestBlueprintCmd_HasSubcommands(t *testing.T) {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "blueprint" {
			subs := []string{"add", "remove", "list", "toggle"}
			for _, sub := range subs {
				found := false
				for _, sc := range cmd.Commands() {
					if sc.Name() == sub {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("blueprint missing subcommand %q", sub)
				}
			}
			return
		}
	}
	t.Fatal("blueprint command not found")
}
```

- [ ] **Step 2: Run full test suite**

Run: `go test -count=1 ./...`
Expected: All tests PASS

- [ ] **Step 3: Build binary and smoke test**

```bash
go build -o /tmp/crex-dev ./cmd/crex
/tmp/crex-dev blueprint list
/tmp/crex-dev bp ls
/tmp/crex-dev workspace list  # hidden alias should work
/tmp/crex-dev --help          # should show "blueprint" not "workspace"
```

- [ ] **Step 4: Commit**

```bash
git add cmd/newcmds_test.go
git commit -m "test: add blueprint and legacy workspace command tests"
```
