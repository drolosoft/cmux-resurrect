# 🔄 cmux-resurrect

> **Session persistence for [cmux](https://github.com/nicholasgasior/cmux) — save, restore, and manage your terminal workspaces.**

cmux is a Ghostty-based terminal multiplexer with 9.3K+ stars — but **sessions don't survive restarts**. That's its #1 weakness. `cmres` fixes it.

```
💾 cmres save work       → captures your entire layout to TOML
🔄 cmres restore work    → recreates everything: tabs, splits, commands
👁️ cmres restore --dry-run → preview what would happen, execute nothing
```

---

## ✨ Features

| | Feature | What it does |
|---|---------|-------------|
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
Restored 6/6 workspaces
```

All workspaces recreated with their original splits and startup commands.

### 👁️ Preview before restoring

```sh
cmres restore work --dry-run
```

Shows every cmux command that **would** be executed — without touching anything:

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
cmux new-workspace --cwd "/home/user/docs"
cmux rename-workspace --workspace workspace:new_2 "2 docs"
cmux select-workspace --workspace workspace:new_0

12 commands for 3 workspaces
```

Inspect, verify, **then** run without `--dry-run` when ready.

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
cmres save -d "Friday standup layout"       # 💬 attach a description
cmres restore work --dry-run                # 👁️ preview without executing
cmres watch autosave --interval 2m          # ⏱️ custom interval
cmres project add api ~/projects/api -t go --icon "⚙️"  # ➕ with template + icon
cmres project add notes ~/docs -t single --disabled     # ➕ disabled by default
cmres project list --all                    # 📋 include disabled projects
cmres show work --raw                       # 🔍 dump raw TOML
```

---

## 📝 Workspace File

The workspace file is a Markdown document with two sections: **Projects** and **Templates**. Fully compatible with Obsidian and any Markdown editor.

```markdown
## Projects
**Icon | Name | Template | Pin | Path**

- [x] | 🌐 | webapp         | dev      | yes | ~/projects/webapp
- [x] | ⚙️ | api-server     | dev      | yes | ~/projects/api-server
- [x] | 🧪 | testing        | go       | yes | ~/projects/testing
- [ ] | 📓 | notes          | single   | no  | ~/documents/notes
- [x] | 📊 | dashboard      | monitor  | yes | ~/projects/dashboard

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

### How it works

| Element | Meaning |
|---------|---------|
| `[x]` / `[ ]` | ✅ Enabled / ⬜ Disabled — controls sync behavior |
| Pipe columns | 🏷️ Icon, name, template, pin, path |
| Template reference | 🧩 Links to a `### template-name` section |
| Unchecked project | ⏸️ Excluded from `cmres sync` without deleting |
| Unchecked pane | ⏸️ That split is skipped during sync |
| Sections after Templates | 📄 Preserved verbatim (notes, docs, etc.) |

### 🧩 Templates

Templates define reusable pane layouts:

| Keyword | What it creates |
|---------|----------------|
| `main terminal` | 🖥️ First pane (no split) |
| `split right:` | ➡️ Vertical split to the right |
| `split down:` | ⬇️ Horizontal split below |
| `(focused)` | 🎯 This pane gets focus after creation |
| `` `command` `` | ⚡ Send this command to the pane |

Built-in templates: **dev** (terminal + dev server + lazygit), **go** (terminal + tests), **single** (terminal only), **monitor** (htop + logs).

---

## ⚙️ Configuration

`~/.config/cmres/config.toml` — all fields optional, defaults applied automatically.

```toml
# 📝 Workspace MD file path (Obsidian vault or anywhere)
workspace_file = "~/Documents/cmux-workspaces.md"

# 📁 Directory for layout TOML files
layouts_dir = "~/.config/cmres/layouts"

# ⏱️ Auto-save interval for watch
watch_interval = "5m"

# 🔄 Max rotated autosave files
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

The service:
- ⏱️ Runs `cmres watch autosave --interval 5m`
- 🔌 Only starts when `/tmp/cmux.sock` exists (cmux is running)
- 📄 Logs to `/tmp/cmres-watch.log`
- 🛡️ Throttles restarts to every 30s

**Hash deduplication**: `watch` compares content hashes before writing — no duplicate files when the layout hasn't changed.

---

## 🏗️ How It Works

```
┌──────────────────┐         ┌──────────────────┐
│  🖥️ cmux          │ ◄─────► │  ⚡ cmres CLI     │
│  (Ghostty mux)   │  cmux   │  (Go binary)     │
└──────────────────┘  CLI    └────────┬─────────┘
                      calls           │
                            ┌─────────┼─────────┐
                            ▼                   ▼
                   ┌──────────────┐    ┌──────────────┐
                   │ 💾 Layouts    │    │ 📝 Workspace  │
                   │ (TOML files) │    │ (Markdown)   │
                   │ ~/.config/   │    │ Obsidian-    │
                   │ cmres/layouts│    │ compatible   │
                   └──────────────┘    └──────────────┘
```

1. **💾 Save** — calls `cmux tree` + `cmux sidebar-state` → captures hierarchy → serializes to TOML
2. **🔄 Restore** — reads TOML → issues `cmux new-workspace`, `new-split`, `rename`, `send` → recreates everything
3. **🔀 Sync** — parses Markdown → resolves templates → creates missing workspaces in cmux
4. **📤 Export** — captures live state → writes to Markdown with templates

---

## 🔨 Building from Source

**Prerequisites**: Go 1.21+ · cmux in `$PATH`

```sh
make build              # → bin/cmres
make install            # → /usr/local/bin/cmres
make test               # 🧪 unit tests (47 passing)
make test-integration   # 🧪 integration tests (needs running cmux)
make lint               # 🔍 go vet
make fmt                # ✨ go fmt
make clean              # 🗑️ remove bin/
```

---

## 📜 License & Philosophy

**MIT License** — free to use, modify, and distribute.

This is a **personal project** born from a real need: I lost an hour of work when cmux crashed and all my workspaces vanished. I built `cmres` to fix that, and I'm sharing it because others have the same problem.

That said, this is **not an actively maintained product**. It works, it's tested (47 tests across 7 packages), and it solves the problem it was built for. I may improve it over time, but I make no promises about timelines, feature requests, or support. If you find a bug, PRs are welcome. If you want a feature, fork it — that's what open source is for.

**Forged by [Drolosoft](https://drolosoft.com)** · Canary Islands, Spain · *Tools we wish existed*
