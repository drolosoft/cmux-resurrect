package client

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeCloseShim installs a fake cmux that logs each invocation on one line
// and answers `close-workspace` the way the given mode asks: "plain" closes
// at once, "confirm" refuses without --force as cmux does for a workspace
// with a running process, and "missing" always fails with another error.
func writeCloseShim(t *testing.T, mode string) (binary, callLog string) {
	t.Helper()

	dir := t.TempDir()
	binary = filepath.Join(dir, "cmux")
	callLog = filepath.Join(dir, "calls.log")

	script := fmt.Sprintf(`#!/bin/sh
echo "$*" >> %[1]q
case %[2]q in
  plain)
    echo "OK workspace:1"
    ;;
  confirm)
    case "$*" in
      *--force*) echo "OK workspace:1" ;;
      *) echo "Error: confirmation_required: Workspace has a running process; retry with --force" >&2; exit 1 ;;
    esac
    ;;
  missing)
    echo "Error: not_found: Workspace not found" >&2
    exit 1
    ;;
esac
`, callLog, mode)
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	return binary, callLog
}

// closeCalls returns the logged invocations of the shim, one per line.
func closeCalls(t *testing.T, callLog string) []string {
	t.Helper()

	data, err := os.ReadFile(callLog)
	if err != nil {
		t.Fatal(err)
	}

	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func TestCloseWorkspace_ClosesWithoutForceWhenCmuxAllowsIt(t *testing.T) {
	binary, callLog := writeCloseShim(t, "plain")
	cli := &CLIClient{Binary: binary, Timeout: 5 * time.Second}

	if err := cli.CloseWorkspace("workspace:1"); err != nil {
		t.Fatalf("CloseWorkspace: %v", err)
	}

	calls := closeCalls(t, callLog)
	if len(calls) != 1 || strings.Contains(calls[0], "--force") {
		t.Errorf("calls = %q, want one close without --force", calls)
	}
}

func TestCloseWorkspace_ForcesWhenCmuxAsksForConfirmation(t *testing.T) {
	binary, callLog := writeCloseShim(t, "confirm")
	cli := &CLIClient{Binary: binary, Timeout: 5 * time.Second}

	if err := cli.CloseWorkspace("workspace:1"); err != nil {
		t.Fatalf("CloseWorkspace: %v", err)
	}

	calls := closeCalls(t, callLog)
	want := []string{
		"close-workspace --workspace workspace:1",
		"close-workspace --workspace workspace:1 --force",
	}
	if strings.Join(calls, "|") != strings.Join(want, "|") {
		t.Errorf("calls = %q, want %q", calls, want)
	}
}

func TestCloseWorkspace_OtherErrorsAreNotRetried(t *testing.T) {
	binary, callLog := writeCloseShim(t, "missing")
	cli := &CLIClient{Binary: binary, Timeout: 5 * time.Second}

	if err := cli.CloseWorkspace("workspace:1"); err == nil {
		t.Fatal("CloseWorkspace returned no error for a missing workspace")
	}

	if calls := closeCalls(t, callLog); len(calls) != 1 {
		t.Errorf("calls = %q, want a single attempt", calls)
	}
}
