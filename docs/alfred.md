[Home](../README.md) > Alfred Integration

# Alfred Integration

Search and restore your saved layouts directly from Alfred.

## Setup

### 1. Create a new Alfred Workflow

Open Alfred Preferences > Workflows > click **+** > **Blank Workflow**.
Name it "crex" and give it a description.

### 2. Add a Script Filter

- Right-click the canvas > **Inputs** > **Script Filter**
- **Keyword:** `crex`
- **Language:** `/bin/bash`
- **Script:** `/usr/local/bin/crex list --alfred`
  - If installed via Homebrew: use the path from `which crex`

### 3. Add a Run Script action

- Right-click the canvas > **Actions** > **Run Script**
- **Language:** `/bin/bash`
- **Script:**

```bash
action="${1%%:*}"
name="${1#*:}"

case "$action" in
  restore) /usr/local/bin/crex restore "$name" ;;
  show)    /usr/local/bin/crex show "$name" ;;
  delete)  /usr/local/bin/crex delete "$name" ;;
  open)    open "${HOME}/.config/crex/layouts/${name}.toml" ;;
esac
```

### 4. Connect them

Drag a line from the Script Filter to the Run Script.

### 5. Add the icon (optional)

Drop the crex icon (`icon.png`) into the workflow directory.

## Usage

| Action | Key | What it does |
|--------|-----|-------------|
| Restore | Enter | `crex restore <name>` |
| Show details | Cmd+Enter | `crex show <name>` |
| Delete | Alt+Enter | `crex delete <name>` |
| Open TOML | Ctrl+Enter | Opens layout file in default editor |

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
    "workspace_summaries": ["claude · shell", "shell"],
    "file_path": "/Users/you/.config/crex/layouts/dev-project.toml"
  }
]
```
