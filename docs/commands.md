[Home](../README.md) > Commands

# 📖 Commands

## Command Reference

| Command | Alias | Description |
|---------|-------|-------------|
| `crex setup` | | 🧙 First-run wizard — detect backend, create config |
| `crex save [name]` | | 💾 Capture current layout to TOML |
| `crex restore [name]` | | 🔄 Recreate tabs, pane arrangements, and commands |
| `crex list` | `ls` | 📋 List saved layouts with tab count |
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
| `crex version` | | ℹ️ Print version, commit, build date |
| `crex template list` | `tpl ls` | 📦 List available templates from the gallery |
| `crex template show <name>` | `tpl show` | 🔍 Preview a template with ASCII diagram |
| `crex template use <template> [path]` | `tpl use` | 🚀 Create a workspace from a gallery template |
| `crex template customize <name>` | `tpl customize` | ✏️ Copy a gallery template into your Blueprint |
| `crex completion` | | 🔤 Generate shell completion scripts (bash, zsh, fish) |

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

An inline REPL with a `crex❯` prompt. Type commands, browse listings with arrow keys, and manage your workspaces without leaving the shell.

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
| `watch start\|stop\|status` | Daemon controls |
| `exit` | Quit |

Listings show numbered items (`[1]`, `[2]`, …) — use the number instead of the name in any command.

## Watch Daemon Mode

```sh
crex watch --daemon                 # start daemon (PID file, log file)
crex watch --stop                   # stop running daemon
crex watch --status                 # check daemon status
crex watch --shell-hook             # print shell auto-start snippet
crex watch --shell-hook >> ~/.zshrc # install the hook
```

| Flag | Description |
|------|-------------|
| `--daemon` | Run in background with PID file and log rotation |
| `--stop` | Kill the running daemon |
| `--status` | Check if the daemon is alive |
| `--shell-hook` | Print a shell snippet that auto-starts the daemon |

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
crex watch autosave --interval 2m
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
