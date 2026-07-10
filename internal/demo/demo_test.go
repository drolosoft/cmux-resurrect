package demo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

func TestEmbeddedLayoutIsValid(t *testing.T) {
	dir := t.TempDir()
	if _, err := Install(dir); err != nil {
		t.Fatalf("install: %v", err)
	}
	store, err := persist.NewFileStore(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	layout, err := store.Load("demo")
	if err != nil {
		t.Fatalf("embedded demo.toml does not parse: %v", err)
	}
	if layout.Name != "demo" {
		t.Errorf("name = %q, want demo", layout.Name)
	}
	if len(layout.Workspaces) < 2 {
		t.Fatalf("workspaces = %d, want >= 2 (multi-workspace showcase)", len(layout.Workspaces))
	}
	// Portability: every cwd must be home-relative (~), never machine-specific.
	for _, ws := range layout.Workspaces {
		if !strings.HasPrefix(ws.CWD, "~") {
			t.Errorf("workspace %q cwd = %q — must be portable (~-relative)", ws.Title, ws.CWD)
		}
		for i, p := range ws.Panes {
			if p.CWD != "" && !strings.HasPrefix(p.CWD, "~") {
				t.Errorf("workspace %q pane %d cwd = %q — must be portable", ws.Title, i, p.CWD)
			}
			if p.Command != "" {
				t.Errorf("workspace %q pane %d has command %q — demo must be side-effect free", ws.Title, i, p.Command)
			}
		}
	}
	// Showcase: at least one split with its own cwd (the #8 feature).
	found := false
	for _, ws := range layout.Workspaces {
		for _, p := range ws.Panes {
			if p.Split != "" && p.CWD != "" {
				found = true
			}
		}
	}
	if !found {
		t.Error("demo should showcase a split pane with its own cwd")
	}
}

func TestInstall_CreatesOnlyWhenMissing(t *testing.T) {
	dir := t.TempDir()
	created, err := Install(dir)
	if err != nil || !created {
		t.Fatalf("first install: created=%v err=%v, want true, nil", created, err)
	}
	// A user-modified demo must NOT be clobbered.
	p := filepath.Join(dir, "demo.toml")
	if err := os.WriteFile(p, []byte("# user edited\nname = 'demo'\nversion = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	created, err = Install(dir)
	if err != nil || created {
		t.Fatalf("second install: created=%v err=%v, want false (no clobber), nil", created, err)
	}
	data, _ := os.ReadFile(p)
	if !strings.Contains(string(data), "# user edited") {
		t.Error("user-edited demo was clobbered")
	}
}
