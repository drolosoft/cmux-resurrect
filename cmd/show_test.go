package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

// showSavedAt is a fixed save time so the "Saved ..." line is predictable.
var showSavedAt = time.Date(2026, 10, 13, 9, 30, 0, 0, time.UTC)

// runShowFixture saves layout into a fresh layouts dir and returns the output
// of `crex show <name>`, with the locale-dependent save time replaced by a
// placeholder so the expected text does not depend on the machine's zone.
func runShowFixture(t *testing.T, layout *model.Layout) string {
	t.Helper()
	useCmuxUnits(t)
	dir, _ := setupTestConfig(t)

	store, err := persist.NewFileStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if err := store.Save(layout.Name, layout); err != nil {
		t.Fatalf("save layout: %v", err)
	}

	out, err := executeCmd(t, "--layouts-dir", dir, "show", layout.Name)
	layoutsDir = ""
	if err != nil {
		t.Fatalf("show: %v", err)
	}

	saved := showSavedAt.Local().Format("Jan 02, 2006 15:04")
	return strings.ReplaceAll(out, saved, "SAVED")
}

// plainShowLayout is a layout without groups, covering every pane kind the
// renderer distinguishes.
func plainShowLayout() *model.Layout {
	return &model.Layout{
		Name:        "plain",
		Description: "no groups here",
		Version:     1,
		SavedAt:     showSavedAt,
		Workspaces: []model.Workspace{
			{
				Title:       "api",
				Description: "the backend",
				CWD:         "/srv/api",
				Pinned:      true,
				Active:      true,
				Index:       0,
				Panes: []model.Pane{
					{Type: "terminal", Command: "go run ./cmd/api", Focus: true},
					{Type: "browser", Split: "right", URL: "http://localhost:8080"},
				},
			},
			{
				Title: "notes",
				CWD:   "/tmp",
				Index: 1,
				Panes: []model.Pane{{Type: "terminal"}},
			},
		},
	}
}

// TestShow_NoGroupsIsByteIdentical pins the output of a layout without
// groups to what show printed before groups existed.
func TestShow_NoGroupsIsByteIdentical(t *testing.T) {
	got := runShowFixture(t, plainShowLayout())

	want := "\n        \n📦 plain\n   no groups here\n   Saved SAVED · 2 workspaces\n\n" +
		"   api 📌 ◀ active\n   the backend\n   cwd /srv/api\n" +
		"   ├── go run ./cmd/api ★\n   └── →right 🌐 http://localhost:8080\n\n" +
		"   notes\n   cwd /tmp\n   └── shell\n\n"
	if got != want {
		t.Errorf("show output changed for a layout without groups:\n%q\nwant\n%q", got, want)
	}
}

func TestShow_GroupedLayout(t *testing.T) {
	layout := &model.Layout{
		Name:    "grouped",
		Version: 1,
		SavedAt: showSavedAt,
		Groups: []model.Group{
			{Name: "🎮 Cmux", Pinned: true, Collapsed: true},
			{Name: "Docs"},
		},
		Workspaces: []model.Workspace{
			{Title: "crex", CWD: "/src/crex", Index: 2, Group: "🎮 Cmux", Panes: []model.Pane{{Type: "terminal"}}},
			{Title: "Group 7", CWD: "/src", Index: 1, Group: "🎮 Cmux", GroupAnchor: true, Panes: []model.Pane{{Type: "terminal"}}},
			{Title: "inbox", CWD: "/tmp", Index: 0, Panes: []model.Pane{{Type: "terminal"}}},
			{Title: "manual", CWD: "/doc", Index: 3, Group: "Docs", GroupAnchor: true, Panes: []model.Pane{{Type: "terminal"}}},
		},
	}

	got := runShowFixture(t, layout)

	// The loose workspace comes first at today's indent, then each group
	// header, its anchor tagged and first, then its members one level in.
	wantLines := []string{
		"   inbox",
		"   🎮 Cmux 📌 ▸ collapsed",
		"     (anchor)",
		"     cwd /src",
		"     └── shell",
		"     crex",
		"     cwd /src/crex",
		"   Docs",
		"     manual anchor",
	}

	lines := strings.Split(got, "\n")
	cursor := 0
	for _, line := range lines {
		if cursor < len(wantLines) && line == wantLines[cursor] {
			cursor++
		}
	}
	if cursor != len(wantLines) {
		t.Errorf("grouped show missing line %q (in order); got:\n%s", wantLines[cursor], got)
	}
}

// TestShow_RawIsUntouched guards --raw, which must keep dumping the file
// as is now that the default view regroups workspaces.
func TestShow_RawIsUntouched(t *testing.T) {
	dir, _ := setupTestConfig(t)
	path := filepath.Join(dir, "raw.toml")
	content := "name = \"raw\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	out, err := executeCmd(t, "--layouts-dir", dir, "show", "raw", "--raw")
	layoutsDir = ""
	showRaw = false
	if err != nil {
		t.Fatalf("show --raw: %v", err)
	}
	if out != content {
		t.Errorf("raw output = %q, want %q", out, content)
	}
}
