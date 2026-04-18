# Template Gallery — Design Spec

**Date:** 2026-04-15
**Status:** Approved
**Scope:** Built-in template gallery with layout and workflow templates, CLI commands, documentation

---

## Overview

Ship a curated gallery of 16 pre-built templates embedded in the crex binary. Two categories:

- **Layout templates** (9): Pure pane geometry — no commands, language-agnostic
- **Workflow templates** (7): Opinionated tool combos for common terminal activities

Inspired by termcn.dev's gallery concept (shadcn model: "copy it, it's yours"), adapted to crex's domain of terminal workspace layouts.

---

## Template Catalog

### Layout Templates (9)

| Icon | Name | Panes | Shape | Split Sequence |
|------|------|-------|-------|----------------|
| `▥` | `cols` | 2 | Side-by-side columns | main → right |
| `▤` | `rows` | 2 | Stacked rows | main → down |
| `◧` | `sidebar` | 2 | Main with sidebar | main(focused) → right |
| `⊤` | `shelf` | 3 | Big top, two bottom | main → down → right |
| `⊢` | `aside` | 3 | Big left, two stacked right | main → right → down |
| `Ⅲ` | `triple` | 3 | Three columns | main → right → right |
| `⊠` | `quad` | 4 | 2×2 grid | main → right → focus[0]+down → focus[1]+down |
| `◱` | `dashboard` | 4 | Big top, three bottom | main → down → right → right |
| `⧉` | `ide` | 4 | File tree + editor + console + tools | main → right → down → right |

#### Layout ASCII Diagrams

```
cols             rows             sidebar          shelf
┌──────┬──────┐  ┌────────────┐  ┌────────┬────┐  ┌────────────┐
│      │      │  │     A      │  │        │    │  │     A      │
│  A   │  B   │  ├────────────┤  │   A    │ B  │  ├──────┬─────┤
│      │      │  │     B      │  │        │    │  │  B   │  C  │
└──────┴──────┘  └────────────┘  └────────┴────┘  └──────┴─────┘

aside            triple           quad             dashboard
┌───────┬────┐   ┌────┬────┬────┐ ┌──────┬──────┐  ┌─────────────┐
│       │ B  │   │    │    │    │ │  A   │  B   │  │      A      │
│   A   ├────┤   │ A  │ B  │ C  │ ├──────┼──────┤  ├────┬────┬───┤
│       │ C  │   │    │    │    │ │  C   │  D   │  │ B  │ C  │ D │
└───────┴────┘   └────┴────┴────┘ └──────┴──────┘  └────┴────┴───┘

ide
┌────┬──────────────┐
│    │      B       │
│ A  ├────────┬─────┤
│    │   C    │  D  │
└────┴────────┴─────┘
```

#### Quad — FocusTarget Sequence

The `quad` layout requires `FocusTarget` to redirect splits to non-focused panes:

1. Pane A (main, focused)
2. `split right` → Pane B (focus moves to B)
3. `FocusPane(A)`, `split down` → Pane C (focus moves to C)
4. `FocusPane(B)`, `split down` → Pane D (focus moves to D)
5. Final focus → A

Pane C has `FocusTarget=0` (A), Pane D has `FocusTarget=1` (B).

### Workflow Templates (7)

| Icon | Name | Panes | Layout | Commands | Description |
|------|------|-------|--------|----------|-------------|
| `🤖` | `claude` | 3 | sidebar+bottom | `lazygit` / `claude --dangerously-skip-permissions --continue` / shell | AI pair-programming |
| `💻` | `code` | 3 | sidebar+bottom | shell / `lazygit` / `watch -n 5 'ls -lt \| head -20'` | General-purpose coding |
| `🔭` | `explore` | 2 | cols | shell / `git log --oneline --graph -20` | Navigate a codebase |
| `📊` | `system` | 2 | cols | `htop` / `df -h` | Monitor system health |
| `📜` | `logs` | 3 | sidebar+bottom | `tail -f /var/log/system.log` / `dmesg -T \| tail -30` / shell | Tail multiple log streams |
| `🌐` | `network` | 2 | cols | shell / `curl -s ifconfig.me && echo` | Debug connectivity |
| `📟` | `single` | 1 | single | shell | Minimal terminal |

#### Workflow Pane Layouts

```
claude                    code                      explore
┌──────────┬───────────┐  ┌──────────┬───────────┐  ┌──────────┬───────────┐
│          │ claude     │  │          │           │  │          │ git log   │
│ lazygit  │ (focused)  │  │  shell   │  lazygit  │  │  shell   │ --graph   │
│          ├───────────┤  │ (focused) ├───────────┤  │ (focused) │           │
│          │ console   │  │          │ watcher   │  └──────────┴───────────┘
└──────────┴───────────┘  └──────────┴───────────┘

system                    logs                      network
┌──────────┬───────────┐  ┌──────────┬───────────┐  ┌──────────┬───────────┐
│          │           │  │ tail -f  │ dmesg     │  │          │ curl      │
│  htop    │  df -h    │  │ syslog   │           │  │  shell   │ ifconfig  │
│          │           │  │ (focused) ├───────────┤  │ (focused) │           │
└──────────┴───────────┘  │          │ console   │  └──────────┴───────────┘
                          └──────────┴───────────┘

single
┌─────────────────────────┐
│  shell (focused)        │
└─────────────────────────┘
```

---

## Data Model Changes

### Template struct — add metadata fields

```go
type Template struct {
    Name        string
    Category    string        // "layout" or "workflow"
    Icon        string        // ▥, 🤖, etc.
    Description string        // one-line summary
    Tags        []string      // for filtering: ["ai", "git"], ["monitoring"], etc.
    Panes       []TemplatePan
}
```

### TemplatePan struct — add FocusTarget and Name

```go
type TemplatePan struct {
    Enabled     bool
    IsMain      bool
    Split       string  // "right", "down", "left", "up"
    Type        string  // "terminal", "browser"
    Command     string  // command in backticks
    Focus       bool    // gets final focus after all panes created
    FocusTarget int     // pane index to focus BEFORE this split (-1 = no refocus)
    Name        string  // display label: "main", "console", "git"
}
```

**Backward compatibility:**
- New fields on `Template` default to `""` / `nil` for existing templates
- `FocusTarget` defaults to `-1` in the parser (not Go's zero `0`, since `0` is a valid pane index)
- `Name` defaults to `""` — display-only, no behavioral change
- User-facing `workspaces.md` format is unchanged
- TOML layout files (save/restore) are unchanged

---

## Embedding Strategy

### File Structure

```
internal/gallery/
  embed.go              // go:embed directive + FS
  gallery.go            // Registry: List(), Get(), ListByCategory(), Tags()
  gallery_test.go       // Parse all templates, validate, test resolution
  templates/
    layout-cols.md
    layout-rows.md
    layout-sidebar.md
    layout-shelf.md
    layout-aside.md
    layout-triple.md
    layout-quad.md
    layout-dashboard.md
    layout-ide.md
    workflow-claude.md
    workflow-code.md
    workflow-explore.md
    workflow-system.md
    workflow-logs.md
    workflow-network.md
    workflow-single.md
```

### Template File Format

Each `.md` file has YAML frontmatter + existing Blueprint pane syntax:

```markdown
---
name: claude
category: workflow
icon: 🤖
description: AI pair-programming with Claude Code
tags: [ai, git, development]
---
### claude
- [x] main terminal: `lazygit`
- [x] split right: `claude --dangerously-skip-permissions --continue` (focused)
- [x] split down:
```

The body after the frontmatter uses the **exact same pane syntax** as user-defined Blueprint templates. The existing `parseTemplatePaneLine()` function in `mdfile/parse.go` parses it — zero changes to the user-facing parser.

The frontmatter parser is new but minimal: split on `---`, parse key-value pairs.

### Three-Tier Template Resolution

When resolving a template name:

1. **User-defined** (in `workspaces.md` `## Templates` section) — highest priority
2. **Built-in gallery** (embedded via `go:embed`) — fallback
3. **Single terminal pane** — last resort

The existing `model.WorkspaceFile.ResolveTemplate()` stays unchanged (user-only lookup). The three-tier resolution lives in the `gallery` package to avoid a circular dependency (`model` cannot import `gallery` because `gallery` imports `model`):

```go
// gallery.ResolveTemplate checks user templates first, then the built-in gallery, then falls back.
func ResolveTemplate(wf *model.WorkspaceFile, name string) []model.Pane {
    // 1. User-defined takes priority
    if panes := wf.ResolveTemplate(name); len(panes) > 0 && panes[0].Type != "" {
        return panes
    }
    // 2. Built-in gallery
    if tmpl, ok := Get(name); ok {
        return buildPanes(tmpl)
    }
    // 3. Fallback
    return []model.Pane{{Type: "terminal", Focus: true}}
}
```

Callers in `orchestrate/import.go` and `cmd/template_use.go` use `gallery.ResolveTemplate()` instead of `wf.ResolveTemplate()` directly.

### Customization: Fork-on-Write

`crex template customize <name>` copies the built-in template into the user's `workspaces.md` `## Templates` section. From then on, the user's copy wins (tier 1 > tier 2). This is the shadcn model.

No "extend" or "inherit" mechanism. Fork is the right primitive.

---

## CLI Commands

### `crex template` command group (alias: `crex tpl`)

New files:
- `cmd/template.go` — command group definition
- `cmd/template_list.go` — list subcommand
- `cmd/template_show.go` — show subcommand
- `cmd/template_use.go` — use subcommand
- `cmd/template_customize.go` — customize subcommand

### `crex template list`

```
  LAYOUTS

  ▥  cols         [2]  Side-by-side columns
  ▤  rows         [2]  Stacked rows
  ◧  sidebar      [2]  Main with sidebar
  ⊤  shelf        [3]  Big top, two bottom
  ⊢  aside        [3]  Big left, two stacked right
  Ⅲ  triple       [3]  Three columns
  ⊠  quad         [4]  2×2 grid
  ◱  dashboard    [4]  Big top, three bottom
  ⧉  ide          [4]  Full IDE layout

  WORKFLOWS

  🤖  claude      [3]  AI pair-programming
  💻  code        [3]  General-purpose coding
  🔭  explore     [2]  Navigate a codebase
  📊  system      [2]  Monitor system health
  📜  logs        [3]  Tail multiple streams
  🌐  network     [2]  Debug connectivity
  📟  single      [1]  Minimal terminal

  16 templates (9 layouts, 7 workflows)
```

Flags:
- `--layout` — show only layout templates
- `--workflow` — show only workflow templates
- `--tag <tag>` — filter by tag

### `crex template show <name>`

```
  🤖 claude — AI pair-programming with Claude Code

  ┌──────────┬─────────────────────────┐
  │          │                         │
  │ lazygit  │  claude --continue      │
  │          │  --dangerously-skip-    │
  │          │  permissions (focused)  │
  │          ├─────────────────────────┤
  │          │                         │
  │          │  console                │
  │          │                         │
  └──────────┴─────────────────────────┘

  Category:  workflow
  Panes:     3
  Splits:    main → right → down
```

The ASCII diagram is generated programmatically from the split definitions using lipgloss box rendering.

### `crex template use <name> [path]`

```
  $ crex template use claude ~/projects/my-app

  🤖 Creating workspace from 'claude'...
  ✅ Workspace created: 🤖 my-app
```

Creates a cmux workspace directly from a template. Does NOT add to `workspaces.md` — it's ephemeral.

Flags:
- `--name <title>` — custom workspace title (default: directory basename)
- `--icon <emoji>` — custom icon (default: template icon for workflows, none for layouts)
- `--dry-run` — preview commands without creating
- `--add` — also add to Workspace Blueprint for persistence
- `--pin` — pin the workspace

### `crex template customize <name>`

```
  $ crex template customize claude

  Copied 'claude' to your Workspace Blueprint.
  Your copy now takes priority over the built-in.
  Edit with: crex edit
```

Copies the built-in template definition into the user's `workspaces.md` `## Templates` section.

### Shell Completions

Update `cmd/completion_helpers.go`:
- `crex template show <TAB>` → completes all template names
- `crex template use <TAB>` → completes all template names
- `crex template customize <TAB>` → completes built-in template names only
- `crex workspace add --template <TAB>` → includes gallery templates (currently only hardcoded `dev/go/single/monitor`)

---

## Orchestrator Changes

### Import flow (`internal/orchestrate/import.go`)

The `Importer.ImportFromMD()` method already calls `wf.ResolveTemplate(p.Template)`. Once `ResolveTemplate` falls through to the gallery, all 16 templates work automatically with `crex import-from-md`. No changes needed to the import flow itself.

### Restore flow (`internal/orchestrate/restore.go`)

Add `FocusTarget` handling in `restoreWorkspace()`:

```go
for i, pane := range ws.Panes {
    if i == 0 { /* existing first-pane handling */ }

    // NEW: Focus a specific pane before splitting (for quad, etc.)
    if pane.FocusTarget >= 0 {
        targetRef := surfaceRefs[pane.FocusTarget]
        _ = r.Client.FocusPane(targetRef, ref)
        time.Sleep(DelayAfterSelect)
    }

    direction := pane.Split
    if direction == "" { direction = "right" }
    surfaceRef, err := r.Client.NewSplit(direction, ref)
    // ... existing error handling and command sending ...
}
```

### Template Use flow (NEW)

New `TemplateUser` orchestrator in `internal/orchestrate/template_use.go`:

Takes a template name + CWD path, resolves the template, creates a workspace with all splits and commands. Reuses the same split/command/rename logic as `Importer` but for a single workspace from a gallery template rather than a Blueprint file.

---

## DefaultTemplates Migration

The existing `DefaultTemplates()` in `mdfile/write.go` returns 4 hardcoded templates (`dev`, `go`, `single`, `monitor`). These migrate into the gallery as workflow templates.

`DefaultTemplates()` is preserved for backward compatibility — it's called when creating a new `workspaces.md` file. It will return `dev` and `single` to seed the user's file with a minimal working set. The full gallery (all 16 templates) is always available via three-tier resolution regardless of what's in the user's file.

---

## Documentation Updates

### New files

- `docs/templates.md` — Full template gallery page with ASCII diagrams, descriptions, and usage examples for all 16 templates
- `docs/template-authoring.md` — Guide for creating custom templates and contributing to the gallery

### Updated files

- `README.md`:
  - Add "Template Gallery" section between "Workspace Blueprints" and "Why crex?"
  - Add `template` to the help output in `styledHelp()`
  - Update the comparison table row about templates
  - Add `docs/templates.md` to the documentation table
- `docs/commands.md` — Add `crex template` command group reference
- `docs/blueprint.md` — Reference gallery templates, explain resolution order
- `ARCHITECTURE.md` — Document gallery package, embedding strategy, resolution chain

### Drolosoft website changes needed

- `/cmux-resurrect.html` — Add template gallery feature description, possibly with visual examples
- Developer Tools section on homepage — Mention template gallery availability
- These changes are for the Drolosoft agent to implement; this spec documents what content needs updating

---

## Testing Strategy

### Unit tests (`internal/gallery/gallery_test.go`)

- **Parse all 16 templates**: Verify every embedded `.md` file parses without error
- **Validate frontmatter**: Name, category, icon, description present and non-empty
- **Validate pane definitions**: At least 1 pane per template, correct split directions
- **Template name uniqueness**: No duplicate names across all templates
- **Icon uniqueness**: No duplicate icons across templates within the same category
- **Category validation**: Only "layout" or "workflow" values
- **Layout templates have no commands**: Verify all layout template panes have empty Command fields
- **Workflow templates**: Verify workflow templates with commands have non-empty Command fields where expected
- **FocusTarget validation**: Only `quad` has non-negative FocusTarget values; all others are -1
- **Resolution tests**: User-defined > gallery > fallback priority chain

### Unit tests (`cmd/template_*_test.go`)

- **List command**: Outputs all templates, grouped by category, with icons and pane counts
- **List with flags**: `--layout`, `--workflow`, `--tag` filter correctly
- **Show command**: Outputs ASCII diagram, metadata, pane details for each template
- **Show unknown**: Error message for non-existent template name
- **Customize command**: Copies template into workspaces.md, verify file content
- **Customize idempotent**: Running twice doesn't duplicate the template
- **Use dry-run**: Outputs correct cmux commands for each template
- **Use with --name and --icon**: Custom title and icon applied correctly

### Integration tests (require cmux running)

- **Use command**: `crex template use cols /tmp/test` creates a 2-pane workspace
- **Use quad**: `crex template use quad /tmp/test` creates 4 panes with correct FocusTarget handling
- **Use claude**: `crex template use claude /tmp/test` creates 3 panes with commands
- **Customize + import**: Customize a template, modify it, import — user copy takes priority

### Model tests (`internal/model/project_test.go`)

- **ResolveTemplate with gallery fallback**: Template not in user file resolves from gallery
- **ResolveTemplate user priority**: User-defined template overrides same-named gallery template
- **FocusTarget defaults**: New TemplatePan has FocusTarget = -1

### ASCII diagram renderer tests

- **Diagram generation**: Each of the 9 layout shapes renders a correct box-drawing diagram
- **Labeled diagrams**: Workflow templates render with pane names/commands in the boxes
- **Edge cases**: Single pane, maximum panes (4)

---

## File Inventory

### New files to create

| File | Purpose |
|------|---------|
| `internal/gallery/embed.go` | `go:embed` directive and FS |
| `internal/gallery/gallery.go` | Registry: List, Get, ListByCategory, Tags, parsing |
| `internal/gallery/gallery_test.go` | Parse, validate, resolve all templates |
| `internal/gallery/templates/*.md` | 16 template files (9 layout + 7 workflow) |
| `cmd/template.go` | `crex template` command group |
| `cmd/template_list.go` | `crex template list` subcommand |
| `cmd/template_show.go` | `crex template show` subcommand + ASCII renderer |
| `cmd/template_use.go` | `crex template use` subcommand |
| `cmd/template_customize.go` | `crex template customize` subcommand |
| `cmd/template_test.go` | Tests for all template subcommands |
| `internal/orchestrate/template_use.go` | TemplateUser orchestrator |
| `docs/templates.md` | Full gallery documentation |
| `docs/template-authoring.md` | Custom template guide |

### Existing files to modify

| File | Change |
|------|--------|
| `internal/model/project.go` | Add Category, Icon, Description, Tags to Template; FocusTarget, Name to TemplatePan |
| `internal/model/project_test.go` | Add tests for new fields, FocusTarget defaults, gallery resolution |
| `internal/mdfile/parse.go` | Initialize FocusTarget to -1 in `parseTemplatePaneLine` |
| `internal/mdfile/write.go` | Update DefaultTemplates to reference gallery; keep backward compat |
| `internal/orchestrate/restore.go` | Add FocusTarget handling in restoreWorkspace |
| `internal/orchestrate/import.go` | Add FocusTarget handling in ImportFromMD |
| `cmd/root.go` | (no change — template cmd self-registers via init()) |
| `cmd/style.go` | Add styles for template list/show output |
| `cmd/completion_helpers.go` | Add completeTemplateNames, update template flag completion |
| `cmd/ws_add.go` | Update --template flag completion to include gallery |
| `README.md` | Add template gallery section, update help, docs table |
| `docs/commands.md` | Add template command group reference |
| `docs/blueprint.md` | Reference gallery, explain resolution order |
| `ARCHITECTURE.md` | Document gallery package |

---

## Non-Goals

- **Template versioning**: Templates are versioned implicitly by the crex binary version. No explicit version field.
- **Network-based template registry**: Templates are embedded. No downloads, no external repos.
- **Template inheritance/extension**: Fork-on-write only. No "extend base template" mechanism.
- **Proportional pane sizing**: cmux doesn't expose pane size control. All splits are equal.
- **Per-pane CWD in templates**: cmux supports one CWD per workspace. Templates use the workspace CWD for all panes.
