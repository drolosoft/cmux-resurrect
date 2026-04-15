package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/config"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// setupTestConfig sets the global cfg to use temp directories for testing.
func setupTestConfig(t *testing.T) (layoutsDir string, wsFile string) {
	t.Helper()
	dir := t.TempDir()
	layoutsDir = filepath.Join(dir, "layouts")
	if err := os.MkdirAll(layoutsDir, 0o755); err != nil {
		t.Fatalf("mkdir layouts: %v", err)
	}
	wsFile = filepath.Join(dir, "workspaces.md")
	cfg = &config.Config{
		LayoutsDir:    layoutsDir,
		WorkspaceFile: wsFile,
	}
	return layoutsDir, wsFile
}

// saveTestLayout creates a layout fixture in the given store directory.
func saveTestLayout(t *testing.T, dir, name, description string, wsCount int) {
	t.Helper()
	store, err := persist.NewFileStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	workspaces := make([]model.Workspace, wsCount)
	for i := range workspaces {
		workspaces[i] = model.Workspace{
			Title: "ws",
			CWD:   "/tmp",
			Panes: []model.Pane{{Type: "terminal"}},
		}
	}
	layout := &model.Layout{
		Name:        name,
		Description: description,
		Version:     1,
		SavedAt:     time.Now().UTC(),
		Workspaces:  workspaces,
	}
	if err := store.Save(name, layout); err != nil {
		t.Fatalf("save layout %s: %v", name, err)
	}
}

// writeTestBlueprint creates a workspace blueprint file with the given projects.
func writeTestBlueprint(t *testing.T, path string, projects []string) {
	t.Helper()
	var lines []string
	lines = append(lines, "## Projects")
	lines = append(lines, "**Icon | Name | Template | Pin | Path**")
	lines = append(lines, "")
	for _, name := range projects {
		lines = append(lines, "- [x] | 📁 | "+name+" | dev | yes | /tmp/"+name+" |")
	}
	lines = append(lines, "")
	lines = append(lines, "## Templates")
	lines = append(lines, "")
	lines = append(lines, "### dev")
	lines = append(lines, "- [x] main (focused)")
	lines = append(lines, "- [x] right `npm run dev`")
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}
}

// executeComplete runs rootCmd with __complete args and returns the stdout output.
// This tests the full Cobra completion pipeline end-to-end.
//
// To ensure initConfig() uses test fixtures instead of ~/.config/crex/, we set
// the package-level layoutsDir and workspaceFile variables that initConfig reads.
func executeComplete(t *testing.T, args ...string) string {
	t.Helper()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer)) // discard stderr

	// Set globals so initConfig() picks up our test paths.
	savedLayoutsDir := layoutsDir
	savedWorkspaceFile := workspaceFile
	layoutsDir = cfg.LayoutsDir
	workspaceFile = cfg.WorkspaceFile
	t.Cleanup(func() {
		layoutsDir = savedLayoutsDir
		workspaceFile = savedWorkspaceFile
	})

	rootCmd.SetArgs(append([]string{"__complete"}, args...))
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("__complete %v failed: %v", args, err)
	}
	return buf.String()
}

// completionLines parses __complete output into name lines (excluding the
// directive line that starts with ":").
func completionLines(output string) []string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line != "" && !strings.HasPrefix(line, ":") {
			lines = append(lines, line)
		}
	}
	return lines
}

// completionNames extracts just the names (before \t) from __complete output.
func completionNames(output string) []string {
	var names []string
	for _, line := range completionLines(output) {
		name := strings.SplitN(line, "\t", 2)[0]
		names = append(names, name)
	}
	return names
}

// completionDirective extracts the ShellCompDirective from __complete output.
func completionDirective(output string) int {
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if strings.HasPrefix(line, ":") {
			var d int
			if _, err := fmt.Sscanf(line, ":%d", &d); err != nil {
				continue
			}
			return d
		}
	}
	return -1
}

// assertContains checks that needle appears in the list of names.
func assertContains(t *testing.T, names []string, needle string) {
	t.Helper()
	for _, n := range names {
		if n == needle {
			return
		}
	}
	t.Errorf("expected %q in %v", needle, names)
}

// assertNotContains checks that needle does NOT appear in the list of names.
func assertNotContains(t *testing.T, names []string, needle string) {
	t.Helper()
	for _, n := range names {
		if n == needle {
			t.Errorf("did NOT expect %q in %v", needle, names)
			return
		}
	}
}

// ---------------------------------------------------------------------------
// 1. Unit tests: completeLayoutNames
// ---------------------------------------------------------------------------

func TestCompleteLayoutNames_EmptyStore(t *testing.T) {
	layoutsDir, _ := setupTestConfig(t)
	_ = layoutsDir

	completions, directive := completeLayoutNames(nil, nil, "")
	if len(completions) != 0 {
		t.Errorf("expected 0 completions, got %d: %v", len(completions), completions)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %d, want ShellCompDirectiveNoFileComp (%d)", directive, cobra.ShellCompDirectiveNoFileComp)
	}
}

func TestCompleteLayoutNames_WithLayouts(t *testing.T) {
	layoutsDir, _ := setupTestConfig(t)
	saveTestLayout(t, layoutsDir, "my-day", "Friday standup layout", 3)
	saveTestLayout(t, layoutsDir, "production", "", 5)

	completions, directive := completeLayoutNames(nil, nil, "")
	if len(completions) != 2 {
		t.Fatalf("expected 2 completions, got %d: %v", len(completions), completions)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %d, want ShellCompDirectiveNoFileComp", directive)
	}

	// Should be sorted alphabetically (store.List sorts by name).
	if !strings.HasPrefix(completions[0], "my-day\t") {
		t.Errorf("completions[0] = %q, want prefix 'my-day\\t'", completions[0])
	}
	if !strings.HasPrefix(completions[1], "production\t") {
		t.Errorf("completions[1] = %q, want prefix 'production\\t'", completions[1])
	}

	// my-day has a description, production falls back to workspace count.
	if !strings.Contains(completions[0], "Friday standup layout") {
		t.Errorf("completions[0] = %q, expected description", completions[0])
	}
	if !strings.Contains(completions[1], "5 workspaces") {
		t.Errorf("completions[1] = %q, expected workspace count fallback", completions[1])
	}
}

func TestCompleteLayoutNames_SecondArgBlocked(t *testing.T) {
	layoutsDir, _ := setupTestConfig(t)
	saveTestLayout(t, layoutsDir, "my-day", "", 2)

	completions, directive := completeLayoutNames(nil, []string{"my-day"}, "")
	if len(completions) != 0 {
		t.Errorf("expected 0 completions for second arg, got %d", len(completions))
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %d, want ShellCompDirectiveNoFileComp", directive)
	}
}

func TestCompleteLayoutNames_StoreError(t *testing.T) {
	cfg = &config.Config{
		LayoutsDir:    "/dev/null/impossible/path",
		WorkspaceFile: "/dev/null/impossible.md",
	}

	completions, directive := completeLayoutNames(nil, nil, "")
	if len(completions) != 0 {
		t.Errorf("expected 0 completions on store error, got %d", len(completions))
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %d, want ShellCompDirectiveNoFileComp", directive)
	}
}

// ---------------------------------------------------------------------------
// 2. Unit tests: completeWorkspaceNames
// ---------------------------------------------------------------------------

func TestCompleteWorkspaceNames_MissingFile(t *testing.T) {
	_, wsFile := setupTestConfig(t)
	_ = wsFile

	completions, directive := completeWorkspaceNames(nil, nil, "")
	if len(completions) != 0 {
		t.Errorf("expected 0 completions for missing blueprint, got %d", len(completions))
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %d, want ShellCompDirectiveNoFileComp", directive)
	}
}

func TestCompleteWorkspaceNames_WithProjects(t *testing.T) {
	_, wsFile := setupTestConfig(t)
	writeTestBlueprint(t, wsFile, []string{"api-server", "webapp", "notes"})

	completions, directive := completeWorkspaceNames(nil, nil, "")
	if len(completions) != 3 {
		t.Fatalf("expected 3 completions, got %d: %v", len(completions), completions)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %d, want ShellCompDirectiveNoFileComp", directive)
	}

	for _, c := range completions {
		parts := strings.SplitN(c, "\t", 2)
		if len(parts) != 2 {
			t.Errorf("completion %q missing tab-separated description", c)
			continue
		}
		name := parts[0]
		if name != "api-server" && name != "webapp" && name != "notes" {
			t.Errorf("unexpected completion name: %q", name)
		}
	}
}

func TestCompleteWorkspaceNames_SecondArgBlocked(t *testing.T) {
	_, wsFile := setupTestConfig(t)
	writeTestBlueprint(t, wsFile, []string{"api-server"})

	completions, directive := completeWorkspaceNames(nil, []string{"api-server"}, "")
	if len(completions) != 0 {
		t.Errorf("expected 0 completions for second arg, got %d", len(completions))
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %d, want ShellCompDirectiveNoFileComp", directive)
	}
}

// ---------------------------------------------------------------------------
// 3. Completion command output tests
// ---------------------------------------------------------------------------

func TestCompletionCommand_GeneratesOutput(t *testing.T) {
	shells := []string{"bash", "zsh", "fish", "powershell"}
	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("pipe: %v", err)
			}
			oldStdout := os.Stdout
			os.Stdout = w

			err = runCompletion(nil, []string{shell})

			os.Stdout = oldStdout
			_ = w.Close()

			if err != nil {
				t.Fatalf("runCompletion(%s) error: %v", shell, err)
			}

			buf := make([]byte, 4096)
			n, _ := r.Read(buf)
			_ = r.Close()

			if n == 0 {
				t.Errorf("runCompletion(%s) produced no output", shell)
			}
		})
	}
}

func TestCompletionCommand_ValidArgs(t *testing.T) {
	if len(completionCmd.ValidArgs) != 4 {
		t.Fatalf("expected 4 valid args, got %d: %v", len(completionCmd.ValidArgs), completionCmd.ValidArgs)
	}
	expected := map[string]bool{"bash": true, "zsh": true, "fish": true, "powershell": true}
	for _, arg := range completionCmd.ValidArgs {
		if !expected[arg] {
			t.Errorf("unexpected valid arg: %q", arg)
		}
	}
}

// ---------------------------------------------------------------------------
// 4. ValidArgsFunction wiring verification
//    Tests that every command that SHOULD have a ValidArgsFunction actually has
//    one set. This catches accidental removal during refactoring.
// ---------------------------------------------------------------------------

func TestWiring_LayoutCommandsHaveValidArgsFunction(t *testing.T) {
	cmds := map[string]*cobra.Command{
		"save":    saveCmd,
		"restore": restoreCmd,
		"delete":  deleteCmd,
		"show":    showCmd,
		"edit":    editCmd,
		"watch":   watchCmd,
	}
	for name, cmd := range cmds {
		t.Run(name, func(t *testing.T) {
			if cmd.ValidArgsFunction == nil {
				t.Errorf("%s command is missing ValidArgsFunction", name)
			}
		})
	}
}

func TestWiring_WorkspaceCommandsHaveValidArgsFunction(t *testing.T) {
	cmds := map[string]*cobra.Command{
		"ws remove": wsRemoveCmd,
		"ws toggle": wsToggleCmd,
		"ws add":    wsAddCmd,
	}
	for name, cmd := range cmds {
		t.Run(name, func(t *testing.T) {
			if cmd.ValidArgsFunction == nil {
				t.Errorf("%s command is missing ValidArgsFunction", name)
			}
		})
	}
}

func TestWiring_CommandsWithoutArgsDontNeedCompletion(t *testing.T) {
	// These commands take no positional args — they should NOT have
	// ValidArgsFunction (Cobra's default handles them correctly).
	cmds := map[string]*cobra.Command{
		"list":           listCmd,
		"version":        versionCmd,
		"export-to-md":   exportToMDCmd,
		"import-from-md": importFromMDCmd,
		"ws list":        wsListCmd,
	}
	for name, cmd := range cmds {
		t.Run(name, func(t *testing.T) {
			if cmd.ValidArgsFunction != nil {
				t.Errorf("%s command has ValidArgsFunction but takes no positional args", name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 5. End-to-end __complete pipeline tests
//    These test the FULL Cobra completion pipeline by executing rootCmd with
//    __complete args, verifying the actual output a user's shell would see.
// ---------------------------------------------------------------------------

func TestE2E_SubcommandCompletion_AllCommands(t *testing.T) {
	setupTestConfig(t)
	output := executeComplete(t, "")
	names := completionNames(output)

	// Must contain all user-facing commands.
	for _, want := range []string{"save", "restore", "list", "show", "edit",
		"delete", "watch", "export-to-md", "import-from-md", "workspace",
		"version", "completion"} {
		assertContains(t, names, want)
	}
	// Must NOT contain hidden commands.
	assertNotContains(t, names, "__complete")
	assertNotContains(t, names, "__completeNoDesc")
}

func TestE2E_SubcommandPartialMatch(t *testing.T) {
	setupTestConfig(t)

	// "res" → "restore"
	output := executeComplete(t, "res")
	names := completionNames(output)
	assertContains(t, names, "restore")
	// Should NOT return unrelated commands.
	assertNotContains(t, names, "save")
	assertNotContains(t, names, "list")

	// "de" → "delete" (not "version")
	output = executeComplete(t, "de")
	names = completionNames(output)
	assertContains(t, names, "delete")
	assertNotContains(t, names, "version")

	// "com" → "completion"
	output = executeComplete(t, "com")
	names = completionNames(output)
	assertContains(t, names, "completion")

	// "w" → "watch", "workspace" (both start with w)
	output = executeComplete(t, "w")
	names = completionNames(output)
	assertContains(t, names, "watch")
	assertContains(t, names, "workspace")
}

func TestE2E_RestoreLayoutNames(t *testing.T) {
	layoutsDir, _ := setupTestConfig(t)
	saveTestLayout(t, layoutsDir, "my-day", "Friday standup", 3)
	saveTestLayout(t, layoutsDir, "prod", "", 5)

	output := executeComplete(t, "restore", "")
	names := completionNames(output)
	assertContains(t, names, "my-day")
	assertContains(t, names, "prod")

	// With description.
	lines := completionLines(output)
	found := false
	for _, line := range lines {
		if strings.HasPrefix(line, "my-day\t") && strings.Contains(line, "Friday standup") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'my-day' with description 'Friday standup' in: %v", lines)
	}

	// Directive should be NoFileComp (4).
	if d := completionDirective(output); d != 4 {
		t.Errorf("directive = %d, want 4 (NoFileComp)", d)
	}
}

func TestE2E_RestoreLayoutNames_PartialMatch(t *testing.T) {
	layoutsDir, _ := setupTestConfig(t)
	saveTestLayout(t, layoutsDir, "my-day", "", 3)
	saveTestLayout(t, layoutsDir, "my-work", "", 2)
	saveTestLayout(t, layoutsDir, "production", "", 5)

	// Cobra's __complete returns ALL completions — the shell does prefix
	// filtering. Verify all three layouts are returned.
	output := executeComplete(t, "restore", "my-")
	names := completionNames(output)
	assertContains(t, names, "my-day")
	assertContains(t, names, "my-work")
	// "production" is also returned by __complete; the shell filters it.
	assertContains(t, names, "production")
	if len(names) != 3 {
		t.Errorf("expected 3 completions, got %d: %v", len(names), names)
	}
}

func TestE2E_DeleteLayoutNames(t *testing.T) {
	layoutsDir, _ := setupTestConfig(t)
	saveTestLayout(t, layoutsDir, "alpha", "", 1)
	saveTestLayout(t, layoutsDir, "beta", "", 2)

	output := executeComplete(t, "delete", "")
	names := completionNames(output)
	assertContains(t, names, "alpha")
	assertContains(t, names, "beta")
}

func TestE2E_ShowLayoutNames(t *testing.T) {
	layoutsDir, _ := setupTestConfig(t)
	saveTestLayout(t, layoutsDir, "alpha", "", 1)

	output := executeComplete(t, "show", "")
	names := completionNames(output)
	assertContains(t, names, "alpha")
}

func TestE2E_EditLayoutNames(t *testing.T) {
	layoutsDir, _ := setupTestConfig(t)
	saveTestLayout(t, layoutsDir, "alpha", "", 1)

	output := executeComplete(t, "edit", "")
	names := completionNames(output)
	assertContains(t, names, "alpha")
}

func TestE2E_SaveLayoutNames(t *testing.T) {
	layoutsDir, _ := setupTestConfig(t)
	saveTestLayout(t, layoutsDir, "existing", "", 1)

	output := executeComplete(t, "save", "")
	names := completionNames(output)
	assertContains(t, names, "existing")
}

func TestE2E_WatchLayoutNames(t *testing.T) {
	layoutsDir, _ := setupTestConfig(t)
	saveTestLayout(t, layoutsDir, "autosave", "", 1)

	output := executeComplete(t, "watch", "")
	names := completionNames(output)
	assertContains(t, names, "autosave")
}

func TestE2E_LayoutNoSecondArg(t *testing.T) {
	layoutsDir, _ := setupTestConfig(t)
	saveTestLayout(t, layoutsDir, "alpha", "", 1)

	// After providing the layout name, no further completions.
	for _, cmd := range []string{"restore", "delete", "show", "edit", "save", "watch"} {
		t.Run(cmd, func(t *testing.T) {
			output := executeComplete(t, cmd, "alpha", "")
			names := completionNames(output)
			if len(names) != 0 {
				t.Errorf("%s: expected 0 completions after layout name, got %d: %v", cmd, len(names), names)
			}
			if d := completionDirective(output); d != 4 {
				t.Errorf("%s: directive = %d, want 4 (NoFileComp)", cmd, d)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 6. Flag completion end-to-end tests
// ---------------------------------------------------------------------------

func TestE2E_RestoreModeFlag(t *testing.T) {
	setupTestConfig(t)
	output := executeComplete(t, "restore", "--mode", "")
	names := completionNames(output)
	assertContains(t, names, "replace")
	assertContains(t, names, "add")
	if len(names) != 2 {
		t.Errorf("expected exactly 2 mode completions, got %d: %v", len(names), names)
	}

	// Verify descriptions.
	lines := completionLines(output)
	for _, line := range lines {
		if strings.HasPrefix(line, "replace") && !strings.Contains(line, "\t") {
			t.Errorf("replace missing description: %q", line)
		}
		if strings.HasPrefix(line, "add") && !strings.Contains(line, "\t") {
			t.Errorf("add missing description: %q", line)
		}
	}
}

func TestE2E_WatchIntervalFlag(t *testing.T) {
	setupTestConfig(t)
	output := executeComplete(t, "watch", "--interval", "")
	names := completionNames(output)
	for _, want := range []string{"1m", "5m", "10m", "30m"} {
		assertContains(t, names, want)
	}
}

func TestE2E_WsAddTemplateFlag(t *testing.T) {
	setupTestConfig(t)
	output := executeComplete(t, "workspace", "add", "--template", "")
	names := completionNames(output)
	for _, want := range []string{"dev", "go", "single", "monitor"} {
		assertContains(t, names, want)
	}

	// Verify descriptions exist.
	lines := completionLines(output)
	for _, line := range lines {
		if !strings.Contains(line, "\t") {
			t.Errorf("template completion missing description: %q", line)
		}
	}
}

// ---------------------------------------------------------------------------
// 7. Workspace subcommand completion end-to-end
// ---------------------------------------------------------------------------

func TestE2E_WorkspaceSubcommands(t *testing.T) {
	setupTestConfig(t)
	output := executeComplete(t, "workspace", "")
	names := completionNames(output)
	assertContains(t, names, "add")
	assertContains(t, names, "remove")
	assertContains(t, names, "toggle")
	assertContains(t, names, "list")
}

func TestE2E_WsRemoveWorkspaceNames(t *testing.T) {
	_, wsFile := setupTestConfig(t)
	writeTestBlueprint(t, wsFile, []string{"api-server", "webapp"})

	output := executeComplete(t, "workspace", "remove", "")
	names := completionNames(output)
	assertContains(t, names, "api-server")
	assertContains(t, names, "webapp")
}

func TestE2E_WsToggleWorkspaceNames(t *testing.T) {
	_, wsFile := setupTestConfig(t)
	writeTestBlueprint(t, wsFile, []string{"notes", "api-server"})

	output := executeComplete(t, "workspace", "toggle", "")
	names := completionNames(output)
	assertContains(t, names, "notes")
	assertContains(t, names, "api-server")
}

func TestE2E_WsAddFirstArgNoFileComp(t *testing.T) {
	setupTestConfig(t)
	output := executeComplete(t, "workspace", "add", "")
	// First arg is freeform name — no completions, no file fallback.
	names := completionNames(output)
	if len(names) != 0 {
		t.Errorf("expected 0 completions for ws add first arg, got %d: %v", len(names), names)
	}
	if d := completionDirective(output); d != 4 {
		t.Errorf("directive = %d, want 4 (NoFileComp)", d)
	}
}

func TestE2E_WsAddSecondArgFilterDirs(t *testing.T) {
	setupTestConfig(t)
	output := executeComplete(t, "workspace", "add", "myproj", "")
	// Second arg is path — directive should be FilterDirs (16).
	if d := completionDirective(output); d != 16 {
		t.Errorf("directive = %d, want 16 (FilterDirs)", d)
	}
}

// ---------------------------------------------------------------------------
// 8. Alias completion tests
// ---------------------------------------------------------------------------

func TestE2E_WsAlias(t *testing.T) {
	setupTestConfig(t)
	// "ws" is an alias for "workspace".
	output := executeComplete(t, "ws", "")
	names := completionNames(output)
	assertContains(t, names, "add")
	assertContains(t, names, "remove")
	assertContains(t, names, "toggle")
	assertContains(t, names, "list")
}

func TestE2E_DeleteAlias(t *testing.T) {
	layoutsDir, _ := setupTestConfig(t)
	saveTestLayout(t, layoutsDir, "alpha", "", 1)

	// "rm" is an alias for "delete".
	output := executeComplete(t, "rm", "")
	names := completionNames(output)
	assertContains(t, names, "alpha")
}

func TestE2E_ListAlias(t *testing.T) {
	setupTestConfig(t)
	// "l" should match both "list" and other l-commands as partial subcommands.
	output := executeComplete(t, "l")
	names := completionNames(output)
	assertContains(t, names, "list")
}

// ---------------------------------------------------------------------------
// 9. Completion command self-completion
// ---------------------------------------------------------------------------

func TestE2E_CompletionShellNames(t *testing.T) {
	setupTestConfig(t)
	output := executeComplete(t, "completion", "")
	names := completionNames(output)
	for _, want := range []string{"bash", "zsh", "fish", "powershell"} {
		assertContains(t, names, want)
	}
}

func TestE2E_CompletionPartialMatch(t *testing.T) {
	setupTestConfig(t)
	output := executeComplete(t, "completion", "ba")
	names := completionNames(output)
	assertContains(t, names, "bash")
	assertNotContains(t, names, "zsh")
	assertNotContains(t, names, "fish")
}

// ---------------------------------------------------------------------------
// 10. Persistent flag completion marks
// ---------------------------------------------------------------------------

func TestE2E_ConfigFlagFiltersTOML(t *testing.T) {
	setupTestConfig(t)
	output := executeComplete(t, "--config", "")
	// Directive 8 = ShellCompDirectiveFilterFileExt — filters by .toml extension.
	if d := completionDirective(output); d != 8 {
		t.Errorf("--config directive = %d, want 8 (FilterFileExt for .toml)", d)
	}
	// The completions should include "toml" as the extension filter.
	lines := completionLines(output)
	found := false
	for _, l := range lines {
		if l == "toml" {
			found = true
		}
	}
	if !found {
		t.Errorf("--config completions should include 'toml' extension, got: %v", lines)
	}
}

func TestE2E_LayoutsDirFlagFiltersDirs(t *testing.T) {
	setupTestConfig(t)
	output := executeComplete(t, "--layouts-dir", "")
	// Directive 16 = ShellCompDirectiveFilterDirs.
	if d := completionDirective(output); d != 16 {
		t.Errorf("--layouts-dir directive = %d, want 16 (FilterDirs)", d)
	}
}

func TestE2E_WorkspaceFileFlagFiltersMD(t *testing.T) {
	setupTestConfig(t)
	output := executeComplete(t, "--workspace-file", "")
	// Directive 8 = ShellCompDirectiveFilterFileExt — filters by .md extension.
	if d := completionDirective(output); d != 8 {
		t.Errorf("--workspace-file directive = %d, want 8 (FilterFileExt for .md)", d)
	}
	lines := completionLines(output)
	found := false
	for _, l := range lines {
		if l == "md" {
			found = true
		}
	}
	if !found {
		t.Errorf("--workspace-file completions should include 'md' extension, got: %v", lines)
	}
}

// ---------------------------------------------------------------------------
// 11. ws add ValidArgsFunction unit tests
// ---------------------------------------------------------------------------

func TestWsAddCompletion_FirstArgNoFileComp(t *testing.T) {
	setupTestConfig(t)
	fn := wsAddCmd.ValidArgsFunction
	if fn == nil {
		t.Fatal("wsAddCmd.ValidArgsFunction is nil")
	}

	completions, directive := fn(wsAddCmd, nil, "")
	if len(completions) != 0 {
		t.Errorf("expected 0 completions for first arg (name), got %d", len(completions))
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("first arg: directive = %d, want ShellCompDirectiveNoFileComp (%d)", directive, cobra.ShellCompDirectiveNoFileComp)
	}
}

func TestWsAddCompletion_SecondArgFilterDirs(t *testing.T) {
	setupTestConfig(t)
	fn := wsAddCmd.ValidArgsFunction
	if fn == nil {
		t.Fatal("wsAddCmd.ValidArgsFunction is nil")
	}

	completions, directive := fn(wsAddCmd, []string{"myproj"}, "")
	if len(completions) != 0 {
		t.Errorf("expected 0 explicit completions for path arg, got %d", len(completions))
	}
	if directive != cobra.ShellCompDirectiveFilterDirs {
		t.Errorf("second arg: directive = %d, want ShellCompDirectiveFilterDirs (%d)", directive, cobra.ShellCompDirectiveFilterDirs)
	}
}

func TestWsAddCompletion_ThirdArgBlocked(t *testing.T) {
	setupTestConfig(t)
	fn := wsAddCmd.ValidArgsFunction
	if fn == nil {
		t.Fatal("wsAddCmd.ValidArgsFunction is nil")
	}

	completions, directive := fn(wsAddCmd, []string{"myproj", "/tmp"}, "")
	if len(completions) != 0 {
		t.Errorf("expected 0 completions for third arg, got %d", len(completions))
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("third arg: directive = %d, want ShellCompDirectiveNoFileComp", directive)
	}
}
