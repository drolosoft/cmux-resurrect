# crex pop Enhanced Picker — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rewrite the crex pop picker as a centered floating window with fzf-style fuzzy search, match highlighting, and two-level workspace drill-in.

**Architecture:** Rewrite `internal/tui/pop.go` with lipgloss centered box, `sahilm/fuzzy` for search, and a viewMode enum for list/drill states. Modify `cmd/pop.go` to pass a layout loader and handle the new workspace-level result. All changes are internal — no public API or CLI flag changes.

**Tech Stack:** Go, bubbletea, lipgloss, sahilm/fuzzy

**Spec:** `docs/superpowers/specs/2026-06-04-crex-pop-enhanced-design.md`

---

### Task 1: Add fuzzy dependency

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add dependency**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go get github.com/sahilm/fuzzy`

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: add sahilm/fuzzy for fzf-style search"
```

---

### Task 2: Rewrite PopModel with centered box + fuzzy + drill-in

**Files:**
- Rewrite: `internal/tui/pop.go`
- Rewrite: `internal/tui/pop_test.go`

This is the core task. The new PopModel has:
- Centered floating window via `lipgloss.Place()`
- Fuzzy search via `sahilm/fuzzy` with match highlighting
- Two modes: `modeList` (layouts+templates) and `modeDrill` (workspaces in a layout)
- Scrolling list with pinned header/footer
- `loadLayout` function injected for drill-in

- [ ] **Step 1: Write the test file**

Rewrite `internal/tui/pop_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/model"
)

func testPopItems() []PopItem {
	return []PopItem{
		{Kind: "layout", Name: "morning", Meta: "3 tabs  2h ago"},
		{Kind: "layout", Name: "afternoon", Meta: "2 tabs  1d ago"},
		{Kind: "template", Name: "ide", Icon: "⧉", Meta: "editor+git+term"},
		{Kind: "template", Name: "claude", Icon: "🤖", Meta: "claude code setup"},
	}
}

func mockLoader(name string) (*model.Layout, error) {
	if name == "morning" {
		return &model.Layout{
			Name: "morning",
			Workspaces: []model.Workspace{
				{Title: "webapp", CWD: "~/projects/webapp", Index: 0, Pinned: true,
					Panes: []model.Pane{{Type: "terminal", Command: "npm run dev"}, {Type: "terminal"}}},
				{Title: "api", CWD: "~/projects/api", Index: 1,
					Panes: []model.Pane{{Type: "terminal", Command: "go test"}}},
			},
		}, nil
	}
	return nil, nil
}

// --- Fuzzy filter tests ---

func TestPop_FilterFuzzyMatch(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 30, mockLoader)
	m.filter = "mor"
	m.applyFilter()
	if len(m.filtered) < 1 {
		t.Fatal("filter 'mor' should match 'morning'")
	}
	if m.filtered[0].Name != "morning" {
		t.Errorf("first match = %q, want morning", m.filtered[0].Name)
	}
}

func TestPop_FilterScatteredMatch(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 30, mockLoader)
	m.filter = "cde"
	m.applyFilter()
	// "cde" should fuzzy-match "claude" (c...d...e)
	found := false
	for _, item := range m.filtered {
		if item.Name == "claude" {
			found = true
		}
	}
	if !found {
		t.Error("filter 'cde' should fuzzy-match 'claude'")
	}
}

func TestPop_FilterNoMatch(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 30, mockLoader)
	m.filter = "zzz"
	m.applyFilter()
	if len(m.filtered) != 0 {
		t.Errorf("filtered = %d, want 0", len(m.filtered))
	}
}

func TestPop_FilterEmpty(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 30, mockLoader)
	m.applyFilter()
	if len(m.filtered) != 4 {
		t.Errorf("no filter: filtered = %d, want 4", len(m.filtered))
	}
}

func TestPop_MatchPositions(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 30, mockLoader)
	m.filter = "mor"
	m.applyFilter()
	if len(m.matchPositions) == 0 {
		t.Fatal("matchPositions empty after fuzzy match")
	}
	if len(m.matchPositions[0]) == 0 {
		t.Error("first match should have non-empty positions")
	}
}

// --- Navigation tests ---

func TestPop_CursorBounds(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 30, mockLoader)
	m.cursorUp()
	if m.cursor != 0 {
		t.Errorf("up from 0: cursor = %d", m.cursor)
	}
	for i := 0; i < 20; i++ {
		m.cursorDown()
	}
	want := len(m.filtered) - 1
	if m.cursor != want {
		t.Errorf("many downs: cursor = %d, want %d", m.cursor, want)
	}
}

func TestPop_NumberSelect(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 30, mockLoader)
	item := m.selectByNumber(2)
	if item == nil || item.Name != "afternoon" {
		t.Errorf("selectByNumber(2) = %v, want afternoon", item)
	}
}

func TestPop_NumberOutOfRange(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 30, mockLoader)
	if m.selectByNumber(9) != nil {
		t.Error("selectByNumber(9) should be nil")
	}
	if m.selectByNumber(0) != nil {
		t.Error("selectByNumber(0) should be nil")
	}
}

func TestPop_CursorResetsOnFilter(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 30, mockLoader)
	m.cursor = 3
	m.filter = "mor"
	m.applyFilter()
	if m.cursor != 0 {
		t.Errorf("cursor after filter = %d, want 0", m.cursor)
	}
}

// --- Drill-in tests ---

func TestPop_DrillEnter(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 30, mockLoader)
	m.enterDrill("morning")
	if m.mode != modeDrill {
		t.Error("mode should be modeDrill after enterDrill")
	}
	if len(m.drillItems) != 2 {
		t.Errorf("drillItems = %d, want 2", len(m.drillItems))
	}
	if m.drillItems[0].Title != "webapp" {
		t.Errorf("first workspace = %q, want webapp", m.drillItems[0].Title)
	}
}

func TestPop_DrillExit(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 30, mockLoader)
	m.enterDrill("morning")
	m.exitDrill()
	if m.mode != modeList {
		t.Error("mode should be modeList after exitDrill")
	}
	if len(m.drillItems) != 0 {
		t.Error("drillItems should be empty after exit")
	}
}

func TestPop_DrillOnTemplate_Noop(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 30, mockLoader)
	// Cursor on template (index 2 = "ide")
	m.cursor = 2
	// enterDrill with template name should fail gracefully
	m.enterDrill("ide")
	// loadLayout returns nil for "ide" → should stay in list mode
	if m.mode != modeList {
		t.Error("drill on non-existent layout should stay in modeList")
	}
}

func TestPop_DrillPaneSummary(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 30, mockLoader)
	m.enterDrill("morning")
	if m.drillItems[0].PaneSummary != "npm run dev | shell" {
		t.Errorf("PaneSummary = %q", m.drillItems[0].PaneSummary)
	}
}

// --- View rendering tests ---

func TestPop_ViewSections(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 30, mockLoader)
	view := m.View()
	if !strings.Contains(view, "LAYOUTS") {
		t.Error("missing LAYOUTS header")
	}
	if !strings.Contains(view, "TEMPLATES") {
		t.Error("missing TEMPLATES header")
	}
}

func TestPop_ViewFooterList(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 30, mockLoader)
	view := m.View()
	if !strings.Contains(view, "tab") && !strings.Contains(view, "drill") {
		t.Error("list footer should mention tab/drill")
	}
}

func TestPop_ViewFooterDrill(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 30, mockLoader)
	m.enterDrill("morning")
	view := m.View()
	if !strings.Contains(view, "back") {
		t.Error("drill footer should mention back")
	}
}

func TestPop_ViewBreadcrumb(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 30, mockLoader)
	m.enterDrill("morning")
	view := m.View()
	if !strings.Contains(view, "morning") {
		t.Error("drill view should show layout name in breadcrumb")
	}
}

func TestPop_ViewHasBorder(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 30, mockLoader)
	view := m.View()
	if !strings.Contains(view, "╭") || !strings.Contains(view, "╯") {
		t.Error("view should have rounded border chars")
	}
}

func TestPop_ViewDrillArrow(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 30, mockLoader)
	view := m.View()
	// Layout items should show → drill indicator
	if !strings.Contains(view, "→") {
		t.Error("layout items should show → drill indicator")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go test ./internal/tui/ -run TestPop -v -count=1`
Expected: FAIL — new constructor signature, DrillItem, etc. not defined.

- [ ] **Step 3: Implement the rewritten pop.go**

Rewrite `internal/tui/pop.go` with the full implementation. The file should contain:

**Types:**
- `viewMode` (int enum: `modeList`, `modeDrill`)
- `PopItem` (unchanged: Kind, Name, Icon, Meta)
- `DrillItem` (LayoutName, Index, Title, CWD, PaneCount, PaneSummary, Pinned)
- `PopResult` (Kind string, Name string, WorkspaceTitle string)
- `PopModel` struct with all fields: items, filtered, matchPositions, filter, cursor, offset, chosen, mode, drillLayout, drillItems, drillFiltered, drillCursor, drillOffset, chosenWorkspace, loadLayout, width, height, quitting

**Constructor:**
- `NewPopModel(items []PopItem, width, height int, loadLayout func(string) (*model.Layout, error)) *PopModel`

**Methods:**
- `Init()` → `tea.EnterAltScreen`
- `Update(msg)` → branches on `m.mode` to `updateList` or `updateDrill`
- `View()` → centered box via `lipgloss.Place()`, pinned header/footer, scrolling list
- `Result() *PopResult` → returns layout/template/workspace result
- `applyFilter()` → uses `fuzzy.FindFrom()`, stores matchPositions
- `enterDrill(name)` → calls loadLayout, builds DrillItem slice
- `exitDrill()` → restores list mode
- `cursorUp/Down()`, `selectByNumber()` — same as before
- `highlightMatches(s string, positions []int) string` — renders matched chars in orange+bold+underline

**View structure:**
```
lipgloss.Place(width, height, Center, Center,
    popBoxStyle.Render(
        header (search bar, 2 lines)
        + list (scrollable, fills remaining)
        + footer (hints, 2 lines)
    )
)
```

**Key behaviors:**
- List mode: Tab/→ on layout → `enterDrill`. Enter → select. Esc → quit.
- Drill mode: Enter → select workspace. Esc/Tab/← → `exitDrill`.
- Number keys 1-9 work in both modes.
- Layout items show `→` indicator on the cursor line.
- Drill view header: `crex > morning > _`

**Styles (reuse existing palette):**
- `popBoxStyle`: RoundedBorder, orange border color, Padding(1, 2)
- `popMatchStyle`: orange + bold + underline (for highlighted fuzzy chars)
- Keep existing: popHeaderStyle, popCursorStyle, popDimStyle, popSectionStyle, etc.

**Fuzzy source:**
```go
type fuzzySource []PopItem
func (s fuzzySource) String(i int) string { return s[i].Name + " " + s[i].Meta }
func (s fuzzySource) Len() int { return len(s) }
```

**Scroll logic:**
- `offset` adjusted in render to keep cursor visible
- `listHeight = innerHeight - headerLines - footerLines`

- [ ] **Step 4: Run tests**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go test ./internal/tui/ -run TestPop -v -count=1`
Expected: ALL PASS.

- [ ] **Step 5: Run full test suite**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go test ./... -count=1`
Expected: ALL PASS (existing tests unaffected).

- [ ] **Step 6: Commit**

```bash
git add internal/tui/pop.go internal/tui/pop_test.go
git commit -m "feat: enhanced PopModel — centered box, fuzzy search, drill-in

Centered floating window via lipgloss.Place(). sahilm/fuzzy for
fzf-style search with match highlighting. Two-level drill-in:
Tab/→ on layout shows workspaces, Esc/← goes back.
21 tests covering fuzzy, navigation, drill-in, and view rendering."
```

---

### Task 3: Wire enhanced PopModel into cmd/pop.go

**Files:**
- Modify: `cmd/pop.go`

- [ ] **Step 1: Update popPicker to pass loadLayout and handle Result**

The changes to `cmd/pop.go`:

1. Update `popPicker()` to pass `store.Load` as the loadLayout function:

```go
func popPicker() error {
	store, err := newStore()
	if err != nil {
		return err
	}

	items, err := buildPopItems()
	if err != nil {
		return err
	}
	if len(items) == 0 {
		fmt.Fprintln(os.Stderr, dimStyle.Render("  No layouts or templates found."))
		return nil
	}

	m := tui.NewPopModel(items, 0, 0, store.Load)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("picker: %w", err)
	}

	pm, ok := finalModel.(*tui.PopModel)
	if !ok {
		return nil
	}

	result := pm.Result()
	if result == nil {
		return nil
	}

	switch result.Kind {
	case "layout":
		return doRestore(result.Name)
	case "template":
		return doTemplateUse(result.Name, ".")
	case "workspace":
		return doRestoreWorkspace(result.Name, result.WorkspaceTitle)
	}
	return nil
}
```

2. Add `doRestoreWorkspace` for single-workspace restore:

```go
// doRestoreWorkspace restores a single workspace from a layout by title.
func doRestoreWorkspace(layoutName, workspaceTitle string) error {
	cl := newClient()
	store, err := newStore()
	if err != nil {
		return err
	}

	restorer := &orchestrate.Restorer{
		Client: cl,
		Store:  store,
		OnProgress: func(title string, panes int, err error) {
			t := padTitle(title)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  %s  %s: %v\n", yellowStyle.Render("FAIL"), t, err)
			} else {
				fmt.Fprintf(os.Stderr, "  %s  %s (%d panes)\n", greenStyle.Render("OK"), t, panes)
			}
		},
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "%s %s > %s\n",
		yellowStyle.Render("➕ Restoring"),
		greenStyle.Render(layoutName),
		cyanStyle.Render(workspaceTitle))

	result, err := restorer.Restore(layoutName, false, orchestrate.RestoreModeAdd, workspaceTitle, true)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  %s\n\n",
		greenStyle.Render(fmt.Sprintf("✅ Restored %d/%d %s",
			result.WorkspacesOK, result.WorkspacesTotal, unitName(result.WorkspacesTotal))))
	return nil
}
```

3. Remove the `_ = strings` import if unused (check after edits).

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go build ./...`
Expected: clean build.

- [ ] **Step 3: Run full test suite**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go test ./... -count=1`
Expected: ALL PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/pop.go
git commit -m "feat: wire enhanced PopModel — workspace drill-in + Result type

Pass store.Load as loadLayout. Handle PopResult with kind=workspace
for single-workspace restore via Restorer with workspaceFilter."
```

---

### Task 4: Build, live test, and polish

**Files:**
- Possibly modify: `internal/tui/pop.go`, `cmd/pop.go` (fixes found during testing)

- [ ] **Step 1: Build test binary**

Run: `cd /Users/txeo/Git/drolosoft/cmux-resurrect && go build -o /tmp/crex-test .`

- [ ] **Step 2: Test picker mode**

Run: `/tmp/crex-test pop`

Verify:
- Centered floating box with rounded border
- LAYOUTS and TEMPLATES sections visible
- Type to filter — results narrow with match highlighting
- Arrow keys move cursor
- Number keys jump to item
- Tab/→ on a layout shows its workspaces
- Esc in drill mode goes back to list
- Enter launches the selected item
- Esc from list quits cleanly

- [ ] **Step 3: Test direct modes still work**

```bash
/tmp/crex-test pop --last
/tmp/crex-test pop now
/tmp/crex-test pop ide .
```

- [ ] **Step 4: Fix any issues, run full suite**

Run: `go test ./... -count=1`
Expected: ALL PASS.

- [ ] **Step 5: Commit final polish**

```bash
git add -A
git commit -m "polish: crex pop picker — tested and refined"
```
