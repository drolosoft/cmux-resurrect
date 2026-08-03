package persist

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/model"
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

func TestFileStore_Rename(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	layout := &model.Layout{
		Name:    "old-name",
		Version: 1,
		SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{Title: "ws1", CWD: "/tmp", Panes: []model.Pane{{Type: "terminal"}}},
		},
	}
	if err := store.Save("old-name", layout); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := store.Rename("old-name", "new-name"); err != nil {
		t.Fatalf("rename: %v", err)
	}

	if store.Exists("old-name") {
		t.Error("old layout still exists after rename")
	}
	if !store.Exists("new-name") {
		t.Error("new layout does not exist after rename")
	}

	// Verify the name inside the TOML was updated.
	loaded, err := store.Load("new-name")
	if err != nil {
		t.Fatalf("load renamed: %v", err)
	}
	if loaded.Name != "new-name" {
		t.Errorf("Name = %q, want %q", loaded.Name, "new-name")
	}
	if len(loaded.Workspaces) != 1 {
		t.Errorf("workspaces lost during rename: got %d, want 1", len(loaded.Workspaces))
	}
}

func TestFileStore_Rename_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	for _, name := range []string{"alpha", "beta"} {
		l := &model.Layout{Name: name, Version: 1, SavedAt: time.Now().UTC()}
		if err := store.Save(name, l); err != nil {
			t.Fatalf("save %s: %v", name, err)
		}
	}

	err = store.Rename("alpha", "beta")
	if err == nil {
		t.Fatal("expected error renaming to existing layout")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want 'already exists'", err.Error())
	}
	// Both originals should still exist.
	if !store.Exists("alpha") {
		t.Error("alpha should still exist")
	}
	if !store.Exists("beta") {
		t.Error("beta should still exist")
	}
}

func TestFileStore_Rename_NotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	err = store.Rename("nonexistent", "whatever")
	if err == nil {
		t.Fatal("expected error renaming nonexistent layout")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err.Error())
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

func TestValidateName_RejectsTraversal(t *testing.T) {
	bad := []string{
		"../../etc/config",
		"../secret",
		"foo/bar",
		"foo/../bar",
		"",
	}
	for _, name := range bad {
		err := validateName(name)
		if err == nil {
			t.Errorf("validateName(%q) should have failed", name)
		}
		if err != nil && !errors.Is(err, ErrInvalidName) {
			t.Errorf("validateName(%q) error = %v, want ErrInvalidName", name, err)
		}
	}

	good := []string{"my-layout", "dev_session", "project.2026"}
	for _, name := range good {
		if err := validateName(name); err != nil {
			t.Errorf("validateName(%q) unexpected error: %v", name, err)
		}
	}
}

func TestFileStore_TraversalBlocked(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	layout := &model.Layout{Name: "x", Version: 1, SavedAt: time.Now().UTC()}

	// Save with traversal name must fail.
	if err := store.Save("../../etc/evil", layout); err == nil {
		t.Fatal("Save should reject traversal name")
	}

	// Load with traversal name must fail.
	if _, err := store.Load("../secret"); err == nil {
		t.Fatal("Load should reject traversal name")
	}

	// Delete with traversal name must fail.
	if err := store.Delete("foo/bar"); err == nil {
		t.Fatal("Delete should reject traversal name")
	}

	// Exists with traversal name must return false.
	if store.Exists("../../etc/passwd") {
		t.Fatal("Exists should return false for traversal name")
	}
}

func TestFileStore_SaveFilePermissions(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	layout := &model.Layout{Name: "perms", Version: 1, SavedAt: time.Now().UTC()}
	if err := store.Save("perms", layout); err != nil {
		t.Fatalf("save: %v", err)
	}

	info, err := os.Stat(store.Path("perms"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	// File should be 0600 (owner read/write only).
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("file permission = %o, want 0600", perm)
	}
}

func TestFileStore_List_WorkspaceDetails(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	layout := &model.Layout{
		Name:    "details",
		Version: 1,
		SavedAt: time.Now().UTC(),
		Workspaces: []model.Workspace{
			{
				Title: "0 dev",
				CWD:   "/tmp/dev",
				Panes: []model.Pane{
					{Type: "terminal"},
				},
			},
			{
				Title: "1 docs",
				CWD:   "/tmp/docs",
				Panes: []model.Pane{
					{Type: "terminal"},
					{Type: "terminal", Split: "right"},
				},
			},
		},
	}

	if err := store.Save("details", layout); err != nil {
		t.Fatalf("save: %v", err)
	}

	metas, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(metas) != 1 {
		t.Fatalf("expected 1 meta, got %d", len(metas))
	}

	m := metas[0]
	if m.WorkspaceCount != 2 {
		t.Errorf("WorkspaceCount = %d, want 2", m.WorkspaceCount)
	}

	if len(m.WorkspaceTitles) != 2 {
		t.Fatalf("WorkspaceTitles len = %d, want 2", len(m.WorkspaceTitles))
	}
	if m.WorkspaceTitles[0] != "0 dev" {
		t.Errorf("WorkspaceTitles[0] = %q, want %q", m.WorkspaceTitles[0], "0 dev")
	}
	if m.WorkspaceTitles[1] != "1 docs" {
		t.Errorf("WorkspaceTitles[1] = %q, want %q", m.WorkspaceTitles[1], "1 docs")
	}

	if len(m.WorkspacePanes) != 2 {
		t.Fatalf("WorkspacePanes len = %d, want 2", len(m.WorkspacePanes))
	}
	if m.WorkspacePanes[0] != 1 {
		t.Errorf("WorkspacePanes[0] = %d, want 1", m.WorkspacePanes[0])
	}
	if m.WorkspacePanes[1] != 2 {
		t.Errorf("WorkspacePanes[1] = %d, want 2", m.WorkspacePanes[1])
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

func TestValidateName_RejectsControlCharacters(t *testing.T) {
	// A newline in the name splits the TOML header comment, producing a
	// layout that saves fine but can never be loaded (write-only) and is
	// silently hidden by List (2026-07-11 audit, M3).
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a\nb", "a\rb", "a\x00b", "a\tb"} {
		if err := store.Save(name, &model.Layout{Name: name, Version: 1}); err == nil {
			t.Errorf("Save(%q) should reject control characters", name)
		}
	}
}

func TestFileStore_BrowserProfileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	layout := &model.Layout{
		Name:    "profiles",
		Version: 1,
		SavedAt: time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC),
		Workspaces: []model.Workspace{
			{
				Title: "0 dev",
				CWD:   "/tmp/project",
				Panes: []model.Pane{
					{Type: "terminal", Focus: true},
					{Type: "browser", Split: "right", URL: "http://localhost:3000", Profile: "work-admin"},
					{
						Type: "terminal", Split: "down",
						Surfaces: []model.Surface{
							{Type: "browser", URL: "http://localhost:3000/user", Profile: "work-user"},
						},
					},
				},
			},
		},
	}

	if err := store.Save("profiles", layout); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := store.Load("profiles")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	panes := got.Workspaces[0].Panes
	if panes[1].Profile != "work-admin" {
		t.Errorf("pane profile = %q, want %q", panes[1].Profile, "work-admin")
	}
	if panes[2].Surfaces[0].Profile != "work-user" {
		t.Errorf("surface profile = %q, want %q", panes[2].Surfaces[0].Profile, "work-user")
	}
	// Terminal panes must not carry a profile field in the TOML.
	data, err := os.ReadFile(filepath.Join(dir, "profiles.toml"))
	if err != nil {
		t.Fatalf("read toml: %v", err)
	}
	if n := strings.Count(string(data), "profile ="); n != 2 {
		t.Errorf("TOML contains %d profile fields, want 2:\n%s", n, data)
	}
}
