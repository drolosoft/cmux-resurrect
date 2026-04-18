[Home](../README.md) > Workflows

# 🔑 Two Workflows

crex offers two distinct ways to manage your terminal workspaces.

## 💾 Save / Restore — Session Recovery

**Use case**: your terminal crashed, your machine rebooted, or you want to switch between layouts.

`save` takes an exact snapshot of your running session — every workspace, split, CWD, pinned state, and active tab — and writes it to a TOML file. `restore` reads that TOML and recreates everything exactly as it was.

```sh
# End of day: snapshot your layout
crex save my-day

# Next morning: bring it all back
crex restore my-day
```

Think of it as **backup and recovery**. The TOML file is a photograph of your session at a point in time.

## 📥 Import from Markdown — Workspace as Code

**Use case**: you maintain a Workspace Blueprint describing your ideal workspace setup, and you want your terminal to match it.

`import-from-md` reads a Workspace Blueprint (.md, compatible with Obsidian), resolves templates into pane layouts, and creates only the workspaces that **don't already exist**. Running it twice does nothing the second time — it's idempotent.

```sh
# Define your workspaces in a .md file, then:
crex import-from-md

# Add a new workspace entry, then import again:
crex workspace add api ~/projects/api -t dev --icon "⚙️"
crex import-from-md
```

Think of it as **infrastructure as code** for your terminal. The Workspace Blueprint is the source of truth; `import-from-md` makes your terminal match it. The reverse operation, `export-to-md`, captures your live state back into the Blueprint.

## Side by Side

| | Save / Restore | Import from Markdown |
|---|---|---|
| Source | TOML file (auto-generated snapshot) | Workspace Blueprint (hand-written or managed via CLI) |
| Creates | Everything, every time | Only what's missing (idempotent) |
| Pane layout | Captured from live session | Defined by templates (`dev`, `go`, `monitor`) |
| Best for | Crash recovery, switching contexts | Standardized workspace setup, onboarding |

## 👁️ Dry-Run Preview

See exactly what will happen **before** it happens:

```sh
crex restore my-day --dry-run
```

```
Dry-run restore of "my-day":

cmux new-workspace --cwd "/home/user/projects/webapp"
cmux rename-workspace --workspace workspace:new_0 "0 webapp"
cmux send --workspace workspace:new_0 "npm run dev"
cmux new-split right --workspace workspace:new_0
cmux send --workspace workspace:new_0 "lazygit"
cmux new-workspace --cwd "/home/user/projects/api-server"
cmux rename-workspace --workspace workspace:new_1 "1 api-server"
cmux new-split right --workspace workspace:new_1
cmux send --workspace workspace:new_1 "go test ./..."
cmux new-workspace --cwd "/home/user/projects/dashboard"
cmux rename-workspace --workspace workspace:new_2 "2 dashboard"
cmux new-workspace --cwd "/home/user/documents/notes"
cmux rename-workspace --workspace workspace:new_3 "3 notes"
cmux select-workspace --workspace workspace:new_0

14 commands for 4 workspaces
```

Every backend command listed. Nothing executed. Inspect, verify, **then** run without `--dry-run`.

> **Backend note:** The dry-run output above shows cmux commands. When running with the Ghostty backend, equivalent AppleScript commands are shown instead. The `--dry-run` flag works the same way regardless of backend.

---

See also: [Commands](commands.md) | [Workspace Blueprints](blueprint.md)
