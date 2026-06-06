package cmd

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/model"
)

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

// executeCmd runs rootCmd with the given args and returns (output, error).
func executeCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	setupTestConfig(t)
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	rootCmd.SetArgs(nil)
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)
	return buf.String(), err
}

// ---------------------------------------------------------------------------
// 1. Setup Command Wiring
// ---------------------------------------------------------------------------

func TestSetupCmd_IsRegistered(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if strings.HasPrefix(c.Use, "setup") {
			found = true
			break
		}
	}
	if !found {
		t.Error("setupCmd is not registered as a child of rootCmd")
	}
}

func TestSetupCmd_DefaultsFlag(t *testing.T) {
	f := setupCmd.Flags().Lookup("defaults")
	if f == nil {
		t.Fatal("setupCmd missing --defaults flag")
	}
	if f.DefValue != "false" {
		t.Errorf("--defaults default = %q, want %q", f.DefValue, "false")
	}
}

// ---------------------------------------------------------------------------
// 2. Watch Daemon Flags Wiring
// ---------------------------------------------------------------------------

func TestWatchCmd_DaemonFlags(t *testing.T) {
	for _, name := range []string{"daemon", "stop", "status", "shell-hook"} {
		t.Run(name, func(t *testing.T) {
			f := watchCmd.Flags().Lookup(name)
			if f == nil {
				t.Fatalf("watchCmd missing --%s flag", name)
			}
			if f.Value.Type() != "bool" {
				t.Errorf("--%s type = %q, want %q", name, f.Value.Type(), "bool")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 3. Watch --status Output
// ---------------------------------------------------------------------------

func TestWatchStatus_NotRunning(t *testing.T) {
	// --status writes to os.Stderr directly, so we can only verify it does not error.
	_, err := executeCmd(t, "watch", "--status")
	if err != nil {
		t.Fatalf("watch --status returned error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 4. Watch --shell-hook Output
// ---------------------------------------------------------------------------

func TestWatchShellHook_ProducesOutput(t *testing.T) {
	t.Setenv("SHELL", "/bin/zsh")

	// --shell-hook writes to os.Stdout via fmt.Print, so the rootCmd.SetOut
	// buffer won't capture it. We verify no error and rely on the known
	// ShellHook behaviour being tested in the orchestrate package.
	_, err := executeCmd(t, "watch", "--shell-hook")
	if err != nil {
		t.Fatalf("watch --shell-hook returned error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 5. TUI Command Wiring
// ---------------------------------------------------------------------------

func TestTuiCmd_IsRegistered(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if strings.HasPrefix(c.Use, "tui") {
			found = true
			break
		}
	}
	if !found {
		t.Error("tuiCmd is not registered as a child of rootCmd")
	}
}

func TestTuiCmd_NoArgs(t *testing.T) {
	if tuiCmd.Args == nil {
		t.Error("tuiCmd.Args is nil; expected cobra.NoArgs")
	}
}

// ---------------------------------------------------------------------------
// 6. E2E Completion for New Commands
// ---------------------------------------------------------------------------

func TestE2E_SubcommandCompletion_IncludesNewCommands(t *testing.T) {
	setupTestConfig(t)
	output := executeComplete(t, "")
	names := completionNames(output)
	assertContains(t, names, "setup")
	assertContains(t, names, "tui")
}

func TestE2E_SetupCompletion_NoPositionalArgs(t *testing.T) {
	setupTestConfig(t)
	output := executeComplete(t, "setup", "")
	names := completionNames(output)
	// setup takes no positional args, so completions should be empty
	// (only flags may appear, but completionNames strips directive lines).
	if len(names) > 0 {
		t.Errorf("setup should have no positional completions, got %v", names)
	}
}

// ---------------------------------------------------------------------------
// 7. Blueprint Command Wiring
// ---------------------------------------------------------------------------

func TestBlueprintCmd_IsRegistered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "blueprint" {
			found = true
			if !slices.Contains(cmd.Aliases, "bp") {
				t.Error("blueprint command should have 'bp' alias")
			}
			break
		}
	}
	if !found {
		t.Error("blueprint command not registered on root")
	}
}

func TestWorkspaceLegacy_IsHidden(t *testing.T) {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "workspace" {
			if !cmd.Hidden {
				t.Error("workspace command should be hidden")
			}
			return
		}
	}
	t.Error("workspace legacy command not registered")
}

// ---------------------------------------------------------------------------
// 8. List --json / --alfred Output
// ---------------------------------------------------------------------------

// executeListCmd runs the list command using the test config's layouts dir.
// It resets flag bools that persist across Cobra invocations and passes
// --layouts-dir so initConfig doesn't override with the real user dir.
func executeListCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	listJSON = false
	listAlfred = false
	listCmd.Flags().Lookup("json").Changed = false
	listCmd.Flags().Lookup("alfred").Changed = false
	dir := cfg.LayoutsDir
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(append([]string{"--layouts-dir", dir, "list"}, args...))
	err := rootCmd.Execute()
	rootCmd.SetArgs(nil)
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)
	layoutsDir = ""
	return buf.String(), err
}

func TestListJSON_WithLayouts(t *testing.T) {
	dir, _ := setupTestConfig(t)
	saveTestLayout(t, dir, "alpha", "First layout", 2)
	saveTestLayout(t, dir, "beta", "", 1)

	out, err := executeListCmd(t, "--json")
	if err != nil {
		t.Fatalf("list --json: %v", err)
	}

	var metas []model.LayoutMeta
	if err := json.Unmarshal([]byte(out), &metas); err != nil {
		t.Fatalf("parse JSON: %v\nraw: %s", err, out)
	}
	if len(metas) != 2 {
		t.Fatalf("got %d items, want 2", len(metas))
	}
	if metas[0].Name != "alpha" {
		t.Errorf("first item name = %q, want alpha", metas[0].Name)
	}
	if metas[0].WorkspaceCount != 2 {
		t.Errorf("alpha workspace_count = %d, want 2", metas[0].WorkspaceCount)
	}
	if metas[1].Name != "beta" {
		t.Errorf("second item name = %q, want beta", metas[1].Name)
	}
}

func TestListJSON_Empty(t *testing.T) {
	setupTestConfig(t)

	out, err := executeListCmd(t, "--json")
	if err != nil {
		t.Fatalf("list --json: %v", err)
	}

	if strings.TrimSpace(out) != "[]" {
		t.Errorf("empty list --json = %q, want []", strings.TrimSpace(out))
	}
}

func TestListAlfred_WithLayouts(t *testing.T) {
	dir, _ := setupTestConfig(t)
	saveTestLayout(t, dir, "dev", "My dev setup", 2)

	out, err := executeListCmd(t, "--alfred")
	if err != nil {
		t.Fatalf("list --alfred: %v", err)
	}

	var result struct {
		Items []struct {
			UID          string `json:"uid"`
			Title        string `json:"title"`
			Subtitle     string `json:"subtitle"`
			Arg          string `json:"arg"`
			Autocomplete string `json:"autocomplete"`
			Mods         struct {
				Cmd  struct{ Arg string `json:"arg"` } `json:"cmd"`
				Alt  struct{ Arg string `json:"arg"` } `json:"alt"`
				Ctrl struct{ Arg string `json:"arg"` } `json:"ctrl"`
			} `json:"mods"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("parse Alfred JSON: %v\nraw: %s", err, out)
	}
	// 1 layout + 2 workspaces = 3 items
	if len(result.Items) != 3 {
		t.Fatalf("got %d items, want 3 (1 layout + 2 workspaces)", len(result.Items))
	}
	// First item is the layout
	item := result.Items[0]
	if item.UID != "layout:dev" {
		t.Errorf("uid = %q, want layout:dev", item.UID)
	}
	if item.Arg != "restore:dev" {
		t.Errorf("arg = %q, want restore:dev", item.Arg)
	}
	if item.Mods.Cmd.Arg != "show:dev" {
		t.Errorf("cmd mod arg = %q, want show:dev", item.Mods.Cmd.Arg)
	}
	if item.Mods.Alt.Arg != "delete:dev" {
		t.Errorf("alt mod arg = %q, want delete:dev", item.Mods.Alt.Arg)
	}
	if item.Mods.Ctrl.Arg != "open:dev" {
		t.Errorf("ctrl mod arg = %q, want open:dev", item.Mods.Ctrl.Arg)
	}
	if !strings.Contains(item.Subtitle, "2") {
		t.Errorf("subtitle should contain workspace count, got %q", item.Subtitle)
	}
}

func TestListAlfred_Empty(t *testing.T) {
	setupTestConfig(t)

	out, err := executeListCmd(t, "--alfred")
	if err != nil {
		t.Fatalf("list --alfred: %v", err)
	}

	var result struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("parse Alfred JSON: %v\nraw: %s", err, out)
	}
	if len(result.Items) != 0 {
		t.Errorf("got %d items, want 0", len(result.Items))
	}
}

func TestListJSON_Alfred_MutuallyExclusive(t *testing.T) {
	setupTestConfig(t)

	_, err := executeListCmd(t, "--json", "--alfred")
	if err == nil {
		t.Fatal("expected error when both --json and --alfred are set")
	}
}

func TestParseAgentList(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"claude, codex, opencode", []string{"claude", "codex", "opencode"}},
		{"claude codex", []string{"claude", "codex"}},
		{"all", []string{"all"}},
		{"  claude , ,  codex  ", []string{"claude", "codex"}},
		{"", nil},
	}
	for _, tt := range tests {
		got := parseAgentList(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("parseAgentList(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("parseAgentList(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestBlueprintCmd_HasSubcommands(t *testing.T) {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "blueprint" {
			subs := []string{"add", "remove", "list", "toggle"}
			for _, sub := range subs {
				found := false
				for _, sc := range cmd.Commands() {
					if sc.Name() == sub {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("blueprint missing subcommand %q", sub)
				}
			}
			return
		}
	}
	t.Fatal("blueprint command not found")
}
