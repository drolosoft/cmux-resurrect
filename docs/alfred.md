[Home](../README.md) > Alfred Integration

# Alfred Integration

Search and restore workspaces directly from Alfred. Supports both full layout restore and individual workspace restore.

## Requirements

- **Ghostty** as your terminal (Alfred integration uses the Ghostty AppleScript backend)
- **cmux limitation:** cmux restricts socket access to child processes. Alfred cannot control cmux directly. If you use cmux, workspaces will open in Ghostty instead.

## Setup

### 1. Create a new Alfred Workflow

Open Alfred Preferences > Workflows > click **+** > **Blank Workflow**.
Name it "crex" and give it a description.

### 2. Add a Script Filter

- Right-click the canvas > **Inputs** > **Script Filter**
- **Keyword:** `crex`
- **Language:** `/bin/bash`
- **Script:**

```bash
/opt/homebrew/bin/crex list --alfred
```

Adjust the path if crex is installed elsewhere (`which crex`).

### 3. Add a Run Script action

- Right-click the canvas > **Actions** > **Run Script**
- **Language:** `/bin/bash`
- **Script:**

```bash
export PATH="/opt/homebrew/bin:$PATH"
export CREX_BACKEND=ghostty

action="${1%%:*}"
rest="${1#*:}"
case "$action" in
  restore) crex restore "$rest" --mode add ;;
  show)    crex show "$rest" ;;
  delete)  crex delete "$rest" ;;
  open)    open "${HOME}/.config/crex/layouts/${rest}.toml" ;;
  workspace)
    layout="${rest%%:*}"
    ws="${rest#*:}"
    crex restore "$layout" "$ws" --mode add ;;
esac

# Focus the last tab and bring Ghostty to front
osascript -e "tell application \"Ghostty\"" -e "activate" \
  -e "set lastTab to count of tabs of front window" \
  -e "select tab (a reference to tab lastTab of front window)" \
  -e "end tell" 2>/dev/null
```

### 4. Connect them

Drag a line from the Script Filter to the Run Script.

### 5. Add the icon (optional)

Drop the crex icon (`icon.png`) into the workflow directory.

## Usage

Alfred shows two types of items:

- **📦 Layout items** — restore the full layout (all workspaces)
- **Workspace items** — restore a single workspace from a layout

| Key | On layout item | On workspace item |
|-----|---------------|-------------------|
| Enter | Restore full layout | Restore single workspace |
| Cmd+Enter | Show layout details | Restore full layout |
| Alt+Enter | Delete layout | Show layout details |
| Ctrl+Enter | Open TOML file | Open TOML file |

Type any workspace name (e.g. `crex soundinbox`) to search across all layouts.

## Environment Variables

The Run Script uses `CREX_BACKEND=ghostty` to force the Ghostty AppleScript backend. This is required because Alfred runs outside the terminal — the standard cmux CLI backend cannot connect from Alfred's process.

| Variable | Purpose | Values |
|----------|---------|--------|
| `CREX_BACKEND` | Override backend detection | `ghostty`, `cmux`, `cmux-applescript` |

- `ghostty` — Ghostty AppleScript backend (recommended for Alfred)
- `cmux` — cmux CLI backend (socket, requires being inside cmux)
- `cmux-applescript` — cmux via Ghostty's AppleScript protocol (experimental)

## JSON Output

For scripting or integration with other tools:

```bash
crex list --json
```

Outputs a JSON array of layout metadata:

```json
[
  {
    "name": "dev-project",
    "description": "My main setup",
    "saved_at": "2026-06-02T14:30:00Z",
    "workspace_count": 2,
    "workspace_titles": ["0 dev", "1 docs"],
    "workspace_panes": [2, 1],
    "workspace_summaries": ["claude · shell", "shell"],
    "file_path": "/Users/you/.config/crex/layouts/dev-project.toml"
  }
]
```
