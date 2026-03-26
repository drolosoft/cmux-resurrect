# 🏗️ Architecture — cmux-resurrect

> Internal documentation for contributors and anyone wanting to understand or extend the project.

## How It Works

```
┌──────────────────┐         ┌──────────────────┐
│  🖥️ cmux          │ ◄─────► │  ⚡ crex CLI     │
│  (Ghostty mux)   │  cmux   │  (Go binary)     │
└──────────────────┘  CLI    └────────┬─────────┘
                      calls           │
                            ┌─────────┼─────────┐
                            ▼                   ▼
                   ┌──────────────┐    ┌──────────────┐
                   │ 💾 Layouts    │    │ 📝 Workspace  │
                   │ (TOML files) │    │ (Blueprint)   │
                   │ ~/.config/   │    │ Obsidian-ok    │
                   │ crex/layouts│    │ (.md)        │
                   └──────────────┘    └──────────────┘
```

## Flows

### 💾 Save

```
cmux tree --json → parse TreeResponse
  → for each workspace: cmux sidebar-state → parse CWD
    → build model.Layout
      → serialize to TOML
        → write to ~/.config/crex/layouts/<name>.toml
```

### 🔄 Restore

```
read TOML → parse model.Layout
  → cmux ping (verify cmux is running)
    → for each workspace (ordered by index):
      1. cmux new-workspace --cwd <cwd>
      2. cmux rename-workspace --workspace <ref> <title>
      3. for each pane[i>0]: cmux new-split <direction> --workspace <ref>
      4. for each pane with command: cmux send --surface <ref> "command\n"
      5. cmux focus-pane (if pane.focus=true)
    → cmux select-workspace (restore active tab)
```

### 📥 Import from Blueprint

Parses Workspace Blueprint (.md) → resolves templates into pane definitions → creates any workspaces that don't already exist in the running cmux instance.

### 📤 Export to Blueprint

The reverse of import: captures live state and writes it to the Workspace Blueprint with default templates.

## Package Structure

```
cmd/                    → Cobra CLI commands
internal/
  client/               → CmuxClient interface + CLI implementation
  config/               → TOML config loading, default paths
  model/                → Layout, Workspace, Pane structs + merge logic
  mdfile/               → Workspace Blueprint (.md) parser + writer
  orchestrate/          → Business logic: save, restore, watch, export
  persist/              → TOML file store (read/write layouts)
```

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
type CmuxClient interface {
    Ping() error
    Tree() (*TreeResponse, error)
    SidebarState(workspaceRef string) (*SidebarState, error)
    ListWorkspaces() ([]WorkspaceInfo, error)
    NewWorkspace(opts NewWorkspaceOpts) (string, error)
    RenameWorkspace(ref, title string) error
    SelectWorkspace(ref string) error
    NewSplit(direction, workspaceRef string) error
    FocusPane(paneRef, workspaceRef string) error
    Send(workspaceRef, surfaceRef, text string) error
    CloseWorkspace(ref string) error
    PinWorkspace(ref string) error
}
```

The CLI backend can be swapped for a direct socket connection without touching any business logic.
