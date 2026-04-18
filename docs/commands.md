[Home](../README.md) > Commands

# 📖 Commands

## Command Reference

| Command | Alias | Description |
|---------|-------|-------------|
| `crex save [name]` | | 💾 Capture current layout to TOML |
| `crex restore [name]` | | 🔄 Recreate workspaces, splits, and commands |
| `crex list` | `ls` | 📋 List saved layouts with workspace count |
| `crex show <name>` | | 🔍 Display layout details (`--raw` for TOML) |
| `crex edit <name>` | | ✏️ Open layout in `$EDITOR` |
| `crex delete <name>` | `rm` | 🗑️ Delete a saved layout |
| `crex import-from-md` | | 📥 Create workspaces from a Workspace Blueprint |
| `crex export-to-md` | | 📤 Export live terminal state to a Workspace Blueprint |
| `crex watch [name]` | | ⏱️ Auto-save at interval (default: 5m) |
| `crex workspace add` | `ws add` | ➕ Add workspace entry to the Blueprint |
| `crex workspace remove` | `ws rm` | ➖ Remove workspace entry from the Blueprint |
| `crex workspace list` | `ws ls` | 📋 List workspace entries in the Blueprint |
| `crex workspace toggle` | `ws toggle` | 🔘 Enable/disable a workspace entry |
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
crex workspace add api ~/projects/api -t dev --icon "⚙️"  # ➕ with template + icon
crex workspace add notes ~/docs -t single --disabled     # ➕ disabled by default
crex workspace list --all                                # 📋 include disabled workspaces
crex show my-day --raw                                 # 🔍 dump raw TOML
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

Copies the built-in template into your Workspace Blueprint. Your copy takes priority over the built-in version.

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

## Shell Completion

crex supports tab completion for commands, layout names, workspace names, and flag values.

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
