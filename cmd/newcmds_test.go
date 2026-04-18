package cmd

import (
	"bytes"
	"strings"
	"testing"
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
