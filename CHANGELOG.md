# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [v1.21.1] — 2026-07-08

### Fixed
- **Restored layouts keep their exact shape** — `crex save` now writes panes in a valid creation order derived from the split tree. cmux numbers panes by visual position (left-to-right, top-to-bottom), and replaying that order could rebuild a different arrangement — e.g. a "left column split in two + full-height right pane" layout came back as a split top row + full-width bottom strip, with every pane's directory in the wrong place (#8)
- **Per-pane working directories survive `save` on current cmux** — recent cmux builds no longer report a `tty` for surfaces in `tree --json`, which silently disabled per-pane CWD capture: restored layouts kept their folders, but re-saving lost them again. Save now falls back to cmux's live surface state (`debug.terminals`) — the same source restore already uses for readiness gating (#8)

### Changed
- Internal lint cleanup (unchecked error returns in tests, if-else chains rewritten as switches) — no behavior change

---

## [v1.21.0] — 2026-06-23

### Added
- **AI session detection via alias, wrapper, or interpreter** — `crex save` now detects AI CLIs launched through a shell alias/function, a wrapper script, or an interpreter (e.g. `node …/claude/cli.js`, where the process name is "node"), not just the bare binary name (#6)
- **Optional tab/pane names in Blueprints** — label any pane or sub-tab with a quoted name after the descriptor (`split right "Diff"`); the label shows as the surface title. Optional and fully backward-compatible — existing Blueprints are unchanged (#7)
- **Per-pane working directory** — save captures each pane's own working directory and restore recreates each pane in its own folder, instead of collapsing every pane to a single path per workspace (#8)
- **`crex now`** — print the live workspace/pane tree (CLI equivalent of the TUI `now`)
- **`crex settings restore-mode get|set|list`** — manage the default restore mode from the CLI; it's persisted to the config and honored by `crex restore`

### Changed
- **`list` and `show` write their data to stdout** (previously stderr), so `crex list | grep` and `crex show x > file` work; progress and status stay on stderr. ANSI styling is disabled automatically when output isn't a terminal or `NO_COLOR` is set
- **`pop` and `update`** are now listed in the root `--help`
- Command-bearing files (Blueprint, config, pid, watch log) are written with `0600` permissions

### Fixed
- **Reliable cmux multi-pane restore** — fixed `focus-pane` rejecting workspace-local `pane:N` refs ("Missing or invalid pane_id") on multi-pane layouts, and made the per-pane `cd` reliable by gating on real per-surface shell readiness (`debug.terminals`) and verifying/retrying the `cd` (previously ~1-in-3 restores lost a pane's directory)
- **`crex setup` honors `CREX_BACKEND`** — the env override was previously ignored during the first-run save
- **Hardened restore** — split direction is validated against an allowlist (blocks AppleScript injection via a crafted layout), unchecked errors fixed, dead code removed

### Note
- In Ghostty, tab/pane names apply to the tab (= workspace) only — Ghostty has no sub-tabs and its splits carry no individual title, so per-surface names render on cmux. The name is always preserved in the Blueprint, keeping layouts portable

---

## [v1.20.0] — 2026-06-09

### Added
- **Multi-surface panes** — save, restore, and import additional tabs (surfaces) within a pane using the `tab N:` Blueprint syntax

### Fixed
- **Ghostty fresh launch** — correct handling when Ghostty is running with no windows / all windows closed; close the restored default tab on a clean launch

### Changed
- **README** redesigned as a visual front page; Alfred guide documents cmux automation mode + Ghostty

---

## [v1.19.2] — 2026-06-06

### Added
- **`pop` in TUI shell** — typing `pop` inside `crex tui` shows usage guidance with examples
- **Workspace focus after pop** — selecting a workspace or template via `crex pop` now focuses the restored workspace automatically

### Fixed
- **Substring search** — "ola" now correctly finds "somosunaola" (exact substring matches always accepted alongside fuzzy)
- **Scattered match rejection** — fuzzy search rejects false positives where matched characters are too spread out
- **Search title cleanup** — shows only `🔍 filter` during search, `🐦‍🔥 crex` when idle (no clutter)
- **Workspace source tags** — `‹layout›` shown right after workspace name so source is always visible

---

## [v1.19.0] — 2026-06-05

### Added
- **Workspace-level search in crex pop** — individual workspaces are now searchable. Typing "drolosoft brain" shows the actual workspace directly, with a WORKSPACES section in results. Selecting one restores that single workspace from its layout. Workspaces are hidden in the default view (no clutter) and appear only when search matches

### Fixed
- **ZWJ emoji sanitization** — `sanitizeEmoji()` replaces complex ZWJ sequences where lipgloss and uniseg disagree on width, preventing line overflow in the picker

---

## [v1.18.0] — 2026-06-05

### Added
- **`crex pop` — instant workspace switcher** — centered floating picker with fuzzy search (sahilm/fuzzy), two-level drill-in (Tab/→ into layout workspaces), number keys 1-9, and fzf-style match highlighting. Works on both cmux and Ghostty
- **Shell hook for Ctrl+G** — `crex setup` offers to install a shell keybinding (zsh/bash/fish) that opens `crex pop` instantly from any terminal. Configurable key, idempotent install/uninstall
- **Direct launch shortcuts** — `crex pop morning` (restore layout), `crex pop ide .` (apply template), `crex pop --last` (most recent layout)
- **Fuzzy search dependency** — `github.com/sahilm/fuzzy` for fzf-style matching with scored results and match position highlighting

### Fixed
- **ZWJ emoji rendering** — complex emoji sequences (🕵🏼‍♀️, 🐦‍🔥, 🧞‍♂️) caused line wrapping due to lipgloss width miscounting. Added `sanitizeEmoji()` using grapheme cluster analysis (`rivo/uniseg`) to detect and replace problematic sequences
- **Accurate terminal width** — `termWidth()` helper uses `uniseg.StringWidth(ansi.Strip(s))` for grapheme-cluster-aware width measurement, replacing lipgloss.Width() where accuracy matters

---

## [v1.17.0] — 2026-05-28

### Added
- **IDE template redesign** — shelf layout with `nvim .` (70% top, focused), `lazygit` (bottom-left), terminal (bottom-right). Uses the new `split_ratio` feature for the 70/30 split
- **Split ratio in templates** — templates support `split down 30%:` syntax for non-equal splits. Applied via `cmux resize-pane` during template execution
- **Browser type in split panes** — templates support `split right browser:` syntax for browser pane creation

### Fixed
- **Template split targeting** — cmux `new-split` does not transfer focus to new panes. Templates now explicitly focus each new pane and track actual pane refs (from the tree) to handle cmux's position-based reindexing. Fixes shelf, quad, aside, and dashboard layouts
- **Template command deferral** — interactive commands (`nvim`, `lazygit`) are deferred until after all splits and resizes are applied. Prevents focus interference during pane creation

### Changed
- **README** — documented Ghostty split sizing limitation (always equal splits; `split_ratio` is cmux-only)

---

## [v1.16.0] — 2026-05-27

### Added
- **Geometry-aware save** — `crex save` now calls `cmux rpc pane.list` to read pane pixel positions and reconstructs the binary split tree (BSP) to infer correct split directions and ratios. An aside layout (left + stacked right) now saves as `split = "down"` instead of the old default `split = "right"`
- **Split ratio preservation** — saved layouts capture the split proportion (e.g. 70/30) and restore applies it via `cmux resize-pane`. Ratios near 50/50 are omitted to keep TOMLs clean
- **Optional `PaneGeometryProvider` interface** — backends that expose pane pixel geometry get automatic split inference; backends without it (Ghostty) fall back to the existing default behavior. Zero changes to the `Backend` interface

### Fixed
- **Browser pane command leak** — `mergeUserEdits` no longer copies stale terminal commands onto browser panes when pane types change between saves
- **Browser pane focus during restore** — after creating a browser pane, the restore now correctly focuses it so subsequent splits target the right region. Fixed for both cmux (index-based) and Ghostty (pane:N format)
- **AI tool binary names** — corrected process names for CodeBuddy (`codebuddy-cli`), Factory (`factory-droid`), and Qoder (`qodercli`)

### Changed
- **README rewrite** — scannable intro, product-first messaging, Ghostty badge

---

## [v1.15.0] — 2026-05-27

### Added
- **AI tool expansion to 15 tools** — full session resume for Claude Code, OpenCode, Codex, Amp, Gemini CLI, Copilot, and Grok Build. Process-aware detection for Cursor, Aider, Pi, Rovo Dev, Hermes Agent, CodeBuddy, Factory, and Qoder
- **Gemini CLI detection** — discovers sessions via SHA-256 project hash in `~/.gemini/tmp/`, resumes with `gemini --resume <id>`
- **Copilot detection** — matches workspace CWD against `~/.copilot/session-state/*/workspace.yaml`, resumes with `copilot --continue`
- **Grok Build detection** — queries `~/.grok/grok.db` (SQLite) for session by CWD, resumes with `grok --continue`
- **Fire phoenix icon** — new logo for README and Drolosoft website

### Changed
- **Repositioned as workspace management tool** — README updated to celebrate cmux's native session restore and clarify crex's role: named layout library, templates, Blueprints, and multi-backend support on top of cmux and Ghostty
- **New comparison section** — side-by-side table showing what cmux restores natively vs what crex adds

### Fixed
- **`crex update` rate limits** — added `User-Agent` header and `GITHUB_TOKEN`/`GH_TOKEN` support for authenticated GitHub API requests (5,000 req/hr vs 60 unauthenticated)

---

## [v1.14.0] — 2026-05-22

### Added
- **Revision-tracked snapshots** — layouts now carry a monotonic `revision` counter that increments on each content change. The watch daemon uses revision comparison instead of SHA-256 hashing for cheaper, more informative change detection. Log output includes the revision number
- **Stability-based shell readiness** — `waitForShellReady` now uses two-phase detection: CWD polling (phase 1) followed by state-diff stability polling (phase 2). Declares ready after 3 consecutive identical reads instead of a fixed 1-second sleep. Configurable via `stable_duration` in config.toml
- **EnsureWorkspace API** — new `EnsureWorkspace()` function with four policies: `CreateOnly`, `CreateOrReuse`, `ReuseOnly`, `ForceRecreate`. Encapsulates workspace detection + creation into a single call for both CLI restore and TUI
- **`crex update` command** — self-update via CLI (`crex update`) or TUI (`update`). Auto-detects install method (Homebrew, `go install`, manual) and runs the appropriate upgrade. Checks GitHub API for latest release before updating
- **Amp thread resume detection** — `save` now captures running [Amp](https://ampcode.com) sessions and emits `amp threads continue T-<id>` on restore. Each amp process keeps its per-thread log file open, which we read via the lsof pass that already runs for CWD detection — so detection is per-pid precise (two amp instances in the same CWD each resolve to their own thread) and adds zero extra subprocesses (contributed by [@Flo4604](https://github.com/Flo4604))
- **Playwright terminal locators** — new `scripts/e2e-helpers.js` module with `getByText()`, `waitForText()`, `waitForStable()`, `expectVisible()`, and `sendCommand()` for E2E tests
- **Feature inventory** — machine-readable `features.yaml` documenting 20 features across cmux and Ghostty backends
- **Golden phoenix icon** — new logo for README and Drolosoft website

---

## [v1.13.1] — 2026-05-14

### Fixed
- **Daemon fork-detach** — `crex watch --daemon` now re-execs as a child process with `setsid`, creating a new session with no controlling terminal (`PPID=1`, `TTY=??`). Previously it ran in the parent shell's foreground process group, blocking new shell tabs until interrupted with Ctrl+C ([#4](https://github.com/drolosoft/cmux-resurrect/issues/4), reported by [@sebafudi](https://github.com/sebafudi))
- **Shell hook backgrounding** — `crex watch --shell-hook` now emits `nohup ... &!` (zsh), `nohup ... & disown` (bash), and `nohup ... &` (fish) for defense in depth. Users who already added the hook should regenerate it with `crex watch --shell-hook`

---

## [v1.13.0] — 2026-05-14

### Added
- **Smart restore pre-detection** — compares existing tabs against the layout before showing prompts. Fresh terminals auto-restore with zero questions. Layouts that already match print "nothing to do." Prompts only appear when the choice leads to different outcomes
- **Skip/Fresh prompt** — when matching tabs exist, asks whether to leave them as-is or close and recreate from the layout (useful when saved commands like AI resume need to be re-sent)
- **Single-keypress CLI prompts** — raw terminal mode reads one key instantly without requiring Enter. Invalid keys are silently ignored; only Escape/q cancels

### Changed
- **Restore prompt flow** — reduced from always-2-questions to 0, 1, or 2 depending on tab state. Most common case (fresh terminal or matching tabs only) needs at most one prompt
- **`DetectRestoreState` function** — new detection engine computes matching/extras/missing tab counts and returns a `RestoreHint` that drives both CLI and TUI prompt logic

### Fixed
- **Raw mode newline alignment** — terminal state is now restored before printing newlines, preventing "Cancelled" from appearing at the wrong column
- **Invalid key cancellation** — pressing random keys at restore prompts no longer cancels the operation; only Escape, q, or Ctrl-C exits

---

## [v1.12.0] — 2026-05-14

### Added
- **Sync-based restore** — restore now intelligently syncs your session instead of blindly destroying and recreating tabs. Matching tabs (by title) are kept untouched in both replace and add modes — no more killing running Claude sessions or dev servers just to reopen them
- **Restore-mode prompt in TUI** — the interactive shell now shows a Replace/Add picker when running `restore <name>` without a pre-configured mode, matching the CLI picker behavior
- **Restore-mode prompt in CLI** — `crex restore <name>` now prompts for Replace/Add instead of silently defaulting to replace. Use `--mode replace` or `--mode add` to skip the prompt in scripts
- **UnpinWorkspace** — new backend method to unpin workspaces before closing, preventing "pinned can't close" errors during replace mode

### Changed
- **Replace mode** — now only closes workspaces NOT in the layout (instead of closing everything). Workspaces matching the layout are preserved. Pinned workspaces are automatically unpinned before closing
- **Add mode** — same skip-matching behavior, but extra workspaces are left alone (not closed)
- **Restore output labels** — CLI now shows "Syncing (replace)" / "Syncing (add)" to reflect the new non-destructive behavior

### Fixed
- **TUI confirmation false success** — `updateConfirm` no longer shows "Done" when the confirmation action wrote an error (e.g. failed delete). The error message is now shown alone without a misleading success indicator

---

## [v1.9.0] — 2026-05-06

### Added
- **AI session auto-detection** — `crex save` now detects running Claude Code, OpenCode, and Codex sessions and auto-populates resume commands in the layout. On restore, each AI session resumes exactly where it left off — zero configuration needed.
- **Dual-signal matching** — detection confirms both process CWD (via `ps`/`lsof`) and terminal surface title before assigning a resume command, eliminating false positives from shared directories
- **Any-pane detection** — AI sessions are detected regardless of pane position (main pane, split right, split down, etc.)

### Changed
- **Shell readiness** — replaced file-sentinel probes with silent CWD polling via the backend API; no `touch` commands, no temp files, no shell history pollution
- **Ghostty Send** — combined `input text` and `send key` into a single atomic AppleScript call, preventing commands from being lost when splits steal focus

### Fixed
- **Session IDs validated** — resume commands only include IDs matching `[a-zA-Z0-9_-]`, preventing malformed commands from corrupted session files
- **Codex session format** — supports both legacy (rollout JSON) and current (dated JSONL) Codex session storage, bounded to 30 days for performance

### Limitations
- AI sessions in split panes are detected only when the pane shares the workspace's working directory. If a split pane has `cd`'d to a different directory, detection cannot match it (per-pane CWD is not captured). For reliable detection, use one project directory per workspace.

---

## [v1.8.0] — 2026-05-06

### Added
- **Rename command** — rename saved layouts from CLI (`crex rename old new`) and TUI (`rename` with tab completion)
- **Shell readiness detection** — replaced fixed-delay timing with file-sentinel polling before sending pane commands; guarantees the shell is interactive regardless of startup time (.zshrc, oh-my-zsh, starship, nvm, etc.)

### Fixed
- **Pane commands lost during shell init** — commands sent to newly created panes could be swallowed if the shell was still sourcing its profile; the new readiness probe retries automatically until the shell responds

---

## [v1.7.0] — 2026-05-02

### Added
- **Restore workspace picker** — two-level layout → workspace selection with →/Tab to drill in, ←/Esc/Backspace to go back
- **Single-workspace restore** — select a specific workspace from a layout to restore just that one
- **`restore_mode` setting** — configure default restore behavior (`ask`, `replace`, `add`) via config or TUI (`settings restore-mode set/get/list`)
- **Digit jump** — press 1-9 in any browse list to jump to that item
- **Combined alt-screen picker** — layout selection and replace/add mode prompt in one clean full-screen program

### Changed
- **CLI restore picker** — replaced huh Select with BrowseModel for identical behavior in CLI and TUI
- **Mode prompt** — replaced raw stdin prompt with Bubble Tea model for proper Esc/arrow key handling

---

## [v1.6.2] — 2026-04-29

### Fixed
- **Restore pane focus** — saved layouts left `FocusTarget` at Go's zero value, causing `cmux focus-pane --pane pane:0` errors during restore of multi-pane workspaces

---

## [v1.6.1] — 2026-04-20

### Fixed
- **Pointer receiver consistency** — converted all `ShellModel` methods from value to pointer receivers, eliminating a latent split-brain bug where shared `*strings.Builder` state could diverge across Bubble Tea copies
- **Help text alignment** — ANSI escape codes no longer break column padding; padding is now computed from unstyled text width
- **Editor launch with spaces** — `$EDITOR` values containing arguments (e.g. `code --wait`) are now split correctly; `$VISUAL` is checked before falling back to `vi`
- **staticcheck QF1012** — replaced `WriteString(Sprintf(...))` with `fmt.Fprintf` in banner settings output

### Changed
- **Completion caching** — `store.List()` filesystem I/O is now cached with a 2-second TTL, keeping per-keystroke completions off disk; explicit `Invalidate()` after all mutation operations (save, delete, bp add/remove/toggle, import, export, template customize)
- **Completion sort order** — commands sorted alphabetically within category groups (Live, Layouts, Templates, Blueprint, Settings, Shell); `exit`/`quit` anchored at the end
- **Dead code removed** — `batchNonNil`, `layoutNames`, `templateNames`, `blueprintNames` helpers deleted

### Added
- Tests for `parseCommand` with `settings banner` subcommands, `padIcon` emoji width, and `resolveNameOrNumber` lookup

---

## [v1.6.0] — 2026-04-20

### Added
- Level-aware tab completion engine in the interactive shell — 3-level depth with icons, descriptions, and argument auto-completion
- Tab/Shift-Tab/Up/Down cycling through completion options; Escape navigates back through levels
- `settings` command group in the TUI — banner configuration moved under `settings > banner > set/get/list`
- Random phoenix-themed farewell messages on exit (15 variants)
- Command header shown in dim after dispatch so you always know what ran
- Usage errors keep the command in the prompt for easy retry
- Confirmation dialogs for destructive operations (`delete`, `bp remove`)
- Prompt always visible at top during browse and confirm modes

### Changed
- Banner commands reorganized from top-level `banner` to `settings banner` inside the TUI (CLI `crex banner` unchanged)
- Bare group commands (`bp`, `settings`) now show subcommands on Enter instead of "Unknown command"
- Shell prompt styled as green `crex` + orange `→`

### Fixed
- Stale completion lists no longer persist after typing or navigating away
- Tab cycling state preserved correctly when using Escape to navigate back

---

## [v1.5.1] — 2026-04-19

### Added
- Quick-start demo GIF showing setup, save, and list in a single flow

### Changed
- CLI output now adds trailing newline spacing for better readability in all contexts
- Shell alias generation updated to reflect the `crex` command name consistently

### Fixed
- Banner was invisible on dark terminal backgrounds — switched to an always-visible color
- Added missing vertical spacing around the banner in several output paths

---

## [v1.5.0] — 2026-04-18

### Added
- Interactive shell mode (`crex shell`) — a persistent REPL with history, prompt, and command dispatch
- `crex tui` — full Bubble Tea launcher with fuzzy filtering, arrow navigation, and action menu
- `crex setup` wizard — detects your terminal backend and writes a config file automatically
- `watch` daemon mode with `--daemon`, `--stop`, and `--status` flags; PID file and log rotation included
- Shell hook generation for zsh, bash, and fish via `crex watch --shell-hook`
- `blueprint` command (replaces `workspace`) with `bp` shorthand alias
- Backend-adaptive labels via `unitName()` — output reads naturally for both cmux and Ghostty users
- Browse model with arrow navigation, live filtering, and cursor highlight inside the shell

### Changed
- `workspace` subcommand renamed to `blueprint`; the old name is no longer supported
- All user-facing output now routes through `unitName()` for consistent backend-aware wording
- Replaced the previous TUI with the new two-level shell/browse architecture
- Bubbletea and Bubbles promoted to direct dependencies (were previously indirect)

### Fixed
- Setup config writer now calls `MkdirAll` before writing to avoid permission errors on fresh installs
- Shell output uses `tea.Println` to prevent inline rendering corruption in Bubble Tea context
- All `golangci-lint` warnings resolved so CI stays green

---

## [v1.4.0] — 2026-04-18

### Added
- Workspace `Description` field — persisted to the Blueprint file and merged on save without overwriting manual edits
- Adaptive theme system with configurable banner styles (light/dark/auto)
- Template shortcut flags for faster one-liner workspace creation from the CLI

### Changed
- `crex template show` output polished — cleaner layout, consistent spacing
- Gallery screenshots added to project docs

---

## [v1.3.0] — 2026-04-17

### Added
- Ghostty backend — save, restore, templates, Workspace Blueprints, watch, and dry-run all work on Ghostty via AppleScript
- Auto-detection of the active terminal; no flags or config changes required
- Backend-aware `--dry-run` — shows actual cmux CLI commands or Ghostty AppleScript depending on your terminal
- 13-method `Backend` interface providing a clean abstraction layer for current and future terminals
- Conditional branding: Ghostty users see just `crex`; cmux users see `crex (cmux-resurrect)`

### Changed
- All core operations delegated through the Backend interface, removing direct cmux assumptions from orchestration logic

---

## [v1.2.0] — 2026-04-17

### Added
- Built-in template gallery with 16 embedded workspace layouts (dev, web, data, quad, and more)
- `crex template list` — browse available templates with category, icon, and description
- `crex template show <name>` — display an ASCII diagram of any template layout
- `crex template use <name>` — one-shot workspace creation from a gallery template
- `crex template customize <name>` — copy a gallery template into your Blueprint for local editing
- `FocusTarget` support in the orchestrator for complex layouts (e.g. quad) that need a specific pane focused on launch
- Tag-based completion and styled help output for the template command group

### Changed
- `DefaultTemplates` simplified to `dev` + `single`; the full gallery is now resolved from the embedded package
- `ws add` completion and help updated to reference the template gallery

### Fixed
- `parseTemplatePaneLine` now initializes `FocusTarget` to `-1` to avoid false-zero confusion

---

## [v1.1.1] — 2026-04-15

### Changed
- README: added Blueprint Markdown format snippet showing the actual file structure
- README: added save-my-day screenshot to the Save & Restore section
- README: updated star counts for cmux (14K+) and tmux-resurrect (12.7K) references
- README: one-liner Homebrew install block added; "Try it" order corrected (save before restore)
- README: added macOS-only platform note and data file location
- README: removed unverifiable marketing claim

---

## [v1.1.0] — 2026-04-14

### Added
- Shell completion for bash, zsh, fish, and PowerShell via `crex completion <shell>`

---

## [v1.0.5] — 2026-04-11

### Added
- Buy Me a Coffee support link and GitHub funding config
- Project logo (phoenix) in the README header
- Import success screenshot in the README

### Changed
- Example layout renamed from `work`/`demo` to `my-day` across all docs, help text, and demo recordings
- YouTube demo URL updated to match the new `my-day` layout recording
- CI: switched to golangci-lint v2 built from source for Go 1.26 compatibility

---

## [v1.0.4] — 2026-04-02

### Fixed
- Help text clarified the relationship between `crex` and `cmux-resurrect`, including the `go install` symlink step

---

## [v1.0.3] — 2026-04-02

### Changed
- Help text updated to note that both `crex` and `cmux-resurrect` are valid command names after installation

---

## [v1.0.2] — 2026-04-02

### Changed
- Both `crex` and `cmux-resurrect` binary names are now installed in all installation methods (Homebrew, `go install`)
- README installation instructions unified around the `crex` binary name

---

## [v1.0.1] — 2026-04-02

### Changed
- Moved entrypoint to `cmd/crex` so the binary installs as `crex` consistently across all package managers

---

## [v1.0.0] — 2026-04-02

Initial public release.

### Added
- `crex save <name>` — snapshot all open cmux windows and panes into a named Blueprint file
- `crex restore <name>` — recreate a saved workspace from a Blueprint, including split layout and pane focus
- `crex import-from-md` — create a live workspace directly from a Blueprint Markdown file
- `crex watch` — background file-watcher that auto-imports on Blueprint changes
- `crex ls` — list saved workspaces with metadata
- `crex delete <name>` — remove a saved workspace
- `--dry-run` flag — preview the exact commands that would run without making any changes
- `--workspace-file` global flag — point crex at a custom Blueprint location
- ASCII banner and styled help output
- Interactive restore picker for selecting among saved workspaces
- Workspace Blueprint format (Markdown-based, human-readable and hand-editable)
- Homebrew tap (`drolosoft/tap/cmux-resurrect`) and GoReleaser release workflow
- Cross-platform compilation targets (macOS arm64/amd64, Linux arm64/amd64)
- Shell completion scaffolding

### Fixed
- Restore now targets split surfaces explicitly and preserves the caller's active workspace
- Sync reliability improved: workspace refs, deferred rename, and pin support all stabilized
- Double-`v` in the version string output corrected

---

[v1.6.1]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.6.1
[v1.6.0]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.6.0
[v1.5.1]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.5.1
[v1.5.0]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.5.0
[v1.4.0]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.4.0
[v1.9.0]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.9.0
[v1.8.0]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.8.0
[v1.7.0]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.7.0
[v1.6.2]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.6.2
[v1.3.0]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.3.0
[v1.2.0]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.2.0
[v1.1.1]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.1.1
[v1.1.0]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.1.0
[v1.0.5]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.0.5
[v1.0.4]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.0.4
[v1.0.3]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.0.3
[v1.0.2]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.0.2
[v1.0.1]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.0.1
[v1.0.0]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.0.0
[v1.12.0]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.12.0
[v1.13.0]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.13.0
[v1.13.1]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.13.1
[v1.14.0]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.14.0
[v1.15.0]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.15.0
[v1.16.0]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.16.0
[v1.17.0]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.17.0
[v1.18.0]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.18.0
[v1.19.0]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.19.0
[v1.19.2]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.19.2
[v1.20.0]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.20.0
[v1.21.0]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.21.0
[v1.21.1]: https://github.com/drolosoft/cmux-resurrect/releases/tag/v1.21.1
