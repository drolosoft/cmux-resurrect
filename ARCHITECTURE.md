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
      3. for each pane[i>0]: Backend.NewSplit(direction, ref)
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
    NewSplit(direction, workspaceRef string) (string, error)  // returns new surface ref
    FocusPane(paneRef, workspaceRef string) error
    Send(workspaceRef, surfaceRef, text string) error
    CloseWorkspace(ref string) error
    PinWorkspace(ref string) error
}
```

Implementations: `CLIClient` (cmux via exec) and `GhosttyClient` (Ghostty via AppleScript). New backends can be added without touching business logic.

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
| Split direction not captured from live state | Both | Neither backend exposes split direction; defaults to "right", editable in TOML |
| Pane CWD not per-pane | Both | Both backends return one CWD per workspace, not per pane |
| Autosave rotation not implemented | Both | `max_autosaves` config field exists but rotation logic is pending |
| launchd auto-save uses cmux socket | cmux only | Ghostty users run `crex watch` directly (no socket activation) |
