# 🔄✨🖥️ cmux-resurrect

[![CI](https://github.com/drolosoft/cmux-resurrect/actions/workflows/ci.yml/badge.svg)](https://github.com/drolosoft/cmux-resurrect/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/drolosoft/cmux-resurrect)](https://goreportcard.com/report/github.com/drolosoft/cmux-resurrect)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Homebrew](https://img.shields.io/badge/Homebrew-tap-orange.svg)](https://github.com/drolosoft/homebrew-tap)
[![cmux](https://img.shields.io/badge/cmux-ecosystem-blueviolet.svg)](https://github.com/manaflow-ai/cmux)

> **Session persistence for [cmux](https://github.com/manaflow-ai/cmux) — your terminal workspaces, resurrected.**

[cmux](https://github.com/manaflow-ai/cmux) is the fastest-growing terminal multiplexer in the Ghostty ecosystem (12K+ stars), but it doesn't persist sessions across restarts — its [most requested missing feature](https://github.com/manaflow-ai/cmux/issues/1984). **crex** fixes that.

One command saves your entire cmux layout. One command brings it back — workspaces, splits, CWDs, pinned state, startup commands, everything.

Inspired by [tmux-resurrect](https://github.com/tmux-plugins/tmux-resurrect) — the beloved session-saver for tmux (12.6K stars) — crex brings the same peace of mind to the cmux ecosystem, and takes it further with **Workspace Blueprints**: define your ideal terminal setup in a Markdown file (Obsidian-compatible), version it, share it with your team, and let crex build it for you.

<p align="center"><img src="assets/demo.gif" alt="crex demo" width="800"></p>

---

## 📑 Table of Contents

- [✨ Why crex?](#-why-crex)
- [✨ Features](#-features)
- [🔑 Two Workflows](#-two-workflows)
- [🚀 Quick Start](#-quick-start)
- [👁️ Dry-Run Preview](#️-dry-run-preview)
- [📖 Commands](#-commands)
- [📝 Workspace Blueprint](#-workspace-blueprint)
- [🧩 Templates](#-templates)
- [⚙️ Configuration](#️-configuration)
- [🍎 Auto-Save with launchd](#-auto-save-with-launchd)
- [🔨 Building from Source](#-building-from-source)
- [🖥️ Platform Compatibility](#️-platform-compatibility)
- [📜 License & Philosophy](#-license--philosophy)
- [🌟 Contributing](#-contributing)

---

## ✨ Why crex?

[tmux-resurrect](https://github.com/tmux-plugins/tmux-resurrect) proved that session persistence is essential for any serious terminal multiplexer workflow. Every multiplexer eventually gets one — crex is that tool for cmux.

| | tmux-resurrect | crex |
|:---:|---|---|
| 🎯 | Saves/restores tmux sessions | Saves/restores cmux sessions |
| 📝 | Plugin configuration | **Workspace Blueprint** — Markdown files, Obsidian-compatible |
| 🧩 | Manual pane recreation | **Reusable templates** (`dev`, `go`, `monitor`) |
| 📥 | One-way restore | **Bidirectional** — import from and export to Markdown |
| 👁️ | Execute immediately | **Dry-run mode** — preview every command first |
| ⏱️ | Manual saves | **Auto-save with launchd** — deduped, zero-maintenance |
| 📋 | Edit config files | **CLI workspace management** — `add`, `remove`, `toggle` from terminal |

---

## ✨ Features

| | Feature | What it does |
|:---:|---------|-------------|
| 💾 | **Full layout capture** | Saves workspaces, splits, CWDs, pinned state, active tab |
| 🔄 | **One-command restore** | Recreates workspaces, splits, sends startup commands |
| 👁️ | **Dry-run mode** | Preview every command before executing anything |
| 📝 | **Workspace Blueprint** | Declare workspaces with checkboxes, icons, templates in a Markdown file |
| 🧩 | **Reusable templates** | Define pane layouts once (`dev`, `go`, `monitor`), reuse everywhere |
| 📥 | **Import from Blueprint** | Reads Workspace Blueprint → creates missing workspaces in cmux |
| 📤 | **Export to Blueprint** | Captures live cmux state → writes it to the Workspace Blueprint |
| ⏱️ | **Auto-save (watch)** | Periodic saves with content-hash deduplication |
| 🍎 | **launchd integration** | macOS auto-save tied to cmux socket availability |
| ✏️ | **Edit in $EDITOR** | TOML files are human-readable and hand-editable |
| 📋 | **Workspace management** | `add`, `remove`, `toggle`, `list` workspace entries from CLI |

---

## 🔑 Two Workflows

crex offers two distinct ways to manage your cmux workspaces. Understanding the difference is key.

### 💾 Save / Restore — Session Recovery

**Use case**: cmux crashed, your machine rebooted, or you want to switch between layouts.

`save` takes an exact snapshot of your running cmux session — every workspace, split, CWD, pinned state, and active tab — and writes it to a TOML file. `restore` reads that TOML and recreates everything exactly as it was.

```sh
# End of day: snapshot your layout
crex save work

# Next morning: bring it all back
crex restore work
```

Think of it as **backup and recovery**. The TOML file is a photograph of your session at a point in time.

### 📥 Import from Markdown — Workspace as Code

**Use case**: you maintain a Workspace Blueprint describing your ideal workspace setup, and you want cmux to match it.

`import-from-md` reads a Workspace Blueprint (.md, compatible with Obsidian), resolves templates into pane layouts, and creates only the workspaces that **don't already exist** in cmux. Running it twice does nothing the second time — it's idempotent.

```sh
# Define your workspaces in a .md file, then:
crex import-from-md

# Add a new workspace entry, then import again:
crex workspace add api ~/projects/api -t dev --icon "⚙️"
crex import-from-md
```

Think of it as **infrastructure as code** for your terminal. The Workspace Blueprint is the source of truth; `import-from-md` makes cmux match it. The reverse operation, `export-to-md`, captures your live cmux state back into the Blueprint.

### Side by Side

| | Save / Restore | Import from Markdown |
|---|---|---|
| Source | TOML file (auto-generated snapshot) | Workspace Blueprint (hand-written or managed via CLI) |
| Creates | Everything, every time | Only what's missing (idempotent) |
| Pane layout | Captured from live session | Defined by templates (`dev`, `go`, `monitor`) |
| Best for | Crash recovery, switching contexts | Standardized workspace setup, onboarding |

---

## 🚀 Quick Start

### Install with Homebrew (recommended)

```sh
brew tap drolosoft/tap
brew install cmux-resurrect
```

That's it — `crex` is ready to use. No Go toolchain required.

### Install with `go install`

```sh
go install github.com/drolosoft/cmux-resurrect@latest
```

### Install from source

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

### 🎯 Try the demo

A demo layout is included with the install. Try it right away:

```sh
crex restore demo --dry-run   # preview what it does
crex restore demo             # run it (choose 'a' to add, 'r' to replace)
```

It restores 3 workspaces from the included demo layout: **webapp**, **api**, and **docs** — skipping any that already exist in your session.

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
| `crex restore [name]` | | 🔄 Recreate workspaces, splits, and commands |
| `crex list` | `ls` | 📋 List saved layouts with workspace count |
| `crex show <name>` | | 🔍 Display layout details (`--raw` for TOML) |
| `crex edit <name>` | | ✏️ Open layout in `$EDITOR` |
| `crex delete <name>` | `rm` | 🗑️ Delete a saved layout |
| `crex import-from-md` | | 📥 Create workspaces from a Workspace Blueprint |
| `crex export-to-md` | | 📤 Export live cmux state to a Workspace Blueprint |
| `crex watch [name]` | | ⏱️ Auto-save at interval (default: 5m) |
| `crex workspace add` | `ws add` | ➕ Add workspace entry to the Blueprint |
| `crex workspace remove` | `ws rm` | ➖ Remove workspace entry from the Blueprint |
| `crex workspace list` | `ws ls` | 📋 List workspace entries in the Blueprint |
| `crex workspace toggle` | `ws toggle` | 🔘 Enable/disable a workspace entry |
| `crex version` | | ℹ️ Print version, commit, build date |

### 🏴 Key Flags

```sh
crex save -d "Friday standup layout"                   # 💬 attach a description
crex restore work --dry-run                            # 👁️ preview without executing
crex watch autosave --interval 2m                      # ⏱️ custom interval
crex workspace add api ~/projects/api -t dev --icon "⚙️"  # ➕ with template + icon
crex workspace add notes ~/docs -t single --disabled     # ➕ disabled by default
crex workspace list --all                                # 📋 include disabled workspaces
crex show work --raw                                   # 🔍 dump raw TOML
```

---

## 📝 Workspace Blueprint

A Workspace Blueprint is a Markdown document (.md) with two sections: **Projects** and **Templates**. Compatible with Obsidian and any Markdown editor.

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
| `[x]` / `[ ]` | ✅ Enabled / ⬜ Disabled — controls import behavior |
| Pipe columns | 🏷️ Icon, name, template, pin status, filesystem path |
| Unchecked workspace | ⏸️ Excluded from `crex import-from-md` without deleting it |
| Unchecked pane | ⏸️ That split is skipped during import |

---

## 🧩 Templates

Templates define reusable pane layouts. Reference them by name from any workspace row.

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
# Workspace Blueprint file path
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
| 📝 Workspace Blueprint | `~/.config/crex/workspaces.md` |

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
make build              # → bin/crex (current platform)
make build-all          # → cross-compile for macOS + Linux
make install            # → /usr/local/bin/crex (short name)
make install-long       # → /usr/local/bin/cmux-resurrect (long name)
make install-both       # → both names (crex + cmux-resurrect)
make test               # 🧪 unit tests
make test-integration   # 🧪 integration tests (needs running cmux)
make lint               # 🔍 go vet
make fmt                # ✨ go fmt
make clean              # 🗑️ remove bin/
```

### 🖥️ Platform Compatibility

crex is a companion to cmux. **If your Mac runs cmux, it runs crex** — no extra dependencies, no compatibility surprises.

The binary is pure Go with zero CGO dependencies, which means it compiles natively for every platform cmux supports. There is nothing to configure — build it and it works.

| Platform | Architecture | Status |
|----------|-------------|--------|
| macOS (Apple Silicon) | M1, M2, M3, M4 | ✅ Tested |
| macOS (Intel) | x86_64 | ✅ Tested |
| Linux | x86_64 | ✅ Builds |
| Linux | ARM64 | ✅ Builds |

`make build-all` produces binaries for all four targets in `bin/`.

> 📐 For architecture details and internal design, see [ARCHITECTURE.md](ARCHITECTURE.md).

---

## 🌟 Contributing

crex is open source and contributions are welcome. Whether it's a bug fix, a new template, or a feature idea — open an issue or submit a PR.

If crex saves your sessions, consider giving it a ⭐ on GitHub — it helps others discover the project.

---

## 📜 License & Philosophy

**MIT License** — free to use, modify, and distribute.

This project was born from a real need: a crashed cmux session took an hour of carefully arranged workspaces with it. `crex` exists so that never happens again.

Standing on the shoulders of [tmux-resurrect](https://github.com/tmux-plugins/tmux-resurrect) — the project that proved session persistence is non-negotiable for terminal power users. Where tmux-resurrect saves tmux sessions, crex saves cmux sessions and adds the Workspace Blueprint layer on top.

This is **shared, not staffed**. It works, it's tested, and it solves the problem it was built for. If you find a bug, PRs are welcome. If you want a feature, fork it — that's what open source is for.

**Forged by [Drolosoft](https://drolosoft.com)** · *Tools we wish existed*
