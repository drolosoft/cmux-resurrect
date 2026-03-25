# 🔄✨🖥️ cmux-resurrect

[![CI](https://github.com/drolosoft/cmux-resurrect/actions/workflows/ci.yml/badge.svg)](https://github.com/drolosoft/cmux-resurrect/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/drolosoft/cmux-resurrect)](https://goreportcard.com/report/github.com/drolosoft/cmux-resurrect)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

> **Session persistence for [cmux](https://github.com/manaflow-ai/cmux) — your terminal workspaces, resurrected.**

cmux is a Ghostty-based terminal multiplexer with 9.3K+ stars — but **sessions don't survive restarts**. `crex` fixes that.

<!-- 🎬 Demo GIF — run ./scripts/record-demo.sh to generate -->
<p align="center">
  <em>🎬 Demo coming soon — <code>./scripts/record-demo.sh</code> to record</em>
  <br><br>
  <strong>💾 Save → 👁️ Preview → 🔄 Restore</strong> — your entire workspace in seconds
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
```

Choose your preferred command name:

```sh
make install        # → /usr/local/bin/crex           (short name)
make install-long   # → /usr/local/bin/cmux-resurrect (long name)
make install-both   # → both names (crex + cmux-resurrect)
```

### 💾 Save your layout

```sh
crex save work
```

Captures every workspace, split, CWD, and pinned state into `~/.config/crex/layouts/work.toml`.

### 🔄 Restore it later

```sh
crex restore work
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
crex restore work --dry-run
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
| `crex save [name]` | | 💾 Capture current layout to TOML |
| `crex restore <name>` | | 🔄 Recreate workspaces, splits, and commands |
| `crex list` | `ls` | 📋 List saved layouts with workspace count |
| `crex show <name>` | | 🔍 Display layout details (`--raw` for TOML) |
| `crex edit <name>` | | ✏️ Open layout in `$EDITOR` |
| `crex delete <name>` | `rm` | 🗑️ Delete a saved layout |
| `crex sync` | | 🔀 Reconcile Markdown workspace file → cmux |
| `crex export` | | 📤 Export live cmux state → Markdown file |
| `crex watch [name]` | | ⏱️ Auto-save at interval (default: 5m) |
| `crex project add` | `p add` | ➕ Add project to workspace file |
| `crex project remove` | `p rm` | ➖ Remove project from workspace file |
| `crex project list` | `p ls` | 📋 List projects in workspace file |
| `crex project toggle` | `p toggle` | 🔘 Enable/disable a project |
| `crex version` | | ℹ️ Print version, commit, build date |

### 🏴 Key Flags

```sh
crex save -d "Friday standup layout"                   # 💬 attach a description
crex restore work --dry-run                            # 👁️ preview without executing
crex watch autosave --interval 2m                      # ⏱️ custom interval
crex project add api ~/projects/api -t dev --icon "⚙️"  # ➕ with template + icon
crex project add notes ~/docs -t single --disabled     # ➕ disabled by default
crex project list --all                                # 📋 include disabled projects
crex show work --raw                                   # 🔍 dump raw TOML
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
| Unchecked project | ⏸️ Excluded from `crex sync` without deleting it |
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

`~/.config/crex/config.toml` — all fields optional, defaults applied automatically.

```toml
# Workspace MD file path
workspace_file = "~/documents/cmux-workspaces.md"

# Directory for layout TOML files
layouts_dir = "~/.config/crex/layouts"

# Auto-save interval for watch
watch_interval = "5m"

# Max rotated autosave files
max_autosaves = 10
```

| Setting | Default |
|---------|---------|
| 📄 Config file | `~/.config/crex/config.toml` |
| 📁 Layouts dir | `~/.config/crex/layouts/` |
| 📝 Workspace file | `~/.config/crex/workspaces.md` |

Override with flags: `crex --config /path/to/config.toml --layouts-dir /path/to/layouts list`

---

## 🍎 Auto-Save with launchd

The `watch` command runs as a macOS service, auto-saving when cmux is active.

```sh
make install-service    # → ~/Library/LaunchAgents/com.crex.watch.plist
make uninstall-service  # remove it
```

- ⏱️ Runs `crex watch autosave --interval 5m`
- 🔌 Only starts when `/tmp/cmux.sock` exists (cmux is running)
- 📄 Logs to `/tmp/crex-watch.log`
- 🛡️ Throttles restarts to every 30s
- 🔗 Content-hash deduplication — no duplicate files when layout hasn't changed

---

## 🔨 Building from Source

**Prerequisites**: Go 1.26+ · cmux in `$PATH`

```sh
make build              # → bin/crex
make install            # → /usr/local/bin/crex (short name)
make install-long       # → /usr/local/bin/cmux-resurrect (long name)
make install-both       # → both names (crex + cmux-resurrect)
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

This is a **personal project** born from a real need: a crashed cmux session took an hour of carefully arranged workspaces with it. `crex` exists so that never happens again.

This is **shared, not staffed**. It works, it's tested, and it solves the problem it was built for. There are no promises about timelines, feature requests, or support. If you find a bug, PRs are welcome. If you want a feature, fork it — that's what open source is for.

**Forged by [Drolosoft](https://drolosoft.com)** · *Tools we wish existed*
