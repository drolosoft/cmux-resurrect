<p align="center"><img src="assets/icon-crex-fire.png" alt="crex logo" width="120"></p>

<h1 align="center">crex <sup><sub>(cmux-resurrect)</sub></sup></h1>

<p align="center">
  <a href="https://ghostty.org/"><img src="https://img.shields.io/badge/Ghostty-supported-5c6bc0.svg" alt="Ghostty"></a>
  <a href="https://github.com/manaflow-ai/cmux"><img src="https://img.shields.io/badge/cmux-ecosystem-blueviolet.svg" alt="cmux"></a>
  <a href="https://github.com/drolosoft/cmux-resurrect/releases/latest"><img src="https://img.shields.io/github/v/release/drolosoft/cmux-resurrect?label=release" alt="GitHub Release"></a>
  <a href="https://goreportcard.com/report/github.com/drolosoft/cmux-resurrect"><img src="https://goreportcard.com/badge/github.com/drolosoft/cmux-resurrect" alt="Go Report Card"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
  <a href="https://github.com/drolosoft/homebrew-tap"><img src="https://img.shields.io/badge/Homebrew-tap-orange.svg" alt="Homebrew"></a>
</p>

> **Design, manage, and automate terminal workspaces — for [cmux](https://github.com/manaflow-ai/cmux) and [Ghostty](https://ghostty.org/).**

Save layouts by name. Restore them anytime. Switch between them in seconds. Resume AI coding sessions mid-conversation. Launch workspaces from Alfred. 🐦‍🔥

<p align="center"><img src="assets/demo.gif" alt="crex demo" width="800"></p>

---

### Features

<table>
<tr>
<td width="50%" valign="top">

**💾 [Save & Restore](docs/commands.md)**<br>
Every tab, pane, CWD, and command — captured and restored by name, with the exact split arrangement.

**📦 [16 Templates](docs/templates.md)**<br>
Instant workspace setup — splits, IDE layouts, AI pair-programming.

**📝 [Markdown Blueprints](docs/blueprint.md)**<br>
Obsidian-compatible, version-controlled, shareable workspace definitions.

</td>
<td width="50%" valign="top">

**🤖 [AI Auto-Detection](docs/commands.md#ai-session-auto-detection)**<br>
15 AI tools detected. Sessions resume mid-conversation automatically.

**🔍 [Alfred Integration](docs/alfred.md)** <sup>NEW</sup><br>
Search and launch any workspace from Alfred with one keystroke.

**⚡ [Auto-Accept](docs/configuration.md#auto-accept-for-ai-agents)** <sup>NEW</sup><br>
Skip permission prompts on restore for Claude, Codex, OpenCode, and 10 more.

</td>
</tr>
</table>

---

### Quick Start

```sh
brew install drolosoft/tap/crex       # install
crex setup                            # first-run wizard (ships a demo layout)
crex restore demo                     # try the bundled example — safe anywhere
crex save my-day                      # snapshot your layout
crex restore my-day                   # bring it all back
crex pop                              # instant workspace picker (Ctrl+G)
```

<p align="center"><img src="assets/quickstart.gif" alt="crex quick start" width="800"></p>

> Also available via `go install github.com/drolosoft/cmux-resurrect/cmd/crex@latest`. See [Building](docs/building.md).

---

### Instant Workspace Picker

Type `crex pop` or hit **Ctrl+G** — a floating picker with fuzzy search across all layouts, templates, and individual workspaces.

<p align="center"><img src="assets/crex-pop.png" alt="crex pop picker" width="800"></p>

Tab into any layout to browse its workspaces:

<p align="center"><img src="assets/crex-pop-workspaces.png" alt="crex pop drill into workspaces" width="800"></p>

---

### Alfred Integration

Search and restore workspaces directly from Alfred. Type `crex` + your query — every saved workspace is searchable by name.

<p align="center"><img src="assets/alfred-search.png" alt="crex Alfred integration" width="600"></p>

| Key | Action |
|-----|--------|
| Enter | Restore workspace |
| Cmd+Enter | Restore full layout |
| Alt+Enter | Show layout details |
| Ctrl+Enter | Open TOML file |

Works with both **cmux** and **Ghostty**. See [Alfred setup guide](docs/alfred.md) for installation.

---

### One Command, Full IDE

```sh
crex template use ide
```

<p align="center"><img src="assets/demo-ide.gif" alt="crex template use ide" width="800"></p>

16 built-in templates — from simple splits to monitoring dashboards. See the [Template Gallery](docs/templates.md).

---

### AI Session Resume

`crex save` detects running AI sessions and captures their session IDs. On restore, each resumes exactly where you left off — per pane **and per tab**, so a workspace holding one tab per git worktree comes back with every tab on its own conversation.

```
crex❯ save my-day

📦 my-day
   7 🧩 drolosoft 📌
   ├── claude --resume 90d6d97b... ★
   └── →right 🌐 https://drolosoft.com/

   🚀 Homepage 📌
   ├── npm run dev
   ├── →right nvim CLAUDE.md ★
   └── →right 🌐 http://localhost:3000/
```

15 tools supported: Claude Code, OpenCode, Codex, Amp, Gemini CLI, Copilot, Grok, Cursor, Aider, and more. Any foreground process (npm, nvim, htop) is also detected and restored.

And it works both ways — `crex skill install` teaches Claude Code and Codex to drive crex themselves: snapshot before risky changes, restore layouts non-interactively, resume sessions. See [crex skill](docs/commands.md#crex-skill--teach-your-ai-agents-to-drive-crex).

Configure [auto-accept](docs/configuration.md#auto-accept-for-ai-agents) to skip permission prompts on restore — agents start in autonomous mode automatically.

---

### Interactive Shell

`crex tui` — a REPL with browse mode, numbered items, history, and tab completion.

<p align="center"><img src="assets/demo-tui.gif" alt="crex interactive shell" width="800"></p>

---

### Supported Backends

| Backend | Status | Detection |
|---------|--------|-----------|
| [cmux](https://github.com/manaflow-ai/cmux) | Full support | Auto-detected via `CMUX_SOCKET_PATH`, verified alive (falls back to a running Ghostty if the socket is dead) |
| [Ghostty](https://ghostty.org/) | Full support | Auto-detected when running |

macOS only. Both backends auto-detected — same commands, same templates, same Blueprints. Per-pane working directories work on both; exact split geometry is cmux-only for now ([backend differences](docs/commands.md#backend-differences)).

---

### vs cmux native restore

| | cmux native | crex |
|---|---|---|
| 🔄 | Restores last session on relaunch | **Named layout library** — switch between saved layouts |
| 📐 | No templates | **16 built-in templates** |
| 📝 | JSON snapshots | **Markdown Blueprints** — Obsidian-compatible |
| 🔍 | AI tools via hooks | **Any foreground process** — npm, vim, htop, all detected |
| ⚡ | Automatic on relaunch | **On-demand** — filter, dry-run, restore specific workspaces |
| ⏱️ | Saves on quit | **Watch daemon** — background auto-save |

---

### Documentation

| | |
|---|---|
| [Commands](docs/commands.md) | Full command reference |
| [Templates](docs/templates.md) | 16 built-in templates with diagrams |
| [Blueprints](docs/blueprint.md) | Markdown workspace definitions |
| [Alfred](docs/alfred.md) | Alfred workflow setup |
| [Configuration](docs/configuration.md) | config.toml, auto-accept, env vars |
| [Auto-Save](docs/auto-save.md) | Daemon, shell hooks, launchd |
| [Shell Completion](docs/shell-completion.md) | bash, zsh, fish setup |
| [Workflows](docs/workflows.md) | Save/Restore vs Import comparison |
| [Building](docs/building.md) | Build from source |

---

### Contributing

Contributions welcome — bug fixes, templates, feature ideas. Open an issue or PR.

If crex saves your sessions, a ⭐ helps others discover it.

### Support

<p align="center"><a href="https://buymeacoffee.com/juan.andres.morenorub.io"><img src="https://cdn.buymeacoffee.com/buttons/v2/default-yellow.png" alt="Buy Me A Coffee" height="50"></a></p>

---

**MIT License** · **Forged by [Drolosoft](https://drolosoft.com)** · *Tools we wish existed*
