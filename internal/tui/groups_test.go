package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

// groupedTUIBackend answers the reads the TUI's now command makes, plus
// ListGroups. The embedded interfaces are nil, so any write panics the test.
type groupedTUIBackend struct {
	client.Backend
	client.GroupManager

	window client.TreeWindow
	groups []client.WorkspaceGroup
}

// Tree returns a single-window tree.
func (b *groupedTUIBackend) Tree() (*client.TreeResponse, error) {
	return &client.TreeResponse{Windows: []client.TreeWindow{b.window}}, nil
}

// SidebarState reports no cwd, which keeps the expected lines short.
func (b *groupedTUIBackend) SidebarState(string) (*client.SidebarState, error) {
	return &client.SidebarState{}, nil
}

// ListGroups returns the fixture groups.
func (b *groupedTUIBackend) ListGroups(string) ([]client.WorkspaceGroup, error) {
	return b.groups, nil
}

// groupedTUILayout is the same shape the CLI show test uses: a loose
// workspace, then a pinned collapsed group whose anchor is stored after its
// member, then a second group.
func groupedTUILayout() *model.Layout {
	return &model.Layout{
		Name:    "grouped",
		SavedAt: time.Now(),
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
}

// storeWith saves layout into a fresh store.
func storeWith(t *testing.T, layout *model.Layout) persist.Store {
	t.Helper()
	store, err := persist.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	if err := store.Save(layout.Name, layout); err != nil {
		t.Fatalf("Save: %v", err)
	}

	return store
}

// assertLinesInOrder fails unless every wanted line appears in text as a
// whole line, in the given order.
func assertLinesInOrder(t *testing.T, text string, want []string) {
	t.Helper()
	cursor := 0
	for _, line := range strings.Split(text, "\n") {
		if cursor < len(want) && line == want[cursor] {
			cursor++
		}
	}
	if cursor != len(want) {
		t.Errorf("missing line %q (in order); got:\n%s", want[cursor], text)
	}
}

func TestShellShow_GroupedLayout(t *testing.T) {
	m := NewShellModel(storeWith(t, groupedTUILayout()), nil, client.BackendCmux, "", "dev")

	m.execShow("grouped")

	assertLinesInOrder(t, m.output.String(), []string{
		"  inbox",
		"  🎮 Cmux 📌 ▸ collapsed",
		"    Group 7 anchor",
		"    cwd /src",
		"    └── shell",
		"    crex",
		"  Docs",
		"    manual anchor",
	})
}

func TestShellShow_NoGroupsUnchanged(t *testing.T) {
	layout := groupedTUILayout()
	layout.Groups = nil
	m := NewShellModel(storeWith(t, layout), nil, client.BackendCmux, "", "dev")

	m.execShow("grouped")

	// Without [[group]] the stale group names are ignored and the
	// workspaces print in file order at the old indent, as before groups.
	assertLinesInOrder(t, m.output.String(), []string{
		"  crex",
		"  cwd /src/crex",
		"  └── shell",
		"",
		"  Group 7",
		"  inbox",
		"  manual",
	})
	if strings.Contains(m.output.String(), "anchor") {
		t.Error("a layout without groups should not tag anchors")
	}
}

func TestShellNow_GroupedLikeCLI(t *testing.T) {
	pane := []client.TreePane{{Ref: "pane:1"}}
	backend := &groupedTUIBackend{
		window: client.TreeWindow{
			Ref: "window:1",
			Workspaces: []client.TreeWorkspace{
				{Ref: "workspace:1", Title: "inbox", Index: 0, Pinned: true, Panes: pane},
				{Ref: "workspace:7", Title: "Group 7", Index: 1, Selected: true, Panes: pane},
				{Ref: "workspace:8", Title: "crex", Index: 2, Panes: pane},
			},
		},
		groups: []client.WorkspaceGroup{{
			Name:       "🎮 Cmux",
			AnchorRef:  "workspace:7",
			MemberRefs: []string{"workspace:7", "workspace:8"},
			Pinned:     true,
			Collapsed:  true,
		}},
	}
	m := NewShellModel(nil, backend, client.BackendCmux, "", "dev")

	m.execNow()

	assertLinesInOrder(t, m.output.String(), []string{
		"  inbox 📌  1 pane(s)",
		"  🎮 Cmux 📌 ▸ collapsed",
		"    Group 7 anchor ●  1 pane(s)",
		"    crex  1 pane(s)",
	})
}

func TestItemsFromLayouts_GroupedSubItems(t *testing.T) {
	layout := groupedTUILayout()
	store := storeWith(t, layout)
	metas, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	items := ItemsFromLayouts(metas)
	subItems := items[0].SubItems

	var names []string
	for _, item := range subItems {
		names = append(names, item.Name)
	}
	want := []string{"inbox", "Group 7", "crex", "manual"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("sub-items = %v, want %v", names, want)
	}

	if subItems[0].Group != nil {
		t.Error("inbox should be loose")
	}
	if subItems[1].Group == nil || subItems[1].Group.Name != "🎮 Cmux" || !subItems[1].Anchor {
		t.Errorf("Group 7 = %+v, want anchor of 🎮 Cmux", subItems[1])
	}
	if subItems[2].Anchor {
		t.Error("crex is a member, not the anchor")
	}
}

func TestBrowseDetail_ShowsGroupHeaders(t *testing.T) {
	metas, err := storeWith(t, groupedTUILayout()).List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	bm := NewBrowseModel(ItemsFromLayouts(metas), "restore")

	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRight})
	view := bm.View()

	// Headers are display-only lines: the numbering keeps counting the
	// pickable rows, so [2] is still the first workspace after "All".
	for _, want := range []string{"🎮 Cmux 📌 ▸ collapsed", "[3]   Group 7 anchor", "[4]   crex", "Docs", "[5]   manual anchor"} {
		if !strings.Contains(view, want) {
			t.Errorf("detail view missing %q:\n%s", want, view)
		}
	}
	if strings.Index(view, "🎮 Cmux") > strings.Index(view, "Group 7") {
		t.Error("the group header must come before its anchor")
	}

	// Picking [3] still selects the anchor workspace by title.
	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	if got := bm.SelectedItem().Name; got != "Group 7" {
		t.Errorf("key 3 selected %q, want Group 7", got)
	}
}

func TestBrowseDetail_NoGroupsUnchanged(t *testing.T) {
	items := []Item{{
		Kind:     KindLayout,
		Name:     "plain",
		SubItems: []Item{{Kind: KindWorkspace, Name: "a", Workspaces: 1}, {Kind: KindWorkspace, Name: "b", Workspaces: 2}},
	}}
	bm := NewBrowseModel(items, "restore")

	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRight})
	view := bm.View()

	// Two blank-free lines of rows, no header in between.
	if !strings.Contains(view, "    [2] a  1 pane\n    [3] b  2 panes\n") {
		t.Errorf("plain detail view changed:\n%q", view)
	}
}
