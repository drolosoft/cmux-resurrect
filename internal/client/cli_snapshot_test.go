package client

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeShim installs a fake cmux binary that logs every invocation and fails
// only for the given subcommand. Everything else succeeds with empty output.
func writeShim(t *testing.T, failCmd string) (binary, callLog string) {
	t.Helper()
	dir := t.TempDir()
	callLog = filepath.Join(dir, "calls.log")
	binary = filepath.Join(dir, "cmux")
	script := fmt.Sprintf(`#!/bin/sh
echo "$@" >> %q
if [ "$1" = %q ]; then
  echo "boom" >&2
  exit 1
fi
exit 0
`, callLog, failCmd)
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return binary, callLog
}

func calls(t *testing.T, callLog string) string {
	t.Helper()
	data, _ := os.ReadFile(callLog)
	return string(data)
}

// The new-ref discovery in createWorkspace/NewSplit/NewPane diffs a pre-op
// snapshot against the post-op state. If the snapshot fails, proceeding with
// an EMPTY before-set makes the diff return a PRE-EXISTING ref — and the
// caller then types cd/commands into the user's live pane (2026-07-11 audit,
// finding H1). A failed snapshot must abort the operation instead.

func TestCreateWorkspace_AbortsWhenSnapshotFails(t *testing.T) {
	bin, log := writeShim(t, "list-workspaces")
	c := &CLIClient{Binary: bin, Timeout: 5 * time.Second}
	if _, err := c.NewWorkspace(NewWorkspaceOpts{CWD: "/tmp"}); err == nil {
		t.Fatal("expected error when the pre-create snapshot fails")
	}
	if strings.Contains(calls(t, log), "new-workspace") {
		t.Fatal("workspace must not be created after a failed snapshot")
	}
}

func TestNewSplit_AbortsWhenSnapshotFails(t *testing.T) {
	bin, log := writeShim(t, "tree")
	c := &CLIClient{Binary: bin, Timeout: 5 * time.Second}
	if _, err := c.NewSplit("right", "workspace:1", ""); err == nil {
		t.Fatal("expected error when the pre-split snapshot fails")
	}
	if strings.Contains(calls(t, log), "new-split") {
		t.Fatal("split must not be created after a failed snapshot")
	}
}

func TestNewPane_AbortsWhenSnapshotFails(t *testing.T) {
	bin, log := writeShim(t, "tree")
	c := &CLIClient{Binary: bin, Timeout: 5 * time.Second}
	if _, err := c.NewPane(NewPaneOpts{Type: "browser", Direction: "right", WorkspaceRef: "workspace:1"}); err == nil {
		t.Fatal("expected error when the pre-create snapshot fails")
	}
	out := calls(t, log)
	if strings.Contains(out, "new-pane") || strings.Contains(out, "new-browser") {
		t.Fatal("pane must not be created after a failed snapshot")
	}
}

func TestNewSurface_AbortsWhenSnapshotFails(t *testing.T) {
	bin, log := writeShim(t, "tree")
	c := &CLIClient{Binary: bin, Timeout: 5 * time.Second}
	if _, err := c.NewSurface("pane:0", "workspace:1"); err == nil {
		t.Fatal("expected error when the pre-create snapshot fails")
	}
	if strings.Contains(calls(t, log), "new-surface") {
		t.Fatal("surface must not be created after a failed snapshot")
	}
}
