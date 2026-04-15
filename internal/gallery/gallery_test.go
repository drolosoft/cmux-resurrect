package gallery

import (
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/model"
)

func TestAllTemplatesParseWithoutError(t *testing.T) {
	templates := List()
	if len(templates) != 16 {
		t.Fatalf("expected 16 templates, got %d", len(templates))
	}
}

func TestAllTemplatesHaveFrontmatter(t *testing.T) {
	for _, tmpl := range List() {
		if tmpl.Name == "" {
			t.Error("template with empty name")
		}
		if tmpl.Category == "" {
			t.Errorf("template %q has empty category", tmpl.Name)
		}
		if tmpl.Icon == "" {
			t.Errorf("template %q has empty icon", tmpl.Name)
		}
		if tmpl.Description == "" {
			t.Errorf("template %q has empty description", tmpl.Name)
		}
		if len(tmpl.Tags) == 0 {
			t.Errorf("template %q has no tags", tmpl.Name)
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
	byCategory := make(map[string]map[string]string) // category -> icon -> name
	for _, tmpl := range List() {
		if byCategory[tmpl.Category] == nil {
			byCategory[tmpl.Category] = make(map[string]string)
		}
		if existing, ok := byCategory[tmpl.Category][tmpl.Icon]; ok {
			t.Errorf("duplicate icon %q in category %q: %q and %q",
				tmpl.Icon, tmpl.Category, existing, tmpl.Name)
		}
		byCategory[tmpl.Category][tmpl.Icon] = tmpl.Name
	}
}

func TestLayoutTemplatesHaveNoCommands(t *testing.T) {
	for _, tmpl := range ListByCategory("layout") {
		for _, p := range tmpl.Panes {
			if p.Command != "" {
				t.Errorf("layout template %q pane has command %q", tmpl.Name, p.Command)
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

func TestGetClaude(t *testing.T) {
	tmpl, ok := Get("claude")
	if !ok {
		t.Fatal("Get(\"claude\") returned false")
	}
	if tmpl.Name != "claude" {
		t.Errorf("name = %q, want \"claude\"", tmpl.Name)
	}
	if tmpl.Category != "workflow" {
		t.Errorf("category = %q, want \"workflow\"", tmpl.Category)
	}
	if tmpl.Icon != "🤖" {
		t.Errorf("icon = %q, want \"🤖\"", tmpl.Icon)
	}
	if tmpl.Description != "AI pair-programming with Claude Code" {
		t.Errorf("description = %q", tmpl.Description)
	}
	if len(tmpl.Panes) != 3 {
		t.Fatalf("panes = %d, want 3", len(tmpl.Panes))
	}
	// First pane: main terminal with lazygit.
	if tmpl.Panes[0].Command != "lazygit" {
		t.Errorf("pane 0 command = %q, want \"lazygit\"", tmpl.Panes[0].Command)
	}
	// Second pane: split right with claude command, focused.
	if tmpl.Panes[1].Split != "right" {
		t.Errorf("pane 1 split = %q, want \"right\"", tmpl.Panes[1].Split)
	}
	if !tmpl.Panes[1].Focus {
		t.Error("pane 1 should be focused")
	}
	if tmpl.Panes[1].Command != "claude --dangerously-skip-permissions --continue" {
		t.Errorf("pane 1 command = %q", tmpl.Panes[1].Command)
	}
}

func TestGetNonexistent(t *testing.T) {
	_, ok := Get("nonexistent")
	if ok {
		t.Error("Get(\"nonexistent\") should return false")
	}
}

func TestQuadFocusTargets(t *testing.T) {
	tmpl, ok := Get("quad")
	if !ok {
		t.Fatal("Get(\"quad\") returned false")
	}
	if len(tmpl.Panes) != 4 {
		t.Fatalf("quad panes = %d, want 4", len(tmpl.Panes))
	}

	// Pane 0 and 1 should have FocusTarget = -1 (no refocus).
	if tmpl.Panes[0].FocusTarget != -1 {
		t.Errorf("pane 0 FocusTarget = %d, want -1", tmpl.Panes[0].FocusTarget)
	}
	if tmpl.Panes[1].FocusTarget != -1 {
		t.Errorf("pane 1 FocusTarget = %d, want -1", tmpl.Panes[1].FocusTarget)
	}
	// Pane 2: @focus=0
	if tmpl.Panes[2].FocusTarget != 0 {
		t.Errorf("pane 2 FocusTarget = %d, want 0", tmpl.Panes[2].FocusTarget)
	}
	// Pane 3: @focus=1
	if tmpl.Panes[3].FocusTarget != 1 {
		t.Errorf("pane 3 FocusTarget = %d, want 1", tmpl.Panes[3].FocusTarget)
	}
}

func TestNonQuadTemplatesHaveDefaultFocusTarget(t *testing.T) {
	for _, tmpl := range List() {
		if tmpl.Name == "quad" {
			continue
		}
		for i, p := range tmpl.Panes {
			if p.FocusTarget != -1 {
				t.Errorf("template %q pane %d has FocusTarget = %d, want -1",
					tmpl.Name, i, p.FocusTarget)
			}
		}
	}
}

func TestResolveTemplate_EmptyWorkspaceFallsToGallery(t *testing.T) {
	wf := &model.WorkspaceFile{
		Templates: make(map[string]*model.Template),
	}
	panes := ResolveTemplate(wf, "cols")
	if len(panes) != 2 {
		t.Fatalf("panes = %d, want 2 (gallery cols)", len(panes))
	}
	if panes[0].Type != "terminal" {
		t.Errorf("pane 0 type = %q, want \"terminal\"", panes[0].Type)
	}
	if panes[1].Split != "right" {
		t.Errorf("pane 1 split = %q, want \"right\"", panes[1].Split)
	}
}

func TestResolveTemplate_UserDefinedTakesPriority(t *testing.T) {
	wf := &model.WorkspaceFile{
		Templates: map[string]*model.Template{
			"cols": {
				Name: "cols",
				Panes: []model.TemplatePan{
					{Enabled: true, IsMain: true, Type: "terminal", Focus: true},
					{Enabled: true, Split: "right", Type: "terminal", Command: "user-cmd"},
				},
			},
		},
	}
	panes := ResolveTemplate(wf, "cols")
	if len(panes) != 2 {
		t.Fatalf("panes = %d, want 2", len(panes))
	}
	if panes[1].Command != "user-cmd" {
		t.Errorf("expected user-defined command, got %q", panes[1].Command)
	}
}

func TestResolveTemplate_UnknownFallsBackToSingle(t *testing.T) {
	wf := &model.WorkspaceFile{
		Templates: make(map[string]*model.Template),
	}
	panes := ResolveTemplate(wf, "totally-unknown-template")
	if len(panes) != 1 {
		t.Fatalf("panes = %d, want 1 (fallback)", len(panes))
	}
	if panes[0].Type != "terminal" || !panes[0].Focus {
		t.Error("fallback should be single focused terminal")
	}
}

func TestResolveTemplate_NilWorkspaceFile(t *testing.T) {
	panes := ResolveTemplate(nil, "cols")
	if len(panes) != 2 {
		t.Fatalf("panes = %d, want 2 (gallery cols)", len(panes))
	}
}

func TestBuildPanes_EmptyTemplate(t *testing.T) {
	tmpl := &model.Template{Name: "empty"}
	panes := BuildPanes(tmpl)
	if len(panes) != 1 {
		t.Fatalf("panes = %d, want 1 (fallback)", len(panes))
	}
	if !panes[0].Focus {
		t.Error("fallback pane should be focused")
	}
}

func TestBuildPanes_AllDisabled(t *testing.T) {
	tmpl := &model.Template{
		Name: "disabled",
		Panes: []model.TemplatePan{
			{Enabled: false, IsMain: true, Type: "terminal"},
		},
	}
	panes := BuildPanes(tmpl)
	if len(panes) != 1 {
		t.Fatalf("panes = %d, want 1 (fallback)", len(panes))
	}
}

func TestBuildPanes_Claude(t *testing.T) {
	tmpl, ok := Get("claude")
	if !ok {
		t.Fatal("gallery missing claude template")
	}
	panes := BuildPanes(tmpl)
	if len(panes) != 3 {
		t.Fatalf("panes = %d, want 3", len(panes))
	}
	if panes[0].Command != "lazygit" {
		t.Errorf("pane 0 command = %q", panes[0].Command)
	}
	if panes[1].Split != "right" || !panes[1].Focus {
		t.Errorf("pane 1: split=%q focus=%v", panes[1].Split, panes[1].Focus)
	}
	if panes[2].Split != "down" {
		t.Errorf("pane 2 split = %q, want \"down\"", panes[2].Split)
	}
}

func TestParseGalleryPaneLine_FocusTarget(t *testing.T) {
	tp := parseGalleryPaneLine("- [x] split down: @focus=0")
	if tp.FocusTarget != 0 {
		t.Errorf("FocusTarget = %d, want 0", tp.FocusTarget)
	}
	if tp.Split != "down" {
		t.Errorf("Split = %q, want \"down\"", tp.Split)
	}
}

func TestParseGalleryPaneLine_NoFocusTarget(t *testing.T) {
	tp := parseGalleryPaneLine("- [x] main terminal (focused)")
	if tp.FocusTarget != -1 {
		t.Errorf("FocusTarget = %d, want -1", tp.FocusTarget)
	}
	if !tp.Focus {
		t.Error("expected Focus = true")
	}
	if !tp.IsMain {
		t.Error("expected IsMain = true")
	}
}

func TestParseGalleryPaneLine_CommandWithFocus(t *testing.T) {
	tp := parseGalleryPaneLine("- [x] split right: `lazygit` (focused)")
	if tp.Command != "lazygit" {
		t.Errorf("Command = %q, want \"lazygit\"", tp.Command)
	}
	if !tp.Focus {
		t.Error("expected Focus = true")
	}
	if tp.Split != "right" {
		t.Errorf("Split = %q, want \"right\"", tp.Split)
	}
}

func TestParseTemplateFile_MissingFrontmatter(t *testing.T) {
	_, err := parseTemplateFile("no frontmatter here")
	if err == nil {
		t.Error("expected error for missing frontmatter")
	}
}

func TestParseTemplateFile_MissingName(t *testing.T) {
	content := "---\ncategory: layout\n---\n### test\n- [x] main terminal\n"
	_, err := parseTemplateFile(content)
	if err == nil {
		t.Error("expected error for missing name")
	}
}

func TestListByCategory_Empty(t *testing.T) {
	result := ListByCategory("nonexistent-category")
	if len(result) != 0 {
		t.Errorf("expected 0 templates for nonexistent category, got %d", len(result))
	}
}

func TestIDETemplate(t *testing.T) {
	tmpl, ok := Get("ide")
	if !ok {
		t.Fatal("Get(\"ide\") returned false")
	}
	if len(tmpl.Panes) != 4 {
		t.Fatalf("ide panes = %d, want 4", len(tmpl.Panes))
	}
	// First pane: main terminal, not focused.
	if !tmpl.Panes[0].IsMain {
		t.Error("pane 0 should be main")
	}
	if tmpl.Panes[0].Focus {
		t.Error("pane 0 should not be focused in ide layout")
	}
	// Second pane: split right, focused.
	if tmpl.Panes[1].Split != "right" {
		t.Errorf("pane 1 split = %q, want \"right\"", tmpl.Panes[1].Split)
	}
	if !tmpl.Panes[1].Focus {
		t.Error("pane 1 should be focused in ide layout")
	}
}

func TestSingleTemplate(t *testing.T) {
	tmpl, ok := Get("single")
	if !ok {
		t.Fatal("Get(\"single\") returned false")
	}
	if len(tmpl.Panes) != 1 {
		t.Fatalf("single panes = %d, want 1", len(tmpl.Panes))
	}
	if !tmpl.Panes[0].IsMain || !tmpl.Panes[0].Focus {
		t.Error("single pane should be main and focused")
	}
}

func TestAllTemplatesHavePanes(t *testing.T) {
	for _, tmpl := range List() {
		if len(tmpl.Panes) == 0 {
			t.Errorf("template %q has no panes", tmpl.Name)
		}
	}
}

func TestCategoryValues(t *testing.T) {
	for _, tmpl := range List() {
		if tmpl.Category != "layout" && tmpl.Category != "workflow" {
			t.Errorf("template %q has invalid category %q", tmpl.Name, tmpl.Category)
		}
	}
}
