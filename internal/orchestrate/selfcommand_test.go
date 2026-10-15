package orchestrate

import (
	"testing"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

// fakeForegroundCommands swaps the process-table lookup for a fixed map of
// tty to command for the duration of a test.
func fakeForegroundCommands(t *testing.T, commandsByTTY map[string]string) {
	t.Helper()

	original := foregroundCommand
	foregroundCommand = func(tty string) string {
		return commandsByTTY[tty]
	}
	t.Cleanup(func() {
		foregroundCommand = original
	})
}

// TestSave_NeverStoresCrexItself: running `crex save now` in a pane makes crex
// that pane's foreground command. Storing it would make a later restore type
// `crex save now` (overwriting the layout mid-restore) or `crex restore now`
// (a recursive restore).
func TestSave_NeverStoresCrexItself(t *testing.T) {
	fakeForegroundCommands(t, map[string]string{
		"crex-test-tty-1": "crex save now",
		"crex-test-tty-2": "crex restore now",
		"crex-test-tty-3": "npm run dev",
	})

	tree := &client.TreeResponse{
		Windows: []client.TreeWindow{{
			Workspaces: []client.TreeWorkspace{{
				Ref:   "workspace:1",
				Title: "proj",
				Panes: []client.TreePane{
					{
						Index:   0,
						Focused: true,
						Surfaces: []client.TreeSurface{
							{Ref: "surface:1", Type: "terminal", TTY: "crex-test-tty-1", CWD: "/home/user"},
							{Ref: "surface:2", Type: "terminal", TTY: "crex-test-tty-2", CWD: "/home/user"},
						},
					},
					{
						Index: 1,
						Surfaces: []client.TreeSurface{
							{Ref: "surface:3", Type: "terminal", TTY: "crex-test-tty-3", CWD: "/home/user"},
						},
					},
				},
			}},
		}},
	}
	mc := &mockClient{
		treeResp:    tree,
		sidebarCWDs: map[string]string{"workspace:1": "/home/user"},
	}
	store, _ := persist.NewFileStore(t.TempDir())
	saver := &Saver{Client: mc, Store: store}

	layout, err := saver.Save("self", "")
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	panes := layout.Workspaces[0].Panes
	if got := panes[0].Command; got != "" {
		t.Errorf("pane running crex saved command %q, want none", got)
	}
	if got := panes[0].Surfaces[0].Command; got != "" {
		t.Errorf("tab running crex saved command %q, want none", got)
	}
	if got := panes[1].Command; got != "npm run dev" {
		t.Errorf("other pane command = %q, want npm run dev", got)
	}
}

// TestSave_ReSaveDropsCrexCommandFromExistingLayout: the merge keeps commands
// from the layout on disk, so a layout that already holds `crex save now`
// would keep it on every later `crex save now` unless the merge drops it.
func TestSave_ReSaveDropsCrexCommandFromExistingLayout(t *testing.T) {
	fakeForegroundCommands(t, map[string]string{})

	store, _ := persist.NewFileStore(t.TempDir())
	existing := &model.Layout{
		Name:    "now",
		Version: 1,
		SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{{
			Title: "proj",
			CWD:   "/home/user",
			Panes: []model.Pane{
				{Type: "terminal", Index: 0, Focus: true, Command: "crex save now"},
				{Type: "terminal", Index: 1, Split: "right", Command: "npm run dev"},
			},
		}},
	}
	if err := store.Save("now", existing); err != nil {
		t.Fatalf("store save: %v", err)
	}

	tree := &client.TreeResponse{
		Windows: []client.TreeWindow{{
			Workspaces: []client.TreeWorkspace{{
				Ref:   "workspace:1",
				Title: "proj",
				Panes: []client.TreePane{
					{Index: 0, Focused: true, Surfaces: []client.TreeSurface{{Ref: "surface:1", Type: "terminal", CWD: "/home/user"}}},
					{Index: 1, Surfaces: []client.TreeSurface{{Ref: "surface:2", Type: "terminal", CWD: "/home/user"}}},
				},
			}},
		}},
	}
	mc := &mockClient{
		treeResp:    tree,
		sidebarCWDs: map[string]string{"workspace:1": "/home/user"},
	}
	saver := &Saver{Client: mc, Store: store}

	layout, err := saver.Save("now", "")
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	panes := layout.Workspaces[0].Panes
	if got := panes[0].Command; got != "" {
		t.Errorf("re-save kept crex's own command %q, want none", got)
	}
	if got := panes[1].Command; got != "npm run dev" {
		t.Errorf("user command lost on re-save: %q, want npm run dev", got)
	}
}

// TestRestore_OldLayoutWithCrexCommandTypesNothing: layouts saved before the
// fix hold `crex save now`. Restore, dry-run included, must not type it.
func TestRestore_OldLayoutWithCrexCommandTypesNothing(t *testing.T) {
	origHeur, origSurf := ShellReadyTimeout, SurfaceReadyTimeout
	ShellReadyTimeout, SurfaceReadyTimeout = 100*time.Millisecond, 100*time.Millisecond
	defer func() { ShellReadyTimeout, SurfaceReadyTimeout = origHeur, origSurf }()

	store, _ := persist.NewFileStore(t.TempDir())
	layout := &model.Layout{
		Name:    "old",
		Version: 1,
		SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{{
			Title: "proj",
			CWD:   "/tmp/project",
			Panes: []model.Pane{
				{
					Type: "terminal", Focus: true, FocusTarget: -1, Command: "crex save now",
					Surfaces: []model.Surface{{Type: "terminal", Command: "/opt/homebrew/bin/crex restore now"}},
				},
				{Type: "terminal", Split: "right", FocusTarget: -1, Command: "vim crex.toml"},
			},
		}},
	}
	if err := store.Save("old", layout); err != nil {
		t.Fatalf("store save: %v", err)
	}

	for _, dryRun := range []bool{true, false} {
		mc := &sendRecorderMock{mockClient: mockClient{
			treeResp:    &client.TreeResponse{},
			sidebarCWDs: map[string]string{},
		}}
		restorer := &Restorer{Client: mc, Store: store, SkipPing: true}

		result, err := restorer.Restore("old", dryRun, RestoreModeAdd, "", false)
		if err != nil {
			t.Fatalf("restore (dryRun=%v): %v", dryRun, err)
		}

		// Real runs type through Send; dry runs only list what they would do.
		typed := append([]string{}, mc.sends...)
		typed = append(typed, result.Commands...)

		var keptArgument bool
		for _, text := range typed {
			if containsStr(text, "crex save") || containsStr(text, "crex restore") {
				t.Errorf("dryRun=%v: restore typed crex itself: %q", dryRun, text)
			}
			if containsStr(text, "vim crex.toml") {
				keptArgument = true
			}
		}
		if !keptArgument {
			t.Errorf("dryRun=%v: a command that only mentions crex was dropped; typed: %v", dryRun, typed)
		}
	}
}
