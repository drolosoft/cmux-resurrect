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
max_autosaves = 10   # reserved — rotation not implemented yet

# Banner style: "flame", "classic", or "plain"
banner_style = "flame"

# Default restore behavior: "ask" (default), "replace", or "add"
restore_mode = "ask"

# Default backend when auto-detection is ambiguous (cmux AND Ghostty both
# running). Unset = auto-detect. Values: "cmux", "ghostty".
backend = "ghostty"
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
| 🔀 Restore mode | `ask` |
| 🖥️ Backend | auto-detect |

## Default Backend

By default crex auto-detects the backend at runtime. When **both** cmux and Ghostty are running, detection is ambiguous — pin a default so every surface (CLI, TUI, and the Alfred workflow, which runs outside any terminal) agrees:

```sh
crex settings backend set ghostty   # or cmux, or auto to clear
crex settings backend get
```

Precedence: `CREX_BACKEND` (env) → this config default → auto-detection. Running `crex` from **inside a live cmux session** always uses cmux regardless of the config default, so `crex save`/`restore` in a cmux tab is never hijacked.

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

The list takes the agent names below. The flag goes right after the command that starts the agent, which is not always the agent's name: `factory` runs as `droid` and `rovo` starts with `acli rovodev run`.

| Agent | Command | Flag injected |
|-------|---------|--------------|
| claude | `claude` | `--dangerously-skip-permissions` |
| opencode | `opencode` | `--auto` |
| codex | `codex` | `--dangerously-bypass-approvals-and-sandbox` |
| amp | `amp` | `--dangerously-allow-all` |
| gemini | `gemini` | `--approval-mode=yolo` |
| copilot | `copilot` | `--allow-all` |
| grok | `grok` | `--always-approve` |
| cursor | `agent` | `--force` |
| aider | `aider` | `--yes-always` |
| pi | `pi` | none (Pi does not ask before running tools) |
| rovo | `acli rovodev run` | `--yolo` |
| hermes | `hermes` | `--yolo` |
| codebuddy | `codebuddy` | `--dangerously-skip-permissions` |
| factory | `droid` | `--auto high` |
| qoder | `qoder` | `--yolo` |
| qwen | `qwen` | `--yolo` |
| goose | `goose` | none (Goose approves tool calls by default) |
| crush | `crush` | `--yolo` |
| kiro | `kiro-cli` | `--trust-all-tools`, after `chat` |
| antigravity | `agy` | `--dangerously-skip-permissions` |
| cline | `cline` | none (Cline approves tool calls by default) |
| vibe | `vibe` | `--auto-approve` |
| omp | `omp` | `--yolo` |
| kimi | `kimi` | `--auto` |

OpenCode releases older than the ones that document `--auto` (1.14.48, for example) reject any flag they do not know and will not start. Leave `opencode` out of the list on those versions.

Kiro takes `--trust-all-tools` only after its `chat` subcommand, so crex types `kiro-cli chat --trust-all-tools --resume-id <id>`, and `kiro-cli chat --trust-all-tools` for a pane with no session. A `kiro-cli` command that runs another subcommand gets no flag.

Hermes keeps the approval mode a session was created with and exits when a resume asks for another, so crex adds `--yolo` only when Hermes starts a fresh session, never to `hermes --resume`.

You can also configure this during `crex setup`.

> **Warning:** This is dangerous. Agents will execute file changes, shell commands,
> and other actions without asking for permission.

## Environment Variables

| Variable | Purpose | Values |
|----------|---------|--------|
| `CREX_THEME` | Force dark or light palette | `dark`, `light` |
| `CREX_BANNER` | Override banner style | `flame`, `classic`, `plain` |
| `CREX_NO_WATCH` | Prevent daemon auto-start from shell hooks | `1` (any truthy value) |
| `CREX_BACKEND` | Override backend detection | `ghostty`, `cmux`, `cmux-applescript` |

Useful when auto-detection fails (e.g. terminal multiplexers blocking OSC 11 passthrough), for scripting, or for [Alfred integration](alfred.md) where the standard detection cannot reach the terminal backend.

## Override with Flags

```sh
crex --config /path/to/config.toml --layouts-dir /path/to/layouts list
```

---

See also: [Commands](commands.md) | [Auto-Save](auto-save.md)
