# Template Gallery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a built-in gallery of 16 templates (9 layout + 7 workflow) embedded in the crex binary, with `crex template list|show|use|customize` commands.

**Architecture:** Templates are `.md` files with YAML frontmatter, embedded via `go:embed` in `internal/gallery/`. Three-tier resolution: user-defined > gallery > fallback. New `crex template` command group with 4 subcommands. `FocusTarget` field on `TemplatePan` enables complex layouts like quad (2x2 grid).

**Tech Stack:** Go 1.26, go:embed, cobra, lipgloss, existing mdfile parser

**Spec:** `docs/superpowers/specs/2026-04-15-template-gallery-design.md`

---

## File Structure

### New files

| File | Responsibility |
|------|---------------|
| `internal/gallery/embed.go` | `go:embed` directive exposing `templates/` as `embed.FS` |
| `internal/gallery/gallery.go` | Registry: parse embedded templates, List/Get/ListByCategory/ResolveTemplate |
| `internal/gallery/gallery_test.go` | Validate all 16 templates parse correctly, resolution priority, tag filtering |
| `internal/gallery/templates/layout-cols.md` | 2-pane side-by-side |
| `internal/gallery/templates/layout-rows.md` | 2-pane stacked |
| `internal/gallery/templates/layout-sidebar.md` | 2-pane main+sidebar |
| `internal/gallery/templates/layout-shelf.md` | 3-pane big top, two bottom |
| `internal/gallery/templates/layout-aside.md` | 3-pane big left, two right |
| `internal/gallery/templates/layout-triple.md` | 3-pane three columns |
| `internal/gallery/templates/layout-quad.md` | 4-pane 2x2 grid (uses FocusTarget) |
| `internal/gallery/templates/layout-dashboard.md` | 4-pane big top, three bottom |
| `internal/gallery/templates/layout-ide.md` | 4-pane IDE layout |
| `internal/gallery/templates/workflow-claude.md` | AI pair-programming |
| `internal/gallery/templates/workflow-code.md` | General coding |
| `internal/gallery/templates/workflow-explore.md` | Codebase navigation |
| `internal/gallery/templates/workflow-system.md` | System monitoring |
| `internal/gallery/templates/workflow-logs.md` | Log tailing |
| `internal/gallery/templates/workflow-network.md` | Network debugging |
| `internal/gallery/templates/workflow-single.md` | Minimal single pane |
| `cmd/template.go` | `crex template` command group (alias `tpl`) |
| `cmd/template_list.go` | `crex template list` subcommand |
| `cmd/template_show.go` | `crex template show` + ASCII diagram renderer |
| `cmd/template_use.go` | `crex template use` subcommand |
| `cmd/template_customize.go` | `crex template customize` subcommand |
| `cmd/template_test.go` | Tests for all template subcommands |
| `internal/orchestrate/template_use.go` | TemplateUser orchestrator for single-workspace creation |
| `docs/templates.md` | Full gallery documentation with diagrams |
| `docs/template-authoring.md` | Custom template authoring guide |

### Modified files

| File | Change |
|------|--------|
| `internal/model/project.go` | Add fields to Template and TemplatePan structs |
| `internal/model/project_test.go` | Add tests for new fields and FocusTarget |
| `internal/mdfile/parse.go` | Initialize FocusTarget to -1 in parseTemplatePaneLine |
| `internal/mdfile/write.go` | Simplify DefaultTemplates to `dev` + `single` |
| `internal/orchestrate/import.go` | Use gallery.ResolveTemplate, handle FocusTarget |
| `internal/orchestrate/restore.go` | Handle FocusTarget between splits |
| `cmd/completion_helpers.go` | Add completeTemplateNames function |
| `cmd/ws_add.go` | Update --template flag completion to include gallery |
| `cmd/style.go` | Add template-specific styles |
| `README.md` | Add Template Gallery section |
| `docs/commands.md` | Add template command reference |
| `docs/blueprint.md` | Reference gallery, resolution order |
| `ARCHITECTURE.md` | Document gallery package |

---

### Task 1: Extend Model Structs

**Files:**
- Modify: `internal/model/project.go:6-29`
- Test: `internal/model/project_test.go`

- [ ] **Step 1: Write tests for new Template fields**

Add to `internal/model/project_test.go`:

```go
func TestTemplate_NewFieldsZeroValues(t *testing.T) {
	tmpl := Template{Name: "test"}
	if tmpl.Category != "" {
		t.Errorf("Category should default to empty, got %q", tmpl.Category)
	}
	if tmpl.Icon != "" {
		t.Errorf("Icon should default to empty, got %q", tmpl.Icon)
	}
	if tmpl.Description != "" {
		t.Errorf("Description should default to empty, got %q", tmpl.Description)
	}
	if tmpl.Tags != nil {
		t.Errorf("Tags should default to nil, got %v", tmpl.Tags)
	}
}

func TestTemplatePan_FocusTargetDefault(t *testing.T) {
	// Go zero value is 0, but we need -1 to mean "no target".
	// The parser must initialize this; the struct zero value is 0.
	tp := TemplatePan{}
	if tp.FocusTarget != 0 {
		t.Errorf("Go zero value should be 0, got %d", tp.FocusTarget)
	}
}

func TestTemplatePan_NameField(t *testing.T) {
	tp := TemplatePan{Name: "console"}
	if tp.Name != "console" {
		t.Errorf("Name = %q, want %q", tp.Name, "console")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/model/ -run "TestTemplate_NewFields|TestTemplatePan_FocusTarget|TestTemplatePan_Name" -v`
Expected: FAIL — `Template` has no field `Category`, `TemplatePan` has no field `FocusTarget`, etc.

- [ ] **Step 3: Add new fields to Template and TemplatePan**

Edit `internal/model/project.go`. Replace the `Template` struct (lines 16-19):

```go
// Template defines a reusable pane layout.
type Template struct {
	Name        string
	Category    string   // "layout" or "workflow"
	Icon        string   // display icon (▥, 🤖, etc.)
	Description string   // one-line summary
	Tags        []string // for filtering: ["ai", "git"], ["monitoring"], etc.
	Panes       []TemplatePan
}
```

Replace the `TemplatePan` struct (lines 22-29):

```go
// TemplatePan is a pane definition within a template.
type TemplatePan struct {
	Enabled     bool   // [x] or [ ]
	IsMain      bool   // "main" keyword = first pane
	Split       string // "right", "down", "left", "up"
	Type        string // "terminal" (default), "browser"
	Command     string // command in backticks
	Focus       bool   // "(focused)" suffix — gets final focus
	FocusTarget int    // pane index to focus BEFORE this split (-1 = no refocus)
	Name        string // display label: "main", "console", "git"
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/model/ -v`
Expected: ALL PASS (existing tests unchanged, new tests pass)

- [ ] **Step 5: Run full test suite to check backward compat**

Run: `go test ./... -count=1`
Expected: ALL PASS — no existing code uses the new fields yet

- [ ] **Step 6: Commit**

```bash
git add internal/model/project.go internal/model/project_test.go
git commit -m "feat(model): add Category, Icon, Description, Tags to Template; FocusTarget, Name to TemplatePan"
```

---

### Task 2: Initialize FocusTarget in Parser

**Files:**
- Modify: `internal/mdfile/parse.go:140-194`
- Test: `internal/mdfile/parse_test.go`

- [ ] **Step 1: Write test for FocusTarget initialization**

Add to `internal/mdfile/parse_test.go`:

```go
func TestParseTemplatePaneLine_FocusTargetDefault(t *testing.T) {
	wf, err := Parse("../../testdata/workspaces/minimal.md")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for name, tmpl := range wf.Templates {
		for i, tp := range tmpl.Panes {
			if tp.FocusTarget != -1 {
				t.Errorf("template %q pane %d: FocusTarget = %d, want -1", name, i, tp.FocusTarget)
			}
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mdfile/ -run TestParseTemplatePaneLine_FocusTargetDefault -v`
Expected: FAIL — `FocusTarget = 0, want -1`

- [ ] **Step 3: Initialize FocusTarget to -1 in parseTemplatePaneLine**

Edit `internal/mdfile/parse.go`, in `parseTemplatePaneLine` function. After line 141 (`var tp model.TemplatePan`), add:

```go
	tp.FocusTarget = -1
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/mdfile/ -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/mdfile/parse.go internal/mdfile/parse_test.go
git commit -m "fix(parser): initialize FocusTarget to -1 in parseTemplatePaneLine"
```

---

### Task 3: Create Gallery Package with Embedded Templates

**Files:**
- Create: `internal/gallery/embed.go`
- Create: `internal/gallery/gallery.go`
- Create: `internal/gallery/templates/layout-cols.md` (and all 15 others)
- Test: `internal/gallery/gallery_test.go`

- [ ] **Step 1: Create all 16 template .md files**

Create `internal/gallery/templates/` directory and all template files. Each has YAML frontmatter + pane definitions.

`internal/gallery/templates/layout-cols.md`:
```markdown
---
name: cols
category: layout
icon: "▥"
description: Side-by-side columns
tags: [basic, 2-pane]
---
### cols
- [x] main terminal (focused)
- [x] split right:
```

`internal/gallery/templates/layout-rows.md`:
```markdown
---
name: rows
category: layout
icon: "▤"
description: Stacked rows
tags: [basic, 2-pane]
---
### rows
- [x] main terminal (focused)
- [x] split down:
```

`internal/gallery/templates/layout-sidebar.md`:
```markdown
---
name: sidebar
category: layout
icon: "◧"
description: Main area with sidebar
tags: [basic, 2-pane]
---
### sidebar
- [x] main terminal (focused)
- [x] split right:
```

`internal/gallery/templates/layout-shelf.md`:
```markdown
---
name: shelf
category: layout
icon: "⊤"
description: Big top, two bottom
tags: [3-pane]
---
### shelf
- [x] main terminal (focused)
- [x] split down:
- [x] split right:
```

`internal/gallery/templates/layout-aside.md`:
```markdown
---
name: aside
category: layout
icon: "⊢"
description: Big left, two stacked right
tags: [3-pane]
---
### aside
- [x] main terminal (focused)
- [x] split right:
- [x] split down:
```

`internal/gallery/templates/layout-triple.md`:
```markdown
---
name: triple
category: layout
icon: "Ⅲ"
description: Three columns
tags: [3-pane]
---
### triple
- [x] main terminal (focused)
- [x] split right:
- [x] split right:
```

`internal/gallery/templates/layout-quad.md`:
```markdown
---
name: quad
category: layout
icon: "⊠"
description: 2×2 grid
tags: [4-pane, grid]
---
### quad
- [x] main terminal (focused)
- [x] split right:
- [x] split down: @focus=0
- [x] split down: @focus=1
```

Note: `@focus=N` is a new syntax extension parsed only by the gallery frontmatter parser. It sets `FocusTarget` on that pane. This syntax is NOT part of the user-facing Blueprint format — it only exists in embedded gallery templates.

`internal/gallery/templates/layout-dashboard.md`:
```markdown
---
name: dashboard
category: layout
icon: "◱"
description: Big top, three bottom
tags: [4-pane, monitoring]
---
### dashboard
- [x] main terminal (focused)
- [x] split down:
- [x] split right:
- [x] split right:
```

`internal/gallery/templates/layout-ide.md`:
```markdown
---
name: ide
category: layout
icon: "⧉"
description: Full IDE layout
tags: [4-pane, development]
---
### ide
- [x] main terminal:
- [x] split right: (focused)
- [x] split down:
- [x] split right:
```

`internal/gallery/templates/workflow-claude.md`:
```markdown
---
name: claude
category: workflow
icon: "🤖"
description: AI pair-programming with Claude Code
tags: [ai, git, development]
---
### claude
- [x] main terminal: `lazygit`
- [x] split right: `claude --dangerously-skip-permissions --continue` (focused)
- [x] split down:
```

`internal/gallery/templates/workflow-code.md`:
```markdown
---
name: code
category: workflow
icon: "💻"
description: General-purpose coding workspace
tags: [development, git]
---
### code
- [x] main terminal (focused)
- [x] split right: `lazygit`
- [x] split down: `watch -n 5 'ls -lt | head -20'`
```

`internal/gallery/templates/workflow-explore.md`:
```markdown
---
name: explore
category: workflow
icon: "🔭"
description: Navigate and understand a codebase
tags: [git, discovery]
---
### explore
- [x] main terminal (focused)
- [x] split right: `git log --oneline --graph -20`
```

`internal/gallery/templates/workflow-system.md`:
```markdown
---
name: system
category: workflow
icon: "📊"
description: Monitor system health and resources
tags: [monitoring, sysadmin]
---
### system
- [x] main terminal: `htop`
- [x] split right: `df -h`
```

`internal/gallery/templates/workflow-logs.md`:
```markdown
---
name: logs
category: workflow
icon: "📜"
description: Tail multiple log streams side-by-side
tags: [monitoring, debugging]
---
### logs
- [x] main terminal: `tail -f /var/log/system.log` (focused)
- [x] split right: `dmesg -T | tail -30`
- [x] split down:
```

`internal/gallery/templates/workflow-network.md`:
```markdown
---
name: network
category: workflow
icon: "🌐"
description: Debug connectivity and API endpoints
tags: [networking, debugging]
---
### network
- [x] main terminal (focused)
- [x] split right: `curl -s ifconfig.me && echo`
```

`internal/gallery/templates/workflow-single.md`:
```markdown
---
name: single
category: workflow
icon: "📟"
description: Minimal single-pane terminal
tags: [basic]
---
### single
- [x] main terminal (focused)
```

- [ ] **Step 2: Create embed.go**

Create `internal/gallery/embed.go`:

```go
package gallery

import "embed"

//go:embed templates/*.md
var templatesFS embed.FS
```

- [ ] **Step 3: Write gallery_test.go (comprehensive validation)**

Create `internal/gallery/gallery_test.go`:

```go
package gallery

import (
	"testing"
)

func TestAllTemplatesParse(t *testing.T) {
	tmpls := List()
	if len(tmpls) != 16 {
		t.Fatalf("expected 16 templates, got %d", len(tmpls))
	}
	for _, tmpl := range tmpls {
		if tmpl.Name == "" {
			t.Error("template with empty name")
		}
		if len(tmpl.Panes) == 0 {
			t.Errorf("template %q has no panes", tmpl.Name)
		}
	}
}

func TestAllTemplatesHaveFrontmatter(t *testing.T) {
	for _, tmpl := range List() {
		if tmpl.Category == "" {
			t.Errorf("%q: missing category", tmpl.Name)
		}
		if tmpl.Category != "layout" && tmpl.Category != "workflow" {
			t.Errorf("%q: invalid category %q", tmpl.Name, tmpl.Category)
		}
		if tmpl.Icon == "" {
			t.Errorf("%q: missing icon", tmpl.Name)
		}
		if tmpl.Description == "" {
			t.Errorf("%q: missing description", tmpl.Name)
		}
		if len(tmpl.Tags) == 0 {
			t.Errorf("%q: missing tags", tmpl.Name)
		}
	}
}

func TestTemplateNameUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for _, tmpl := range List() {
		if seen[tmpl.Name] {
			t.Errorf("duplicate template name: %q", tmpl.Name)
		}
		seen[tmpl.Name] = true
	}
}

func TestIconUniquenessWithinCategory(t *testing.T) {
	byCategory := make(map[string]map[string]bool)
	for _, tmpl := range List() {
		if byCategory[tmpl.Category] == nil {
			byCategory[tmpl.Category] = make(map[string]bool)
		}
		if byCategory[tmpl.Category][tmpl.Icon] {
			t.Errorf("duplicate icon %q in category %q", tmpl.Icon, tmpl.Category)
		}
		byCategory[tmpl.Category][tmpl.Icon] = true
	}
}

func TestLayoutTemplatesHaveNoCommands(t *testing.T) {
	for _, tmpl := range ListByCategory("layout") {
		for i, pane := range tmpl.Panes {
			if pane.Command != "" {
				t.Errorf("layout %q pane %d has command %q", tmpl.Name, i, pane.Command)
			}
		}
	}
}

func TestWorkflowTemplateCount(t *testing.T) {
	workflows := ListByCategory("workflow")
	if len(workflows) != 7 {
		t.Errorf("expected 7 workflow templates, got %d", len(workflows))
	}
}

func TestLayoutTemplateCount(t *testing.T) {
	layouts := ListByCategory("layout")
	if len(layouts) != 9 {
		t.Errorf("expected 9 layout templates, got %d", len(layouts))
	}
}

func TestGet_Existing(t *testing.T) {
	tmpl, ok := Get("claude")
	if !ok {
		t.Fatal("expected to find 'claude' template")
	}
	if tmpl.Category != "workflow" {
		t.Errorf("category = %q, want workflow", tmpl.Category)
	}
	if tmpl.Icon != "🤖" {
		t.Errorf("icon = %q, want 🤖", tmpl.Icon)
	}
	if len(tmpl.Panes) != 3 {
		t.Errorf("panes = %d, want 3", len(tmpl.Panes))
	}
}

func TestGet_NonExistent(t *testing.T) {
	_, ok := Get("nonexistent")
	if ok {
		t.Error("expected false for non-existent template")
	}
}

func TestQuadHasFocusTargets(t *testing.T) {
	tmpl, ok := Get("quad")
	if !ok {
		t.Fatal("quad template not found")
	}
	if len(tmpl.Panes) != 4 {
		t.Fatalf("quad panes = %d, want 4", len(tmpl.Panes))
	}
	// Pane 0 (main) and pane 1 (split right): no FocusTarget
	if tmpl.Panes[0].FocusTarget != -1 {
		t.Errorf("pane 0 FocusTarget = %d, want -1", tmpl.Panes[0].FocusTarget)
	}
	if tmpl.Panes[1].FocusTarget != -1 {
		t.Errorf("pane 1 FocusTarget = %d, want -1", tmpl.Panes[1].FocusTarget)
	}
	// Pane 2: FocusTarget=0 (go back to pane A before splitting down)
	if tmpl.Panes[2].FocusTarget != 0 {
		t.Errorf("pane 2 FocusTarget = %d, want 0", tmpl.Panes[2].FocusTarget)
	}
	// Pane 3: FocusTarget=1 (go back to pane B before splitting down)
	if tmpl.Panes[3].FocusTarget != 1 {
		t.Errorf("pane 3 FocusTarget = %d, want 1", tmpl.Panes[3].FocusTarget)
	}
}

func TestNonQuadTemplatesHaveNoFocusTargets(t *testing.T) {
	for _, tmpl := range List() {
		if tmpl.Name == "quad" {
			continue
		}
		for i, pane := range tmpl.Panes {
			if pane.FocusTarget != -1 {
				t.Errorf("%q pane %d: FocusTarget = %d, want -1", tmpl.Name, i, pane.FocusTarget)
			}
		}
	}
}

func TestResolveTemplate_GalleryFallback(t *testing.T) {
	// Empty workspace file — no user templates
	wf := &model.WorkspaceFile{Templates: map[string]*model.Template{}}
	panes := ResolveTemplate(wf, "cols")
	if len(panes) != 2 {
		t.Fatalf("expected 2 panes for cols, got %d", len(panes))
	}
}

func TestResolveTemplate_UserPriority(t *testing.T) {
	// User defines their own "cols" with 3 panes
	wf := &model.WorkspaceFile{
		Templates: map[string]*model.Template{
			"cols": {
				Name: "cols",
				Panes: []model.TemplatePan{
					{Enabled: true, IsMain: true, Type: "terminal", Focus: true, FocusTarget: -1},
					{Enabled: true, Split: "right", Type: "terminal", FocusTarget: -1},
					{Enabled: true, Split: "right", Type: "terminal", FocusTarget: -1},
				},
			},
		},
	}
	panes := ResolveTemplate(wf, "cols")
	// User's 3-pane version should win over gallery's 2-pane
	if len(panes) != 3 {
		t.Fatalf("expected 3 panes (user override), got %d", len(panes))
	}
}

func TestResolveTemplate_FallbackSinglePane(t *testing.T) {
	wf := &model.WorkspaceFile{Templates: map[string]*model.Template{}}
	panes := ResolveTemplate(wf, "totally-unknown")
	if len(panes) != 1 {
		t.Fatalf("expected 1 fallback pane, got %d", len(panes))
	}
	if panes[0].Type != "terminal" || !panes[0].Focus {
		t.Error("fallback should be single focused terminal")
	}
}
```

Note: add `"github.com/drolosoft/cmux-resurrect/internal/model"` to imports for the resolve tests.

- [ ] **Step 4: Run tests to verify they fail**

Run: `go test ./internal/gallery/ -v`
Expected: FAIL — `gallery.go` doesn't exist yet

- [ ] **Step 5: Implement gallery.go**

Create `internal/gallery/gallery.go`:

```go
package gallery

import (
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"sync"

	"github.com/drolosoft/cmux-resurrect/internal/model"
)

var (
	once      sync.Once
	templates []*model.Template
	byName    map[string]*model.Template
)

func load() {
	once.Do(func() {
		byName = make(map[string]*model.Template)
		entries, err := fs.ReadDir(templatesFS, "templates")
		if err != nil {
			return
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			data, err := fs.ReadFile(templatesFS, "templates/"+entry.Name())
			if err != nil {
				continue
			}
			tmpl, err := parseTemplateFile(string(data))
			if err != nil {
				continue
			}
			templates = append(templates, tmpl)
			byName[tmpl.Name] = tmpl
		}
	})
}

// List returns all gallery templates.
func List() []*model.Template {
	load()
	return templates
}

// Get returns a gallery template by name.
func Get(name string) (*model.Template, bool) {
	load()
	tmpl, ok := byName[name]
	return tmpl, ok
}

// ListByCategory returns templates filtered by category ("layout" or "workflow").
func ListByCategory(category string) []*model.Template {
	load()
	var out []*model.Template
	for _, t := range templates {
		if t.Category == category {
			out = append(out, t)
		}
	}
	return out
}

// ResolveTemplate checks user templates first, then gallery, then falls back to single pane.
func ResolveTemplate(wf *model.WorkspaceFile, name string) []model.Pane {
	// 1. User-defined takes priority
	panes := wf.ResolveTemplate(name)
	if len(panes) > 0 && (len(panes) > 1 || panes[0].Type != "terminal" || !panes[0].Focus || panes[0].Command != "") {
		return panes
	}
	// Check if the user actually had this template defined (not just fallback)
	if _, ok := wf.Templates[name]; ok {
		return panes
	}
	// 2. Gallery fallback
	tmpl, ok := Get(name)
	if !ok {
		// 3. Ultimate fallback
		return []model.Pane{{Type: "terminal", Focus: true}}
	}
	return buildPanes(tmpl)
}

func buildPanes(tmpl *model.Template) []model.Pane {
	var panes []model.Pane
	for i, tp := range tmpl.Panes {
		if !tp.Enabled {
			continue
		}
		pane := model.Pane{
			Type:    tp.Type,
			Command: tp.Command,
			Focus:   tp.Focus,
		}
		if pane.Type == "" {
			pane.Type = "terminal"
		}
		if i > 0 && tp.Split != "" {
			pane.Split = tp.Split
		}
		panes = append(panes, pane)
	}
	if len(panes) == 0 {
		return []model.Pane{{Type: "terminal", Focus: true}}
	}
	return panes
}

// parseTemplateFile parses a template .md file with YAML frontmatter.
func parseTemplateFile(content string) (*model.Template, error) {
	// Split frontmatter from body
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("missing frontmatter delimiters")
	}

	frontmatter := parts[1]
	body := parts[2]

	tmpl := &model.Template{}

	// Parse frontmatter (simple key: value)
	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		// Strip quotes
		val = strings.Trim(val, "\"")

		switch key {
		case "name":
			tmpl.Name = val
		case "category":
			tmpl.Category = val
		case "icon":
			tmpl.Icon = val
		case "description":
			tmpl.Description = val
		case "tags":
			// Parse [tag1, tag2]
			val = strings.Trim(val, "[]")
			for _, tag := range strings.Split(val, ",") {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					tmpl.Tags = append(tmpl.Tags, tag)
				}
			}
		}
	}

	if tmpl.Name == "" {
		return nil, fmt.Errorf("template missing name in frontmatter")
	}

	// Parse pane definitions from body using same syntax as Blueprint
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- [") {
			continue
		}
		tp := parseGalleryPaneLine(trimmed)
		tmpl.Panes = append(tmpl.Panes, tp)
	}

	return tmpl, nil
}

// parseGalleryPaneLine parses a pane line, including the @focus=N extension for gallery templates.
func parseGalleryPaneLine(line string) model.TemplatePan {
	var tp model.TemplatePan
	tp.FocusTarget = -1

	if strings.HasPrefix(line, "- [x]") || strings.HasPrefix(line, "- [X]") {
		tp.Enabled = true
	}

	// Strip checkbox
	rest := line
	if idx := strings.Index(rest, "]"); idx >= 0 {
		rest = strings.TrimSpace(rest[idx+1:])
	}

	// Extract @focus=N (gallery-only extension)
	if idx := strings.Index(rest, "@focus="); idx >= 0 {
		numStr := rest[idx+7:]
		// Take digits only
		end := 0
		for end < len(numStr) && numStr[end] >= '0' && numStr[end] <= '9' {
			end++
		}
		if end > 0 {
			if n, err := strconv.Atoi(numStr[:end]); err == nil {
				tp.FocusTarget = n
			}
		}
		rest = strings.TrimSpace(rest[:idx])
	}

	// Extract command in backticks
	if backtickStart := strings.Index(rest, "`"); backtickStart >= 0 {
		backtickEnd := strings.Index(rest[backtickStart+1:], "`")
		if backtickEnd >= 0 {
			tp.Command = rest[backtickStart+1 : backtickStart+1+backtickEnd]
			rest = rest[:backtickStart]
		}
	}

	// Check for (focused)
	if strings.Contains(rest, "(focused)") {
		tp.Focus = true
		rest = strings.Replace(rest, "(focused)", "", 1)
	}

	rest = strings.TrimSpace(rest)

	switch {
	case strings.HasPrefix(rest, "main"):
		tp.IsMain = true
		tp.Type = "terminal"
		remaining := strings.TrimSpace(strings.TrimPrefix(rest, "main"))
		if remaining != "" && remaining != ":" {
			tp.Type = strings.TrimSuffix(remaining, ":")
		}
	case strings.HasPrefix(rest, "split "):
		parts := strings.Fields(rest)
		if len(parts) >= 2 {
			tp.Split = strings.TrimSuffix(parts[1], ":")
		}
		tp.Type = "terminal"
	default:
		tp.Type = "terminal"
	}

	return tp
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/gallery/ -v`
Expected: ALL PASS

- [ ] **Step 7: Run full test suite**

Run: `go test ./... -count=1`
Expected: ALL PASS

- [ ] **Step 8: Commit**

```bash
git add internal/gallery/
git commit -m "feat(gallery): add embedded template gallery with 16 templates (9 layout + 7 workflow)"
```

---

### Task 4: FocusTarget Support in Orchestrators

> **Dependency note:** This task adds `FocusTarget` to `model.Pane` (the runtime struct) and updates `gallery.buildPanes` to copy it from `TemplatePan` to `Pane`. Task 3's gallery tests validate `TemplatePan` fields directly; this task wires FocusTarget through to the runtime execution path.

**Files:**
- Modify: `internal/orchestrate/restore.go:181-212`
- Modify: `internal/orchestrate/import.go:129-156`
- Test: `internal/orchestrate/restore_test.go`

- [ ] **Step 1: Write test for FocusTarget in restore flow**

Add to `internal/orchestrate/restore_test.go` (or create if sparse). This tests the dry-run output includes focus-pane commands:

```go
func TestRestoreWorkspace_FocusTarget_DryRun(t *testing.T) {
	store := &mockStore{
		layouts: map[string]*model.Layout{
			"quad-test": {
				Name:    "quad-test",
				Version: 1,
				Workspaces: []model.Workspace{
					{
						Title: "test",
						CWD:   "/tmp",
						Panes: []model.Pane{
							{Type: "terminal", Focus: true, Index: 0},
							{Type: "terminal", Split: "right", Index: 1},
							{Type: "terminal", Split: "down", Index: 2, FocusTarget: 0},
							{Type: "terminal", Split: "down", Index: 3, FocusTarget: 1},
						},
					},
				},
			},
		},
	}

	r := &Restorer{Store: store}
	result, err := r.Restore("quad-test", true, RestoreModeAdd)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}

	// Check dry-run commands include focus-pane
	cmds := strings.Join(result.Commands, "\n")
	if !strings.Contains(cmds, "focus-pane") {
		t.Error("dry-run should include focus-pane commands for FocusTarget panes")
	}
}
```

Note: this requires adding `FocusTarget` to `model.Pane` struct in `internal/model/layout.go`. Add the field:

```go
type Pane struct {
	Type        string `toml:"type"`
	Split       string `toml:"split,omitempty"`
	CWD         string `toml:"cwd,omitempty"`
	Command     string `toml:"command,omitempty"`
	Focus       bool   `toml:"focus,omitempty"`
	URL         string `toml:"url,omitempty"`
	Index       int    `toml:"index,omitempty"`
	FocusTarget int    `toml:"focus_target,omitempty"`
}
```

And update `buildPanes` in `gallery.go` to copy FocusTarget from TemplatePan to Pane. In the `buildPanes` function, add after setting `pane.Focus`:

```go
		pane.FocusTarget = tp.FocusTarget
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/orchestrate/ -run TestRestoreWorkspace_FocusTarget -v`
Expected: FAIL — restore doesn't emit focus-pane commands yet

- [ ] **Step 3: Add FocusTarget handling to restore.go**

Edit `internal/orchestrate/restore.go`. In `restoreWorkspace()`, the pane creation loop starts at line 182. Before the `NewSplit` call (line 197), add FocusTarget handling. Replace the loop body for `i > 0` panes (lines 192-211):

```go
		// Focus a specific pane before splitting (for quad, etc.)
		if pane.FocusTarget >= 0 {
			targetRef := fmt.Sprintf("pane:%d", pane.FocusTarget)
			if err := r.Client.FocusPane(targetRef, ref); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("  pane %d focus target: %v", i, err))
			}
			time.Sleep(DelayAfterSelect)
		}

		direction := pane.Split
		if direction == "" {
			direction = "right"
		}
		surfaceRef, err := r.Client.NewSplit(direction, ref)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("  pane %d split: %v", i, err))
			continue
		}

		time.Sleep(DelayAfterSplit)

		if pane.Command != "" {
			if err := r.Client.Send(ref, surfaceRef, pane.Command+"\\n"); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("  pane %d send command: %v", i, err))
			}
		}
```

Also update `dryRunWorkspace()` to emit focus-pane commands. In the loop at line 254, before the `new-split` command, add:

```go
		if pane.FocusTarget >= 0 {
			result.Commands = append(result.Commands,
				fmt.Sprintf("cmux focus-pane --pane pane:%d --workspace %s", pane.FocusTarget, ref))
		}
```

- [ ] **Step 4: Add same FocusTarget handling to import.go**

Edit `internal/orchestrate/import.go`. In the split creation loop (lines 130-156), add before `NewSplit`:

```go
			// Focus a specific pane before splitting (for quad, etc.)
			if pane.FocusTarget >= 0 {
				targetRef := fmt.Sprintf("pane:%d", pane.FocusTarget)
				if err := im.Client.FocusPane(targetRef, ref); err != nil {
					im.emit(ImportEvent{
						Status: ImportWarn,
						Title:  title,
						Panes:  panes,
						Warn:   fmt.Sprintf("%s pane %d: focus target failed: %v", title, j, err),
					})
				}
				time.Sleep(DelayAfterSelect)
			}
```

Note: the `panes` variable here is `[]model.Pane` (from `ResolveTemplate`), which now carries `FocusTarget`.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/orchestrate/ -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add internal/model/layout.go internal/gallery/gallery.go internal/orchestrate/restore.go internal/orchestrate/import.go internal/orchestrate/restore_test.go
git commit -m "feat(orchestrate): add FocusTarget support for complex layouts like quad"
```

---

### Task 5: Wire Gallery Resolution into Import Flow

**Files:**
- Modify: `internal/orchestrate/import.go:78`
- Modify: `cmd/import_from_md.go`

- [ ] **Step 1: Update import.go to use gallery.ResolveTemplate**

In `internal/orchestrate/import.go`, line 78 currently calls:
```go
panes := wf.ResolveTemplate(p.Template)
```

Change to:
```go
panes := gallery.ResolveTemplate(wf, p.Template)
```

Add import: `"github.com/drolosoft/cmux-resurrect/internal/gallery"`

- [ ] **Step 2: Run full test suite**

Run: `go test ./... -count=1`
Expected: ALL PASS — existing import tests still work, gallery fallback now active

- [ ] **Step 3: Commit**

```bash
git add internal/orchestrate/import.go
git commit -m "feat(import): use gallery.ResolveTemplate for three-tier template resolution"
```

---

### Task 6: Template Command Group + List Subcommand

**Files:**
- Create: `cmd/template.go`
- Create: `cmd/template_list.go`
- Modify: `cmd/style.go`
- Test: `cmd/template_test.go`

- [ ] **Step 1: Create template.go (command group)**

Create `cmd/template.go`:

```go
package cmd

import "github.com/spf13/cobra"

var templateCmd = &cobra.Command{
	Use:     "template",
	Short:   "Browse and use the built-in template gallery",
	Long:    "Discover, preview, and use pre-built workspace templates for common developer workflows.",
	Aliases: []string{"tpl"},
}

func init() {
	rootCmd.AddCommand(templateCmd)
}
```

- [ ] **Step 2: Add template-specific styles to style.go**

Add to `cmd/style.go`:

```go
var (
	templateIconStyle = lipgloss.NewStyle().Width(3)
	templateNameStyle = lipgloss.NewStyle().Foreground(colorGreen).Bold(true).Width(14)
	templatePaneStyle = lipgloss.NewStyle().Foreground(colorCyan).Width(5)
	templateDescStyle = lipgloss.NewStyle().Foreground(colorDim)
	categoryStyle     = lipgloss.NewStyle().Bold(true).Foreground(colorYellow).MarginTop(1)
)
```

- [ ] **Step 3: Write test for template list**

Create `cmd/template_test.go`:

```go
package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestTemplateList_ShowsAllTemplates(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"template", "list"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("template list: %v", err)
	}

	output := buf.String()

	// Check categories present
	if !strings.Contains(output, "LAYOUTS") {
		t.Error("missing LAYOUTS header")
	}
	if !strings.Contains(output, "WORKFLOWS") {
		t.Error("missing WORKFLOWS header")
	}

	// Check some template names present
	for _, name := range []string{"cols", "rows", "sidebar", "quad", "ide", "claude", "code", "single"} {
		if !strings.Contains(output, name) {
			t.Errorf("missing template %q in output", name)
		}
	}

	// Check icons present
	for _, icon := range []string{"▥", "▤", "◧", "⊠", "⧉", "🤖", "💻", "📟"} {
		if !strings.Contains(output, icon) {
			t.Errorf("missing icon %q in output", icon)
		}
	}

	// Check counts
	if !strings.Contains(output, "16 templates") {
		t.Error("missing template count summary")
	}
}

func TestTemplateList_LayoutFilter(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"template", "list", "--layout"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("template list --layout: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "cols") {
		t.Error("layout filter should show layout templates")
	}
	if strings.Contains(output, "claude") {
		t.Error("layout filter should NOT show workflow templates")
	}
}

func TestTemplateList_WorkflowFilter(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"template", "list", "--workflow"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("template list --workflow: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "claude") {
		t.Error("workflow filter should show workflow templates")
	}
	if strings.Contains(output, "cols") {
		t.Error("workflow filter should NOT show layout templates")
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./cmd/ -run TestTemplateList -v`
Expected: FAIL — `template list` command not implemented

- [ ] **Step 5: Implement template_list.go**

Create `cmd/template_list.go`:

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/drolosoft/cmux-resurrect/internal/gallery"
	"github.com/spf13/cobra"
)

var (
	listLayoutOnly   bool
	listWorkflowOnly bool
	listTag          string
)

var templateListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all available templates",
	Aliases: []string{"ls"},
	RunE:    runTemplateList,
}

func init() {
	templateListCmd.Flags().BoolVar(&listLayoutOnly, "layout", false, "show only layout templates")
	templateListCmd.Flags().BoolVar(&listWorkflowOnly, "workflow", false, "show only workflow templates")
	templateListCmd.Flags().StringVar(&listTag, "tag", "", "filter by tag")
	templateCmd.AddCommand(templateListCmd)
}

func runTemplateList(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStderr()

	layouts := gallery.ListByCategory("layout")
	workflows := gallery.ListByCategory("workflow")

	if listTag != "" {
		layouts = filterByTag(layouts, listTag)
		workflows = filterByTag(workflows, listTag)
	}

	total := 0

	if !listWorkflowOnly {
		fmt.Fprintln(w)
		fmt.Fprintln(w, categoryStyle.Render("  LAYOUTS"))
		fmt.Fprintln(w)
		for _, tmpl := range layouts {
			renderTemplateLine(w, tmpl)
		}
		total += len(layouts)
	}

	if !listLayoutOnly {
		fmt.Fprintln(w)
		fmt.Fprintln(w, categoryStyle.Render("  WORKFLOWS"))
		fmt.Fprintln(w)
		for _, tmpl := range workflows {
			renderTemplateLine(w, tmpl)
		}
		total += len(workflows)
	}

	fmt.Fprintln(w)
	layoutCount := len(layouts)
	workflowCount := len(workflows)
	if listWorkflowOnly {
		layoutCount = 0
	}
	if listLayoutOnly {
		workflowCount = 0
	}
	fmt.Fprintf(w, "  %s\n\n",
		dimStyle.Render(fmt.Sprintf("%d templates (%d layouts, %d workflows)",
			total, layoutCount, workflowCount)))

	return nil
}

func renderTemplateLine(w *os.File, tmpl *gallery.TemplateInfo) {
	// This won't compile yet — we need to figure out the type.
	// Actually gallery.List() returns []*model.Template, so we use that directly.
}
```

Wait — I need to reconsider. `gallery.List()` returns `[]*model.Template`. Let me fix the implementation:

```go
package cmd

import (
	"fmt"
	"io"

	"github.com/drolosoft/cmux-resurrect/internal/gallery"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/spf13/cobra"
)

var (
	listLayoutOnly   bool
	listWorkflowOnly bool
	listTag          string
)

var templateListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all available templates",
	Aliases: []string{"ls"},
	RunE:    runTemplateList,
}

func init() {
	templateListCmd.Flags().BoolVar(&listLayoutOnly, "layout", false, "show only layout templates")
	templateListCmd.Flags().BoolVar(&listWorkflowOnly, "workflow", false, "show only workflow templates")
	templateListCmd.Flags().StringVar(&listTag, "tag", "", "filter by tag")
	templateCmd.AddCommand(templateListCmd)
}

func runTemplateList(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStderr()

	layouts := gallery.ListByCategory("layout")
	workflows := gallery.ListByCategory("workflow")

	if listTag != "" {
		layouts = filterByTag(layouts, listTag)
		workflows = filterByTag(workflows, listTag)
	}

	total := 0

	if !listWorkflowOnly {
		fmt.Fprintln(w)
		fmt.Fprintln(w, categoryStyle.Render("  LAYOUTS"))
		fmt.Fprintln(w)
		for _, tmpl := range layouts {
			renderTemplateLine(w, tmpl)
		}
		total += len(layouts)
	}

	if !listLayoutOnly {
		fmt.Fprintln(w)
		fmt.Fprintln(w, categoryStyle.Render("  WORKFLOWS"))
		fmt.Fprintln(w)
		for _, tmpl := range workflows {
			renderTemplateLine(w, tmpl)
		}
		total += len(workflows)
	}

	fmt.Fprintln(w)
	layoutCount := len(layouts)
	workflowCount := len(workflows)
	if listWorkflowOnly {
		layoutCount = 0
	}
	if listLayoutOnly {
		workflowCount = 0
	}
	fmt.Fprintf(w, "  %s\n\n",
		dimStyle.Render(fmt.Sprintf("%d templates (%d layouts, %d workflows)",
			total, layoutCount, workflowCount)))

	return nil
}

func renderTemplateLine(w io.Writer, tmpl *model.Template) {
	paneCount := fmt.Sprintf("[%d]", len(tmpl.Panes))
	fmt.Fprintf(w, "  %s %s %s %s\n",
		templateIconStyle.Render(tmpl.Icon),
		templateNameStyle.Render(tmpl.Name),
		templatePaneStyle.Render(paneCount),
		templateDescStyle.Render(tmpl.Description))
}

func filterByTag(templates []*model.Template, tag string) []*model.Template {
	var out []*model.Template
	for _, tmpl := range templates {
		for _, t := range tmpl.Tags {
			if t == tag {
				out = append(out, tmpl)
				break
			}
		}
	}
	return out
}
```

- [ ] **Step 6: Run tests**

Run: `go test ./cmd/ -run TestTemplateList -v`
Expected: ALL PASS

- [ ] **Step 7: Commit**

```bash
git add cmd/template.go cmd/template_list.go cmd/template_test.go cmd/style.go
git commit -m "feat(cli): add 'crex template list' command with category filtering"
```

---

### Task 7: Template Show Subcommand with ASCII Diagrams

**Files:**
- Create: `cmd/template_show.go`
- Test: `cmd/template_test.go` (append)

- [ ] **Step 1: Write tests for template show**

Append to `cmd/template_test.go`:

```go
func TestTemplateShow_ExistingTemplate(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"template", "show", "claude"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("template show claude: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "claude") {
		t.Error("should show template name")
	}
	if !strings.Contains(output, "🤖") {
		t.Error("should show icon")
	}
	if !strings.Contains(output, "workflow") {
		t.Error("should show category")
	}
	// Should contain box-drawing characters for ASCII diagram
	if !strings.Contains(output, "┌") || !strings.Contains(output, "┘") {
		t.Error("should show ASCII diagram with box-drawing characters")
	}
}

func TestTemplateShow_NonExistent(t *testing.T) {
	rootCmd.SetArgs([]string{"template", "show", "nonexistent"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent template")
	}
}

func TestTemplateShow_AllTemplatesRender(t *testing.T) {
	for _, tmpl := range gallery.List() {
		t.Run(tmpl.Name, func(t *testing.T) {
			var buf bytes.Buffer
			rootCmd.SetOut(&buf)
			rootCmd.SetErr(&buf)
			rootCmd.SetArgs([]string{"template", "show", tmpl.Name})

			if err := rootCmd.Execute(); err != nil {
				t.Fatalf("template show %s: %v", tmpl.Name, err)
			}

			output := buf.String()
			if !strings.Contains(output, tmpl.Name) {
				t.Errorf("output should contain template name %q", tmpl.Name)
			}
			if !strings.Contains(output, tmpl.Icon) {
				t.Errorf("output should contain icon %q", tmpl.Icon)
			}
		})
	}
}
```

Add import for `"github.com/drolosoft/cmux-resurrect/internal/gallery"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run TestTemplateShow -v`
Expected: FAIL — show command not implemented

- [ ] **Step 3: Implement template_show.go with ASCII renderer**

Create `cmd/template_show.go`:

```go
package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/drolosoft/cmux-resurrect/internal/gallery"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/spf13/cobra"
)

var templateShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show template details with ASCII preview",
	Args:  cobra.ExactArgs(1),
	RunE:  runTemplateShow,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeTemplateNames(cmd, args, toComplete)
	},
}

func init() {
	templateCmd.AddCommand(templateShowCmd)
}

func runTemplateShow(cmd *cobra.Command, args []string) error {
	name := args[0]
	w := cmd.OutOrStderr()

	tmpl, ok := gallery.Get(name)
	if !ok {
		return fmt.Errorf("template %q not found. Run 'crex template list' to see available templates", name)
	}

	fmt.Fprintln(w)
	// Header: icon + name + description
	fmt.Fprintf(w, "  %s %s — %s\n",
		tmpl.Icon,
		greenStyle.Render(tmpl.Name),
		tmpl.Description)
	fmt.Fprintln(w)

	// ASCII diagram
	diagram := renderDiagram(tmpl)
	for _, line := range strings.Split(diagram, "\n") {
		fmt.Fprintf(w, "  %s\n", line)
	}
	fmt.Fprintln(w)

	// Metadata
	fmt.Fprintf(w, "  %s  %s\n", dimStyle.Render("Category:"), tmpl.Category)
	fmt.Fprintf(w, "  %s     %s\n", dimStyle.Render("Panes:"), fmt.Sprintf("%d", len(tmpl.Panes)))

	// Split sequence
	splits := []string{"main"}
	for _, p := range tmpl.Panes[1:] {
		if p.FocusTarget >= 0 {
			splits = append(splits, fmt.Sprintf("focus[%d]", p.FocusTarget))
		}
		dir := p.Split
		if dir == "" {
			dir = "right"
		}
		splits = append(splits, dir)
	}
	fmt.Fprintf(w, "  %s   %s\n", dimStyle.Render("Splits:"), strings.Join(splits, " → "))

	if len(tmpl.Tags) > 0 {
		fmt.Fprintf(w, "  %s     %s\n", dimStyle.Render("Tags:"), strings.Join(tmpl.Tags, ", "))
	}

	fmt.Fprintln(w)
	return nil
}

// renderDiagram generates an ASCII box diagram for a template.
func renderDiagram(tmpl *model.Template) string {
	panes := tmpl.Panes
	n := len(panes)
	if n == 0 {
		return ""
	}

	// Build pane labels
	labels := make([]string, n)
	for i, p := range panes {
		if p.Command != "" {
			labels[i] = p.Command
		} else if p.Name != "" {
			labels[i] = p.Name
		} else if p.IsMain {
			labels[i] = "main"
		} else {
			labels[i] = "shell"
		}
		if p.Focus {
			labels[i] += " *"
		}
	}

	// Hardcoded diagrams per template name for perfect rendering.
	// These are carefully crafted to match the spec diagrams.
	switch tmpl.Name {
	case "single":
		return singleDiagram(labels)
	case "cols", "sidebar", "explore", "system", "network":
		return twoPaneHorizontalDiagram(labels)
	case "rows":
		return twoPaneVerticalDiagram(labels)
	case "shelf":
		return shelfDiagram(labels)
	case "aside", "claude", "code", "logs":
		return asideDiagram(labels)
	case "triple":
		return tripleDiagram(labels)
	case "quad":
		return quadDiagram(labels)
	case "dashboard":
		return dashboardDiagram(labels)
	case "ide":
		return ideDiagram(labels)
	default:
		return genericDiagram(labels, n)
	}
}

func singleDiagram(labels []string) string {
	l := truncLabel(labels[0], 30)
	return fmt.Sprintf("┌────────────────────────────────┐\n│ %-30s │\n│                                │\n│                                │\n└────────────────────────────────┘", l)
}

func twoPaneHorizontalDiagram(labels []string) string {
	l0 := truncLabel(labels[0], 14)
	l1 := truncLabel(safeLabel(labels, 1), 14)
	return fmt.Sprintf("┌────────────────┬───────────────┐\n│ %-14s │ %-13s │\n│                │               │\n│                │               │\n└────────────────┴───────────────┘", l0, l1)
}

func twoPaneVerticalDiagram(labels []string) string {
	l0 := truncLabel(labels[0], 30)
	l1 := truncLabel(safeLabel(labels, 1), 30)
	return fmt.Sprintf("┌────────────────────────────────┐\n│ %-30s │\n├────────────────────────────────┤\n│ %-30s │\n└────────────────────────────────┘", l0, l1)
}

func shelfDiagram(labels []string) string {
	l0 := truncLabel(labels[0], 30)
	l1 := truncLabel(safeLabel(labels, 1), 14)
	l2 := truncLabel(safeLabel(labels, 2), 13)
	return fmt.Sprintf("┌────────────────────────────────┐\n│ %-30s │\n│                                │\n├────────────────┬───────────────┤\n│ %-14s │ %-13s │\n└────────────────┴───────────────┘", l0, l1, l2)
}

func asideDiagram(labels []string) string {
	l0 := truncLabel(labels[0], 10)
	l1 := truncLabel(safeLabel(labels, 1), 18)
	l2 := truncLabel(safeLabel(labels, 2), 18)
	return fmt.Sprintf("┌────────────┬────────────────────┐\n│ %-10s │ %-18s │\n│            │                    │\n│            ├────────────────────┤\n│            │ %-18s │\n└────────────┴────────────────────┘", l0, l1, l2)
}

func tripleDiagram(labels []string) string {
	l0 := truncLabel(labels[0], 9)
	l1 := truncLabel(safeLabel(labels, 1), 9)
	l2 := truncLabel(safeLabel(labels, 2), 9)
	return fmt.Sprintf("┌───────────┬───────────┬─────────┐\n│ %-9s │ %-9s │ %-7s │\n│           │           │         │\n│           │           │         │\n└───────────┴───────────┴─────────┘", l0, l1, l2)
}

func quadDiagram(labels []string) string {
	l0 := truncLabel(labels[0], 14)
	l1 := truncLabel(safeLabel(labels, 1), 14)
	l2 := truncLabel(safeLabel(labels, 2), 14)
	l3 := truncLabel(safeLabel(labels, 3), 14)
	return fmt.Sprintf("┌────────────────┬────────────────┐\n│ %-14s │ %-14s │\n├────────────────┼────────────────┤\n│ %-14s │ %-14s │\n└────────────────┴────────────────┘", l0, l1, l2, l3)
}

func dashboardDiagram(labels []string) string {
	l0 := truncLabel(labels[0], 30)
	l1 := truncLabel(safeLabel(labels, 1), 9)
	l2 := truncLabel(safeLabel(labels, 2), 9)
	l3 := truncLabel(safeLabel(labels, 3), 7)
	return fmt.Sprintf("┌────────────────────────────────┐\n│ %-30s │\n├───────────┬───────────┬────────┤\n│ %-9s │ %-9s │ %-6s │\n└───────────┴───────────┴────────┘", l0, l1, l2, l3)
}

func ideDiagram(labels []string) string {
	l0 := truncLabel(labels[0], 6)
	l1 := truncLabel(safeLabel(labels, 1), 22)
	l2 := truncLabel(safeLabel(labels, 2), 12)
	l3 := truncLabel(safeLabel(labels, 3), 7)
	return fmt.Sprintf("┌────────┬────────────────────────┐\n│ %-6s │ %-22s │\n│        ├──────────────┬─────────┤\n│        │ %-12s │ %-7s │\n└────────┴──────────────┴─────────┘", l0, l1, l2, l3)
}

func genericDiagram(labels []string, n int) string {
	// Simple fallback: show pane count
	return fmt.Sprintf("[%d panes]", n)
}

func truncLabel(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func safeLabel(labels []string, i int) string {
	if i < len(labels) {
		return labels[i]
	}
	return "shell"
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./cmd/ -run TestTemplateShow -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/template_show.go cmd/template_test.go
git commit -m "feat(cli): add 'crex template show' with ASCII diagram previews"
```

---

### Task 8: Template Use Subcommand

**Files:**
- Create: `internal/orchestrate/template_use.go`
- Create: `cmd/template_use.go`
- Test: `cmd/template_test.go` (append)

- [ ] **Step 1: Write test for template use dry-run**

Append to `cmd/template_test.go`:

```go
func TestTemplateUse_DryRun(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"template", "use", "cols", "/tmp/test", "--dry-run"})

	err := rootCmd.Execute()
	// May fail if cmux not running, but dry-run should work
	if err != nil {
		// Only fail if it's not a cmux-not-reachable error
		if !strings.Contains(err.Error(), "cmux") {
			t.Fatalf("template use --dry-run: %v", err)
		}
	}
}
```

- [ ] **Step 2: Create template_use orchestrator**

Create `internal/orchestrate/template_use.go`:

```go
package orchestrate

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/model"
)

// TemplateUseOpts configures a one-shot workspace creation from a template.
type TemplateUseOpts struct {
	Title string
	Icon  string
	CWD   string
	Pin   bool
}

// TemplateUseResult reports what happened.
type TemplateUseResult struct {
	Title    string
	Panes    int
	DryRun   bool
	Commands []string
}

// TemplateUser creates a single workspace from resolved template panes.
type TemplateUser struct {
	Client     client.CmuxClient
	OnProgress func(msg string)
}

// Use creates a workspace from template panes.
func (tu *TemplateUser) Use(panes []model.Pane, opts TemplateUseOpts, dryRun bool) (*TemplateUseResult, error) {
	title := opts.Title
	if title == "" {
		title = filepath.Base(opts.CWD)
	}
	if opts.Icon != "" {
		title = opts.Icon + " " + title
	}

	result := &TemplateUseResult{
		Title:  title,
		Panes:  len(panes),
		DryRun: dryRun,
	}

	if dryRun {
		ref := "workspace:new"
		result.Commands = append(result.Commands,
			fmt.Sprintf("cmux new-workspace --cwd %q", opts.CWD))
		for i, pane := range panes {
			if i == 0 {
				if pane.Command != "" {
					result.Commands = append(result.Commands,
						fmt.Sprintf("cmux send --workspace %s %q", ref, pane.Command))
				}
				continue
			}
			if pane.FocusTarget >= 0 {
				result.Commands = append(result.Commands,
					fmt.Sprintf("cmux focus-pane --pane pane:%d --workspace %s", pane.FocusTarget, ref))
			}
			dir := pane.Split
			if dir == "" {
				dir = "right"
			}
			result.Commands = append(result.Commands,
				fmt.Sprintf("cmux new-split %s --workspace %s", dir, ref))
			if pane.Command != "" {
				result.Commands = append(result.Commands,
					fmt.Sprintf("cmux send --workspace %s %q", ref, pane.Command))
			}
		}
		result.Commands = append(result.Commands,
			fmt.Sprintf("cmux rename-workspace --workspace %s %q", ref, title))
		if opts.Pin {
			result.Commands = append(result.Commands,
				fmt.Sprintf("cmux pin-workspace --workspace %s", ref))
		}
		return result, nil
	}

	// Real execution
	ref, err := tu.Client.NewWorkspace(client.NewWorkspaceOpts{CWD: opts.CWD})
	if err != nil {
		return nil, fmt.Errorf("new-workspace: %w", err)
	}
	time.Sleep(DelayAfterCreate)

	if err := tu.Client.SelectWorkspace(ref); err != nil {
		return nil, fmt.Errorf("select-workspace: %w", err)
	}
	time.Sleep(DelayAfterSelect)

	for i, pane := range panes {
		if i == 0 {
			if pane.Command != "" {
				_ = tu.Client.Send(ref, "", pane.Command+"\\n")
			}
			continue
		}
		if pane.FocusTarget >= 0 {
			targetRef := fmt.Sprintf("pane:%d", pane.FocusTarget)
			_ = tu.Client.FocusPane(targetRef, ref)
			time.Sleep(DelayAfterSelect)
		}
		dir := pane.Split
		if dir == "" {
			dir = "right"
		}
		surfaceRef, err := tu.Client.NewSplit(dir, ref)
		if err != nil {
			continue
		}
		time.Sleep(DelayAfterSplit)
		if pane.Command != "" {
			_ = tu.Client.Send(ref, surfaceRef, pane.Command+"\\n")
		}
	}

	time.Sleep(DelayBeforeRename)
	_ = tu.Client.RenameWorkspace(ref, title)

	if opts.Pin {
		_ = tu.Client.PinWorkspace(ref)
	}

	return result, nil
}
```

- [ ] **Step 3: Create template_use.go CLI command**

Create `cmd/template_use.go`:

```go
package cmd

import (
	"fmt"

	"github.com/drolosoft/cmux-resurrect/internal/config"
	"github.com/drolosoft/cmux-resurrect/internal/gallery"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/orchestrate"
	"github.com/spf13/cobra"
)

var (
	useTemplateName    string
	useTemplateIcon    string
	useTemplateDryRun  bool
	useTemplatePin     bool
)

var templateUseCmd = &cobra.Command{
	Use:   "use <template> [path]",
	Short: "Create a workspace from a template",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runTemplateUse,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			return completeTemplateNames(cmd, args, toComplete)
		case 1:
			return nil, cobra.ShellCompDirectiveFilterDirs
		default:
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	},
}

func init() {
	templateUseCmd.Flags().StringVar(&useTemplateName, "name", "", "custom workspace title")
	templateUseCmd.Flags().StringVar(&useTemplateIcon, "icon", "", "custom workspace icon")
	templateUseCmd.Flags().BoolVar(&useTemplateDryRun, "dry-run", false, "preview commands without creating")
	templateUseCmd.Flags().BoolVar(&useTemplatePin, "pin", false, "pin the workspace")
	templateCmd.AddCommand(templateUseCmd)
}

func runTemplateUse(cmd *cobra.Command, args []string) error {
	templateName := args[0]
	cwd := "."
	if len(args) > 1 {
		cwd = config.ExpandHome(args[1])
	}

	tmpl, ok := gallery.Get(templateName)
	if !ok {
		return fmt.Errorf("template %q not found. Run 'crex template list' to see available templates", templateName)
	}

	// Resolve panes
	panes := gallery.BuildPanes(tmpl)

	w := cmd.OutOrStderr()
	cl := newClient()

	if !useTemplateDryRun {
		if err := cl.Ping(); err != nil {
			return fmt.Errorf("cmux not reachable: %w", err)
		}
	}

	icon := useTemplateIcon
	if icon == "" && tmpl.Category == "workflow" {
		icon = tmpl.Icon
	}

	tu := &orchestrate.TemplateUser{Client: cl}
	result, err := tu.Use(panes, orchestrate.TemplateUseOpts{
		Title: useTemplateName,
		Icon:  icon,
		CWD:   cwd,
		Pin:   useTemplatePin,
	}, useTemplateDryRun)

	if err != nil {
		return err
	}

	if result.DryRun {
		fmt.Fprintf(w, "\n  %s %s from '%s'\n\n",
			tmpl.Icon, yellowStyle.Render("Dry-run"), greenStyle.Render(templateName))
		for _, c := range result.Commands {
			fmt.Fprintf(w, "  %s\n", dimStyle.Render(c))
		}
		fmt.Fprintln(w)
	} else {
		fmt.Fprintf(w, "\n  %s %s\n\n",
			greenStyle.Render("✅ Workspace created:"),
			greenStyle.Render(result.Title))
	}

	return nil
}
```

Note: We need to export `BuildPanes` from the gallery package. In `internal/gallery/gallery.go`, rename the existing `buildPanes` to `BuildPanes` (capitalize):

```go
func BuildPanes(tmpl *model.Template) []model.Pane {
```

And update `ResolveTemplate` to call `BuildPanes` (capitalized).

- [ ] **Step 4: Run tests**

Run: `go test ./cmd/ -run TestTemplateUse -v`
Expected: PASS (or skip if cmux not available for non-dry-run)

- [ ] **Step 5: Commit**

```bash
git add internal/orchestrate/template_use.go cmd/template_use.go internal/gallery/gallery.go
git commit -m "feat(cli): add 'crex template use' for one-shot workspace creation"
```

---

### Task 9: Template Customize Subcommand

**Files:**
- Create: `cmd/template_customize.go`
- Test: `cmd/template_test.go` (append)

- [ ] **Step 1: Write test**

Append to `cmd/template_test.go`:

```go
func TestTemplateCustomize_CopiesToBlueprint(t *testing.T) {
	// Create a temp workspace file
	tmpDir := t.TempDir()
	wsFile := filepath.Join(tmpDir, "workspaces.md")

	// Write a minimal workspace file
	os.WriteFile(wsFile, []byte("## Projects\n**Icon | Name | Template | Pin | Path**\n\n## Templates\n\n### dev\n- [x] main terminal (focused)\n"), 0o644)

	// Override config for this test
	oldWsFile := cfg.WorkspaceFile
	cfg.WorkspaceFile = wsFile
	defer func() { cfg.WorkspaceFile = oldWsFile }()

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"template", "customize", "claude"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("template customize claude: %v", err)
	}

	// Read the file and check it contains the claude template
	data, err := os.ReadFile(wsFile)
	if err != nil {
		t.Fatalf("read workspace file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "### claude") {
		t.Error("workspace file should contain '### claude' after customize")
	}
}
```

Add imports: `"os"`, `"path/filepath"`.

- [ ] **Step 2: Implement template_customize.go**

Create `cmd/template_customize.go`:

```go
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/drolosoft/cmux-resurrect/internal/gallery"
	"github.com/drolosoft/cmux-resurrect/internal/mdfile"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/spf13/cobra"
)

var templateCustomizeCmd = &cobra.Command{
	Use:   "customize <name>",
	Short: "Fork a built-in template into your Workspace Blueprint",
	Args:  cobra.ExactArgs(1),
	RunE:  runTemplateCustomize,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// Only complete built-in template names
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		var names []string
		for _, tmpl := range gallery.List() {
			names = append(names, fmt.Sprintf("%s\t%s %s", tmpl.Name, tmpl.Icon, tmpl.Description))
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	},
}

func init() {
	templateCmd.AddCommand(templateCustomizeCmd)
}

func runTemplateCustomize(cmd *cobra.Command, args []string) error {
	name := args[0]
	w := cmd.OutOrStderr()

	tmpl, ok := gallery.Get(name)
	if !ok {
		return fmt.Errorf("template %q not found. Run 'crex template list' to see available templates", name)
	}

	wsFile := cfg.WorkspaceFile

	// Parse existing workspace file (or create empty)
	wf, err := mdfile.Parse(wsFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		wf = &model.WorkspaceFile{
			Templates: make(map[string]*model.Template),
		}
	}

	// Check if user already has this template
	if _, exists := wf.Templates[name]; exists {
		return fmt.Errorf("template %q already exists in your Workspace Blueprint. Edit it with: crex edit", name)
	}

	// Copy the gallery template (without gallery-only metadata)
	userTmpl := &model.Template{
		Name:  tmpl.Name,
		Panes: make([]model.TemplatePan, len(tmpl.Panes)),
	}
	copy(userTmpl.Panes, tmpl.Panes)
	// Strip FocusTarget from user copy (not supported in Blueprint syntax)
	for i := range userTmpl.Panes {
		userTmpl.Panes[i].FocusTarget = -1
	}

	wf.Templates[name] = userTmpl

	if err := mdfile.Write(wsFile, wf); err != nil {
		return fmt.Errorf("write workspace file: %w", err)
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %s Copied '%s' to your Workspace Blueprint.\n",
		greenStyle.Render("✅"),
		greenStyle.Render(name))
	fmt.Fprintf(w, "  %s\n", dimStyle.Render("Your copy now takes priority over the built-in."))
	fmt.Fprintf(w, "  %s %s\n\n", dimStyle.Render("Edit with:"), cyanStyle.Render("crex edit"))

	return nil
}

// completeTemplateNames provides completion for all template names.
func completeTemplateNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var names []string
	for _, tmpl := range gallery.List() {
		names = append(names, fmt.Sprintf("%s\t%s %s", tmpl.Name, tmpl.Icon, tmpl.Description))
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}
```

Wait — `completeTemplateNames` is better placed in `cmd/completion_helpers.go` since it's shared. Move it there instead.

- [ ] **Step 3: Move completeTemplateNames to completion_helpers.go**

Add to `cmd/completion_helpers.go`:

```go
// completeTemplateNames provides dynamic completion of gallery template names.
// Used by: template show, template use, template customize.
func completeTemplateNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var names []string
	for _, tmpl := range gallery.List() {
		names = append(names, fmt.Sprintf("%s\t%s %s", tmpl.Name, tmpl.Icon, tmpl.Description))
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}
```

Add import `"github.com/drolosoft/cmux-resurrect/internal/gallery"` to completion_helpers.go.

Remove `completeTemplateNames` from `template_customize.go`.

- [ ] **Step 4: Run tests**

Run: `go test ./cmd/ -run TestTemplateCustomize -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/template_customize.go cmd/completion_helpers.go cmd/template_test.go
git commit -m "feat(cli): add 'crex template customize' to fork gallery templates"
```

---

### Task 10: Update ws add Completion + Help Output

**Files:**
- Modify: `cmd/ws_add.go:31-38`
- Modify: `cmd/style.go:60-76`

- [ ] **Step 1: Update --template flag completion in ws_add.go**

Replace the hardcoded template list in `cmd/ws_add.go` lines 31-38:

```go
	_ = wsAddCmd.RegisterFlagCompletionFunc("template", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeTemplateNames(cmd, args, toComplete)
	})
```

Also update the flag description at line 28:

```go
	wsAddCmd.Flags().StringVarP(&addTemplate, "template", "t", "dev", "template name (run 'crex template list' for options)")
```

- [ ] **Step 2: Add template command to styledHelp in style.go**

In `cmd/style.go`, in the `styledHelp()` function, add after line 73 (`helpCmd(&b, "workspace", "<cmd>", ...)`):

```go
	helpCmd(&b, "template", "<cmd>", "Template gallery (list|show|use|customize)")
```

- [ ] **Step 3: Run full test suite**

Run: `go test ./... -count=1`
Expected: ALL PASS

- [ ] **Step 4: Commit**

```bash
git add cmd/ws_add.go cmd/style.go
git commit -m "feat(cli): update ws add completion and help to reference template gallery"
```

---

### Task 11: Simplify DefaultTemplates

**Files:**
- Modify: `internal/mdfile/write.go:169-202`
- Test: existing mdfile tests

- [ ] **Step 1: Simplify DefaultTemplates to dev + single**

Replace `DefaultTemplates()` in `internal/mdfile/write.go` (lines 169-202):

```go
// DefaultTemplates returns the starter templates written to new workspace files.
// The full gallery (16 templates) is available via gallery.ResolveTemplate().
func DefaultTemplates() map[string]*model.Template {
	return map[string]*model.Template{
		"dev": {
			Name: "dev",
			Panes: []model.TemplatePan{
				{Enabled: true, IsMain: true, Type: "terminal", Focus: true, FocusTarget: -1},
				{Enabled: true, Split: "right", Type: "terminal", Command: "npm run dev", FocusTarget: -1},
				{Enabled: true, Split: "right", Type: "terminal", Command: "lazygit", FocusTarget: -1},
			},
		},
		"single": {
			Name: "single",
			Panes: []model.TemplatePan{
				{Enabled: true, IsMain: true, Type: "terminal", Focus: true, FocusTarget: -1},
			},
		},
	}
}
```

- [ ] **Step 2: Run full test suite**

Run: `go test ./... -count=1`
Expected: ALL PASS — `go` and `monitor` templates are now in the gallery, not in DefaultTemplates

- [ ] **Step 3: Commit**

```bash
git add internal/mdfile/write.go
git commit -m "refactor(mdfile): simplify DefaultTemplates to dev + single; full gallery available via resolution"
```

---

### Task 12: Documentation

**Files:**
- Create: `docs/templates.md`
- Create: `docs/template-authoring.md`
- Modify: `README.md`
- Modify: `docs/commands.md`
- Modify: `docs/blueprint.md`
- Modify: `ARCHITECTURE.md`

- [ ] **Step 1: Create docs/templates.md**

Full gallery documentation with ASCII diagrams for all 16 templates, grouped by category. Include for each: name, icon, description, ASCII diagram, pane labels, split sequence, usage example.

- [ ] **Step 2: Create docs/template-authoring.md**

Guide for creating custom templates: syntax reference, how to contribute to the gallery, selection criteria.

- [ ] **Step 3: Update README.md**

Add a "Template Gallery" section after "Workspace Blueprints":

```markdown
## 📦 Template Gallery

crex ships with 16 ready-to-use workspace templates for common developer workflows.

| | Layout Templates | | Workflow Templates |
|---|---|---|---|
| ▥ | `cols` — side-by-side | 🤖 | `claude` — AI pair-programming |
| ▤ | `rows` — stacked | 💻 | `code` — general coding |
| ◧ | `sidebar` — main + side | 🔭 | `explore` — navigate codebase |
| ⊤ | `shelf` — big top, 2 bottom | 📊 | `system` — monitor health |
| ⊢ | `aside` — big left, 2 right | 📜 | `logs` — tail streams |
| Ⅲ | `triple` — three columns | 🌐 | `network` — debug connectivity |
| ⊠ | `quad` — 2×2 grid | 📟 | `single` — minimal terminal |
| ◱ | `dashboard` — top + 3 bottom | | |
| ⧉ | `ide` — full IDE layout | | |

```sh
crex template list                    # browse all templates
crex template show claude             # preview with ASCII diagram
crex template use claude ~/project    # create workspace instantly
crex template customize claude        # fork to your Blueprint
```

> Templates are starting points. Run `crex template customize <name>` to fork any template and make it yours.

See [docs/templates.md](docs/templates.md) for the full gallery with diagrams.
```

Add to the help output and docs table.

Update the comparison table row for templates:
```markdown
| 🧩 | Manual pane recreation | **16 built-in templates** + custom Blueprints |
```

Add to documentation table:
```markdown
| [Template Gallery](docs/templates.md) | Built-in templates, ASCII previews, customization |
| [Template Authoring](docs/template-authoring.md) | Create and contribute custom templates |
```

- [ ] **Step 4: Update docs/commands.md**

Add template command group reference with all 4 subcommands, flags, and examples.

- [ ] **Step 5: Update docs/blueprint.md**

Add note about three-tier resolution: "Templates defined in your Blueprint take priority over built-in gallery templates. Run `crex template list` to see all available templates."

- [ ] **Step 6: Update ARCHITECTURE.md**

Document: gallery package, embedding strategy, three-tier resolution, FocusTarget mechanism.

- [ ] **Step 7: Commit**

```bash
git add docs/templates.md docs/template-authoring.md README.md docs/commands.md docs/blueprint.md ARCHITECTURE.md
git commit -m "docs: add template gallery documentation, update README and architecture"
```

---

### Task 13: Final Verification

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -v -count=1`
Expected: ALL PASS

- [ ] **Step 2: Run linter**

Run: `make lint`
Expected: No errors

- [ ] **Step 3: Build binary**

Run: `make build`
Expected: `bin/crex` built successfully

- [ ] **Step 4: Manual smoke test**

Run: `bin/crex template list`
Expected: Shows all 16 templates with icons, pane counts, descriptions

Run: `bin/crex template show claude`
Expected: Shows ASCII diagram with pane labels and metadata

Run: `bin/crex template show quad`
Expected: Shows 2x2 grid diagram

Run: `bin/crex template use cols /tmp/test --dry-run`
Expected: Shows cmux commands that would be executed

- [ ] **Step 5: Document Drolosoft website changes needed**

Create a note for the Drolosoft agent listing required website updates:
- `/cmux-resurrect.html` — Add template gallery feature with visual examples
- Homepage Developer Tools section — Mention "16 built-in templates" in feature list
- Consider adding template gallery screenshots

- [ ] **Step 6: Commit any remaining changes**

```bash
git add -A
git commit -m "feat: template gallery — final verification and cleanup"
```
