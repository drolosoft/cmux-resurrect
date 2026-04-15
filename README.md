<p align="center"><img src="assets/logo.png" alt="crex logo" width="120"></p>

<h1 align="center">cmux-resurrect</h1>

<p align="center">
  <a href="https://github.com/drolosoft/cmux-resurrect/actions/workflows/ci.yml"><img src="https://github.com/drolosoft/cmux-resurrect/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://goreportcard.com/report/github.com/drolosoft/cmux-resurrect"><img src="https://goreportcard.com/badge/github.com/drolosoft/cmux-resurrect" alt="Go Report Card"></a>
  <a href="https://pkg.go.dev/github.com/drolosoft/cmux-resurrect"><img src="https://pkg.go.dev/badge/github.com/drolosoft/cmux-resurrect.svg" alt="Go Reference"></a>
  <a href="https://codecov.io/gh/drolosoft/cmux-resurrect"><img src="https://codecov.io/gh/drolosoft/cmux-resurrect/branch/main/graph/badge.svg" alt="codecov"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
  <a href="https://github.com/drolosoft/homebrew-tap"><img src="https://img.shields.io/badge/Homebrew-tap-orange.svg" alt="Homebrew"></a>
  <a href="https://github.com/drolosoft/cmux-resurrect/releases"><img src="https://img.shields.io/github/v/release/drolosoft/cmux-resurrect" alt="GitHub Release"></a>
  <a href="https://github.com/manaflow-ai/cmux"><img src="https://img.shields.io/badge/cmux-ecosystem-blueviolet.svg" alt="cmux"></a>
</p>

> **Session persistence for [cmux](https://github.com/manaflow-ai/cmux) — your terminal workspaces, resurrected.**

[cmux](https://github.com/manaflow-ai/cmux) is a popular terminal multiplexer in the Ghostty ecosystem (14K+ stars). It handles session restoration well most of the time, but crashes, forced updates, and unexpected reboots can still wipe your workspace. **crex** (short for cmux-resurrect) is a safety net for those moments.

⚡️ One command saves your entire cmux layout. One command brings it back — workspaces, splits, CWDs, pinned state, startup commands, everything.

Inspired by [tmux-resurrect](https://github.com/tmux-plugins/tmux-resurrect) (12.7K stars) — crex does for cmux what tmux-resurrect does for tmux, and takes it further with **Workspace Blueprints**: define your ideal terminal setup in a Markdown file (Obsidian-compatible), version it, share it with your team, and let crex build it for you.

<p align="center"><img src="assets/demo.gif" alt="crex demo" width="800"></p>

---

## 🚀 Quick Start

### Install with Homebrew (recommended)

```sh
brew install drolosoft/tap/cmux-resurrect
```

Both `crex` and `cmux-resurrect` are ready to use, with shell completions installed automatically. No Go toolchain required. macOS only (cmux is a macOS terminal).

### Install with `go install`

```sh
go install github.com/drolosoft/cmux-resurrect/cmd/crex@latest
```

> For building from source, see [docs/building.md](docs/building.md).

### Enable Shell Completion

Homebrew users get completions automatically. For manual installs, add one line to your shell config:

```sh
eval "$(crex completion zsh)"    # zsh — add to ~/.zshrc
eval "$(crex completion bash)"   # bash — add to ~/.bashrc
crex completion fish | source    # fish — run once
```

Now `crex <TAB>` shows all commands, `crex restore <TAB>` completes your saved layout names, and flags like `--mode` complete their values. See [docs/shell-completion.md](docs/shell-completion.md) for the full guide.

### Try it

```sh
crex save my-day                # snapshot your current layout
crex save my-day --dry-run      # or preview first without saving
```

---

## 💾 Save & Restore

```sh
crex save my-day              # snapshot your layout
crex restore my-day           # bring it all back
```

Every workspace, split, CWD, pinned state, and startup command — captured and restored. Layouts are saved to `~/.config/crex/layouts/`.

<p align="center"><img src="assets/save-my-day.png" alt="crex save my-day" width="700"></p>

## 📥 Workspace Blueprints

Define your workspaces in Obsidian-compatible Markdown. Import creates only what's missing — it's idempotent.

```markdown
## Projects
**Icon | Name | Template | Pin | Path**

- [x] | 🌐 | webapp    | dev     | yes | ~/projects/webapp
- [x] | ⚙️ | api       | dev     | yes | ~/projects/api-server
- [x] | 🧪 | tests     | go      | yes | ~/projects/testing

## Templates

### dev
- [x] main terminal (focused)
- [x] split right: `npm run dev`
- [x] split right: `lazygit`
```

```sh
crex import-from-md           # create workspaces from Blueprint
crex export-to-md             # capture live state to Blueprint
```

<p align="center"><img src="assets/import-success.png" alt="crex import-from-md in action" width="800"></p>

> For the full Blueprint format, templates, and CLI management, see [docs/blueprint.md](docs/blueprint.md).

---

## ✨ Why crex?

[tmux-resurrect](https://github.com/tmux-plugins/tmux-resurrect) proved that session persistence is essential for any serious terminal multiplexer workflow. Every multiplexer eventually gets one — crex is that tool for cmux.

| | tmux-resurrect | crex |
|:---:|---|---|
| 📝 | Plugin configuration | **Workspace Blueprint** — Markdown files, Obsidian-compatible |
| 🧩 | Manual pane recreation | **Reusable templates** (`dev`, `go`, `monitor`) |
| 📥 | One-way restore | **Bidirectional** — import from and export to Markdown |
| 👁️ | Execute immediately | **Dry-run mode** — preview every command first |
| ⏱️ | Manual saves | **Auto-save with launchd** — deduped, zero-maintenance |
| 📋 | Edit config files | **CLI workspace management** — `add`, `remove`, `toggle` from terminal |
| 🔤 | Basic tab completion | **Dynamic completions** — layout names, workspace names, flag values (bash/zsh/fish) |

---

## 📚 Documentation

| Doc | Description |
|-----|-------------|
| [Commands](docs/commands.md) | Full command reference, flags, and recipes |
| [Workspace Blueprints](docs/blueprint.md) | Blueprint format, templates, CLI management |
| [Workflows](docs/workflows.md) | Save/Restore vs Import, dry-run, side-by-side comparison |
| [Configuration](docs/configuration.md) | config.toml reference and defaults |
| [Auto-Save](docs/auto-save.md) | launchd integration for macOS |
| [Shell Completion](docs/shell-completion.md) | Setup, troubleshooting, what gets completed |
| [Building from Source](docs/building.md) | Makefile targets, cross-compilation, platform support |
| [Architecture](ARCHITECTURE.md) | Internal design for contributors |

---

## 🌟 Contributing

Contributions are welcome — bug fixes, new templates, feature ideas. Open an issue or submit a PR.

If crex saves your sessions, consider giving it a ⭐ on GitHub — it helps others discover the project.

---

## ☕ Support

If crex saved you time or made your workflow easier, consider buying me a coffee — it keeps the next one coming!

<p align="center"><a href="https://buymeacoffee.com/juan.andres.morenorub.io"><img src="https://cdn.buymeacoffee.com/buttons/v2/default-yellow.png" alt="Buy Me A Coffee" height="50"></a></p>

---

## 📜 License

**MIT License** — free to use, modify, and distribute.

Born from a real need: a crashed cmux session took an hour of carefully arranged workspaces with it. `crex` exists so that never happens again.

**Forged by [Drolosoft](https://drolosoft.com)** · *Tools we wish existed*
