[Home](../README.md) > Alfred Integration

# Alfred Integration

Search and restore workspaces directly from Alfred. Supports both full layout restore and individual workspace restore.

Works natively with both **cmux** and **Ghostty**.

## cmux Setup

cmux requires one setting change to allow Alfred to communicate with it.

### 1. Enable Automation Mode

Open cmux **Settings > Automation > Socket Control Mode** and set it to **"Automation mode"**.

<p align="center"><img src="../assets/cmux-automation-settings.png" alt="cmux Settings > Automation > Socket Control Mode" width="700"></p>

<p align="center"><img src="../assets/cmux-automation-dropdown.png" alt="Socket Control Mode dropdown" width="250"></p>

Alternatively, add this to `~/.config/cmux/cmux.json`:

```json
{
  "automation": {
    "socketControlMode": "automation"
  }
}
```

Then reload: `cmux reload-config` (or restart cmux).

> **Why is this needed?** By default, cmux only accepts commands from processes running inside its terminals (`cmuxOnly` mode). Alfred runs outside cmux, so it needs `automation` mode to connect via the socket. This is safe — it only allows connections from your user account.

### 2. Create the Alfred Workflow

Open Alfred Preferences > Workflows > click **+** > **Blank Workflow**.
Name it "crex" and give it a description.

### 3. Add a Script Filter

- Right-click the canvas > **Inputs** > **Script Filter**
- **Keyword:** `crex`
- **Language:** `/bin/bash`
- **Script:**

```bash
/opt/homebrew/bin/crex list --alfred
```

Adjust the path if crex is installed elsewhere (`which crex`).

### 4. Add a Run Script action

- Right-click the canvas > **Actions** > **Run Script**
- **Language:** `/bin/bash`
- **Script:**

```bash
export PATH="/opt/homebrew/bin:/Applications/cmux.app/Contents/Resources/bin:$PATH"

# Auto-discover cmux socket
for sock in "$HOME/.local/state/cmux/cmux.sock" \
            "$HOME/Library/Application Support/cmux/cmux-501.sock" \
            "$HOME/Library/Application Support/cmux/cmux.sock"; do
  if [ -S "$sock" ]; then
    export CMUX_SOCKET_PATH="$sock"
    break
  fi
done

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

osascript -e "tell application \"cmux\" to activate" 2>/dev/null
```

### 5. Connect them

Drag a line from the Script Filter to the Run Script.

### 6. Add the icon (optional)

Drop the crex icon (`icon.png`) into the workflow directory.

## Ghostty Setup

Ghostty works without any configuration changes. Use `CREX_BACKEND=ghostty` in the Run Script:

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

osascript -e "tell application \"Ghostty\"" -e "activate" \
  -e "set lastTab to count of tabs of front window" \
  -e "select tab (a reference to tab lastTab of front window)" \
  -e "end tell" 2>/dev/null
```

## Which backend does Alfred restore to?

Alfred runs outside any terminal, so it can't tell whether you're "in" cmux or Ghostty — and if **both** are running, auto-detection is ambiguous. Two ways to make it deterministic:

1. **Pin a default backend in config** (recommended if you use one backend most of the time). Then use the *combined* Run Script below — it sets no `CREX_BACKEND`, so crex honors your config default:

   ```sh
   crex settings backend set ghostty   # or cmux
   ```

   ```bash
   export PATH="/opt/homebrew/bin:/Applications/cmux.app/Contents/Resources/bin:$PATH"

   # Discover the cmux socket. last-socket-path is canonical — stale .sock
   # files linger after cmux restarts and cause "broken pipe" from Alfred.
   for lsp in "$HOME/.local/state/cmux/last-socket-path" \
              "$HOME/Library/Application Support/cmux/last-socket-path"; do
     if [ -f "$lsp" ]; then
       sock=$(cat "$lsp")
       [ -S "$sock" ] && export CMUX_SOCKET_PATH="$sock" && break
     fi
   done
   if [ -z "$CMUX_SOCKET_PATH" ]; then
     for sock in "$HOME/.local/state/cmux/cmux-501.sock" \
                 "$HOME/.local/state/cmux/cmux.sock" \
                 "$HOME/Library/Application Support/cmux/cmux-501.sock" \
                 "$HOME/Library/Application Support/cmux/cmux.sock"; do
       [ -S "$sock" ] && export CMUX_SOCKET_PATH="$sock" && break
     done
   fi

   # Launch the configured backend if it isn't running — from Alfred the
   # terminal may be closed, and you can't restore into a dead app.
   backend=$(crex settings backend get 2>/dev/null | grep -oE 'cmux|ghostty' | head -1)
   case "$backend" in
     ghostty) pgrep -xi ghostty >/dev/null 2>&1 || { open -a Ghostty; sleep 2; } ;;
     cmux)    pgrep -xi cmux    >/dev/null 2>&1 || { open -a cmux;    sleep 2; } ;;
   esac

   action="${1%%:*}"; rest="${1#*:}"
   case "$action" in
     restore)   crex restore "$rest" --mode add ;;
     show)      crex show "$rest" ;;
     delete)    crex delete "$rest" -f ;;
     open)      open "${HOME}/.config/crex/layouts/${rest}.toml" ;;
     workspace) layout="${rest%%:*}"; ws="${rest#*:}"; crex restore "$layout" "$ws" --mode add ;;
   esac
   ```

2. **Two keywords** — a second workflow (keyword e.g. `crexg`) whose Run Script sets `export CREX_BACKEND=ghostty`, keeping `crex` for cmux. Pick the target per invocation.

Note: running crex from *inside a live cmux session* always targets cmux, so the config default only affects external launchers like Alfred.

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

Type any workspace name (e.g. `crex soundinbox`) to search across all saved layouts.

## Environment Variables

| Variable | Purpose | Values |
|----------|---------|--------|
| `CREX_BACKEND` | Override backend detection | `ghostty`, `cmux`, `cmux-applescript` |
| `CMUX_SOCKET_PATH` | Path to cmux socket | Auto-discovered if not set |

## JSON Output

For scripting or integration with other tools:

```bash
crex list --json
```

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

## Troubleshooting

**"Broken pipe" or "Connection refused"**
- cmux socket control mode is not set to "Automation". See [cmux Setup](#cmux-setup).
- **A stale `.sock` file is being picked up** — old sockets linger after cmux restarts. Always discover via `last-socket-path` (the scripts above do); never probe fixed `.sock` paths first.
- **cmux ≥0.64: restart cmux after changing Socket Control Mode** — the socket server takes the mode at startup; the Settings toggle alone may not re-apply it to a live socket.
- cmux socket path changed after restart. The auto-discover script handles this, but if you hardcoded the path, check `cat ~/.local/state/cmux/last-socket-path` or `cat ~/Library/Application\ Support/cmux/last-socket-path`.

**Alfred shows file results instead of workspaces**
- The Script Filter can't find `crex`. Check the path: `which crex`.

**Workspace opens but isn't focused**
- The `osascript activate` at the end of the Run Script brings cmux/Ghostty to the front. If it's missing, add it.
