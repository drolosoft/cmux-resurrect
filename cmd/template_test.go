package cmd

import (
	"bytes"
	"strings"
	"testing"
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
