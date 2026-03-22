# ⚰️➡️🖥️ cmux-resurrect

> **Session persistence for [cmux](https://github.com/nicholasgasior/cmux) — your terminal workspaces, resurrected.**

cmux is a Ghostty-based terminal multiplexer with 9.3K+ stars — but **sessions don't survive restarts**. `cmres` fixes that.

<!-- TODO: Replace with actual recording (see scripts/record-demo.sh) -->
<p align="center">
  <img src="assets/demo.gif" alt="cmux-resurrect demo: save, dry-run, restore" width="700">
  <br>
  <em>💾 Save → 👁️ Preview → 🔄 Restore — your entire workspace in seconds</em>
</p>

---

## 📑 Table of Contents

- [✨ Features](#-features)
- [🚀 Quick Start](#-quick-start)
- [👁️ Dry-Run Preview](#️-dry-run-preview)
- [📖 Commands](#-commands)
- [📝 Workspace File](#-workspace-file)
- [🧩 Templates](#-templates)
- [⚙️ Configuration](#️-configuration)
- [🍎 Auto-Save with launchd](#-auto-save-with-launchd)
- [🔨 Building from Source](#-building-from-source)
- [📜 License & Philosophy](#-license--philosophy)

---

## ✨ Features

| | Feature | What it does |
|:---:|---------|-------------|
| 💾 | **Full layout capture** | Saves workspaces, splits, CWDs, pinned state, active tab |
| 🔄 | **One-command restore** | Recreates workspaces, splits, sends startup commands |
| 👁️ | **Dry-run mode** | Preview every command before executing anything |
| 📝 | **Markdown workspace file** | Declare projects with checkboxes, icons, templates |
| 🧩 | **Reusable templates** | Define pane layouts once (`dev`, `go`, `monitor`), reuse everywhere |
| 🔀 | **Sync from Markdown** | Reads workspace file → creates missing workspaces in cmux |
| 📤 | **Export to Markdown** | Captures live cmux state → writes it to the workspace file |
| ⏱️ | **Auto-save (watch)** | Periodic saves with content-hash deduplication |
| 🍎 | **launchd integration** | macOS auto-save tied to cmux socket availability |
| ✏️ | **Edit in $EDITOR** | TOML files are human-readable and hand-editable |
| 📋 | **Project management** | `add`, `remove`, `toggle`, `list` projects from CLI |

---

## 🚀 Quick Start

### Install

```sh
git clone https://github.com/drolosoft/cmux-resurrect.git
cd cmux-resurrect
make build
make install   # → /usr/local/bin/cmres
```

### 💾 Save your layout

```sh
cmres save work
```

Captures every workspace, split, CWD, and pinned state into `~/.config/cmres/layouts/work.toml`.

### 🔄 Restore it later

```sh
cmres restore work
```

```
Restoring layout "work"...
Restored 4/4 workspaces
```

All workspaces recreated with their original splits and startup commands.

---

## 👁️ Dry-Run Preview

See exactly what will happen **before** it happens:

```sh
cmres restore work --dry-run
```

```
Dry-run restore of "work":

cmux new-workspace --cwd "/home/user/projects/webapp"
cmux rename-workspace --workspace workspace:new_0 "0 webapp"
cmux send --workspace workspace:new_0 "npm run dev"
cmux new-split right --workspace workspace:new_0
cmux send --workspace workspace:new_0 "lazygit"
cmux new-workspace --cwd "/home/user/projects/api-server"
cmux rename-workspace --workspace workspace:new_1 "1 api-server"
cmux new-split right --workspace workspace:new_1
cmux send --workspace workspace:new_1 "go test ./..."
cmux new-workspace --cwd "/home/user/projects/dashboard"
cmux rename-workspace --workspace workspace:new_2 "2 dashboard"
cmux new-workspace --cwd "/home/user/documents/notes"
cmux rename-workspace --workspace workspace:new_3 "3 notes"
cmux select-workspace --workspace workspace:new_0

14 commands for 4 workspaces
```

Every `cmux` command listed. Nothing executed. Inspect, verify, **then** run without `--dry-run`.

---

## 📖 Commands

| Command | Alias | Description |
|---------|-------|-------------|
| `cmres save [name]` | | 💾 Capture current layout to TOML |
| `cmres restore <name>` | | 🔄 Recreate workspaces, splits, and commands |
| `cmres list` | `ls` | 📋 List saved layouts with workspace count |
| `cmres show <name>` | | 🔍 Display layout details (`--raw` for TOML) |
| `cmres edit <name>` | | ✏️ Open layout in `$EDITOR` |
| `cmres delete <name>` | `rm` | 🗑️ Delete a saved layout |
| `cmres sync` | | 🔀 Reconcile Markdown workspace file → cmux |
| `cmres export` | | 📤 Export live cmux state → Markdown file |
| `cmres watch [name]` | | ⏱️ Auto-save at interval (default: 5m) |
| `cmres project add` | `p add` | ➕ Add project to workspace file |
| `cmres project remove` | `p rm` | ➖ Remove project from workspace file |
| `cmres project list` | `p ls` | 📋 List projects in workspace file |
| `cmres project toggle` | `p toggle` | 🔘 Enable/disable a project |
| `cmres version` | | ℹ️ Print version, commit, build date |

### 🏴 Key Flags

```sh
cmres save -d "Friday standup layout"                   # 💬 attach a description
cmres restore work --dry-run                            # 👁️ preview without executing
cmres watch autosave --interval 2m                      # ⏱️ custom interval
cmres project add api ~/projects/api -t dev --icon "⚙️"  # ➕ with template + icon
cmres project add notes ~/docs -t single --disabled     # ➕ disabled by default
cmres project list --all                                # 📋 include disabled projects
cmres show work --raw                                   # 🔍 dump raw TOML
```

---

## 📝 Workspace File

A Markdown document with two sections: **Projects** and **Templates**. Compatible with Obsidian and any Markdown editor.

```markdown
## Projects
**Icon | Name | Template | Pin | Path**

- [x] | 🌐 | webapp         | dev      | yes | ~/projects/webapp
- [x] | ⚙️ | api-server     | dev      | yes | ~/projects/api-server
- [x] | 🧪 | testing        | go       | yes | ~/projects/testing
- [ ] | 📓 | notes          | single   | no  | ~/documents/notes
- [x] | 📊 | dashboard      | monitor  | yes | ~/projects/dashboard
```

| Element | Meaning |
|---------|---------|
| `[x]` / `[ ]` | ✅ Enabled / ⬜ Disabled — controls sync behavior |
| Pipe columns | 🏷️ Icon, name, template, pin status, filesystem path |
| Unchecked project | ⏸️ Excluded from `cmres sync` without deleting it |
| Unchecked pane | ⏸️ That split is skipped during sync |

---

## 🧩 Templates

Templates define reusable pane layouts. Reference them by name from any project row.

```markdown
## Templates

### dev
- [x] main terminal (focused)
- [x] split right: `npm run dev`
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

| Keyword | What it creates |
|---------|----------------|
| `main terminal` | 🖥️ First pane in the workspace |
| `split right:` | ➡️ Vertical split to the right |
| `split down:` | ⬇️ Horizontal split below |
| `(focused)` | 🎯 This pane gets focus after creation |
| `` `command` `` | ⚡ Send this command to the pane |

Define your own templates by adding `### template-name` sections. Uncheck any pane line to disable that split.

---

## ⚙️ Configuration

`~/.config/cmres/config.toml` — all fields optional, defaults applied automatically.

```toml
# Workspace MD file path
workspace_file = "~/documents/cmux-workspaces.md"

# Directory for layout TOML files
layouts_dir = "~/.config/cmres/layouts"

# Auto-save interval for watch
watch_interval = "5m"

# Max rotated autosave files
max_autosaves = 10
```

| Setting | Default |
|---------|---------|
| 📄 Config file | `~/.config/cmres/config.toml` |
| 📁 Layouts dir | `~/.config/cmres/layouts/` |
| 📝 Workspace file | `~/.config/cmres/workspaces.md` |

Override with flags: `cmres --config /path/to/config.toml --layouts-dir /path/to/layouts list`

---

## 🍎 Auto-Save with launchd

The `watch` command runs as a macOS service, auto-saving when cmux is active.

```sh
make install-service    # → ~/Library/LaunchAgents/com.cmres.watch.plist
make uninstall-service  # remove it
```

- ⏱️ Runs `cmres watch autosave --interval 5m`
- 🔌 Only starts when `/tmp/cmux.sock` exists (cmux is running)
- 📄 Logs to `/tmp/cmres-watch.log`
- 🛡️ Throttles restarts to every 30s
- 🔗 Content-hash deduplication — no duplicate files when layout hasn't changed

---

## 🔨 Building from Source

**Prerequisites**: Go 1.21+ · cmux in `$PATH`

```sh
make build              # → bin/cmres
make install            # → /usr/local/bin/cmres
make test               # 🧪 unit tests
make test-integration   # 🧪 integration tests (needs running cmux)
make lint               # 🔍 go vet
make fmt                # ✨ go fmt
make clean              # 🗑️ remove bin/
```

> 📐 For architecture details and internal design, see [ARCHITECTURE.md](ARCHITECTURE.md).

---

## 📜 License & Philosophy

**MIT License** — free to use, modify, and distribute.

This is a **personal project** born from a real need: a crashed cmux session took an hour of carefully arranged workspaces with it. `cmres` exists so that never happens again.

This is **shared, not staffed**. It works, it's tested, and it solves the problem it was built for. There are no promises about timelines, feature requests, or support. If you find a bug, PRs are welcome. If you want a feature, fork it — that's what open source is for.

**Forged by [Drolosoft](https://drolosoft.com/cmux-resurrect)** · *Tools we wish existed*
