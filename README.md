# cmux-resurrect

**Save, restore, and manage [cmux](https://github.com/nicholasgasior/cmux) workspace layouts.**

cmux is a Ghostty-based terminal multiplexer with 9.3K stars and a loyal following -- but its most cited weakness is that **sessions don't survive restarts**. `cmres` fixes that. It captures your entire workspace layout (tabs, splits, working directories, pinned state, running commands) to TOML files and recreates them on demand. It also provides a human-editable Markdown workspace file for declarative project management, compatible with Obsidian and any text editor.

---

## Features

- **Full layout capture** -- saves all workspaces, split panes, CWDs, pinned state, and active tab
- **One-command restore** -- recreates workspaces, splits, and sends startup commands
- **Dry-run mode** -- preview restore commands without executing them
- **Markdown workspace file** -- declare projects with checkbox-driven enable/disable, icons, templates, and paths
- **Reusable templates** -- define pane layouts once (e.g., `dev`, `go`, `monitor`), apply them to any project
- **Sync from Markdown** -- `cmres sync` reads the workspace file and creates missing workspaces in the running cmux instance
- **Export to Markdown** -- `cmres export` captures the live cmux state into the workspace file
- **Auto-save with watch** -- periodic background saves with content-hash deduplication
- **launchd integration** -- ships with a plist for macOS auto-save tied to cmux socket availability
- **Edit layouts in $EDITOR** -- raw TOML files are human-readable and hand-editable
- **Project management CLI** -- add, remove, toggle, and list projects without opening the Markdown file

## Quick Start

### Install

```sh
git clone https://github.com/drolosoft/cmux-resurrect.git
cd cmux-resurrect
make build
make install   # copies bin/cmres to /usr/local/bin
```

### Save your current layout

```sh
cmres save work
```

This captures every workspace, split, CWD, and pinned state into `~/.config/cmres/layouts/work.toml`.

### Restore it later

```sh
cmres restore work
```

All workspaces are recreated with their original splits and startup commands.

### Preview before restoring

```sh
cmres restore work --dry-run
```

## Commands

| Command | Alias | Description |
|---------|-------|-------------|
| `cmres save [name]` | | Capture current cmux layout to a TOML file |
| `cmres restore <name>` | | Recreate workspaces, splits, and commands from a saved layout |
| `cmres list` | `ls` | List all saved layouts with workspace count and timestamp |
| `cmres show <name>` | | Display details of a saved layout (use `--raw` for TOML) |
| `cmres edit <name>` | | Open a layout file in `$EDITOR` |
| `cmres delete <name>` | `rm` | Delete a saved layout |
| `cmres sync` | | Reconcile workspace Markdown file with live cmux (create missing workspaces) |
| `cmres export` | | Export live cmux state to the workspace Markdown file |
| `cmres watch [name]` | | Auto-save layout at a configurable interval (default: 5m) |
| `cmres project add <name> <path>` | `p add` | Add a project to the workspace file |
| `cmres project remove <name>` | `p rm` | Remove a project from the workspace file |
| `cmres project list` | `p ls` | List projects in the workspace file |
| `cmres project toggle <name>` | `p toggle` | Enable or disable a project |
| `cmres version` | | Print version, commit, and build date |

### Key Flags

```
cmres save -d "Friday standup layout"     # attach a description
cmres restore work --dry-run              # preview without executing
cmres watch autosave --interval 2m        # custom watch interval
cmres project add MyApp ~/src/myapp -t go --icon "🚀"
cmres project add Notes ~/notes -t single --disabled
cmres project list --all                  # include disabled projects
cmres show work --raw                     # dump raw TOML
```

## Workspace File Format

The workspace file is a Markdown document with two sections: **Projects** and **Templates**. It is fully compatible with Obsidian and any Markdown editor.

```markdown
## Projects
**Icon | Name | Template | Pin | Path**

- [x] | 🏟️ | LaPorrA        | dev      | yes | ~/Git/htmx/laporra                     |
- [x] | 🥌 | ioc-events     | dev      | yes | ~/Git/go/44-ioc-events                 |
- [x] | 📸 | Gallery        | go       | yes | ~/Git/yo/gallery                       |
- [ ] | 🗿 | Obsidian       | single   | yes | ~/Library/Mobile Documents/iCloud~md~obsidian/Documents |

## Templates

### dev
- [x] main terminal (focused)
- [x] split right: `claude`
- [x] split right: `lazygit`

### go
- [x] main terminal (focused)
- [x] split right: `go test ./...`

### single
- [x] main terminal (focused)

### monitor
- [x] main terminal: `htop`
- [x] split right: `tail -f /var/log/system.log`
```

### How the format works

- **Checkboxes** (`[x]` / `[ ]`) control whether a project or template pane is enabled
- **Pipe-delimited columns** define icon, name, template, pin status, and filesystem path
- **Templates** are referenced by name from the project row and define the pane layout
- Unchecking a project excludes it from `cmres sync` without deleting it
- Unchecking a template pane disables that split during sync
- Any `## ` sections after Templates are preserved verbatim (notes, docs, etc.)

## Templates

Templates define reusable pane configurations. Each template has a list of panes:

| Pane keyword | Meaning |
|--------------|---------|
| `main terminal` | The first pane in the workspace (no split) |
| `split right:` | Create a vertical split to the right |
| `split down:` | Create a horizontal split below |
| `(focused)` | This pane receives focus after creation |
| `` `command` `` | Send this command to the pane after creation |

### Built-in templates

When creating a new workspace file, `cmres` generates four starter templates:

- **dev** -- main terminal (focused) + `claude` split + `lazygit` split
- **go** -- main terminal (focused) + `go test ./...` split
- **single** -- main terminal only
- **monitor** -- `htop` main + `tail -f /var/log/system.log` split

You can define your own templates by adding `### template-name` sections under `## Templates`.

## Configuration

Configuration lives at `~/.config/cmres/config.toml`. All fields are optional; defaults are applied automatically.

```toml
# Workspace MD file path (Obsidian vault or anywhere)
workspace_file = "~/Documents/cmux-workspaces.md"

# Directory where layout TOML files are stored
layouts_dir = "~/.config/cmres/layouts"

# Auto-save interval for the watch command
watch_interval = "5m"

# Maximum number of rotated autosave files to keep
max_autosaves = 10
```

### Default paths

| Setting | Default |
|---------|---------|
| Config file | `~/.config/cmres/config.toml` |
| Layouts directory | `~/.config/cmres/layouts/` |
| Workspace file | `~/.config/cmres/workspaces.md` (configurable) |

Override any path with flags:

```sh
cmres --config /path/to/config.toml --layouts-dir /path/to/layouts list
```

## Auto-Save with launchd

`cmres` ships with a launchd plist that starts the watch daemon automatically when cmux is running (detected via `/tmp/cmux.sock`).

### Install the service

```sh
make install-service
```

This copies the plist to `~/Library/LaunchAgents/com.cmres.watch.plist` and loads it. The service:

- Starts `cmres watch autosave --interval 5m`
- Only runs when `/tmp/cmux.sock` exists (cmux is active)
- Logs to `/tmp/cmres-watch.log`
- Throttles restarts to every 30 seconds

### Remove the service

```sh
make uninstall-service
```

### How watch deduplication works

The `watch` command saves periodically but compares content hashes to avoid writing duplicate files when the layout hasn't changed. Combined with `max_autosaves`, this keeps disk usage minimal.

## How It Works

```
+------------------+          +------------------+
|   cmux (Ghostty) | <------> |   cmres CLI      |
|   multiplexer    |  cmux    |   (Go binary)    |
+------------------+  CLI     +--------+---------+
                      calls            |
                                       v
                              +--------+---------+
                              |  Layout Store    |
                              |  (TOML files)    |
                              |  ~/.config/cmres/|
                              |  layouts/        |
                              +--------+---------+
                                       |
                                       v
                              +--------+---------+
                              |  Workspace File  |
                              |  (Markdown)      |
                              |  Obsidian-ready  |
                              +------------------+
```

1. **Save** -- `cmres` calls `cmux` CLI commands (`tree`, `sidebar-state`) to capture the full workspace hierarchy, then serializes it as TOML.
2. **Restore** -- `cmres` reads the TOML file and issues `cmux` CLI commands (`new-workspace`, `new-split`, `rename-workspace`, `send`) to recreate everything.
3. **Sync** -- `cmres` parses the Markdown workspace file, resolves templates into pane definitions, and creates any workspaces that don't already exist in the running cmux instance.
4. **Export** -- The reverse of sync: captures live state and writes it to the Markdown file with default templates.

The client interface abstracts cmux interaction behind a Go interface, making it straightforward to swap the CLI backend for a direct socket connection in the future.

## Building from Source

### Prerequisites

- Go 1.21+
- cmux installed and in `$PATH`

### Build

```sh
make build          # produces bin/cmres
make install        # copies to /usr/local/bin/cmres
```

### Test

```sh
make test           # unit tests
make test-integration  # integration tests (requires running cmux)
```

### Other targets

```sh
make lint           # go vet
make fmt            # go fmt
make clean          # remove bin/
```

## Requirements

- **cmux** -- the Ghostty-based terminal multiplexer
- **Go 1.21+** -- for building from source
- **macOS** -- launchd integration is macOS-specific; the core CLI works on any platform where cmux runs

## Contributing

Contributions are welcome. Please:

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Run `make test && make lint` before submitting
5. Open a pull request with a clear description of the change

## License

MIT
