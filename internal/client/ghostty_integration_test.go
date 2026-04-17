//go:build integration && darwin

package client

import "testing"

func TestGhosttyPing_Integration(t *testing.T) {
	gc := NewGhosttyClient()
	if err := gc.Ping(); err != nil {
		t.Skipf("Ghostty not running: %v", err)
	}
}

func TestGhosttyListWorkspaces_Integration(t *testing.T) {
	gc := NewGhosttyClient()
	if err := gc.Ping(); err != nil {
		t.Skipf("Ghostty not running: %v", err)
	}
	ws, err := gc.ListWorkspaces()
	if err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}
	if len(ws) == 0 {
		t.Fatal("expected at least one workspace (tab)")
	}
	for _, w := range ws {
		if w.Ref == "" || w.Title == "" {
			t.Errorf("workspace with empty ref or title: %+v", w)
		}
	}
}

func TestGhosttyTree_Integration(t *testing.T) {
	gc := NewGhosttyClient()
	if err := gc.Ping(); err != nil {
		t.Skipf("Ghostty not running: %v", err)
	}
	tree, err := gc.Tree()
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}
	if len(tree.Windows) == 0 {
		t.Fatal("expected at least one window")
	}
	if len(tree.Windows[0].Workspaces) == 0 {
		t.Fatal("expected at least one workspace in first window")
	}
}

func TestGhosttyPinWorkspace_Integration(t *testing.T) {
	gc := NewGhosttyClient()
	// PinWorkspace is a no-op — should always succeed.
	if err := gc.PinWorkspace("tab:1"); err != nil {
		t.Fatalf("PinWorkspace should be no-op: %v", err)
	}
}
