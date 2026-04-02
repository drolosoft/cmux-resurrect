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
| `crex export-to-md` | | 📤 Export live cmux state to a Workspace Blueprint |
| `crex watch [name]` | | ⏱️ Auto-save at interval (default: 5m) |
| `crex workspace add` | `ws add` | ➕ Add workspace entry to the Blueprint |
| `crex workspace remove` | `ws rm` | ➖ Remove workspace entry from the Blueprint |
| `crex workspace list` | `ws ls` | 📋 List workspace entries in the Blueprint |
| `crex workspace toggle` | `ws toggle` | 🔘 Enable/disable a workspace entry |
| `crex version` | | ℹ️ Print version, commit, build date |

## Key Flags

```sh
crex save -d "Friday standup layout"                   # 💬 attach a description
crex restore work --dry-run                            # 👁️ preview without executing
crex watch autosave --interval 2m                      # ⏱️ custom interval
crex workspace add api ~/projects/api -t dev --icon "⚙️"  # ➕ with template + icon
crex workspace add notes ~/docs -t single --disabled     # ➕ disabled by default
crex workspace list --all                                # 📋 include disabled workspaces
crex show work --raw                                   # 🔍 dump raw TOML
```

## Common Recipes

### Save before a reboot
```sh
crex save work
# reboot, then:
crex restore work
```

### Set up a new machine from a Blueprint
```sh
# Copy your workspaces.md to the new machine, then:
crex import-from-md --workspace-file ~/workspaces.md
```

### Preview before restoring
```sh
crex restore work --dry-run
# Review the output, then:
crex restore work
```

### Auto-save every 2 minutes
```sh
crex watch autosave --interval 2m
```

---

See also: [Workflows](workflows.md) | [Workspace Blueprints](blueprint.md) | [Configuration](configuration.md)
