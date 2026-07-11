[Home](../README.md) > Commands

# 📖 Commands

## Command Reference

| Command | Alias | Description |
|---------|-------|-------------|
| `crex setup` | | 🧙 First-run wizard — detect backend, create config |
| `crex save [name]` | | 💾 Capture current layout to TOML |
| `crex restore [name] [workspace]` | | 🔄 Recreate tabs, pane arrangements, and commands (optionally a single workspace) |
| `crex pop [name]` | | ⚡ Quick-launch a layout (Ctrl+G picker, `--last`) |
| `crex now` | | 👀 Show current live workspaces |
| `crex rename <old> <new>` | `mv` | 🏷️ Rename a saved layout |
| `crex list` | `ls` | 📋 List saved layouts (`--json`, `--alfred`) |
| `crex show <name>` | | 🔍 Display layout details (`--raw` for TOML) |
| `crex edit <name>` | | ✏️ Open layout in `$EDITOR` |
| `crex delete <name>` | `rm` | 🗑️ Delete a saved layout |
| `crex import-from-md` | | 📥 Create tabs from a Blueprint |
| `crex export-to-md` | | 📤 Export live terminal state to a Blueprint |
| `crex watch [name]` | | ⏱️ Auto-save at interval (--daemon, --stop, --status, --shell-hook) |
| `crex tui` | | 🖥️ Interactive shell (browse layouts, templates, live state) |
| `crex blueprint add` | `bp add` | ➕ Add entry to the Blueprint |
| `crex blueprint remove` | `bp rm` | ➖ Remove entry from the Blueprint |
| `crex blueprint list` | `bp ls` | 📋 List entries in the Blueprint |
| `crex blueprint toggle` | `bp toggle` | 🔘 Enable/disable a Blueprint entry |
| `crex settings <cmd>` | | ⚙️ Settings (`restore-mode get\|set\|list`) |
| `crex update` | | ⬆️ Check for and install the latest release |
| `crex version` | | ℹ️ Print version, commit, build date (also `--version`, `-v`) |
| `crex template list` | `tpl ls` | 📦 List available templates from the gallery |
| `crex template show <name>` | `tpl show` | 🔍 Preview a template with ASCII diagram |
| `crex template use <template> [path]` | `tpl use` | 🚀 Create a workspace from a gallery template |
| `crex template customize <name>` | `tpl customize` | ✏️ Copy a gallery template into your Blueprint |
| `crex skill install` | | 🤖 Install the agent skill for Claude Code (`--codex` for Codex) |
| `crex skill show` | | 📄 Print the agent skill to stdout |
| `crex completion` | | 🔤 Generate shell completion scripts (bash, zsh, fish, powershell) |

## Key Flags

```sh
crex save -d "Friday standup layout"                   # 💬 attach a description
crex restore my-day --dry-run                          # 👁️ preview without executing
crex watch autosave --interval 2m                      # ⏱️ custom interval
crex blueprint add api ~/projects/api -t dev --icon "⚙️"  # ➕ with template + icon
crex blueprint add notes ~/docs -t single --disabled      # ➕ disabled by default
crex blueprint list --all                                 # 📋 include disabled entries
crex show my-day --raw                                 # 🔍 dump raw TOML
crex setup                                              # 🧙 run the first-time wizard
crex setup --defaults                                   # 🧙 accept all defaults (CI/scripting)
crex watch --daemon                                     # ⏱️ start background auto-persistence
crex watch --status                                     # ⏱️ check if daemon is running
crex watch --stop                                       # ⏱️ stop the daemon
crex watch --shell-hook                                 # ⏱️ print auto-start snippet for your shell
```

## Try It: the Demo Layout

`crex setup` installs a portable example layout named `demo` — two workspaces (🏠 home, 📁 files with a split), every path home-relative (`~`, `~/Documents`, `~/Downloads`), no commands. It restores safely on any machine:

```sh
crex show demo                  # inspect it first
crex restore demo --mode add    # try it without touching your open workspaces
```

It's a real layout file (`~/.config/crex/layouts/demo.toml`) — edit it, re-save over it, or `crex delete demo` when done. Setup never overwrites your copy.

> Layout `cwd` fields support `~` — expanded at restore time, so layouts written with home-relative paths are portable across machines.

## Template Commands

The `template` command group (alias: `tpl`) lets you browse and use the built-in gallery.

### `crex template list`

```sh
crex template list                    # all templates
crex template list --layout           # layout templates only
crex template list --workflow         # workflow templates only
crex template list --tag monitoring   # filter by tag
```

| Flag | Description |
|------|-------------|
| `--layout` | Show only layout templates |
| `--workflow` | Show only workflow templates |
| `--tag <tag>` | Filter templates by tag |

<p align="center"><img src="../assets/template-list.png" alt="crex template list output showing all 16 templates" width="600"></p>

### `crex template show <name>`

```sh
crex template show claude             # preview with ASCII diagram
crex template show ide                # see pane layout and metadata
```

Displays the template's icon, description, ASCII diagram, category, pane count, split sequence, and tags.

### `crex template use <template> [path]`

> **Shortcut:** `crex template <name>` is equivalent to `crex template use <name>`.

```sh
crex template use claude ~/project    # create workspace at path
crex template use ide                 # create workspace in current dir
crex template use cols --name "notes" # custom workspace title
crex template use claude --dry-run    # preview commands
```

| Flag | Description |
|------|-------------|
| `--name <title>` | Workspace title (default: directory name) |
| `--icon <icon>` | Workspace icon (default: template icon for workflows) |
| `--dry-run` | Show commands without executing |
| `--pin` | Pin the workspace after creation |

### `crex template customize <name>`

```sh
crex template customize claude        # fork to your Blueprint
crex template customize ide           # then edit with: crex edit
```

Copies the built-in template into your Blueprint. Your copy takes priority over the built-in version.

## `crex setup`

```sh
crex setup                # interactive wizard
crex setup --defaults     # accept defaults (CI-friendly)
```

| Flag | Description |
|------|-------------|
| `--defaults` | Accept all defaults without prompts |

Steps: (1) detect backend, (2) create config, (3) ensure layouts dir, (4) offer first save.

## `crex tui`

```sh
crex tui                  # launch the interactive shell
crex                      # also launches the shell when config exists
```

An inline REPL with a `crex →` prompt. Type commands, browse listings with arrow keys, and manage your workspaces without leaving the shell.

| Command | Description |
|---------|-------------|
| `help` | Show all commands |
| `now` | Show live terminal state |
| `ls` | List saved layouts (browse with ↑/↓) |
| `save [name]` | Save current layout |
| `restore <name\|#>` | Restore a layout |
| `delete <name\|#>` | Delete a layout |
| `templates` | Browse gallery templates |
| `use <name\|#>` | Create workspace from template |
| `bp list` | List Blueprint entries |
| `bp add <name> <path>` | Add Blueprint entry |
| `bp remove <name\|#>` | Remove Blueprint entry |
| `bp toggle <name\|#>` | Enable/disable entry |
| `settings banner set <style>` | Set banner style (`flame`, `classic`, `plain`) |
| `settings banner get` | Show current banner style |
| `settings banner list` | List all available banner styles |
| `watch start\|stop\|status` | Daemon controls |
| `skill install [codex]` | Install the agent skill |
| `skill show` | Print the agent skill |
| `exit` | Quit |

Listings show numbered items (`[1]`, `[2]`, …) — use the number instead of the name in any command.

**Tab completion** — inside the TUI, press Tab or Shift-Tab to cycle through available commands and subcommands. Use Up/Down arrows to navigate history or listings. Press Escape to go back to the parent level.

**Bare group commands** — typing a group name alone (`bp`, `settings`) and pressing Enter shows its available subcommands without executing anything.

**Command header** — after a command is dispatched, the TUI displays a header identifying the command that ran. If a command is entered with incorrect usage, the prompt retains the input so you can correct it without retyping.

## Watch Daemon Mode

```sh
crex watch --daemon                 # start daemon — saves as "autosave"
crex watch my-project --daemon      # start daemon — saves as "my-project"
crex watch --stop                   # stop running daemon
crex watch --status                 # check daemon status
crex watch --shell-hook             # print shell auto-start snippet
crex watch --shell-hook >> ~/.zshrc # install the hook
```

**Default name:** When no name is given, watch saves under `autosave`. To recover: `crex restore autosave`.

| Flag | Description |
|------|-------------|
| `--daemon` | Run in background with PID file and log rotation |
| `--stop` | Kill the running daemon |
| `--status` | Check if the daemon is alive |
| `--shell-hook` | Print a shell snippet that auto-starts the daemon |
| `-i, --interval` | Save interval (default: 5m) |

## `crex skill` — teach your AI agents to drive crex

crex ships an agent skill: a reference document that teaches AI coding agents (Claude Code, Codex, and compatible tools) how to drive crex safely — non-interactive restore flags, snapshot-before-risk, AI-session resume semantics, and programmatic layout queries.

```sh
crex skill install           # → ~/.claude/skills/crex/SKILL.md (Claude Code)
crex skill install --codex   # → ~/.agents/skills/crex/SKILL.md
crex skill show              # print it (pipe anywhere you like)
```

Agents pick it up on their next session. Re-run `install` after upgrading crex to refresh it. Also available inside the TUI (`skill install`).

## Backend Differences

crex targets full parity between cmux and Ghostty; where the platform APIs differ, this is the honest map:

| Capability | cmux | Ghostty |
|---|---|---|
| Per-tab working directory | ✅ | ✅ |
| Per-pane (split) working directory | ✅ save + restore | ✅ save + restore¹ |
| Exact split arrangement (positions, sizes) | ✅ pixel-exact | ⚠️ splits recreated in order, geometry not captured² |
| Sub-tabs within a pane (multi-surface) | ✅ | — (Ghostty has no sub-tabs) |
| Browser panes | ✅ | opens URL in default browser |
| AI session resume | ✅ | ✅ |

¹ Ghostty reports each terminal's directory via OSC 7 (shell integration). When a shell doesn't emit OSC 7, crex falls back to the terminal title (`user@host: ~/path`), accepted only if the directory exists. Restore always sends each pane's `cd`.

² Ghostty's AppleScript API exposes no pane frames, so saved layouts restore their splits as a right-chain. Edit the layout's `split =` directions by hand (`crex edit <name>`) to customize — re-saves preserve your edits. Exact geometry lands with libghostty.

## Common Recipes

### Save before a reboot
```sh
crex save my-day
# reboot, then:
crex restore my-day
```

### Set up a new machine from a Blueprint
```sh
# Copy your workspaces.md to the new machine, then:
crex import-from-md --workspace-file ~/workspaces.md
```

### Preview before restoring
```sh
crex restore my-day --dry-run
# Review the output, then:
crex restore my-day
```

### Auto-save every 2 minutes
```sh
crex watch --interval 2m            # saves as "autosave" (default name)
crex watch my-project --interval 2m # saves as "my-project"
```

### Recover after a crash
```sh
crex restore autosave               # if using default watch name
crex restore my-project             # if using a custom name
```

### Auto-start daemon on shell login
```sh
crex watch --shell-hook >> ~/.zshrc  # zsh
crex watch --shell-hook >> ~/.bashrc # bash
```

Set `CREX_NO_WATCH=1` to disable auto-start.

## Shell Completion

crex supports tab completion for commands, layout names, blueprint names, and flag values.

```sh
# Zsh (add to ~/.zshrc)
eval "$(crex completion zsh)"

# Bash (add to ~/.bashrc)
eval "$(crex completion bash)"

# Fish (run once)
crex completion fish > ~/.config/fish/completions/crex.fish
```

Homebrew users get completions automatically — no setup needed.

See [Shell Completion](shell-completion.md) for the full guide.

---

See also: [Template Gallery](templates.md) | [Workflows](workflows.md) | [Workspace Blueprints](blueprint.md) | [Configuration](configuration.md) | [Shell Completion](shell-completion.md)
