# 🔄✨🖥️ cmux-resurrect

[![CI](https://github.com/drolosoft/cmux-resurrect/actions/workflows/ci.yml/badge.svg)](https://github.com/drolosoft/cmux-resurrect/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/drolosoft/cmux-resurrect)](https://goreportcard.com/report/github.com/drolosoft/cmux-resurrect)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Homebrew](https://img.shields.io/badge/Homebrew-tap-orange.svg)](https://github.com/drolosoft/homebrew-tap)
[![cmux](https://img.shields.io/badge/cmux-ecosystem-blueviolet.svg)](https://github.com/manaflow-ai/cmux)

> **Session persistence for [cmux](https://github.com/manaflow-ai/cmux) — your terminal workspaces, resurrected.**

[cmux](https://github.com/manaflow-ai/cmux) is the fastest-growing terminal multiplexer in the Ghostty ecosystem (12K+ stars). It handles session restoration well most of the time, but crashes, forced updates, and unexpected reboots can still wipe your workspace. **crex** is a safety net for those moments.

⚡️ One command saves your entire cmux layout. One command brings it back — workspaces, splits, CWDs, pinned state, startup commands, everything.

Inspired by [tmux-resurrect](https://github.com/tmux-plugins/tmux-resurrect) (12.6K stars) — crex brings the same peace of mind to the cmux ecosystem, and takes it further with **Workspace Blueprints**: define your ideal terminal setup in a Markdown file (Obsidian-compatible), version it, share it with your team, and let crex build it for you.

<p align="center"><img src="assets/demo.gif" alt="crex demo" width="800"></p>

---

## 🚀 Quick Start

### Install with Homebrew (recommended)

```sh
brew tap drolosoft/tap
brew install cmux-resurrect
```

Both `crex` and `cmux-resurrect` are ready to use. No Go toolchain required.

### Install with `go install`

```sh
go install github.com/drolosoft/cmux-resurrect/cmd/crex@latest
```

> For building from source, see [docs/building.md](docs/building.md).

### Try it

```sh
crex restore demo --dry-run   # preview what it does
crex restore demo             # run it
```

---

## 💾 Save & Restore

```sh
crex save work                # snapshot your layout
crex restore work             # bring it all back
```

Every workspace, split, CWD, pinned state, and startup command — captured and restored.

## 📥 Workspace Blueprints

Define your workspaces in Obsidian-compatible Markdown. Import creates only what's missing — it's idempotent.

```sh
crex import-from-md           # create workspaces from Blueprint
crex export-to-md             # capture live state to Blueprint
```

<p align="center"><img src="assets/import-success.png" alt="crex import-from-md in action" width="800"></p>

> For the Blueprint format, templates, and CLI management, see [docs/blueprint.md](docs/blueprint.md).

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

## 📚 Documentation

| Doc | Description |
|-----|-------------|
| [Commands](docs/commands.md) | Full command reference, flags, and recipes |
| [Workspace Blueprints](docs/blueprint.md) | Blueprint format, templates, CLI management |
| [Workflows](docs/workflows.md) | Save/Restore vs Import, dry-run, side-by-side comparison |
| [Configuration](docs/configuration.md) | config.toml reference and defaults |
| [Auto-Save](docs/auto-save.md) | launchd integration for macOS |
| [Building from Source](docs/building.md) | Makefile targets, cross-compilation, platform support |
| [Architecture](ARCHITECTURE.md) | Internal design for contributors |

---

## 🌟 Contributing

Contributions are welcome — bug fixes, new templates, feature ideas. Open an issue or submit a PR.

If crex saves your sessions, consider giving it a ⭐ on GitHub — it helps others discover the project.

---

## ☕ Support

If crex saved you time or made your workflow easier, consider buying me a coffee — it keeps the next one coming!

<a href="https://buymeacoffee.com/juan.andres.morenorub.io"><img src="https://cdn.buymeacoffee.com/buttons/v2/default-yellow.png" alt="Buy Me A Coffee" height="50"></a>

<img src="assets/bmc-qr.png" alt="Buy Me A Coffee QR" width="150">

---

## 📜 License

**MIT License** — free to use, modify, and distribute.

Born from a real need: a crashed cmux session took an hour of carefully arranged workspaces with it. `crex` exists so that never happens again.

**Forged by [Drolosoft](https://drolosoft.com)** · *Tools we wish existed*
