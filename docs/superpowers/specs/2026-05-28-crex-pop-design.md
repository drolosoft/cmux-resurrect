# crex pop — Universal Workspace Switcher

**Date:** 2026-05-28
**Status:** Approved
**Scope:** Instant workspace picker with shell hook activation

## Problem

Launching a crex workspace requires opening a terminal, typing `crex tui` or
`crex restore <name>`, and navigating. The competitor gtab offers a Ghostty
keybinding (Cmd+G) that opens their TUI instantly — but it only works on
Ghostty and only shows saved sessions.

## Solution

A focused **Spotlight-style picker** (`crex pop`) that shows saved layouts and
templates in one unified view. Activated via a shell hook (`Ctrl+G` default),
it works on both cmux and Ghostty with no terminal-specific configuration.

## Differentiators vs gtab

| Aspect | gtab | crex pop |
|--------|------|----------|
| Backends | Ghostty only | cmux + Ghostty (shell binding is universal) |
| Activation | Terminal keybinding | Shell hook (universal) + optional terminal binding |
| UI | Full TUI | Focused picker (<100ms, alternate screen) |
| Content | Saved sessions only | Layouts + templates in one view |
| Scriptable | No | `crex pop <name>`, `crex pop --last` |

## Picker UI

```
┌──────────────────────────────────────────┐
│  crex > _                                │
│                                          │
│  LAYOUTS                                 │
│  [1]  > morning      3 tabs  May 28     │
│  [2]    afternoon    2 tabs  May 27     │
│                                          │
│  TEMPLATES                               │
│  [3]    ⧉  ide       editor+git+term     │
│  [4]    🤖  claude    claude code setup   │
│                                          │
│  ↵ launch · type to filter · esc quit    │
└──────────────────────────────────────────┘
```

### Behavior

- **Two sections**: saved layouts (from store) + gallery templates (from embed.FS)
- **Fuzzy filter**: type to filter across both sections simultaneously
- **Number keys**: 1-9 for instant selection without arrow navigation
- **Arrow keys**: up/down to navigate, Enter to launch
- **Escape**: exits cleanly (bubbletea alternate screen, no residue)
- **Launch action**: restore for saved layouts, template use for templates

### Startup performance target: <100ms

- Layout list: file listing from `~/.config/crex/layouts/` (fast)
- Template list: already in memory via `embed.FS`
- No `client.Tree()` call — no live workspace section
- No network calls

## Scriptable Shortcuts

`crex pop` also works as a direct-action command without the picker:

```sh
crex pop morning        # restore "morning" layout immediately
crex pop ide .          # apply IDE template to current directory
crex pop --last         # restore most recently saved layout
```

When arguments are provided, no TUI is shown — the action executes and exits.

## Activation: 3-Tier Approach

### Tier 1: Shell Hook (universal, default)

`crex setup` detects the user's shell and offers to add a `Ctrl+G` binding:

**zsh** (`~/.zshrc`):
```sh
bindkey -s '^G' 'crex pop\n'
```

**bash** (`~/.bashrc`):
```sh
bind '"\C-g": "crex pop\n"'
```

**fish** (`~/.config/fish/config.fish`):
```sh
bind \cg 'crex pop; commandline -f repaint'
```

The key is configurable during `crex setup`. The shell hook line includes a
comment marker (`# crex-pop-hook`) for idempotent add/remove.

### Tier 2: Terminal-Native Binding (optional)

`crex setup` also offers backend-specific keybindings:

- **Ghostty**: adds `keybind = ctrl+g=launch:crex pop` to Ghostty config
- **cmux**: registers a cmux keyboard shortcut via `cmux set-hook`

These are opt-in, offered during setup alongside the shell hook.

### Tier 3: Manual / DIY

`crex pop` is a standard CLI command. Users can bind it to any key in any
tool. Documented in help text and README.

## Architecture

### New files

| File | Responsibility |
|------|---------------|
| `cmd/pop.go` | Cobra command: parse args, dispatch to picker or direct action |
| `internal/tui/pop.go` | PopModel: bubbletea model for the picker UI |
| `internal/tui/pop_test.go` | Unit tests for PopModel (filtering, selection, rendering) |
| `internal/setup/shellhook.go` | Shell detection, hook line generation, rc file modification |

### Modified files

| File | Change |
|------|--------|
| `internal/setup/setup.go` | Add shell hook step to setup wizard |
| `cmd/root.go` | Register `popCmd` |

### PopModel (bubbletea)

```go
type PopModel struct {
    filter    string          // current filter text
    items     []PopItem       // all items (layouts + templates)
    filtered  []PopItem       // items matching filter
    cursor    int             // selected index in filtered list
    chosen    *PopItem        // set when user picks, signals quit
    width     int
    height    int
}

type PopItem struct {
    Kind        string // "layout" or "template"
    Name        string
    Description string
    Icon        string
    Meta        string // "3 tabs  May 28" or "editor+git+term"
}
```

- Uses `tea.WithAltScreen` for clean overlay
- `Update` handles: key input for filter, arrow keys, number keys 1-9, Enter, Escape
- `View` renders the two-section list with highlight on cursor
- Returns `chosen` item to the caller; caller dispatches restore or template use

### Shell Hook Generation

```go
// DetectShell returns "zsh", "bash", or "fish" based on $SHELL.
func DetectShell() string

// HookLine returns the shell-specific binding line for the given key.
func HookLine(shell, key string) string

// InstallHook appends the hook to the shell rc file if not already present.
// Uses the comment marker "# crex-pop-hook" for idempotent operations.
func InstallHook(shell, key string) error

// UninstallHook removes the hook line from the shell rc file.
func UninstallHook(shell string) error
```

### cmd/pop.go Flow

```
crex pop              → launch picker, restore/use selection
crex pop <name>       → match against layouts first, then templates; execute immediately
crex pop <name> <dir> → template use with directory
crex pop --last       → find most recent layout by saved_at, restore immediately
```

## Testing

### Unit tests (pop_test.go)

- Filter: empty filter shows all items, typing narrows results, fuzzy matching
- Selection: cursor movement, number key selection, Enter confirms
- Rendering: correct section headers, highlight on cursor, item count
- Edge cases: empty layout list, empty filter result, single item

### Shell hook tests (shellhook_test.go)

- Detect zsh/bash/fish from $SHELL
- Generate correct hook line for each shell
- Install: appends to rc file, idempotent (doesn't duplicate)
- Uninstall: removes hook line, preserves rest of file

### Integration test

- `crex pop --last` with a saved layout → verify it triggers restore
- `crex pop ide .` → verify it triggers template use

## What We Don't Build (YAGNI)

- No live workspace section (avoids Tree() latency)
- No window frame sync (cmux handles this)
- No directory bookmarks (Blueprints cover this)
- No Ghostty-specific popup API (shell hook is universal)

## Files Changed Summary

| File | Type |
|------|------|
| `cmd/pop.go` | new |
| `internal/tui/pop.go` | new |
| `internal/tui/pop_test.go` | new |
| `internal/setup/shellhook.go` | new |
| `internal/setup/shellhook_test.go` | new |
| `internal/setup/setup.go` | modify |
| `cmd/root.go` | modify |
