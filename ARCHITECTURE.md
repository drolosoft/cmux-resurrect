# 🏗️ Architecture — crex (cmux-resurrect)

crex supports multiple terminal backends through the `Backend` interface in `internal/client/`. The backend is auto-detected at startup: cmux (via `CMUX_SOCKET_PATH`) or Ghostty (via AppleScript probe).

> Internal documentation for contributors and anyone wanting to understand or extend the project.

## How It Works

```
┌──────────────────┐                    ┌──────────────────┐
│  🖥️ cmux          │ ◄── cmux CLI ───► │                  │
│  (terminal mux)  │                    │  ⚡ crex CLI     │
└──────────────────┘                    │  (Go binary)     │
                                        │                  │
┌──────────────────┐                    │  auto-detects    │
│  👻 Ghostty       │ ◄── AppleScript ► │  active backend  │
│  (terminal)      │                    └────────┬─────────┘
└──────────────────┘                             │
                                       ┌─────────┼─────────┐
                                       ▼                   ▼
                              ┌──────────────┐    ┌──────────────┐
                              │ 💾 Layouts    │    │ 📝 Workspace  │
                              │ (TOML files) │    │ (Blueprint)   │
                              │ ~/.config/   │    │ Obsidian-ok   │
                              │ crex/layouts │    │ (.md)         │
                              └──────────────┘    └──────────────┘
```

## Flows

### 💾 Save

```
Backend.Tree() → parse TreeResponse
  → for each workspace: Backend.SidebarState() → parse CWD
    → build model.Layout
      → serialize to TOML
        → write to ~/.config/crex/layouts/<name>.toml
```

> cmux backend: `cmux tree --json`, `cmux sidebar-state`. Ghostty backend: AppleScript queries.

### 🔄 Restore

```
read TOML → parse model.Layout
  → Backend.Ping() (verify backend is running)
    → for each workspace (ordered by index):
      1. Backend.NewWorkspace(cwd)
      2. Backend.RenameWorkspace(ref, title)
      3. for each pane[i>0]: Backend.NewSplit(direction, workspaceRef, surfaceRef)
      4. for each pane with command: Backend.Send(ref, "command\n")
      5. Backend.FocusPane(if pane.focus=true)
    → Backend.SelectWorkspace (restore active tab)
```

### 📥 Import from Blueprint

Parses Workspace Blueprint (.md) → resolves templates into pane definitions → creates any workspaces that don't already exist in the running terminal.

### 📤 Export to Blueprint

The reverse of import: captures live state and writes it to the Workspace Blueprint with default templates.

## Package Structure

```
cmd/                    → Cobra CLI commands
internal/
  client/               → Backend interface + CLIClient (cmux) + GhosttyClient (Ghostty)
  config/               → TOML config loading, default paths
  gallery/              → Built-in template gallery (embedded .md files)
  model/                → Layout, Workspace, Pane structs + merge logic
  mdfile/               → Workspace Blueprint (.md) parser + writer
  orchestrate/          → Business logic: save, restore, import, watch, export
  persist/              → TOML file store (read/write layouts)
```

## Template Gallery

### Package: `internal/gallery/`

The gallery provides 16 built-in workspace templates (9 layouts, 7 workflows) embedded directly in the binary.

### Embedding Strategy

Templates are stored as individual `.md` files in `internal/gallery/templates/` and compiled into the binary via `//go:embed`:

```go
//go:embed templates/*.md
var templatesFS embed.FS
```

Each file uses YAML frontmatter (`name`, `category`, `icon`, `description`, `tags`) followed by standard Blueprint pane syntax. The `ensureLoaded()` function lazily parses all embedded files exactly once using `sync.Once`.

### Three-Tier Resolution

When resolving a template name (e.g., during `import-from-md` or `template use`), the `ResolveTemplate` function checks three tiers:

```
1. User Blueprint  →  templates in workspaces.md
2. Gallery         →  built-in embedded templates
3. Fallback        →  single focused terminal pane
```

User-defined templates always win. The `template customize` command copies a gallery template into the user's Blueprint, promoting it to tier 1.

### FocusTarget Mechanism

Some layouts (e.g., `quad`) need splits to target specific existing panes rather than the currently focused pane. The `@focus=N` annotation in gallery template files sets `FocusTarget` on a pane — the orchestrator focuses pane N before creating the split. This is a gallery-only feature; user Blueprint syntax does not support it.

### ASCII Diagrams

The `cmd/template_show.go` file contains hardcoded diagram rendering functions for each layout shape (`singleDiagram`, `twoPaneHorizontalDiagram`, `asideDiagram`, `quadDiagram`, etc.). The `renderDiagram` dispatcher maps template names to the appropriate function.

## Key Design Decisions

| Decision | Choice | Reason |
|----------|--------|--------|
| IPC | CLI (`exec cmux`) | Most stable API surface; interface abstraction allows future socket swap |
| Split direction | Default "right", editable | cmux tree JSON doesn't expose split direction |
| Merge on save | Preserve user edits | split, command, description fields kept from existing TOML |
| Atomic writes | temp file + rename | Prevents corruption on crash |
| Autosave dedup | SHA-256 content hash | Avoids writing identical files |
| Error isolation | Per-workspace | Failure in workspace 3 doesn't block workspace 4 |

## Client Interface

```go
type Backend interface {
    Ping() error
    Tree() (*TreeResponse, error)
    SidebarState(workspaceRef string) (*SidebarState, error)
    ListWorkspaces() ([]WorkspaceInfo, error)
    NewWorkspace(opts NewWorkspaceOpts) (string, error)
    RenameWorkspace(ref, title string) error
    SelectWorkspace(ref string) error
    NewSplit(direction, workspaceRef, surfaceRef string) (string, error) // new surface ref; surfaceRef targets a specific surface
    NewPane(opts NewPaneOpts) (string, error)                            // terminal/browser pane, supports URL
    NewSurface(paneRef, workspaceRef string) (string, error)             // extra tab in a pane; cmux-only (Ghostty: ErrNotSupported)
    FocusPane(paneRef, workspaceRef string) error
    Send(workspaceRef, surfaceRef, text string) error
    PinWorkspace(ref string) error
    UnpinWorkspace(ref string) error
    CloseWorkspace(ref string) error
    DryRunFormatter() DryRunFormatter
}

// Optional capabilities — detected via type assertion; backends that don't
// implement them fall back to default behavior (equal splits, no resize).
type PaneGeometryProvider interface {
    PaneList(workspaceRef string) (*PaneListResponse, error) // infer split direction/ratio on save
}
type PaneResizer interface {
    ResizePane(opts ResizePaneOpts) error                    // apply saved split ratios on restore
}

// GroupManager reads and rebuilds cmux sidebar workspace groups (cmux 0.64
// and later). Only CLIClient implements it; Ghostty has no groups.
type GroupManager interface {
    ListGroups(windowRef string) ([]WorkspaceGroup, error)
    CreateGroup(name string, fromRefs []string) (WorkspaceGroup, error)
    SetGroupAnchor(groupRef, workspaceRef string) error
    AddToGroup(groupRef, workspaceRef string) error
    RenameGroup(groupRef, name string) error
    CollapseGroup(groupRef string) error
    PinGroup(groupRef string) error
    SetGroupColor(groupRef, hex string) error
    SetGroupIcon(groupRef, symbol string) error
}
```

Implementations: `CLIClient` (cmux via exec) and `GhosttyClient` (Ghostty via AppleScript; also drives cmux-applescript). New backends can be added without touching business logic — implement the core interface and opt into the optional capabilities as needed.

## Workspace Groups

cmux 0.64 can group workspaces in the sidebar. A group owns no panes: its header is one of its member workspaces, the anchor. crex keeps groups only on cmux, through the optional `GroupManager` interface above, and detects it by type assertion. In dry-run mode the matching formatter is `GroupDryRunFormatter` in `internal/client/dryrun.go`, implemented only by `CmuxDryRun`.

**Model.** `Layout.Groups` (`[[group]]` in TOML) lists the groups in sidebar order, each with `name`, `collapsed`, `pinned` and optional `color` (`"#RRGGBB"`) and `icon` (an SF Symbol name). Each workspace points back with `Workspace.Group` (`group`, empty for a loose workspace) and `Workspace.GroupAnchor` (`group_anchor = true` on the anchor). A layout without groups writes none of these keys, so it is identical to a layout saved before groups existed, and an older crex ignores them.

**Save.** Save calls `ListGroups(win.Ref)` on the caller's window and marks every workspace with its group and its anchor. If two live groups share a name, the second is saved as `<name> (2)`. If listing groups fails, save prints a warning and keeps the groups of the existing layout. A change that touches only groups still raises the revision. An anchor still titled with cmux's "Group N" placeholder is stored under the group name (`model.AnchorTitle`). Dedupe drops a workspace only when it has no tty and a live workspace with the same group and title exists, so two real workspaces that share a title are both kept. The pane crex runs in is never stored with crex's own command (`detect.IsCrexCommand`).

**Restore.** Workspaces are created in index order. The first workspace of each group is its anchor: crex creates it with its panes, creates the group from it, sets it as the anchor and closes the extra anchor that `cmux workspace-group create --from` always adds. The other members are created inside the group with `--group <ref> --group-placement end`, and at the end crex applies collapse, color, icon and pin (pins in reverse order, because pinning moves a group to the top). Anchors are created with the group name as title, older layouts with a "Group N" anchor included; when restore joins a live group, a "Group N" anchor is renamed and the group is expanded or unpinned to match the layout. On Ghostty the workspaces come back loose with one entry under "Warnings", never an error.

**Display.** `crex show`, `crex now` and the TUI list workspaces under their group through one helper, `model.GroupSections` (reached through `Layout.Sections()`), so the CLI and the TUI cannot drift. For a live window, `orchestrate.LiveSections` in `internal/orchestrate/livesections.go` builds the same sections from `ListGroups` and falls back to the flat listing when the backend has no groups. The code is in `internal/model/sections.go`.

## Ref Detection Strategy

cmux's `new-workspace` and `new-split` commands do not return the ref of the newly created resource. To discover it, the CLIClient uses a snapshot-and-diff strategy:

1. List existing workspace/surface refs before creation
2. Execute the creation command
3. Poll `list-workspaces` or `tree` until a new ref appears (not in the original snapshot)
4. Return the new ref

Polling uses `client.PollInterval` (100ms) with deadlines of 5s for workspaces and 3s for splits.

## Timing Budget

cmux processes commands asynchronously. Orchestrators insert deliberate pauses between operations to avoid race conditions:

| Constant | Value | Used after |
|----------|-------|-----------|
| `DelayAfterCreate` | 300ms | `cmux new-workspace` |
| `DelayAfterSelect` | 100ms | `cmux select-workspace` |
| `DelayAfterSplit` | 500ms | `cmux new-split` |
| `DelayBeforeRename` | 500ms | Before `cmux rename-workspace` |
| `DelayAfterClose` | 100ms | `cmux close-workspace` |
| `DelayAfterCloseAll` | 300ms | After a batch of workspace closes |

These values are empirically determined. Changing them may cause race conditions on slower machines.

## Blueprint Tail Preservation

The Markdown parser preserves everything after the Templates section as opaque text. Users may have documentation, notes, or other sections in their Blueprint file — these are written back verbatim on save.

## Known Limitations

| Limitation | Backends | Reason |
|-----------|----------|--------|
| Split geometry inferred as a right-chain | Ghostty | Ghostty's AppleScript API exposes no pane pixel frames; cmux captures exact geometry from `pane.list` |
| Autosave rotation not implemented | Both | `max_autosaves` config field is reserved but rotation logic is pending |
| launchd auto-save uses cmux socket | cmux only | Ghostty users run `crex watch` directly (no socket activation) |
