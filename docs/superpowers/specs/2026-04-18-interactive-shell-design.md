# crex v1.5.0 — Interactive Shell & Terminology Fixes

## Goal

Replace the flat TUI with an interactive shell (REPL) inspired by [nb](https://xwmx.github.io/nb/#-interactive-shell), and ship backend-adaptive terminology + `workspace` → `blueprint` rename — all prerequisites before launching on r/Ghostty.

## Architecture

The interactive shell is a Bubble Tea program that presents a `crex>` prompt where users type commands (`now`, `ls`, `save morning`, `restore 2`, `templates`, `bp add`, etc.). After listing commands (`ls`, `templates`, `bp list`), a browse mode activates with arrow-key navigation. The shell reuses existing orchestrate/client/persist packages — it is a new UI layer, not new business logic.

Priority 0 terminology fixes (unitName, blueprint rename, splits wording) ship alongside, as the shell uses adaptive labels throughout.

## Tech Stack

- Go + [Bubble Tea](https://github.com/charmbracelet/bubbletea) (interactive shell model)
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) with `AdaptiveColor` (dark/light theming)
- [Bubbles](https://github.com/charmbracelet/bubbles) `textinput` (prompt line editing)
- Existing: `internal/client` (Backend interface), `internal/orchestrate` (Saver/Restorer/Watcher), `internal/persist` (Store), `internal/gallery` (templates)

---

## Part 1: Priority 0 — Terminology Fixes

### 0a: Backend-adaptive output labels

Add `unitName(count int) string` to `cmd/branding.go`:
- Returns `"tab"/"tabs"` when `cachedBackend == client.BackendGhostty`
- Returns `"workspace"/"workspaces"` for cmux or unknown

Update every user-facing string in these files:
- `cmd/save.go` — "Saved N workspaces" → `fmt.Sprintf("Saved %d %s", n, unitName(n))`
- `cmd/restore.go` — "Restored N/M workspaces", "Close existing workspaces", mode descriptions
- `cmd/list.go` — workspace count display
- `cmd/show.go` — workspace details header
- `cmd/import_from_md.go` — "Create workspaces from Blueprint"
- `cmd/export_to_md.go` — export messages
- `cmd/watch.go` — watch status output
- `cmd/tui.go` — all "workspaces" strings in handlers

Internal model (`model.Workspace`), TOML field (`[[workspace]]`), and config keys stay unchanged.

### 0b: Rename `crex workspace` → `crex blueprint`

1. Rename source files:
   - `cmd/ws.go` → `cmd/blueprint.go`
   - `cmd/ws_add.go` → `cmd/blueprint_add.go`
   - `cmd/ws_remove.go` → `cmd/blueprint_remove.go`
   - `cmd/ws_list.go` → `cmd/blueprint_list.go`
   - `cmd/ws_toggle.go` → `cmd/blueprint_toggle.go`

2. In the parent command (currently `workspaceCmd`):
   - `Use: "blueprint"`, `Aliases: []string{"bp"}`
   - Cobra shows aliases in help output, so `workspace`/`ws` cannot be Cobra aliases (they'd appear in `--help`). Instead: register a separate hidden command that delegates to `blueprintCmd`:
     ```go
     var workspaceLegacyCmd = &cobra.Command{
         Use:    "workspace",
         Hidden: true,
         Aliases: []string{"ws"},
     }
     // Copy subcommands from blueprintCmd to workspaceLegacyCmd in init()
     ```

3. Update help text: "Manage entries in the Workspace Blueprint" → "Manage entries in the Blueprint"

4. Update all subcommand short/long descriptions to say "Blueprint" not "workspace"

5. Update `cmd/completion_helpers.go` if it references workspace command names

### 0c: Fix "splits" language

- `cmd/save.go` Long description: "Captures all workspaces, splits, CWDs" → "Captures all tabs, pane arrangements, CWDs, and pinned state from the running terminal."
- `cmd/restore.go` Long description: "Recreates workspaces, splits, and sends commands" → "Recreates tabs, pane arrangements, and sends commands from a saved layout."
- README tagline: "saves your entire layout and brings it back: all your tabs, pane arrangements, working directories, pinned state, and startup commands."

Use "tabs" as the default phrasing since Ghostty is the growth audience and cmux users already understand.

---

## Part 2: Interactive Shell

### File structure

```
internal/tui/
  shell.go          # ShellModel — main Bubble Tea model (prompt + dispatch)
  shell_commands.go  # Command registry, parsing, dispatch table
  shell_browse.go    # BrowseModel — arrow-key navigation sub-model
  shell_view.go      # View rendering (prompt, output, browse, help)
  shell_styles.go    # Lipgloss styles with AdaptiveColor (replaces view.go styles)
  shell_help.go      # Help text rendering with icons and groups
  shell_test.go      # Tests for parsing, state transitions, command dispatch
  items.go           # Keep as-is (Item, ItemKind, converters)

cmd/
  tui.go             # Updated: launches ShellModel instead of old Model
```

Files to delete after migration:
- `internal/tui/model.go` (replaced by `shell.go`)
- `internal/tui/view.go` (replaced by `shell_view.go`)
- `internal/tui/keys.go` (replaced by inline key handling in `shell.go`)

### ShellModel

```go
type shellMode int

const (
    modePrompt  shellMode = iota  // user types at crex>
    modeBrowse                    // arrow-key navigation on a listing
    modeConfirm                   // waiting for y/n on destructive action
)

type ShellModel struct {
    mode       shellMode
    prompt     textinput.Model       // line editor
    browse     BrowseModel           // sub-model for arrow navigation
    output     strings.Builder       // accumulated output (scrollback)
    lastItems  []Item                // items from last listing command
    history    []string              // command history ring buffer (max 50)
    histIdx    int                   // current position in history (-1 = new input)
    backend    client.DetectedBackend
    store      *persist.Store
    client     client.Backend
    quitting   bool

    // Confirmation state (modeConfirm)
    confirmMsg string                // e.g. "Delete 'morning'? [y/N]"
    confirmFn  func()               // called on 'y'
}
```

### Three modes

**Prompt mode** — User types at `crex>`. Line editing via bubbles `textinput`. Up/Down recalls command history (stored in `history` ring buffer, max 50 entries). Enter parses and dispatches the command. Output appends to scrollback.

**Browse mode** — Activated after `ls`, `templates`, `bp list`. Shows listing with `[1] [2] [3]` indices and a `>` cursor on the first item. Keys:
- Up/Down: move cursor
- Enter: natural action (restore for layouts, use for templates, toggle for bp list)
- `/`: enter filter (inline, narrows the list)
- `q`: return to prompt mode
- Any letter: exit browse mode, switch to prompt mode with that letter typed

**Confirm mode** — Activated by destructive commands (`delete`). Shows confirmation prompt (e.g., `"Delete 'morning'? [y/N]"`). `y` executes, any other key cancels. Returns to prompt mode after.

### Command registry

```go
type shellCmd struct {
    name    string
    aliases []string
    args    string            // display: "<name|#>", "[name]", etc.
    desc    func() string     // closure for adaptive text
    icon    string
    group   string            // "Live", "Layouts", "Templates", "Blueprint", "Shell"
    run     func(m *ShellModel, args []string) tea.Cmd
    browse  bool              // true if command output enters browse mode
}
```

Commands (in help display order):

| Group | Icon | Command | Args | Description (Ghostty) | Browse |
|-------|------|---------|------|-----------------------|--------|
| Live | `\U1f5a5` | `now` | — | Show current tabs | read-only |
| Live | `\u23F1` | `watch` | `start\|stop\|status` | Auto-save daemon | no |
| Layouts | `\U1f4cb` | `ls` | — | List saved layouts | yes, Enter=restore |
| Layouts | `\U1f504` | `restore` | `<name\|#>` | Restore a saved layout | no |
| Layouts | `\U1f4be` | `save` | `[name]` | Save current layout | no |
| Layouts | `\U1f5d1` | `delete` | `<name\|#>` | Delete a saved layout | no |
| Templates | `\U1f4e6` | `templates` | — | Browse template gallery | yes, Enter=use |
| Templates | `\U1f680` | `use` | `<template\|#>` | Create tab from template | no |
| Blueprint | `\U1f4d0` | `bp add` | `<name> <path>` | Add Blueprint entry | no |
| Blueprint | `\U1f4d0` | `bp list` | — | List Blueprint entries | yes |
| Blueprint | `\U1f4d0` | `bp remove` | `<name\|#>` | Remove Blueprint entry | no |
| Blueprint | `\U1f4d0` | `bp toggle` | `<name\|#>` | Enable/disable entry | no |
| Shell | `\u2753` | `help` | — | Show this help | no |
| Shell | `\U1f44b` | `exit` | — | Exit the shell | no |

### Number references

After listing commands (`ls`, `templates`, `bp list`), items are stored in `lastItems`. Commands accepting `<name|#>` check if the argument is a number and resolve it against `lastItems`. If out of range: `"No item #99 in last listing"`. Each listing command overwrites the previous `lastItems`.

### Command: `now`

Calls `m.client.Tree()` to get live workspace/tab state. Renders:

```
  Current Tabs                 (or "Current Workspaces" for cmux)
  [1] pin webapp      ~/projects/webapp  *
  [2]     api-server  ~/projects/api
  [3]     notes       ~/docs
  [4] pin crex-dev    ~/Git/cmux-resurrect

  pin pinned  *  active
```

- Uses `unitName()` for the header ("Tabs" vs "Workspaces")
- Shows pinned indicator, active star
- CWD via `m.client.SidebarState(ref)`
- Read-only: does NOT enter browse mode and does NOT populate `lastItems`. The `now` command shows live terminal state — there is no action to take on individual items. If the user wants to save, they type `save`.

### Command: `ls`

Calls `m.store.List()`, renders items with `[N]` indices, enters browse mode. Browse mode Enter on an item triggers restore.

### Command: `templates`

Calls `gallery.List()`, renders grouped by category (Layouts, Workflows, etc.) with template icons, enters browse mode. Browse mode Enter triggers `use`.

### Blueprint commands (`bp add|list|remove|toggle`)

All blueprint commands call `internal/mdfile` library functions directly (not via Cobra):
- `bp add <name> <path>`: calls `mdfile.AddProject(wsFile, project)` with default template=`"dev"`, pin=`true`
- `bp list`: calls `mdfile.Parse(wsFile)`, renders entries with enabled/disabled status, enters browse mode. Browse mode Enter triggers `toggle`.
- `bp remove <name|#>`: calls `mdfile.RemoveProject(wsFile, name)`
- `bp toggle <name|#>`: calls `mdfile.ToggleProject(wsFile, name)`

The `cfg.WorkspaceFile` path comes from the loaded config.

### Command: `watch start|stop|status`

Delegates to existing daemon functions in `internal/orchestrate/daemon.go`:
- `start`: calls `StartDaemon()` or equivalent
- `stop`: calls `StopDaemon()`
- `status`: calls `IsDaemonRunning()`, displays status

### Command: `save [name]`

Delegates to `orchestrate.Saver.Save()`. Default name: `"default"`. Shows progress per-workspace/tab with adaptive labels.

### Command: `restore <name|#>`

Resolves name or `#` from `lastItems` (must be from a prior `ls` listing). Delegates to `orchestrate.Restorer.Restore()` in add mode (shell context = don't ask replace/add interactively). Shows per-workspace progress.

### Command: `delete <name|#>`

Resolves name or `#`. Calls `m.store.Delete()`. Uses a pending-confirmation state in the model: after the user types `delete morning`, the shell renders `"Delete 'morning'? [y/N]"` and enters `modeConfirm`. The next keypress (`y` or `n`/Enter) resolves the confirmation and returns to prompt mode. This avoids blocking I/O inside the Bubble Tea event loop.

### Command: `use <template|#>`

Resolves template name or `#` from `lastItems` (must be from a prior `templates` listing). Delegates to existing template use logic.

### Styling

All styles use `lipgloss.AdaptiveColor` for dark/light terminal support:

```go
var (
    promptStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#5FFF87", Light: "#1A8A3E"}).Bold(true)
    headingStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#FFD787", Light: "#B8860B"}).Bold(true)
    dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#8C8C8C", Light: "#6C6C6C"})
    errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#FF6B6B", Light: "#CC3333"})
    successStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#5FFF87", Light: "#1A8A3E"})
)
```

### Help rendering

Grouped by section (Live, Layouts, Templates, Blueprint, Shell). Each line:
```
  ICON  command  args        description
```

Footer tip: `"Tip: Use # from the last listing, or Up/Down to navigate results."`

### Empty states

- `ls` with no layouts: `"No saved layouts yet. Try save my-day to snapshot your current tabs."`
- `templates` with no templates: `"No templates available."` (shouldn't happen — 16 built-in)
- `bp list` with no entries: `"No Blueprint entries. Try bp add myapp ~/projects/myapp"`
- Unknown command: `"Unknown command: wat. Type help for available commands."`
- Invalid `#` ref: `"No item #99 in last listing"`

### Shell launch behavior

- `crex` (no args): if layouts exist, launch shell. If no layouts and first run, show banner + suggest `crex setup`.
- `crex tui`: always launch shell (explicit entry point).
- `crex --help`: still shows Cobra help text (not the shell).

### Bubble Tea program options

The shell does NOT use `tea.WithAltScreen()`. It runs inline in the terminal, like `nb`. Output scrolls naturally. The prompt stays at the bottom of visible output.

This is a key difference from the old TUI — no alt-screen means the user sees their shell history above the crex session, and output persists after exit.

### cmd/tui.go changes

```go
func runTUI(cmd *cobra.Command, args []string) error {
    store, err := newStore()
    if err != nil {
        return err
    }
    cl := newClient()
    backend := cachedBackend

    m := tui.NewShellModel(store, cl, backend)
    p := tea.NewProgram(m)  // no AltScreen
    _, err = p.Run()
    return err
}
```

The `handleTUISelect`, `handleTUISave`, `handleTUIDelete` functions in cmd/tui.go are removed — all action handling moves inside the shell model.

### Root command behavior

Update `cmd/root.go`: when no subcommand is given, check if layouts exist. If yes, run the shell. If no, show banner + help (or suggest `crex setup` if config doesn't exist).

---

## Part 3: Documentation

### HTML mockups

The design mockups created during brainstorming are documentation assets. They live at:
```
.superpowers/brainstorm/57571-1776522187/content/
  final-design.html              # Complete reference
  interactive-shell.html          # Evolution v1
  interactive-shell-v2.html       # With icons + browse
  interactive-shell-v3-adaptive.html  # Backend-adaptive
  interactive-shell-v4-final.html     # With tabs command
```

These will be adapted into:
1. A standalone HTML documentation page for the Drolosoft website
2. Reference material for the README interactive shell section

### README updates

- Tagline: adaptive phrasing (tabs/pane arrangements)
- Feature section: "Interactive Shell" replaces "TUI Launcher"
- Command reference table with icons
- Updated comparison table vs. gtab/gpane/summon

### docs/ updates

- `docs/commands.md`: add interactive shell section, update blueprint commands
- `docs/auto-save.md`: already updated for daemon mode
- `docs/configuration.md`: already updated

---

## Part 4: Testing Strategy

### Unit tests (`internal/tui/shell_test.go`)

1. **Command parsing**: `"save morning"` → cmd=`save`, args=`["morning"]`; `"bp add api ~/p"` → cmd=`bp`, subcmd=`add`, args=`["api", "~/p"]`
2. **Number resolution**: `"restore 2"` with `lastItems` of 3 items → resolves to item[1]; `"restore 99"` → error
3. **State transitions**: prompt → run `ls` → browse mode; browse → press `q` → prompt; browse → type letter → prompt with letter
4. **Help rendering**: correct groups, icons, adaptive labels
5. **Empty states**: correct messages for each command
6. **Command history**: up/down recalls previous commands
7. **Filter in browse**: `/` activates, narrows items, Escape clears

### Integration tests

8. **Backend-adaptive output**: mock cmux backend → "workspaces"; mock Ghostty backend → "tabs"
9. **unitName()**: `unitName(1)` → singular; `unitName(3)` → plural; both backends
10. **Blueprint command aliases**: `crex blueprint add` = `crex bp add` = `crex workspace add` (hidden)

### Existing tests

All 214+ existing tests must continue passing. The old TUI model tests in `model_test.go` will be replaced by the new shell tests.

---

## Naming Hierarchy (post-fix)

```
Layout (saved session snapshot — stored as TOML)
  +-- Workspace / Tab (backend-adaptive label for a terminal tab)
       +-- Pane (terminal surface within a tab)

Blueprint (declarative Markdown setup file)
  +-- Project (an entry defining what to create)
       +-- Template (pane arrangement to apply)
```

Two clean concept trees. No collisions.

---

## What ships together in v1.5.0

| Component | Scope |
|-----------|-------|
| Priority 0a | `unitName()` in `cmd/branding.go`, update 7+ files |
| Priority 0b | Rename `ws*.go` → `blueprint*.go`, update command tree |
| Priority 0c | Fix Long descriptions in save/restore, README tagline |
| Setup wizard | Already implemented, enhance with adaptive labels |
| Watch daemon | Already implemented, expose in shell as `watch start/stop/status` |
| Interactive shell | Complete rewrite of `internal/tui/` + `cmd/tui.go` |
| Docs | README, commands.md, HTML mockup pages for website |
