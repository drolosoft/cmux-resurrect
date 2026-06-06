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

# Banner style: "flame", "classic", or "plain"
banner_style = "flame"
```

## Defaults

| Setting | Default |
|---------|---------|
| 📄 Config file | `~/.config/crex/config.toml` |
| 📁 Layouts dir | `~/.config/crex/layouts/` |
| 📝 Workspace Blueprint | `~/.config/crex/workspaces.md` |
| ⏱️ Watch interval | `5m` |
| 🔄 Max autosaves | `10` |
| 🎨 Banner style | `flame` |

## Banner Styles

The `banner_style` setting controls how the startup banner looks when you run `crex` with no arguments.

| Style | Description |
|-------|-------------|
| `flame` | Ember→gold→green gradient across the ASCII art (default) |
| `classic` | Solid green — the traditional terminal look |
| `plain` | Monochrome gray — minimal and quiet |

Set it in `config.toml`:

```toml
banner_style = "plain"
```

Or change it interactively inside the TUI with `settings banner set <style>` — the change is written to config immediately.

Or override with the `CREX_BANNER` environment variable (takes precedence over the config file):

```sh
CREX_BANNER=classic crex
```

## Auto-Accept for AI Agents

When restoring workspaces, crex can inject "skip permissions" flags into AI agent resume commands so they start in autonomous mode automatically.

```toml
# Enable for specific agents:
auto_accept = ["claude", "codex", "opencode"]

# Enable for all supported agents:
auto_accept = ["all"]
```

**Supported agents and their flags:**

| Agent | Flag injected |
|-------|--------------|
| claude | `--dangerously-skip-permissions` |
| opencode | `--yolo` |
| codex | `--full-auto` |
| amp | `--dangerously-allow-all` |
| gemini | `--sandbox` |
| copilot | `--allow-all` |
| grok | `--always-approve` |
| aider | `--yes` |
| pi | `--approve` |
| codebuddy | `--dangerously-skip-permissions` |
| factory | `--skip-permissions-unsafe` |
| qoder | `--permission-mode auto` |
| hermes | `--yolo` |

You can also configure this during `crex setup`.

> **Warning:** This is dangerous. Agents will execute file changes, shell commands,
> and other actions without asking for permission.

## Environment Variables

| Variable | Purpose | Values |
|----------|---------|--------|
| `CREX_THEME` | Force dark or light palette | `dark`, `light` |
| `CREX_BANNER` | Override banner style | `flame`, `classic`, `plain` |
| `CREX_NO_WATCH` | Prevent daemon auto-start from shell hooks | `1` (any truthy value) |

Both are useful when auto-detection fails (e.g. terminal multiplexers blocking OSC 11 passthrough) or for scripting.

## Override with Flags

```sh
crex --config /path/to/config.toml --layouts-dir /path/to/layouts list
```

---

See also: [Commands](commands.md) | [Auto-Save](auto-save.md)
