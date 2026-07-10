package orchestrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

func TestCwdCommand(t *testing.T) {
	tests := []struct {
		name    string
		cwd     string
		command string
		want    string
	}{
		{"no cwd, with command", "", "go test", "go test"},
		{"no cwd, no command", "", "", ""},
		{"cwd, with command", "/tmp/api", "claude --resume x", "cd '/tmp/api' && claude --resume x"},
		{"cwd, no command (bare cd)", "/tmp/api", "", "cd '/tmp/api'"},
		{"cwd with space", "/tmp/my proj", "ls", "cd '/tmp/my proj' && ls"},
		{"cwd with single quote", "/tmp/o'brien", "", `cd '/tmp/o'\''brien'`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cwdCommand(tt.cwd, tt.command); got != tt.want {
				t.Fatalf("cwdCommand(%q, %q) = %q, want %q", tt.cwd, tt.command, got, tt.want)
			}
		})
	}
}

// TestRestore_DryRun_PerPaneCWD verifies that a pane carrying its own CWD (one
// that differs from the workspace CWD) produces a `cd` into that directory in
// the restore plan — GitHub #8.
func TestRestore_DryRun_PerPaneCWD(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name:    "cwd-test",
		Version: 1,
		SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{
				Title: "0 dev",
				CWD:   "/tmp/project",
				Index: 0,
				Panes: []model.Pane{
					// First pane in a different dir, no command → bare cd.
					{Type: "terminal", Focus: true, CWD: "/tmp/api"},
					// Split pane in another dir, with a command → cd && command.
					{Type: "terminal", Split: "right", CWD: "/tmp/web", Command: "npm run dev"},
					// Split pane with no CWD → inherits workspace path, no cd.
					{Type: "terminal", Split: "right", Command: "claude"},
				},
			},
		},
	}
	if err := store.Save("cwd-test", layout); err != nil {
		t.Fatalf("save: %v", err)
	}

	mc := &mockClient{sidebarCWDs: map[string]string{}}
	restorer := &Restorer{Client: mc, Store: store}

	result, err := restorer.Restore("cwd-test", true, RestoreModeAdd, "", true)
	if err != nil {
		t.Fatalf("restore dry-run: %v", err)
	}
	plan := strings.Join(result.Commands, "\n")

	for _, want := range []string{
		"cd '/tmp/api'",                // bare cd for the first pane
		"cd '/tmp/web' && npm run dev", // cd + command for the split
	} {
		if !strings.Contains(plan, want) {
			t.Errorf("dry-run plan missing %q\n--- plan ---\n%s", want, plan)
		}
	}
	// The no-CWD pane must NOT get a spurious cd before its command.
	if strings.Contains(plan, "cd '/tmp/project'") {
		t.Errorf("did not expect a cd into the workspace CWD\n--- plan ---\n%s", plan)
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	tests := []struct {
		in   string
		want string
	}{
		{"~", home},
		{"~/Documents", filepath.Join(home, "Documents")},
		{"/absolute/path", "/absolute/path"},
		{"", ""},
		{"~otheruser/dir", "~otheruser/dir"}, // not ours to expand
		{"relative/path", "relative/path"},
	}
	for _, tt := range tests {
		if got := expandHome(tt.in); got != tt.want {
			t.Errorf("expandHome(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCwdCommand_ExpandsTilde(t *testing.T) {
	// Portable layouts store '~/Documents'; a quoted `cd '~/Documents'`
	// would NOT expand in the shell, so crex must expand before quoting.
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	got := cwdCommand("~/Documents", "")
	want := "cd '" + filepath.Join(home, "Documents") + "'"
	if got != want {
		t.Errorf("cwdCommand(~/Documents) = %q, want %q", got, want)
	}
}
