# Prompt 3: Multi-Backend Identity — Add Ghostty to README, CLI, and Homebrew Alias

**Branch:** `feat/multi-backend`
**Repo:** `/Users/txeo/Git/drolosoft/cmux-resurrect`
**Prerequisite:** Prompts 1 and 2 must be completed first

---

## Goal

Update the README, CLI help, banner, and Homebrew tap so that crex presents itself as a multi-backend tool (cmux + Ghostty) while fully preserving the cmux-resurrect identity, the cmux origin, and the tmux-resurrect lineage. Also create a Homebrew alias so new users can `brew install drolosoft/tap/crex` while existing users' `brew install drolosoft/tap/cmux-resurrect` keeps working.

**No Go module path changes.** The module stays `github.com/drolosoft/cmux-resurrect`. It only changes if and when the GitHub repo is actually renamed — and there's no rush for that.

## Guiding Principle

crex was born as cmux-resurrect — a tool inspired by tmux-resurrect that does for cmux what tmux-resurrect does for tmux. That heritage is a strength, not baggage. The corncrake (*Crex crex*) is the project's phoenix: a bird that returns to the same ground, just like your workspaces come back from the dead. The "resurrect" metaphor runs through everything.

When writing any copy:
- **cmux is the origin.** Always mention it first.
- **tmux-resurrect is the inspiration.** Always credit it.
- **Ghostty is the expansion.** Additive, not a replacement.
- **The corncrake is the phoenix.** Resurrection is the theme.
- **cmux-resurrect is the full name.** crex is the short name. Both are valid, both stay.

## What To Do

### Step 1: Update README.md — Hero Section

Replace lines 1-23 with:

```html
<p align="center"><img src="assets/logo.png" alt="crex logo" width="120"></p>

<h1 align="center">crex <sup><sub>(cmux-resurrect)</sub></sup></h1>

<p align="center">
  <a href="https://github.com/drolosoft/cmux-resurrect/actions/workflows/ci.yml"><img src="https://github.com/drolosoft/cmux-resurrect/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://goreportcard.com/report/github.com/drolosoft/cmux-resurrect"><img src="https://goreportcard.com/badge/github.com/drolosoft/cmux-resurrect" alt="Go Report Card"></a>
  <a href="https://pkg.go.dev/github.com/drolosoft/cmux-resurrect"><img src="https://pkg.go.dev/badge/github.com/drolosoft/cmux-resurrect.svg" alt="Go Reference"></a>
  <a href="https://codecov.io/gh/drolosoft/cmux-resurrect"><img src="https://codecov.io/gh/drolosoft/cmux-resurrect/branch/main/graph/badge.svg" alt="codecov"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
  <a href="https://github.com/drolosoft/homebrew-tap"><img src="https://img.shields.io/badge/Homebrew-tap-orange.svg" alt="Homebrew"></a>
  <a href="https://github.com/drolosoft/cmux-resurrect/releases"><img src="https://img.shields.io/github/v/release/drolosoft/cmux-resurrect" alt="GitHub Release"></a>
  <a href="https://github.com/manaflow-ai/cmux"><img src="https://img.shields.io/badge/cmux-ecosystem-blueviolet.svg" alt="cmux"></a>
</p>

> **Save, restore, and template your terminal workspaces — for [cmux](https://github.com/manaflow-ai/cmux) and [Ghostty](https://ghostty.org/).**

Inspired by [tmux-resurrect](https://github.com/tmux-plugins/tmux-resurrect) (12.7K stars), **crex** (short for cmux-resurrect) was born to do for [cmux](https://github.com/manaflow-ai/cmux) what tmux-resurrect does for tmux — and then went further. With **Workspace Blueprints**, a **template gallery**, and now **multi-backend support**, crex saves your entire layout and brings it back: workspaces, splits, CWDs, pinned state, startup commands, everything.

crex takes its name from the corncrake (*Crex crex*) — a migratory bird that returns to the same ground year after year. A phoenix of the grasslands. Much like your terminal workspaces, resurrected.
```

**What stays the same:**
- All badge URLs remain `cmux-resurrect` (that's the real repo name)
- cmux ecosystem badge stays
- tmux-resurrect credit is prominent in the first paragraph
- cmux is mentioned first everywhere

**What's new:**
- Title shows both names: `crex (cmux-resurrect)`
- Tagline mentions both backends: cmux and Ghostty
- Corncrake line with phoenix metaphor

### Step 2: Update README.md — Quick Start Section

Current line 36:
```
Both `crex` and `cmux-resurrect` are ready to use, with shell completions installed automatically. No Go toolchain required. macOS only (cmux is a macOS terminal).
```

New:
```
Both `crex` and `cmux-resurrect` are ready to use, with shell completions installed automatically. No Go toolchain required. macOS only (both cmux and Ghostty's AppleScript API are macOS-native).
```

### Step 3: Update README.md — Add "Supported Backends" Section

Insert a new section after the Template Gallery section (after line 134) and before the "Why crex?" section:

```markdown
## Supported Backends

| Backend | Status | Detection |
|---------|--------|-----------|
| [cmux](https://github.com/manaflow-ai/cmux) | Full support (original backend) | Auto-detected via `CMUX_SOCKET_PATH` |
| [Ghostty](https://ghostty.org/) | Full support (v1.3+ macOS) | Auto-detected when Ghostty is running |

crex auto-detects your terminal backend, or you can specify it explicitly:

```sh
crex save my-day                    # auto-detect backend
crex --backend cmux save my-day     # force cmux
crex --backend ghostty save my-day  # force Ghostty
```

All features — save, restore, import, export, templates, Blueprints — work identically across backends. The template gallery is 100% backend-agnostic.
```

### Step 4: Update README.md — "Why crex?" Section

Current (line 140):
```
crex is that tool for cmux.
```

New:
```
crex started as that tool for cmux, and now brings the same power to Ghostty.
```

Keep the entire comparison table unchanged — it's accurate and valuable.

### Step 5: Update `cmd/root.go` — Command Descriptions

Current:
```go
Short: "Resurrect your cmux sessions",
Long:  "cmux-resurrect (crex) saves/restores cmux layouts and manages workspaces from a Workspace Blueprint.",
```

New:
```go
Short: "Save, restore, and template your terminal workspaces",
Long:  "crex (cmux-resurrect) saves, restores, and templates your terminal workspaces.\nWorks with cmux and Ghostty. Inspired by tmux-resurrect.",
```

Note: cmux is listed first — it's the origin.

### Step 6: Update `cmd/style.go` — Banner Tagline

Current (line 61):
```go
b.WriteString(tagStyle.Render("  Session persistence for cmux — your terminal workspaces, resurrected."))
```

New:
```go
b.WriteString(tagStyle.Render("  Terminal workspace manager for cmux and Ghostty — your sessions, resurrected."))
```

**Keep** the "(crex is the short name for cmux-resurrect)" block (lines 91-96) exactly as-is.

### Step 7: Update `.goreleaser.yml` — Homebrew Description

Current:
```yaml
description: "Session persistence for cmux — save, restore, and manage your terminal workspaces"
```

New:
```yaml
description: "Terminal workspace manager for cmux and Ghostty — save, restore, and template your workspaces"
```

**Keep everything else in .goreleaser.yml unchanged:**
- `project_name: cmux-resurrect` — stays (matches repo)
- `name: cmux-resurrect` in brews — stays (existing users)
- `cmux-resurrect` symlink in install — stays (backward compat)
- All ldflags, release URLs — stay as `cmux-resurrect`

### Step 8: Create Homebrew Alias in the Tap Repo

This step requires a change **outside** the main repo, in the `drolosoft/homebrew-tap` repository. Create an `Aliases` directory with a symlink:

```
homebrew-tap/
  Formula/
    cmux-resurrect.rb       ← the real formula (unchanged)
  Aliases/
    crex → ../Formula/cmux-resurrect.rb   ← NEW symlink
```

To create this:
```sh
cd /path/to/homebrew-tap
mkdir -p Aliases
ln -s ../Formula/cmux-resurrect.rb Aliases/crex
git add Aliases/crex
git commit -m "feat: add crex alias for cmux-resurrect formula"
git push
```

After this, both work:
```sh
brew install drolosoft/tap/cmux-resurrect   # existing users — nothing breaks
brew install drolosoft/tap/crex             # new users — discovers it as crex
```

Same formula, same binary, same completions. Zero breakage.

**Note to the implementing instance:** If you don't have access to the homebrew-tap repo, document this step as a manual follow-up for the user.

### Step 9: Update `ARCHITECTURE.md`

Add after the title line:
```
crex supports multiple terminal backends through the `Backend` interface in `internal/client/`.
```

Update the `client/` entry in the package listing:
```
client/               → Backend interface + CLIClient (cmux) + GhosttyClient (Ghostty)
```

### Step 10: Run tests

```sh
go test ./... -count=1
go vet ./...
```

No Go import paths were changed, so all tests should pass without any code changes beyond the string literals in `root.go` and `style.go`.

## Files to Modify

| File | Change |
|------|--------|
| `README.md` | Hero section, Quick Start note, new Supported Backends section, Why crex intro |
| `cmd/root.go` | Short and Long descriptions |
| `cmd/style.go` | Banner tagline |
| `.goreleaser.yml` | Homebrew description |
| `ARCHITECTURE.md` | Add multi-backend mention |
| `drolosoft/homebrew-tap` (separate repo) | Add `Aliases/crex` symlink |

## What Does NOT Change

- **Go module path** — stays `github.com/drolosoft/cmux-resurrect` (matches repo URL)
- **All Go import statements** — no changes
- **GitHub repo name** — stays `cmux-resurrect`
- **Homebrew formula name** — stays `cmux-resurrect.rb`
- **Binary names** — still `crex` + `cmux-resurrect` symlink
- **All badge URLs** — still point to `cmux-resurrect`
- **`go install` command** — still `github.com/drolosoft/cmux-resurrect/cmd/crex@latest`
- **Config directory** — still `~/.config/crex/`
- **Makefile** — no changes
- **SECURITY.md, CONTRIBUTING.md, doc.go** — no changes needed
- **Template gallery** — backend-agnostic, no changes

## Commit

One commit in the main repo: "feat: update README and CLI for multi-backend support (cmux + Ghostty)"

One commit in homebrew-tap repo: "feat: add crex alias for cmux-resurrect formula"

## Success Criteria

- `go test ./... -count=1` passes (no Go changes except string literals)
- `go vet ./...` clean
- README title shows `crex (cmux-resurrect)` — both names visible
- README mentions cmux, Ghostty, tmux-resurrect, and the corncrake
- cmux ecosystem badge is present
- "Supported Backends" section shows both backends with auto-detection
- `crex --help` mentions both cmux and Ghostty
- Banner tagline says "for cmux and Ghostty"
- "(crex is the short name for cmux-resurrect)" is preserved in styled help
- After Homebrew alias: `brew install drolosoft/tap/crex` works
- After Homebrew alias: `brew install drolosoft/tap/cmux-resurrect` still works
- Existing users see zero breakage
