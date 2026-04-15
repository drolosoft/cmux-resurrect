package cmd

import (
	"bytes"
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
