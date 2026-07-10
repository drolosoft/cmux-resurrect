---
name: crex
description: Use when saving, restoring, or switching terminal workspace layouts (cmux or Ghostty), resuming AI coding sessions after a restart or crash, setting up project workspaces from templates, snapshotting terminal state before risky operations, or querying saved layouts programmatically.
---

# Driving crex (terminal workspace manager)

crex saves and restores terminal workspaces — tabs, splits, per-pane working directories, running commands — as named layouts, on cmux and Ghostty (auto-detected). It also captures running AI CLI sessions (Claude Code, Codex, OpenCode, and 12 more) and resumes them mid-conversation on restore.

## Quick reference

| Task | Command |
|---|---|
| Snapshot current terminal state | `crex save <name> -d "why"` |
| Restore, keeping what's open | `crex restore <name> --mode add` |
| Restore, replacing everything | `crex restore <name> --mode replace` |
| Preview a restore (no changes) | `crex restore <name> --dry-run --mode add` |
| List saved layouts (JSON) | `crex list --json` |
| Inspect a layout's contents | `crex show <name>` (styled) / `crex show <name> --raw` (TOML) |
| Live workspaces + panes | `crex now` (human-readable only) |
| New workspace from template | `crex template use ide <path>` (16 templates: `crex template list`) |
| Create tabs from a Markdown Blueprint | `crex import-from-md [--workspace-file <path>] --dry-run` first |

## Rules for agents

1. **Always pass `--mode` when scripting a restore.** Without it, crex prompts interactively (replace vs add) and your command hangs. `add` is non-destructive: it syncs by workspace title — already-open workspaces are skipped, missing ones are created.
2. **Verify the layout exists first** — restoring a nonexistent name falls into an interactive picker (hangs scripts). Guard: `crex list --json | jq -e --arg n "NAME" 'map(.name) | index($n)'`. JSON fields: `name`, `description`, `saved_at`, `workspace_count`, `workspace_titles`, `workspace_summaries`, `file_path`. If the name is close-but-wrong, suggest the nearest existing name to the user — never guess.
3. **Preview before mutating**: `--dry-run` prints the exact backend commands a restore would run, without executing. It does NOT skip the mode prompt — always pair it with `--mode`.
4. **Snapshot-before-risk pattern**: `crex save pre-<task> -d "rollback point"` → do the risky thing → roll back with `crex restore pre-<task> --mode replace` if needed.
5. **Re-saving over an existing name is safe and idempotent.** It refreshes the layout from live state, preserves user-edited descriptions and hand-typed commands, and bumps the revision only when content changed. Reuse names freely. There is NO history/undo of previous revisions — if unsure, snapshot to a new name first.
6. **Never `crex delete` without explicit user confirmation.**

## AI session resume (not obvious from --help)

`crex save` detects running AI CLI sessions and stores their resume commands (e.g. `claude --resume <session-id>`). `crex restore` replays them — the session continues mid-conversation, not from scratch. To check what a layout will resume: `crex show <name>` lists each pane's command.

`--mode add` skips workspaces that are already open (matched by title) — it won't repair a dead session inside an open workspace. For that, read the pane's resume command from `crex show <name>` and run it in that pane yourself (or ask the user).

## Querying layout contents programmatically

`list --json` does NOT include directories. For contents, parse the TOML: `crex show <name> --raw`. Traps:
- Paths are stored **absolute** (`/Users/x/...`) — grepping for `~/...` finds nothing.
- Anchor path matches to the full quoted value (`cwd = '/exact/path'`) — substring greps false-positive on sibling dirs (`laporra` matches `laporra-go`).
- Layout files live in `~/.config/crex/layouts/*.toml` (override: `--layouts-dir`).

## Blueprints (Markdown workspace definitions)

A Blueprint is a Markdown file (Obsidian-compatible) declaring workspaces/tabs. Default path `~/.config/crex/workspaces.md`, override with `--workspace-file`. `crex import-from-md` is idempotent (creates only missing tabs); validate an unfamiliar file with `--dry-run`. Add entries with `crex blueprint add`. Format docs: `crex help blueprint` or the repo's docs/blueprint.md.

## Backends

cmux and Ghostty are auto-detected; force with `CREX_BACKEND=cmux|ghostty`. Split geometry restores pixel-exact on cmux; on Ghostty splits recreate as a right-chain (its API exposes no pane frames) but every pane's directory is preserved.

## Common mistakes

| Mistake | Fix |
|---|---|
| `crex restore x` in a script hangs | Add `--mode add` (or `replace`) |
| `crex now --json` | No such flag — `now` is human-only; use `list --json` for saved layouts |
| Grepping layouts for `~/path` | Paths are absolute in TOML |
| Treating `crex now` as a snapshot | It only displays; `crex save` persists |
| Inventing layout names | Check `crex list --json` first |
