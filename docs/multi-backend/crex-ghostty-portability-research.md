# Ghostty Portability Assessment for crex (cmux-resurrect)

**Date:** 2026-04-17
**Research:** Deep ecosystem analysis — Ghostty, cmux, competitive landscape, technical feasibility

---

## 1. Executive Summary

Ghostty (51K stars) is a **terminal emulator**, not a multiplexer -- it has built-in splits/tabs but no workspace abstraction, no session persistence model, and no template/layout system. As of Ghostty 1.3 (March 2026), a **preview AppleScript API** on macOS exposes windows, tabs, splits, and terminals with creation/input/focus commands -- enough to port crex's core orchestration. However, the API is macOS-only, explicitly unstable (breaking changes expected in 1.4), and missing critical operations crex needs (notably: listing/enumerating current state for save, and getting CWD from panes). A Ghostty port is **technically feasible on macOS today** with significant caveats, and the community demand for exactly this kind of tool is strong. The existing **ghostty-workspace** project (Python/YAML, also AppleScript-based) validates the approach but covers only ~40% of what crex does. Your strongest play is **Option B: evolve crex into a multi-backend tool** with a clean abstraction layer, shipping a Ghostty backend alongside the existing cmux backend.

---

## 2. Ghostty vs cmux: What They Are

### Ghostty
- **Type**: Terminal emulator (GPU-accelerated, cross-platform: macOS + Linux)
- **Stars**: 51,012 (as of today) -- the fastest-growing terminal emulator on GitHub
- **Created by**: Mitchell Hashimoto (HashiCorp co-founder), now under Hack Club 501(c)(3)
- **Key features**: Platform-native UI, built-in splits/tabs, shell integration, Ghostty config format
- **What it is NOT**: A terminal multiplexer. No session detach/reattach. No workspace concept. No layout persistence beyond macOS `window-save-state` (which only restores the *previous* session, not named layouts)

### cmux
- **Type**: Native macOS terminal multiplexer built on libghostty
- **Stars**: 14,511
- **Created by**: Manaflow (YC-backed, 2-person team)
- **Key features**: Vertical tabs as "workspaces", split panes, embedded browser, socket API + CLI, environment variables for detection (`CMUX_WORKSPACE_ID`, `CMUX_SURFACE_ID`, `CMUX_SOCKET_PATH`)
- **Relationship to Ghostty**: Uses libghostty as a rendering library (like using WebKit for web views). Reads `~/.config/ghostty/config` for themes/fonts/keybindings. NOT a fork -- completely separate app with its own workspace/multiplexer layer on top.

### The Critical Distinction

| Concept | cmux | Ghostty |
|---------|------|---------|
| Workspaces (named, pinnable) | Native first-class concept | Does not exist |
| Tabs | Within workspaces | Top-level tabs |
| Split panes | Within workspaces | Within tabs |
| Session persistence | No (that's what crex provides) | `window-save-state` on macOS only, no named layouts |
| Programmatic API | Full CLI + Unix socket (`/tmp/cmux.sock`) | AppleScript (macOS, preview), D-Bus (Linux, planned) |
| CWD introspection | `cmux sidebar-state` returns CWD per workspace | AppleScript exposes `working directory` per terminal |
| Tree inspection | `cmux tree --json` returns full hierarchy | No equivalent JSON tree -- must enumerate via AppleScript |

---

## 3. Ghostty's Control Surface (API/IPC/Scripting)

### macOS: AppleScript (Ghostty 1.3+, March 2026)

**Object hierarchy**: `application -> windows -> tabs -> terminals`

**Available commands**:
| Command | What it does | crex equivalent |
|---------|-------------|----------------|
| `new window` | Create window with optional config | N/A (crex works within one window) |
| `new tab` | Create tab in target window | `NewWorkspace` |
| `split` | Split terminal (right/left/down/up) | `NewSplit` |
| `input text` | Send paste-style content | `Send` |
| `send key` | Trigger key events with modifiers | N/A |
| `focus` | Activate terminal and window | `FocusPane` + `SelectWorkspace` |
| `select tab` | Switch active tab | `SelectWorkspace` |
| `close` | Close terminal/tab/window | `CloseWorkspace` |
| `set_tab_title` (via `perform action`) | Rename tab | `RenameWorkspace` |

**Readable properties**:
| Property | Object | crex equivalent |
|----------|--------|----------------|
| `id` | window, tab, terminal | Refs (workspace:N, pane:N) |
| `name` | window, tab, terminal | Title |
| `working directory` | terminal | `SidebarState.CWD` |
| `index` | tab | Workspace index |
| `selected` | tab | Active/Selected workspace |

**Surface configuration** (for `new window`/`new tab`/`split`):
- `initial working directory` -- sets CWD
- `command` -- runs a command
- `initial input` -- sends initial text
- `font size`, `environment variables`

**What's MISSING for crex**:
- **No `pinned` concept** -- Ghostty tabs cannot be pinned
- **No `tree --json` equivalent** -- you must iterate through AppleScript objects; no single-call snapshot
- **No `sidebar-state`** -- but `working directory` on terminal objects serves the same purpose
- **Preview/unstable** -- breaking changes expected in Ghostty 1.4 (September 2026)

### Linux: D-Bus (Planned for 1.4, September 2026)

Not available today. The Ghostty team has committed to D-Bus for Linux, leveraging existing GTK integration. No timeline guarantees.

### Cross-platform CLI: Not planned

Mitchell Hashimoto explicitly chose platform-specific IPC over a unified cross-platform API. There will be no `ghostty` CLI equivalent to `cmux` CLI for controlling a running instance.

---

## 4. What crex Needs (Operation-by-Operation Feasibility)

### Core Operations (from `CmuxClient` interface)

| crex Operation | cmux Implementation | Ghostty Equivalent | Feasibility |
|---------------|--------------------|--------------------|-------------|
| `Ping()` | `cmux ping` | Check if Ghostty app is running | EASY |
| `Tree()` | `cmux tree --json` | Enumerate windows/tabs/terminals via AppleScript | MODERATE -- no single call, must loop |
| `SidebarState()` | `cmux sidebar-state` | Read `working directory` from terminal object | EASY |
| `ListWorkspaces()` | `cmux list-workspaces` | List tabs from front window | EASY |
| `NewWorkspace()` | `cmux new-workspace --cwd` | `new tab` with `initial working directory` config | EASY |
| `RenameWorkspace()` | `cmux rename-workspace` | `perform action "set_tab_title:..."` | MODERATE |
| `SelectWorkspace()` | `cmux select-workspace` | `select tab` | EASY |
| `NewSplit()` | `cmux new-split right` | `split` direction on target terminal | EASY |
| `FocusPane()` | `cmux focus-pane` | `focus` on specific terminal | MODERATE -- need terminal ref mapping |
| `Send()` | `cmux send` | `input text` on terminal | EASY |
| `PinWorkspace()` | `cmux workspace-action pin` | **NOT POSSIBLE** -- Ghostty has no pin concept | IMPOSSIBLE |
| `CloseWorkspace()` | `cmux close-workspace` | `close` on tab | EASY |

### Verdict: 11 of 12 core operations are feasible. Only `PinWorkspace` has no Ghostty equivalent.

---

## 5. Competitive Landscape

### Existing Ghostty Session/Layout Tools

| Tool | Approach | Capabilities | Limitations |
|------|----------|-------------|-------------|
| **ghostty-workspace** | Python + YAML + AppleScript | Tabs, splits, titles, commands, working dirs, dry-run | No save (create-only), no gallery, no watch mode, split sizing approximate |
| **AppleScript blog scripts** | Raw AppleScript via System Events keystroke | Basic split creation + commands | Fragile keystroke-based, no state query |

**Nobody is doing what crex does for Ghostty**: save/restore, template gallery, watch mode, Blueprint (MD-based workspace definitions), import/export. The field is wide open.

### Session Management in Other Terminal Emulators

| Terminal | Session Management | Programmatic API | Save/Restore | Templates/Layouts |
|----------|-------------------|-----------------|-------------|-------------------|
| **iTerm2** | Window Arrangements (save/restore named layouts) | Full Python API + AppleScript | Built-in | Profiles (not templates) |
| **Kitty** | Session files (text format defining tabs/panes/commands) | Full remote control via `kitten @` | `save_as_session` command | Session files ARE templates |
| **WezTerm** | Lua-based workspace config | Full Lua API + CLI | resurrect.wezterm plugin | Lua config IS the template |
| **Ghostty** | `window-save-state` (macOS only, previous session only) | AppleScript preview (macOS) | **No named layouts** | **Nothing** |
| **Alacritty** | None (no splits/tabs) | None | None | None |

### Key Insight

Kitty and WezTerm already have strong built-in session management. iTerm2 has mature arrangements. **Ghostty is the one major terminal emulator with 51K stars that has NO workspace/session/template tooling** -- this is a massive gap. The community has been requesting it since at least January 2025 (Discussion #2480, #3358, #4396, #9825 -- all highly upvoted, some locked due to "me too" posts).

---

## 6. Strategic Options Assessment

### Option A: Fork crex -- New Tool "gx" for Ghostty Only

**Approach**: Separate binary, Ghostty-specific, clean slate.

| Dimension | Assessment |
|-----------|-----------|
| Technical feasibility | HIGH -- AppleScript API covers ~92% of needed operations |
| Effort | MEDIUM -- ~3-4 weeks to build Ghostty backend + adapt orchestration |
| Community reception | HIGH -- fills the biggest gap in the Ghostty ecosystem |
| Template Gallery | FULLY PORTABLE -- templates are backend-agnostic (they're just pane layouts + commands) |
| Risk | Ghostty AppleScript is preview; breaking changes in 1.4 |
| Platform | macOS only until Linux D-Bus ships |

### Option B: Evolve crex -- Multi-Backend Architecture (RECOMMENDED)

**Approach**: Abstract `CmuxClient` interface into a `Backend` interface. Implement `CmuxBackend` (existing code) and `GhosttyBackend` (new AppleScript-based). Single binary, runtime backend selection.

| Dimension | Assessment |
|-----------|-----------|
| Technical feasibility | HIGH -- the interface is already clean (`CmuxClient` has 12 methods) |
| Effort | MEDIUM -- 2-3 weeks for abstraction + Ghostty backend |
| Community reception | HIGHEST -- one tool for both ecosystems, bigger addressable market |
| Template Gallery | FULLY PORTABLE -- zero changes needed, templates are backend-independent |
| Risk | Same AppleScript preview risk, but cmux backend keeps working |
| Platform | Ghostty backend macOS-only initially; cmux backend macOS-only anyway |

**Architecture sketch**:
```
                    Backend interface
                   /                \
          CmuxBackend            GhosttyBackend
         (existing CLI)       (AppleScript via osascript)
              |                       |
            cmux                   Ghostty
```

The `CmuxClient` interface in `internal/client/client.go` is already a perfect abstraction boundary. You would:
1. Rename `CmuxClient` to `Backend` (or create a `Backend` type alias)
2. Move existing `CLIClient` to `internal/backend/cmux/`
3. Create `internal/backend/ghostty/` implementing the same interface via `osascript` calls
4. Auto-detect backend at runtime (check for `CMUX_WORKSPACE_ID` env var for cmux, or fall back to Ghostty if the app is running)

### Option C: Ghostty Plugin/Extension

**Assessment**: NOT POSSIBLE today -- Ghostty has no plugin system.

### Option D: Not Feasible

**Assessment**: REJECTED. The AppleScript API, while preview, provides sufficient control surface.

---

## 7. Template Gallery Portability

This is the best news: **the Template Gallery is 100% backend-agnostic**.

Templates produce `[]model.Pane` -- a pure data structure containing split direction, command, focus target, and type. This data is completely backend-independent. The `Restorer`, `Importer`, and `TemplateUser` all consume `[]model.Pane` and call the `CmuxClient` interface methods.

**All 16 gallery templates work without modification** for any backend that can:
1. Create a tab/workspace
2. Split in a direction (right/down)
3. Focus a specific pane before splitting (for quad/complex layouts)
4. Send a command to a specific pane

Ghostty's AppleScript API supports all four.

The Workspace Blueprint (MD file) format is also fully portable.

---

## 8. Risks and Unknowns

### High Risk
- **AppleScript API instability**: Explicitly preview in 1.3; breaking changes expected in 1.4 (September 2026)
- **macOS-only**: Until D-Bus lands on Linux (1.4 at earliest), Ghostty backend only serves macOS users

### Medium Risk
- **No tree/snapshot API**: Must enumerate via AppleScript loops (slower, more fragile)
- **No pin concept**: `pinned` field would be ignored
- **Split sizing**: No way to set split ratios on creation (equal splits only)

### Low Risk
- **Timing/delays**: Same pattern crex already uses for cmux
- **Rename timing**: Shell prompts overwrite tab titles -- existing delay pattern transfers

### Unknowns
- **`working directory` reliability**: Does it update when you `cd`? Critical for save.
- **D-Bus API shape**: Will it mirror AppleScript? May need third backend.
- **Ghostty 1.4 scope**: May add proper scripting API superseding AppleScript.

---

## 9. Recommended Next Steps

### Phase 1: Validate (1-2 days)
1. PoC: Go function calling `osascript` for Ghostty AppleScript (create tab, split, send command, read CWD, rename)
2. Test terminal enumeration (ref-diffing after split)
3. Test `working directory` property updates on `cd`

### Phase 2: Abstract (1 week)
1. Extract `CmuxClient` into `Backend` interface
2. Move existing code to `internal/backend/cmux/`
3. Implement `internal/backend/ghostty/` via `osascript`
4. Add backend auto-detection

### Phase 3: Ship (1-2 weeks)
1. Add `--backend ghostty|cmux|auto` flag
2. Integration tests for Ghostty backend
3. Update docs and README

### Phase 4: Community (ongoing)
1. Post in Ghostty Discussions (#2480, #3358, #9825) -- "I built this"
2. Post on r/ghostty, r/commandline, Hacker News
3. The Ghostty community is **actively asking for this tool**

---

## Key Numbers

| Metric | Value |
|--------|-------|
| Ghostty GitHub stars | 51,012 |
| cmux GitHub stars | 14,511 |
| Addressable user increase | ~4.5x |
| crex operations feasible on Ghostty | 11/12 (92%) |
| Template Gallery changes needed | 0 (fully portable) |
| Existing Ghostty session tools | 1 (ghostty-workspace, covers ~40% of crex) |
| Community discussions requesting this | 4+ (some locked for too many upvotes) |

---

## Sources

- [Ghostty GitHub](https://github.com/ghostty-org/ghostty) (51K stars)
- [cmux GitHub](https://github.com/manaflow-ai/cmux) (14.5K stars)
- [Ghostty AppleScript Docs](https://ghostty.org/docs/features/applescript)
- [Ghostty 1.3.0 Release Notes](https://ghostty.org/docs/install/release-notes/1-3-0)
- [Ghostty Keybinding Action Reference](https://ghostty.org/docs/config/keybind/reference)
- [ghostty-workspace tool](https://github.com/manonstreet/ghostty-workspace)
- [Discussion #2480: Define split layouts](https://github.com/ghostty-org/ghostty/discussions/2480)
- [Discussion #3358: Session manager](https://github.com/ghostty-org/ghostty/discussions/3358)
- [Discussion #9825: Save workspace configuration](https://github.com/ghostty-org/ghostty/discussions/9825)
- [Discussion #2353: Scripting API](https://github.com/ghostty-org/ghostty/discussions/2353)
- [Ghostty AppleScript automation blog](https://samuellawrentz.com/blog/ghostty-applescript-project-terminal-layouts/)
- [Kitty session management](https://sw.kovidgoyal.net/kitty/sessions/)
- [iTerm2 Python API](https://iterm2.com/python-api/window.html)
- [WezTerm resurrect plugin](https://mwop.net/blog/2024-10-21-wezterm-resurrect.html)
- [cmux API Reference](https://www.cmux.dev/docs/api)

*Research coordinated across architecture, API integration, and backend experts.*
