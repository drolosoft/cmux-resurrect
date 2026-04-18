# crex v1.5.0 — Ship Prompt

> Paste this into a new Claude Code session in the cmux-resurrect project directory.
> This is a self-contained brief. The session doesn't need prior context.

---

## Context

crex (cmux-resurrect) is a Go CLI that saves and restores terminal workspaces for cmux and Ghostty. It ships via Homebrew (`brew install drolosoft/tap/crex`), has 16 built-in templates, Markdown blueprints, adaptive theming, and is the only multi-backend workspace manager in the space.

Current version: v1.4.0. We're building v1.5.0 to close competitive gaps before launching on r/Ghostty.

## Why This Matters

We did a full competitive analysis (saved in the Obsidian vault at `buzz/competitive-landscape-crex-2026-04-18.md`). The landscape:

| Project | Stars | Lang | Backends | Key strength |
|---------|-------|------|----------|-------------|
| gtab | 64 | Rust | Ghostty only | TUI launcher (Cmd+G), 103 upvotes on r/Ghostty |
| gpane | 61 | Shell | Ghostty only | One-command launch with presets |
| summon | 54 | TypeScript | Ghostty only | Setup wizard, port detection, dashboard |
| **crex** | **10** | **Go** | **cmux + Ghostty** | **Only multi-backend. Only templates. Only one that could work on Linux** |

The star gap is a marketing problem, not a product problem. But we need to close three UX gaps before posting:

1. **Invisible persistence** — `crex watch` exists but isn't the default experience
2. **First-run wizard** — competitors walk you through setup; we drop you into a blank config
3. **Interactive TUI** — gtab's Cmd+G fuzzy launcher is their most-loved feature

## Feature 1: Invisible Persistence (`crex watch` evolution)

### What exists
`cmd/watch.go` + `internal/orchestrate/watch.go` — polls on an interval, saves with content-hash dedup. Works but requires manual `crex watch` invocation.

### What to build

**1a. `crex watch --daemon` mode**
- Backgrounding support: fork to background, write PID to `~/.config/crex/crex.pid`
- `crex watch --stop` to kill the daemon
- `crex watch --status` to check if running
- Log to `~/.config/crex/watch.log` (rotate at 1MB)

**1b. Shell integration for auto-start**
- `crex watch --shell-hook` outputs a shell snippet (zsh/bash/fish) that:
  - On terminal launch: starts `crex watch` if not already running
  - On terminal exit: optionally stops the daemon
- User adds `eval "$(crex watch --shell-hook)"` to .zshrc
- The hook should detect the backend (cmux vs Ghostty) and only activate when relevant

**1c. Event-driven saves (stretch goal)**
- Instead of polling every 5m, detect workspace creation/destruction events
- For cmux: watch the socket or use `cmux` CLI events if available
- For Ghostty: may need AppleScript polling or Accessibility API
- Fall back to interval polling if event detection isn't available

**1d. Auto-restore on launch**
- `crex watch --auto-restore` flag: on daemon start, if a previous autosave exists, offer to restore it
- Or: a separate `crex restore --last` that restores the most recent autosave
- The shell hook could integrate this: terminal launches → check for autosave → restore

### Design principle
Make persistence feel built-in, like macterm's automatic save. The user shouldn't think about save/restore — it just happens. But keep manual `crex save`/`crex restore` for explicit control.

### Tests
- Daemon start/stop/status lifecycle
- PID file creation and cleanup
- Content-hash dedup still works in daemon mode
- Shell hook output is valid for zsh/bash/fish
- Auto-restore prompts correctly (or skips if no autosave)

---

## Feature 2: First-Run Wizard (`crex setup`)

### What to build

**`crex setup` command** — interactive guided configuration:

1. **Backend detection** — auto-detect cmux/Ghostty, show what was found
2. **Config creation** — create `~/.config/crex/config.toml` with sensible defaults
3. **First save** — offer to save the current workspace layout right now
4. **Shell completion** — detect shell, offer to install completions
5. **Watch setup** — ask if they want auto-persistence, add shell hook if yes
6. **Template preview** — show 2-3 gallery templates that match their backend

### UX guidelines
- Use the adaptive theme system (dark/light) for wizard output
- Show progress: step 1/5, 2/5, etc.
- Every step should have a default (press Enter to accept)
- Non-destructive: if config already exists, show diff and ask before overwriting
- Complete in under 30 seconds

### Competitive reference
- summon has `summon setup` with interactive prompts for editor, sidebar, layout, shell
- gtab has `gtab init` (one command, writes keybind config)
- Our wizard should be richer than gtab's but faster than summon's

### Tests
- Config file creation with defaults
- Backend auto-detection (mock cmux/Ghostty)
- Idempotent: running twice doesn't break anything
- Non-interactive mode: `crex setup --defaults` for CI/scripting

---

## Feature 3: Interactive TUI (`crex` with no args, or `crex tui`)

### What to build

**Fuzzy-search TUI launcher** — when user types `crex` with no args (or `crex tui`):

1. Show saved layouts and gallery templates in a searchable list
2. Fuzzy filter as user types
3. Enter to restore/apply selected layout
4. Preview pane showing workspace details (names, CWDs, pane count)
5. Keyboard shortcuts: `d` delete, `s` save current, `e` edit, `q` quit

### Tech
- Use [Bubble Tea](https://github.com/charmbracelet/bubbletea) — it's the standard Go TUI framework
- Use [Lip Gloss](https://github.com/charmbracelet/lipgloss) for styling (already may be in deps for adaptive theming)
- Use [Bubbles](https://github.com/charmbracelet/bubbles) for the text input and list components

### Current behavior
`cmd/root.go` — with no args, prints the banner + styled help. The TUI replaces this:
- `crex` → launches TUI (if layouts exist) or shows banner + help (if first run)
- `crex --help` → still shows help text
- `crex tui` → explicit TUI entry point (always)

### Competitive reference
- gtab: TUI with fuzzy search, keybindings table (/, Enter, a, n, d, e, q)
- Our TUI should match gtab's keybindings but add: template browsing, layout preview, backend indicator

### Design principle
Don't add Bubble Tea as a required dependency for the entire CLI. The TUI is one command — isolate it in `cmd/tui.go` and `internal/tui/`. The rest of crex stays lean.

### Tests
- Model state transitions (list → filter → select → restore)
- Keyboard shortcut handling
- Empty state (no layouts)
- Layout preview rendering

---

## Architecture Notes

### Project structure
```
cmd/                    # Cobra commands
  root.go               # Entry point, banner
  save.go, restore.go   # Core commands
  watch.go              # Existing watch (enhance for daemon)
  setup.go              # NEW: first-run wizard
  tui.go                # NEW: TUI launcher
  template.go           # Template system
  theme.go              # Adaptive theming
  style.go              # Rendering helpers
internal/
  client/               # Backend abstraction (cmux, ghostty)
  config/               # TOML config
  gallery/              # Template gallery
  model/                # Data models (Layout, Workspace)
  orchestrate/          # Business logic (save, restore, watch, import, export)
  persist/              # File storage
  tui/                  # NEW: Bubble Tea TUI
```

### Backends
- `internal/client/detect.go` — auto-detects cmux vs Ghostty
- `internal/client/cli.go` — cmux backend (CLI API)
- `internal/client/ghostty.go` — Ghostty backend (AppleScript)
- Both implement the `client.Backend` interface

### Config
- `~/.config/crex/config.toml` — TOML format
- `internal/config/config.go` — struct with defaults
- Fields: `layouts_dir`, `workspace_file`, `watch_interval`, `max_autosaves`, `banner_style`

### Testing
- Run: `go test -count=1 ./...`
- Current: 214 tests across 8 packages, all passing
- Style: table-driven tests, mock backends where needed
- Every new feature needs tests before claiming done

---

## Version & Release

- Version is set via goreleaser ldflags — no hardcoded version in source
- CI triggers on `v*` tag push
- After all features pass tests:
  1. Update README.md with new features
  2. Update website at `/Users/txeo/Git/mac/go/drolosoft/public/cmux-resurrect.html`
  3. Update RUNBOOK.md if new manual test scenarios needed
  4. Tag `v1.5.0` and push
  5. Push to both remotes: `origin` (juanatsap) and `drolosoft`
  6. Update all open directory PRs with new version info

---

## Priority Order

Ship in this order — each builds on the previous:

1. **Setup wizard** — smallest scope, immediate UX improvement, unblocks the "try it" story
2. **Watch daemon** — the killer differentiator, makes persistence invisible
3. **TUI** — the polish layer, matches gtab's most-loved feature

If time is tight, ship wizard + watch as v1.5.0 and TUI as v1.6.0. The watch feature alone is enough to differentiate on r/Ghostty — nobody else has invisible persistence.

---

## Post-Ship: r/Ghostty Launch

After shipping, the community launch follows the Drolosoft Buzz Playbook (vault: `buzz/drolosoft-buzz-playbook.md`). Key points:

- Build r/Ghostty karma first (comment helpfully for 48h+)
- Post with "journey" template: problem → what I tried → the surprise → proof
- Lead with **Linux + multi-backend** angle — no other tool can claim this
- Title format: effort/time + concrete outcome (never "I built X")
- Tool link in comments, not post body
- Reply to every comment for 2-3 hours

Use the post-review-team agent (8-expert pipeline) before publishing.
