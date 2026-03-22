package persist

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/juanatsap/cmux-resurrect/internal/model"
)

func TestFileStore_SaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	layout := &model.Layout{
		Name:    "test",
		Version: 1,
		SavedAt: time.Date(2026, 3, 22, 11, 0, 0, 0, time.UTC),
		Workspaces: []model.Workspace{
			{
				Title:  "0 dev",
				CWD:    "/tmp/project",
				Pinned: true,
				Index:  0,
				Panes: []model.Pane{
					{Type: "terminal", Focus: true},
					{Type: "terminal", Split: "right"},
				},
			},
		},
	}

	if err := store.Save("test", layout); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Verify file exists.
	if !store.Exists("test") {
		t.Error("Exists() returned false after save")
	}

	// Load back.
	loaded, err := store.Load("test")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if loaded.Name != "test" {
		t.Errorf("Name = %q", loaded.Name)
	}
	if len(loaded.Workspaces) != 1 {
		t.Fatalf("Workspaces = %d", len(loaded.Workspaces))
	}
	if loaded.Workspaces[0].CWD != "/tmp/project" {
		t.Errorf("CWD = %q", loaded.Workspaces[0].CWD)
	}
	if len(loaded.Workspaces[0].Panes) != 2 {
		t.Fatalf("Panes = %d", len(loaded.Workspaces[0].Panes))
	}
}

func TestFileStore_List(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	// Empty list initially.
	metas, err := store.List()
	if err != nil {
		t.Fatalf("list empty: %v", err)
	}
	if len(metas) != 0 {
		t.Errorf("expected 0 layouts, got %d", len(metas))
	}

	// Save two layouts.
	for _, name := range []string{"alpha", "beta"} {
		layout := &model.Layout{
			Name:    name,
			Version: 1,
			SavedAt: time.Now().UTC(),
			Workspaces: []model.Workspace{
				{Title: "ws", CWD: "/tmp", Panes: []model.Pane{{Type: "terminal"}}},
			},
		}
		if err := store.Save(name, layout); err != nil {
			t.Fatalf("save %s: %v", name, err)
		}
	}

	metas, err = store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(metas) != 2 {
		t.Fatalf("expected 2 layouts, got %d", len(metas))
	}
	// Should be sorted alphabetically.
	if metas[0].Name != "alpha" || metas[1].Name != "beta" {
		t.Errorf("order: %s, %s", metas[0].Name, metas[1].Name)
	}
}

func TestFileStore_Delete(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	layout := &model.Layout{Name: "gone", Version: 1, SavedAt: time.Now().UTC()}
	if err := store.Save("gone", layout); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := store.Delete("gone"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if store.Exists("gone") {
		t.Error("layout still exists after delete")
	}
}

func TestFileStore_DeleteNonexistent(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	if err := store.Delete("nope"); err == nil {
		t.Error("expected error deleting nonexistent layout")
	}
}

func TestFileStore_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	layout := &model.Layout{Name: "atomic", Version: 1, SavedAt: time.Now().UTC()}
	if err := store.Save("atomic", layout); err != nil {
		t.Fatalf("save: %v", err)
	}

	// No temp file should remain.
	matches, _ := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if len(matches) > 0 {
		t.Errorf("temp file remains: %v", matches)
	}

	// File should exist with correct name.
	info, err := os.Stat(store.Path("atomic"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() == 0 {
		t.Error("file is empty")
	}
}

func TestFileStore_LoadFromFixture(t *testing.T) {
	store := &FileStore{Dir: "../../testdata/layouts"}

	layout, err := store.Load("minimal")
	if err != nil {
		t.Fatalf("load minimal fixture: %v", err)
	}

	if layout.Name != "minimal" {
		t.Errorf("Name = %q", layout.Name)
	}
	if len(layout.Workspaces) != 1 {
		t.Fatalf("Workspaces = %d", len(layout.Workspaces))
	}
	if layout.Workspaces[0].Title != "0 main" {
		t.Errorf("Title = %q", layout.Workspaces[0].Title)
	}
}

func TestFileStore_LoadWithSplits(t *testing.T) {
	store := &FileStore{Dir: "../../testdata/layouts"}

	layout, err := store.Load("with-splits")
	if err != nil {
		t.Fatalf("load with-splits fixture: %v", err)
	}

	if len(layout.Workspaces) != 2 {
		t.Fatalf("Workspaces = %d", len(layout.Workspaces))
	}
	ws := layout.Workspaces[0]
	if len(ws.Panes) != 2 {
		t.Fatalf("Panes = %d", len(ws.Panes))
	}
	if ws.Panes[1].Split != "right" {
		t.Errorf("Split = %q", ws.Panes[1].Split)
	}
	if ws.Panes[1].Command != "go test ./..." {
		t.Errorf("Command = %q", ws.Panes[1].Command)
	}
}
