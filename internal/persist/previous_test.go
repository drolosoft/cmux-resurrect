package persist

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/model"
)

// newPreviousTestStore returns a FileStore in a fresh temporary directory.
func newPreviousTestStore(t *testing.T) *FileStore {
	t.Helper()

	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	return store
}

// layoutWithCWD builds a one-workspace layout whose only variable part is the
// working directory, so tests can make a real change by passing another path.
func layoutWithCWD(name, cwd string, savedAt time.Time) *model.Layout {
	return &model.Layout{
		Name:     name,
		Version:  1,
		Revision: 1,
		SavedAt:  savedAt,
		Workspaces: []model.Workspace{
			{Title: "dev", CWD: cwd, Panes: []model.Pane{{Type: "terminal"}}},
		},
	}
}

// readFile returns the bytes of path or fails the test.
func readFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

// assertNoFile fails the test when path exists.
func assertNoFile(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("%s exists, want no file (stat err: %v)", filepath.Base(path), err)
	}
}

func TestFileStore_SaveChangedKeepsPrevious(t *testing.T) {
	store := newPreviousTestStore(t)
	first := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)

	if err := store.Save("now", layoutWithCWD("now", "/tmp/first", first)); err != nil {
		t.Fatalf("first save: %v", err)
	}
	firstContent := readFile(t, store.Path("now"))

	changed := layoutWithCWD("now", "/tmp/second", first.Add(time.Hour))
	changed.Revision = 2
	if err := store.Save("now", changed); err != nil {
		t.Fatalf("second save: %v", err)
	}

	previous := readFile(t, store.PreviousPath("now"))
	if !bytes.Equal(previous, firstContent) {
		t.Errorf("previous copy differs from the first version:\n got: %s\nwant: %s", previous, firstContent)
	}

	loaded, err := store.Load("now")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Workspaces[0].CWD != "/tmp/second" {
		t.Errorf("current CWD = %q, want /tmp/second", loaded.Workspaces[0].CWD)
	}

	info, err := os.Stat(store.PreviousPath("now"))
	if err != nil {
		t.Fatalf("stat previous: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("previous copy permission = %o, want 0600", perm)
	}
}

func TestFileStore_SaveUnchangedLeavesPreviousAlone(t *testing.T) {
	store := newPreviousTestStore(t)
	first := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)

	// Two different saves, so a good previous copy exists to be protected.
	if err := store.Save("now", layoutWithCWD("now", "/tmp/good", first)); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if err := store.Save("now", layoutWithCWD("now", "/tmp/current", first.Add(time.Hour))); err != nil {
		t.Fatalf("second save: %v", err)
	}

	previousPath := store.PreviousPath("now")
	hashBefore := sha256.Sum256(readFile(t, previousPath))
	infoBefore, err := os.Stat(previousPath)
	if err != nil {
		t.Fatalf("stat previous: %v", err)
	}

	// Make sure a rewrite would show up as a different modification time.
	time.Sleep(20 * time.Millisecond)

	// Same content: only saved_at and the description differ, as with an
	// autosave that finds nothing new.
	identical := layoutWithCWD("now", "/tmp/current", first.Add(2*time.Hour))
	identical.Description = "autosave"
	if err := store.Save("now", identical); err != nil {
		t.Fatalf("identical save: %v", err)
	}

	hashAfter := sha256.Sum256(readFile(t, previousPath))
	if hashAfter != hashBefore {
		t.Error("an identical save rewrote the previous copy")
	}

	infoAfter, err := os.Stat(previousPath)
	if err != nil {
		t.Fatalf("stat previous after: %v", err)
	}
	if !infoAfter.ModTime().Equal(infoBefore.ModTime()) {
		t.Errorf("previous mtime changed: %v -> %v", infoBefore.ModTime(), infoAfter.ModTime())
	}
}

func TestFileStore_FirstSaveCreatesNoPrevious(t *testing.T) {
	store := newPreviousTestStore(t)

	if err := store.Save("fresh", layoutWithCWD("fresh", "/tmp", time.Now().UTC())); err != nil {
		t.Fatalf("save: %v", err)
	}

	assertNoFile(t, store.PreviousPath("fresh"))
}

func TestFileStore_SaveFailsWhenPreviousCannotBeWritten(t *testing.T) {
	store := newPreviousTestStore(t)
	first := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)

	if err := store.Save("now", layoutWithCWD("now", "/tmp/first", first)); err != nil {
		t.Fatalf("first save: %v", err)
	}
	firstContent := readFile(t, store.Path("now"))

	// A non-empty directory where the copy should go makes the copy fail on
	// any OS, since a rename cannot replace it.
	blocker := store.PreviousPath("now")
	if err := os.Mkdir(blocker, 0o700); err != nil {
		t.Fatalf("block previous path: %v", err)
	}
	if err := os.WriteFile(filepath.Join(blocker, "keep"), []byte("x"), 0o600); err != nil {
		t.Fatalf("fill blocker: %v", err)
	}

	err := store.Save("now", layoutWithCWD("now", "/tmp/second", first.Add(time.Hour)))
	if err == nil {
		t.Fatal("save succeeded although the previous copy could not be written")
	}

	if got := readFile(t, store.Path("now")); !bytes.Equal(got, firstContent) {
		t.Error("a failed save changed the layout file")
	}

	matches, _ := filepath.Glob(filepath.Join(store.Dir, "*.tmp"))
	if len(matches) > 0 {
		t.Errorf("temp files left behind: %v", matches)
	}
}

func TestFileStore_DeleteKeepsPrevious(t *testing.T) {
	store := newPreviousTestStore(t)

	if err := store.Save("gone", layoutWithCWD("gone", "/tmp/gone", time.Now().UTC())); err != nil {
		t.Fatalf("save: %v", err)
	}
	content := readFile(t, store.Path("gone"))

	if err := store.Delete("gone"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if store.Exists("gone") {
		t.Error("layout still exists after delete")
	}
	if got := readFile(t, store.PreviousPath("gone")); !bytes.Equal(got, content) {
		t.Error("previous copy is not the deleted layout")
	}

	metas, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, meta := range metas {
		if meta.Name == "gone" {
			t.Error("deleted layout still appears in List")
		}
	}
}

func TestFileStore_DeleteReplacesOlderPrevious(t *testing.T) {
	store := newPreviousTestStore(t)
	first := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)

	if err := store.Save("gone", layoutWithCWD("gone", "/tmp/old", first)); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if err := store.Save("gone", layoutWithCWD("gone", "/tmp/last", first.Add(time.Hour))); err != nil {
		t.Fatalf("second save: %v", err)
	}
	lastContent := readFile(t, store.Path("gone"))

	if err := store.Delete("gone"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if got := readFile(t, store.PreviousPath("gone")); !bytes.Equal(got, lastContent) {
		t.Error("previous copy after delete is not the last saved version")
	}
}

func TestFileStore_RenameMovesPrevious(t *testing.T) {
	store := newPreviousTestStore(t)
	first := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)

	if err := store.Save("old", layoutWithCWD("old", "/tmp/first", first)); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if err := store.Save("old", layoutWithCWD("old", "/tmp/second", first.Add(time.Hour))); err != nil {
		t.Fatalf("second save: %v", err)
	}

	if err := store.Rename("old", "new"); err != nil {
		t.Fatalf("rename: %v", err)
	}

	assertNoFile(t, store.PreviousPath("old"))

	// The moved copy must be the first version, but under the new name, so
	// copying it back gives a layout that lists and restores as "new".
	previous := readFile(t, store.PreviousPath("new"))
	if err := os.WriteFile(store.Path("new"), previous, 0o600); err != nil {
		t.Fatalf("restore previous: %v", err)
	}

	restored, err := store.Load("new")
	if err != nil {
		t.Fatalf("load restored: %v", err)
	}
	if restored.Name != "new" {
		t.Errorf("restored Name = %q, want %q", restored.Name, "new")
	}
	if restored.Workspaces[0].CWD != "/tmp/first" {
		t.Errorf("restored CWD = %q, want the first version's /tmp/first", restored.Workspaces[0].CWD)
	}
	if !strings.HasPrefix(string(previous), "# crex layout: new\n") {
		t.Errorf("moved copy header does not name the new layout:\n%s", previous)
	}

	info, err := os.Stat(store.PreviousPath("new"))
	if err != nil {
		t.Fatalf("stat moved previous: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("moved previous permission = %o, want 0600", perm)
	}
}

func TestFileStore_RenameWithoutPrevious(t *testing.T) {
	store := newPreviousTestStore(t)

	if err := store.Save("old", layoutWithCWD("old", "/tmp", time.Now().UTC())); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := store.Rename("old", "new"); err != nil {
		t.Fatalf("rename: %v", err)
	}

	assertNoFile(t, store.PreviousPath("old"))
	assertNoFile(t, store.PreviousPath("new"))
}

func TestFileStore_ListIgnoresPrevious(t *testing.T) {
	store := newPreviousTestStore(t)
	first := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)

	if err := store.Save("kept", layoutWithCWD("kept", "/tmp/first", first)); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if err := store.Save("kept", layoutWithCWD("kept", "/tmp/second", first.Add(time.Hour))); err != nil {
		t.Fatalf("second save: %v", err)
	}
	if err := store.Save("gone", layoutWithCWD("gone", "/tmp", first)); err != nil {
		t.Fatalf("save gone: %v", err)
	}
	if err := store.Delete("gone"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	metas, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(metas) != 1 || metas[0].Name != "kept" {
		t.Errorf("List = %+v, want only \"kept\"", metas)
	}
}
