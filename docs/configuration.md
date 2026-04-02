[Home](../README.md) > Configuration

# ⚙️ Configuration

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

## Defaults

| Setting | Default |
|---------|---------|
| 📄 Config file | `~/.config/crex/config.toml` |
| 📁 Layouts dir | `~/.config/crex/layouts/` |
| 📝 Workspace Blueprint | `~/.config/crex/workspaces.md` |

## Override with Flags

```sh
crex --config /path/to/config.toml --layouts-dir /path/to/layouts list
```

---

See also: [Commands](commands.md) | [Auto-Save](auto-save.md)
