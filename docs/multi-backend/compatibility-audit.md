# crex Ghostty Backend Compatibility Audit

**Date:** 2026-04-17
**Scope:** Full codebase audit of crex for Ghostty backend portability
**Ghostty API reference:** AppleScript API, Ghostty 1.3 (preview, macOS-only)

---

## Section A: Feature-by-Feature Compatibility Matrix

| Feature | cmux | Ghostty | Notes |
|---------|------|---------|-------|
| `save` | Full | **Limitations** | No `tree --json`; must enumerate via AppleScript loops. No surface type/URL. `Pinned` always false. CWD via `working directory` property. |
| `restore` | Full | **Limitations** | `PinWorkspace` silently skipped. No browser panes. Caller workspace detection needs different mechanism (no `tree.Caller`). |
| `import-from-md` | Full | **Limitations** | Pin field in Blueprint ignored. Browser pane type unsupported. Everything else works. |
| `export-to-md` | Full | **Limitations** | `Pinned` always `false`. No browser surfaces. |
| `template use` | Full | **Limitations** | Pin flag ignored. All split/command/rename operations work. |
| `template customize` | Full | **Identical** | Pure file operation. |
| `template list` | Full | **Identical** | Reads embedded gallery files only. |
| `template show` | Full | **Identical** | Renders ASCII diagrams from gallery data only. |
| `watch` | Full | **Limitations** | Same limitations as `save`. |
| `ws add` | Full | **Identical** | Pure file operation. |
| `ws remove` | Full | **Identical** | Pure file operation. |
| `ws list` | Full | **Identical** | Pure file operation. |
| `ws toggle` | Full | **Identical** | Pure file operation. |
| `list` (saved layouts) | Full | **Identical** | Reads TOML files from disk. |
| `show` (layout details) | Full | **Identical** | Reads TOML files from disk. |
| `edit` | Full | **Identical** | Opens TOML in `$EDITOR`. |
| `delete` | Full | **Identical** | Removes TOML from disk. |
| `dry-run` | Full | **Limitations** | Must show `osascript` commands instead of `cmux` CLI calls. |
| `completion` | Full | **Identical** | Completions are file-based (layout names, templates, etc.). |

**Summary:** 11/18 identical, 7/18 with limitations, 0/18 impossible.

---

## Section B: Interface Method Compatibility (12 Methods)

| # | Method | Ghostty Feasibility | Key Differences |
|---|--------|-------------------|-----------------|
| 1 | `Ping()` | **EASY** | Check if Ghostty app is running via AppleScript. |
| 2 | `Tree()` | **MODERATE** | No single JSON call; must iterate windows/tabs/terminals. No `Caller` context. No `Pinned`. No `URL`. No surface types. |
| 3 | `SidebarState()` | **EASY** | CWD from `working directory`. Git info not available from API — must shell out to `git`. |
| 4 | `ListWorkspaces()` | **EASY** | Enumerate tabs. Ref format differs (tab IDs vs `workspace:N`). |
| 5 | `NewWorkspace()` | **EASY** | `new tab` with `initial working directory`. Same poll-and-diff pattern. |
| 6 | `RenameWorkspace()` | **MODERATE** | Must select tab first, then `perform action "set_tab_title:..."`. |
| 7 | `SelectWorkspace()` | **EASY** | `select tab N of window 1`. |
| 8 | `NewSplit()` | **EASY** | `split` with direction. Same poll-and-diff for new terminal. |
| 9 | `FocusPane()` | **MODERATE** | `focus terminal N`. Index mapping differs (cmux 0-based, AppleScript 1-based). |
| 10 | `Send()` | **EASY** | `input text` on terminal. May need `\n` → `return` conversion. |
| 11 | `PinWorkspace()` | **IMPOSSIBLE** | Ghostty has no pin concept. |
| 12 | `CloseWorkspace()` | **EASY** | `close tab N`. |

**Score: 11/12 implementable. 1 impossible (PinWorkspace).**

---

## Section C: Data Model Compatibility

### Workspace

| Field | Ghostty Populated? | Notes |
|-------|-------------------|-------|
| `Title` | Yes | Tab `name` property. |
| `CWD` | Yes | Terminal `working directory`. |
| `Pinned` | **No** | Always `false`. No equivalent. |
| `Index` | Yes | Tab `index`. |
| `Active` | Yes | Tab `selected`. |
| `Panes` | Yes | Built from terminals in tab. |

### Pane

| Field | Ghostty Populated? | Notes |
|-------|-------------------|-------|
| `Type` | **Partial** | Always `"terminal"`. No browser panes. |
| `Split` | **Heuristic** | Can't query direction; defaults to `"right"` (same as cmux save). |
| `Command` | **No** | Not available from live state on either backend. |
| `Focus` | Yes | From terminal focused state. |
| `URL` | **No** | Always empty. No browser panes. |
| `Index` | Yes | Terminal index within tab. |
| `FocusTarget` | Yes | Used during restore, not save. |

### SidebarState

| Field | Ghostty Available? | Notes |
|-------|-------------------|-------|
| `CWD` | Yes | Terminal `working directory`. |
| `FocusedCWD` | **Partial** | Must find focused terminal in tab, read its CWD. |
| `GitBranch` | **Not from API** | Must shell out to `git rev-parse --abbrev-ref HEAD`. |
| `GitDirty` | **Not from API** | Must shell out to `git status --porcelain`. |

---

## Section D: 100% Backend-Agnostic (Zero Changes)

| Component | Package | Why |
|-----------|---------|-----|
| Template Gallery (all 16 templates) | `internal/gallery/` | Pure embedded data. |
| Blueprint parser/writer | `internal/mdfile/` | Markdown I/O only. |
| Layout persistence (TOML) | `internal/persist/` | File I/O only. |
| Configuration | `internal/config/` | TOML config, path expansion. |
| Data models | `internal/model/` | Pure data structures. |
| `ws add/remove/list/toggle` | `cmd/ws_*.go` | Blueprint file operations. |
| `template list/show/customize` | `cmd/template_*.go` | Gallery reads, file copies. |
| `list/show/edit/delete` | `cmd/*.go` | TOML file operations. |
| `version/completion` | `cmd/*.go` | No backend calls. |
| Styling/Picker | `cmd/style.go`, `cmd/picker.go` | UI rendering. |
| Completion helpers | `cmd/completion_helpers.go` | File-based completions. |

**25 of ~35 source files need zero changes (71%).**

---

## Section E: What's Impossible on Ghostty

| Feature | Why | Impact |
|---------|-----|--------|
| **Workspace pinning** | No pin concept in Ghostty. No API, no UI, no equivalent. | `PinWorkspace()` is a no-op. `Pinned` field meaningless. |
| **Browser panes** | Ghostty is a terminal emulator only. | `Pane.Type = "browser"` and `Pane.URL` can't be created or detected. |
| **Caller context** (`tree.Caller`) | No env vars like `CMUX_WORKSPACE_ID`. | Must save active tab before restore, return to it after. |
| **Linux backend** (today) | D-Bus planned for 1.4 (September 2026), not available. | Ghostty backend is macOS-only for now. |

---

## Section F: Graceful Degradation

| Feature | cmux Behavior | Ghostty Strategy |
|---------|--------------|-----------------|
| Pin on restore/import | Calls `PinWorkspace(ref)` | Skip silently. No error. |
| Pin on `template use --pin` | Pins the created workspace | Accept flag, skip action, warn: "Pinning not supported on Ghostty." |
| Pin in Blueprint | `Pin: yes` column | Parse and store, ignore during execution. |
| Pin in saved layout | `Pinned: true` in TOML | Always save as `false`. |
| Browser panes on save | Captures `type: "browser"` + `URL` | Save as `"terminal"`. URL empty. |
| Browser panes on restore | Creates split + navigates to URL | Degrade to terminal pane. Warn user. |
| Caller workspace return | `tree.Caller.WorkspaceRef` | Save active tab ID before restore, select it after. |
| Git info in SidebarState | Parsed from `cmux sidebar-state` | Shell out to `git -C <cwd>`. Slightly slower, functionally identical. |
| Tree enumeration | Single `cmux tree --json` | Multiple AppleScript calls. ~200-500ms slower. |
| Dry-run commands | Shows `cmux` CLI commands | Must show `osascript` equivalents. |

---

## Section G: Risk Assessment

### API Stability (HIGH risk)

| Risk | Severity | Likelihood | Mitigation |
|------|----------|------------|------------|
| Ghostty 1.4 breaks AppleScript API | HIGH | HIGH (explicitly preview) | Backend interface isolates all calls. ~1-2 weeks to reimplement. |
| `perform action "set_tab_title"` renamed | MEDIUM | MEDIUM | Internal action name, not a first-class verb. |
| `working directory` doesn't update on `cd` | MEDIUM | LOW | **Must validate in PoC before full implementation.** |

### Platform Risk (MEDIUM)

| Risk | Severity | Likelihood | Mitigation |
|------|----------|------------|------------|
| Linux D-Bus API differs from AppleScript | MEDIUM | HIGH | May need a third backend. `Backend` interface shields the rest. |
| macOS Accessibility permission prompts | MEDIUM | HIGH | First-run UX must guide user through System Preferences. One-time. |

### Performance Risk (LOW)

| Risk | Severity | Likelihood | Mitigation |
|------|----------|------------|------------|
| `osascript` overhead (~50-100ms per call) | LOW | LOW | Batch operations into single scripts where possible. |
| Poll-and-diff timing | LOW | LOW | Same pattern already works for cmux. |

---

## Summary Scorecard

| Dimension | Score |
|-----------|-------|
| Features identical | **11/18** (61%) |
| Features with limitations | **7/18** (39%) |
| Features impossible | **0/18** (0%) |
| Interface methods implementable | **11/12** (92%) |
| Data model fields populated | **~85%** |
| Source files needing zero changes | **25/~35** (71%) |
| Template gallery portability | **100%** |
| Blueprint format portability | **100%** |
