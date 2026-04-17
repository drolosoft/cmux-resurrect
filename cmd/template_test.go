package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/gallery"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// executeTemplateCmd runs rootCmd with the given args and captures output.
func executeTemplateCmd(t *testing.T, args ...string) string {
	t.Helper()
	setupTestConfig(t)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	// Reset flag defaults to avoid cross-test pollution.
	tplListLayout = false
	tplListWorkflow = false
	tplListTag = ""
	tplShowAll = false
	tplUseName = ""
	tplUseIcon = ""
	tplUseDryRun = false
	tplUsePin = false

	rootCmd.SetArgs(args)
	err := rootCmd.Execute()

	// Clean up before checking error.
	rootCmd.SetArgs(nil)
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)

	if err != nil {
		t.Fatalf("execute %v failed: %v", args, err)
	}
	return buf.String()
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestTemplateList_ShowsAllTemplates(t *testing.T) {
	output := executeTemplateCmd(t, "template", "list")

	// Category headers must be present.
	if !strings.Contains(output, "LAYOUTS") {
		t.Error("output missing LAYOUTS header")
	}
	if !strings.Contains(output, "WORKFLOWS") {
		t.Error("output missing WORKFLOWS header")
	}

	// Template names that must appear.
	for _, name := range []string{"cols", "rows", "sidebar", "quad", "ide", "claude", "code", "single"} {
		if !strings.Contains(output, name) {
			t.Errorf("output missing template name %q", name)
		}
	}

	// Icons that must appear.
	for _, icon := range []string{"▥", "▤", "◧", "⊠", "⧉", "🤖", "💻", "📟"} {
		if !strings.Contains(output, icon) {
			t.Errorf("output missing icon %q", icon)
		}
	}

	// Summary line.
	if !strings.Contains(output, "16 templates") {
		t.Errorf("output missing '16 templates' summary; output:\n%s", output)
	}
}

func TestTemplateList_LayoutFilter(t *testing.T) {
	output := executeTemplateCmd(t, "template", "list", "--layout")

	if !strings.Contains(output, "cols") {
		t.Error("--layout output should contain 'cols'")
	}
	if strings.Contains(output, "claude") {
		t.Error("--layout output should NOT contain 'claude'")
	}
}

func TestTemplateList_WorkflowFilter(t *testing.T) {
	output := executeTemplateCmd(t, "template", "list", "--workflow")

	if !strings.Contains(output, "claude") {
		t.Error("--workflow output should contain 'claude'")
	}
	if strings.Contains(output, "cols") {
		t.Error("--workflow output should NOT contain 'cols'")
	}
}

func TestTemplateList_TagFilter(t *testing.T) {
	output := executeTemplateCmd(t, "template", "list", "--tag", "ai")

	// "claude" has tag "ai".
	if !strings.Contains(output, "claude") {
		t.Error("--tag ai should include 'claude'")
	}
	// "cols" has tags "basic, 2-pane" — should not appear.
	if strings.Contains(output, "cols") {
		t.Error("--tag ai should NOT include 'cols'")
	}
}

func TestTemplateList_TplAlias(t *testing.T) {
	output := executeTemplateCmd(t, "tpl", "list")

	if !strings.Contains(output, "LAYOUTS") {
		t.Error("tpl alias: output missing LAYOUTS header")
	}
	if !strings.Contains(output, "16 templates") {
		t.Errorf("tpl alias: output missing '16 templates' summary")
	}
}

func TestTemplateList_PaneCountsInBrackets(t *testing.T) {
	output := executeTemplateCmd(t, "template", "list")

	// cols has 2 panes, claude has 3 panes.
	if !strings.Contains(output, "[2]") {
		t.Error("output should contain pane count '[2]'")
	}
	if !strings.Contains(output, "[3]") {
		t.Error("output should contain pane count '[3]'")
	}
}

// executeTemplateCmdErr runs rootCmd with the given args and returns (output, error).
func executeTemplateCmdErr(t *testing.T, args ...string) (string, error) {
	t.Helper()
	setupTestConfig(t)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	tplListLayout = false
	tplListWorkflow = false
	tplListTag = ""
	tplShowAll = false
	tplUseName = ""
	tplUseIcon = ""
	tplUseDryRun = false
	tplUsePin = false

	rootCmd.SetArgs(args)
	err := rootCmd.Execute()

	rootCmd.SetArgs(nil)
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)

	return buf.String(), err
}

func TestTemplateList_CombinedFilters(t *testing.T) {
	// --layout + --tag basic should only show layout templates with "basic" tag.
	output := executeTemplateCmd(t, "template", "list", "--layout", "--tag", "basic")

	if !strings.Contains(output, "cols") {
		t.Error("--layout --tag basic should include 'cols'")
	}
	// claude is a workflow, should not appear even if it somehow had "basic" tag.
	if strings.Contains(output, "claude") {
		t.Error("--layout --tag basic should NOT include 'claude'")
	}
}

// ---------------------------------------------------------------------------
// template show tests
// ---------------------------------------------------------------------------

func TestTemplateShow_ExistingTemplate(t *testing.T) {
	output := executeTemplateCmd(t, "template", "show", "claude")

	// Must contain template name and icon.
	if !strings.Contains(output, "claude") {
		t.Error("output missing template name 'claude'")
	}
	if !strings.Contains(output, "🤖") {
		t.Error("output missing icon '🤖'")
	}

	// Must contain category.
	if !strings.Contains(output, "workflow") {
		t.Error("output missing category 'workflow'")
	}

	// Must contain box-drawing characters.
	if !strings.Contains(output, "┌") {
		t.Error("output missing box-drawing character '┌'")
	}
	if !strings.Contains(output, "┘") {
		t.Error("output missing box-drawing character '┘'")
	}
}

func TestTemplateShow_NonExistent(t *testing.T) {
	_, err := executeTemplateCmdErr(t, "template", "show", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent template, got nil")
	}
}

func TestTemplateShow_AllTemplatesRender(t *testing.T) {
	templates := gallery.List()
	if len(templates) == 0 {
		t.Fatal("gallery.List() returned 0 templates")
	}

	for _, tmpl := range templates {
		t.Run(tmpl.Name, func(t *testing.T) {
			output := executeTemplateCmd(t, "template", "show", tmpl.Name)

			if !strings.Contains(output, tmpl.Name) {
				t.Errorf("output missing template name %q", tmpl.Name)
			}
			if !strings.Contains(output, tmpl.Icon) {
				t.Errorf("output missing icon %q", tmpl.Icon)
			}
			// Must contain box-drawing characters.
			if !strings.Contains(output, "┌") {
				t.Error("output missing box-drawing character '┌'")
			}
			if !strings.Contains(output, "┘") {
				t.Error("output missing box-drawing character '┘'")
			}
		})
	}
}

func TestTemplateShow_ContainsPaneCount(t *testing.T) {
	output := executeTemplateCmd(t, "template", "show", "claude")
	if !strings.Contains(output, "Panes:") {
		t.Error("output missing 'Panes:' label")
	}
	if !strings.Contains(output, "3") {
		t.Error("output missing pane count '3' for claude template")
	}
}

func TestTemplateShow_ContainsSplitSequence(t *testing.T) {
	output := executeTemplateCmd(t, "template", "show", "claude")
	if !strings.Contains(output, "Splits:") {
		t.Error("output missing 'Splits:' label")
	}
}

func TestTemplateShow_ContainsTags(t *testing.T) {
	output := executeTemplateCmd(t, "template", "show", "claude")
	if !strings.Contains(output, "Tags:") {
		t.Error("output missing 'Tags:' label")
	}
	if !strings.Contains(output, "ai") {
		t.Error("output missing tag 'ai' for claude template")
	}
}

func TestTemplateShow_DiagramHasPaneLabels(t *testing.T) {
	output := executeTemplateCmd(t, "template", "show", "claude")
	// The claude template has lazygit and claude commands.
	if !strings.Contains(output, "lazygit") {
		t.Error("diagram missing pane label 'lazygit'")
	}
}

func TestTemplateShow_FocusedPaneMarked(t *testing.T) {
	output := executeTemplateCmd(t, "template", "show", "claude")
	// The focused pane should have a * marker.
	if !strings.Contains(output, "*") {
		t.Error("diagram missing focused pane marker '*'")
	}
}

// ---------------------------------------------------------------------------
// template use tests
// ---------------------------------------------------------------------------

func TestTemplateUse_DryRun(t *testing.T) {
	// Run "template use cols /tmp --dry-run"
	// Should succeed (dry-run doesn't need cmux)
	output := executeTemplateCmd(t, "template", "use", "cols", "/tmp", "--dry-run")

	// Output should contain cmux commands.
	if !strings.Contains(output, "new-workspace") {
		t.Error("dry-run output missing 'new-workspace'")
	}
	if !strings.Contains(output, "rename-workspace") {
		t.Error("dry-run output missing 'rename-workspace'")
	}
}

func TestTemplateUse_NonExistentTemplate(t *testing.T) {
	_, err := executeTemplateCmdErr(t, "template", "use", "nonexistent", "/tmp", "--dry-run")
	if err == nil {
		t.Error("expected error for nonexistent template, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestTemplateUse_DryRunShowsCommands(t *testing.T) {
	// Run "template use claude /tmp/test --dry-run"
	output := executeTemplateCmd(t, "template", "use", "claude", "/tmp/test", "--dry-run")

	// Output should contain cmux commands for a 3-pane template.
	if !strings.Contains(output, "new-workspace") {
		t.Error("dry-run output missing 'new-workspace'")
	}
	if !strings.Contains(output, "new-split") {
		t.Error("dry-run output missing 'new-split'")
	}
	if !strings.Contains(output, "rename-workspace") {
		t.Error("dry-run output missing 'rename-workspace'")
	}
}

func TestTemplateUse_DryRunDefaultCWD(t *testing.T) {
	// When no path argument is given, CWD defaults to "." (resolved to absolute).
	output := executeTemplateCmd(t, "template", "use", "cols", "--dry-run")

	if !strings.Contains(output, "new-workspace") {
		t.Error("dry-run output missing 'new-workspace'")
	}
}

func TestTemplateUse_DryRunWithName(t *testing.T) {
	output := executeTemplateCmd(t, "template", "use", "cols", "/tmp", "--dry-run", "--name", "my-ws")

	if !strings.Contains(output, "my-ws") {
		t.Error("dry-run output missing custom name 'my-ws'")
	}
}

func TestTemplateUse_DryRunWithPin(t *testing.T) {
	output := executeTemplateCmd(t, "template", "use", "cols", "/tmp", "--dry-run", "--pin")

	if !strings.Contains(output, "workspace-action") && !strings.Contains(output, "pin") {
		t.Error("dry-run output missing pin command")
	}
}

func TestTemplateUse_TplAlias(t *testing.T) {
	// The tpl alias should also work for use subcommand.
	output := executeTemplateCmd(t, "tpl", "use", "cols", "/tmp", "--dry-run")

	if !strings.Contains(output, "new-workspace") {
		t.Error("tpl alias: dry-run output missing 'new-workspace'")
	}
}

// ---------------------------------------------------------------------------
// template customize tests
// ---------------------------------------------------------------------------

func TestTemplateCustomize_CopiesToBlueprint(t *testing.T) {
	_, wsFile := setupTestConfig(t)

	// Create a minimal workspace file so Parse works.
	content := "## Projects\n**Icon | Name | Template | Pin | Path**\n\n## Templates\n\n### dev\n- [x] main (focused)\n"
	if err := os.WriteFile(wsFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write workspace file: %v", err)
	}

	output := executeTemplateCmd(t, "--workspace-file", wsFile, "template", "customize", "claude")

	// Verify success message.
	if !strings.Contains(output, "claude") {
		t.Error("output missing template name 'claude'")
	}
	if !strings.Contains(output, "Copied") {
		t.Error("output missing 'Copied' success text")
	}

	// Read the file and verify it contains "### claude".
	data, err := os.ReadFile(wsFile)
	if err != nil {
		t.Fatalf("read workspace file: %v", err)
	}
	if !strings.Contains(string(data), "### claude") {
		t.Errorf("workspace file missing '### claude'; contents:\n%s", string(data))
	}

	// Verify panes were copied (lazygit command from claude template).
	if !strings.Contains(string(data), "lazygit") {
		t.Errorf("workspace file missing 'lazygit' pane command; contents:\n%s", string(data))
	}
}

func TestTemplateCustomize_CreatesFileIfMissing(t *testing.T) {
	_, wsFile := setupTestConfig(t)

	// Don't create wsFile — it should be created automatically.
	output := executeTemplateCmd(t, "--workspace-file", wsFile, "template", "customize", "claude")

	if !strings.Contains(output, "Copied") {
		t.Error("output missing 'Copied' success text")
	}

	data, err := os.ReadFile(wsFile)
	if err != nil {
		t.Fatalf("read workspace file: %v", err)
	}
	if !strings.Contains(string(data), "### claude") {
		t.Errorf("workspace file missing '### claude'; contents:\n%s", string(data))
	}
}

func TestTemplateCustomize_NonExistent(t *testing.T) {
	_, wsFile := setupTestConfig(t)

	_, err := executeTemplateCmdErr(t, "--workspace-file", wsFile, "template", "customize", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent template, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestTemplateCustomize_AlreadyExists(t *testing.T) {
	_, wsFile := setupTestConfig(t)

	// Create a workspace file that already has "### claude".
	content := "## Projects\n**Icon | Name | Template | Pin | Path**\n\n## Templates\n\n### claude\n- [x] main (focused)\n"
	if err := os.WriteFile(wsFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write workspace file: %v", err)
	}

	_, err := executeTemplateCmdErr(t, "--workspace-file", wsFile, "template", "customize", "claude")
	if err == nil {
		t.Error("expected error for already existing template, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' in error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// template show --all tests
// ---------------------------------------------------------------------------

func TestTemplateShow_All(t *testing.T) {
	output := executeTemplateCmd(t, "template", "show", "--all")

	// Must contain both category headers.
	if !strings.Contains(output, "Layouts") {
		t.Error("--all output missing 'Layouts' header")
	}
	if !strings.Contains(output, "Workflows") {
		t.Error("--all output missing 'Workflows' header")
	}

	// Must contain diagrams (box-drawing chars) for multiple templates.
	count := strings.Count(output, "┌")
	if count < 10 {
		t.Errorf("expected at least 10 diagrams (one per template), got %d", count)
	}

	// Must contain all template names.
	for _, name := range []string{"cols", "rows", "sidebar", "quad", "ide", "claude", "code", "single"} {
		if !strings.Contains(output, name) {
			t.Errorf("--all output missing template name %q", name)
		}
	}

	// Summary line.
	if !strings.Contains(output, "16 templates") {
		t.Errorf("--all output missing '16 templates' summary; output:\n%s", output)
	}
}

func TestTemplateShow_NoArgsNoFlag(t *testing.T) {
	_, err := executeTemplateCmdErr(t, "template", "show")
	if err == nil {
		t.Error("expected error for 'show' with no args and no --all")
	}
}

// ---------------------------------------------------------------------------
// template (bare command) styled help tests
// ---------------------------------------------------------------------------

func TestTemplateBareCommand_ShowsStyledHelp(t *testing.T) {
	output := executeTemplateCmd(t, "template")

	// Must show the gallery header.
	if !strings.Contains(output, "Template Gallery") {
		t.Error("bare 'template' output missing 'Template Gallery' header")
	}

	// Must list subcommands.
	for _, sub := range []string{"list", "show", "use", "customize"} {
		if !strings.Contains(output, sub) {
			t.Errorf("bare 'template' output missing subcommand %q", sub)
		}
	}

	// Must show template names (gallery preview).
	if !strings.Contains(output, "cols") {
		t.Error("bare 'template' output missing template name 'cols'")
	}
	if !strings.Contains(output, "claude") {
		t.Error("bare 'template' output missing template name 'claude'")
	}

	// Must show examples section.
	if !strings.Contains(output, "Examples") {
		t.Error("bare 'template' output missing 'Examples' section")
	}
}

// ---------------------------------------------------------------------------
// template customize tests (continued)
// ---------------------------------------------------------------------------

func TestTemplateCustomize_StripsFocusTarget(t *testing.T) {
	_, wsFile := setupTestConfig(t)

	// Create minimal workspace file.
	content := "## Projects\n**Icon | Name | Template | Pin | Path**\n\n## Templates\n\n### dev\n- [x] main (focused)\n"
	if err := os.WriteFile(wsFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write workspace file: %v", err)
	}

	executeTemplateCmd(t, "--workspace-file", wsFile, "template", "customize", "claude")

	data, err := os.ReadFile(wsFile)
	if err != nil {
		t.Fatalf("read workspace file: %v", err)
	}

	// The output should NOT contain @focus= since FocusTarget is stripped.
	if strings.Contains(string(data), "@focus=") {
		t.Errorf("workspace file should not contain '@focus=' after customize; contents:\n%s", string(data))
	}
}
