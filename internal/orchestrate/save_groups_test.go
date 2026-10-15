package orchestrate

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

// groupSaveClient is a cmux-like backend with sidebar groups. It wraps the
// plain mockClient so the tree and sidebar answers stay the same, and records
// which window save asked the groups of.
type groupSaveClient struct {
	*mockClient
	groups       []client.WorkspaceGroup
	listErr      error
	listedWindow string
	listCalls    int
}

// ListGroups answers with the configured groups, or the configured error, and
// remembers the window it was asked about.
func (c *groupSaveClient) ListGroups(windowRef string) ([]client.WorkspaceGroup, error) {
	c.listCalls++
	c.listedWindow = windowRef

	if c.listErr != nil {
		return nil, c.listErr
	}

	return c.groups, nil
}

// The mutating group calls belong to restore; save must never reach them.
func (c *groupSaveClient) CreateGroup(name string, fromRefs []string) (client.WorkspaceGroup, error) {
	return client.WorkspaceGroup{}, nil
}
func (c *groupSaveClient) SetGroupAnchor(groupRef, workspaceRef string) error { return nil }
func (c *groupSaveClient) AddToGroup(groupRef, workspaceRef string) error     { return nil }
func (c *groupSaveClient) RenameGroup(groupRef, name string) error            { return nil }
func (c *groupSaveClient) CollapseGroup(groupRef string) error                { return nil }
func (c *groupSaveClient) PinGroup(groupRef string) error                     { return nil }
func (c *groupSaveClient) ExpandGroup(groupRef string) error                  { return nil }
func (c *groupSaveClient) UnpinGroup(groupRef string) error                   { return nil }
func (c *groupSaveClient) SetGroupColor(groupRef, hex string) error           { return nil }
func (c *groupSaveClient) SetGroupIcon(groupRef, symbol string) error         { return nil }

// groupTestWorkspace builds a one-terminal tree workspace. The surface has a
// tty so the ghost-workspace dedupe treats it as a real workspace.
func groupTestWorkspace(ref, title string, index int) client.TreeWorkspace {
	return client.TreeWorkspace{
		Ref:   ref,
		Title: title,
		Index: index,
		Panes: []client.TreePane{{
			Index:    0,
			Surfaces: []client.TreeSurface{{Ref: "surface-of-" + ref, Type: "terminal"}},
		}},
	}
}

// groupTestTree puts the given workspaces in one current window.
func groupTestTree(workspaces ...client.TreeWorkspace) *client.TreeResponse {
	return &client.TreeResponse{
		Windows: []client.TreeWindow{{Ref: "window:1", Current: true, Workspaces: workspaces}},
	}
}

// newGroupSaver wires a Saver over a fresh store in a temporary directory.
func newGroupSaver(t *testing.T, backend client.Backend) (*Saver, *persist.FileStore) {
	t.Helper()

	store, err := persist.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	return &Saver{Client: backend, Store: store}, store
}

// captureStderr runs the function with os.Stderr redirected and returns what
// it printed, because save reports soft failures there and nowhere else.
func captureStderr(t *testing.T, run func()) string {
	t.Helper()

	original := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = writer
	defer func() {
		os.Stderr = original
	}()

	run()

	// Closing the write end is what lets ReadAll see EOF; a failure here
	// would make the read hang, so it fails the test instead.
	if err := writer.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	printed, _ := io.ReadAll(reader)

	return string(printed)
}

// workspaceTitled finds a saved workspace by title and group, failing the test
// when it is missing.
func workspaceTitled(t *testing.T, layout *model.Layout, group, title string) model.Workspace {
	t.Helper()

	for _, workspace := range layout.Workspaces {
		if workspace.Title == title && workspace.Group == group {
			return workspace
		}
	}
	t.Fatalf("no workspace %q in group %q among %+v", title, group, layout.Workspaces)

	return model.Workspace{}
}

// groupNames lists the saved group names in order.
func groupNames(layout *model.Layout) []string {
	var names []string
	for _, group := range layout.Groups {
		names = append(names, group.Name)
	}

	return names
}

func TestSaveGroups_ListsTheGroupsOfTheCallersWindow(t *testing.T) {
	tree := &client.TreeResponse{
		Caller: &client.CallerInfo{WindowRef: "window:5"},
		Windows: []client.TreeWindow{
			{Ref: "window:1", Current: true, Workspaces: []client.TreeWorkspace{groupTestWorkspace("workspace:1", "focused", 0)}},
			{Ref: "window:5", Workspaces: []client.TreeWorkspace{groupTestWorkspace("workspace:5", "mine", 0)}},
		},
	}
	backend := &groupSaveClient{mockClient: &mockClient{treeResp: tree}}
	saver, _ := newGroupSaver(t, backend)

	if _, err := saver.Save("callerwin", ""); err != nil {
		t.Fatalf("save: %v", err)
	}

	if backend.listCalls != 1 {
		t.Fatalf("ListGroups called %d times, want 1", backend.listCalls)
	}
	if backend.listedWindow != "window:5" {
		t.Errorf("groups listed for %q, want the caller's window:5", backend.listedWindow)
	}
}

func TestSaveGroups_MarksMembersAndTheAnchor(t *testing.T) {
	tree := groupTestTree(
		groupTestWorkspace("workspace:1", "loose", 0),
		groupTestWorkspace("workspace:2", "Group 7", 1),
		groupTestWorkspace("workspace:3", "api", 2),
	)
	backend := &groupSaveClient{
		mockClient: &mockClient{treeResp: tree},
		groups: []client.WorkspaceGroup{{
			Ref:        "workspace_group:1",
			Name:       "backend",
			AnchorRef:  "workspace:2",
			MemberRefs: []string{"workspace:2", "workspace:3"},
		}},
	}
	saver, _ := newGroupSaver(t, backend)

	layout, err := saver.Save("members", "")
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	loose := workspaceTitled(t, layout, "", "loose")
	if loose.GroupAnchor {
		t.Error("a loose workspace must not be an anchor")
	}
	anchor := workspaceTitled(t, layout, "backend", "backend")
	if !anchor.GroupAnchor {
		t.Error("the group's anchor lost GroupAnchor")
	}
	member := workspaceTitled(t, layout, "backend", "api")
	if member.GroupAnchor {
		t.Error("a plain member was marked as anchor")
	}
}

// TestSaveGroups_AnchorTitles: an anchor still carrying cmux's placeholder,
// or no title at all, is stored under its group's name, so the file, `crex
// show` and a title filter on restore all speak of the group the user named.
// An anchor the user renamed keeps that name.
func TestSaveGroups_AnchorTitles(t *testing.T) {
	cases := []struct {
		name      string
		liveTitle string
		want      string
	}{
		{"placeholder", "Group 3", "Backend"},
		{"empty", "", "Backend"},
		{"chosen by the user", "API gateway", "API gateway"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			tree := groupTestTree(
				groupTestWorkspace("workspace:1", testCase.liveTitle, 0),
				groupTestWorkspace("workspace:2", "api", 1),
			)
			backend := &groupSaveClient{
				mockClient: &mockClient{treeResp: tree},
				groups: []client.WorkspaceGroup{{
					Ref:        "workspace_group:1",
					Name:       "Backend",
					AnchorRef:  "workspace:1",
					MemberRefs: []string{"workspace:1", "workspace:2"},
				}},
			}
			saver, store := newGroupSaver(t, backend)

			if _, err := saver.Save("anchor-title", ""); err != nil {
				t.Fatalf("save: %v", err)
			}

			stored, err := store.Load("anchor-title")
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			anchor := workspaceTitled(t, stored, "Backend", testCase.want)
			if !anchor.GroupAnchor {
				t.Error("the stored anchor lost GroupAnchor")
			}
		})
	}
}

// TestSaveGroups_ReSaveOverOldPlaceholderAnchorKeepsItsEdits: a layout saved
// by v1.28.0 stored the anchor as "Group 2". Re-saving it now, with the live
// anchor at another placeholder number, must still pair the two and carry the
// anchor's description and command over, while the title becomes the group
// name.
func TestSaveGroups_ReSaveOverOldPlaceholderAnchorKeepsItsEdits(t *testing.T) {
	tree := groupTestTree(
		groupTestWorkspace("workspace:1", "Group 5", 0),
		groupTestWorkspace("workspace:2", "api", 1),
	)
	backend := &groupSaveClient{
		mockClient: &mockClient{treeResp: tree},
		groups: []client.WorkspaceGroup{{
			Ref:        "workspace_group:1",
			Name:       "Backend",
			AnchorRef:  "workspace:1",
			MemberRefs: []string{"workspace:1", "workspace:2"},
		}},
	}
	saver, store := newGroupSaver(t, backend)

	old := &model.Layout{
		Name:    "old-anchor",
		Version: 1,
		Groups:  []model.Group{{Name: "Backend"}},
		Workspaces: []model.Workspace{
			{
				Title: "Group 2", Description: "the anchor", Group: "Backend", GroupAnchor: true,
				Panes: []model.Pane{{Type: "terminal", Index: 0, Command: "make watch"}},
			},
			{Title: "api", Index: 1, Group: "Backend", Panes: []model.Pane{{Type: "terminal", Index: 0}}},
		},
	}
	if err := store.Save("old-anchor", old); err != nil {
		t.Fatalf("store: %v", err)
	}

	layout, err := saver.Save("old-anchor", "")
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	anchor := workspaceTitled(t, layout, "Backend", "Backend")
	if anchor.Description != "the anchor" {
		t.Errorf("anchor description = %q, want the saved one", anchor.Description)
	}
	if anchor.Panes[0].Command != "make watch" {
		t.Errorf("anchor command = %q, want the saved one", anchor.Panes[0].Command)
	}
}

func TestSaveGroups_OrdersGroupsBySidebarAndSkipsEmptyOnes(t *testing.T) {
	tree := groupTestTree(
		groupTestWorkspace("workspace:1", "web", 0),
		groupTestWorkspace("workspace:2", "docs", 1),
		groupTestWorkspace("workspace:3", "api", 2),
	)
	backend := &groupSaveClient{
		mockClient: &mockClient{treeResp: tree},
		groups: []client.WorkspaceGroup{
			{
				Ref: "workspace_group:1", Name: "backend",
				AnchorRef: "workspace:3", MemberRefs: []string{"workspace:3"},
				Collapsed: true, Color: "#FF0000",
			},
			{
				Ref: "workspace_group:2", Name: "frontend",
				AnchorRef: "workspace:1", MemberRefs: []string{"workspace:1"},
				Pinned: true, Icon: "flame",
			},
			{
				// Every member of this group lives outside the saved window.
				Ref: "workspace_group:3", Name: "elsewhere",
				AnchorRef: "workspace:9", MemberRefs: []string{"workspace:9"},
			},
		},
	}
	saver, _ := newGroupSaver(t, backend)

	layout, err := saver.Save("ordered", "")
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	got := strings.Join(groupNames(layout), ",")
	if got != "frontend,backend" {
		t.Fatalf("groups = %q, want frontend,backend (sidebar order, empty group dropped)", got)
	}
	frontend := layout.Groups[0]
	if !frontend.Pinned || frontend.Icon != "flame" || frontend.Collapsed || frontend.Color != "" {
		t.Errorf("frontend state not copied: %+v", frontend)
	}
	backendGroup := layout.Groups[1]
	if !backendGroup.Collapsed || backendGroup.Color != "#FF0000" || backendGroup.Pinned || backendGroup.Icon != "" {
		t.Errorf("backend state not copied: %+v", backendGroup)
	}
}

func TestSaveGroups_RenamesDuplicateGroupNames(t *testing.T) {
	tree := groupTestTree(
		groupTestWorkspace("workspace:1", "one", 0),
		groupTestWorkspace("workspace:2", "two", 1),
		groupTestWorkspace("workspace:3", "three", 2),
	)
	backend := &groupSaveClient{
		mockClient: &mockClient{treeResp: tree},
		groups: []client.WorkspaceGroup{
			// Listed out of sidebar order on purpose: the suffix follows the
			// sidebar, not the order cmux happens to report.
			{Ref: "workspace_group:2", Name: "work", AnchorRef: "workspace:2", MemberRefs: []string{"workspace:2"}},
			{Ref: "workspace_group:1", Name: "work", AnchorRef: "workspace:1", MemberRefs: []string{"workspace:1"}},
			{Ref: "workspace_group:3", Name: "work", AnchorRef: "workspace:3", MemberRefs: []string{"workspace:3"}},
		},
	}
	saver, _ := newGroupSaver(t, backend)

	var layout *model.Layout
	printed := captureStderr(t, func() {
		saved, err := saver.Save("dupes", "")
		if err != nil {
			t.Fatalf("save: %v", err)
		}
		layout = saved
	})

	got := strings.Join(groupNames(layout), ",")
	if got != "work,work (2),work (3)" {
		t.Fatalf("groups = %q, want work,work (2),work (3)", got)
	}
	workspaceTitled(t, layout, "work", "one")
	workspaceTitled(t, layout, "work (2)", "two")
	workspaceTitled(t, layout, "work (3)", "three")
	if !strings.Contains(printed, "warning") || !strings.Contains(printed, `"work (2)"`) {
		t.Errorf("no warning about the renamed group, stderr = %q", printed)
	}
}

func TestSaveGroups_ListErrorKeepsTheExistingGroups(t *testing.T) {
	tree := groupTestTree(
		groupTestWorkspace("workspace:1", "Group 7", 0),
		groupTestWorkspace("workspace:2", "api", 1),
		groupTestWorkspace("workspace:3", "brand new", 2),
	)
	backend := &groupSaveClient{
		mockClient: &mockClient{treeResp: tree},
		groups: []client.WorkspaceGroup{{
			Ref: "workspace_group:1", Name: "backend", Collapsed: true,
			AnchorRef: "workspace:1", MemberRefs: []string{"workspace:1", "workspace:2"},
		}},
	}
	saver, _ := newGroupSaver(t, backend)
	if _, err := saver.Save("listerr", ""); err != nil {
		t.Fatalf("first save: %v", err)
	}

	backend.listErr = errors.New("socket closed")
	var layout *model.Layout
	printed := captureStderr(t, func() {
		saved, err := saver.Save("listerr", "")
		if err != nil {
			t.Fatalf("save with a failing ListGroups must not fail: %v", err)
		}
		layout = saved
	})

	if !strings.Contains(printed, "warning") || !strings.Contains(printed, "socket closed") {
		t.Errorf("no warning for the failed group list, stderr = %q", printed)
	}
	if got := strings.Join(groupNames(layout), ","); got != "backend" {
		t.Fatalf("groups = %q, want the existing backend group kept", got)
	}
	if !layout.Groups[0].Collapsed {
		t.Error("the kept group lost its collapsed state")
	}
	if !workspaceTitled(t, layout, "backend", "backend").GroupAnchor {
		t.Error("the kept anchor lost GroupAnchor")
	}
	workspaceTitled(t, layout, "backend", "api")
	workspaceTitled(t, layout, "", "brand new")
	if layout.Revision != 1 {
		t.Errorf("revision = %d, want 1: nothing changed from the user's side", layout.Revision)
	}
}

func TestSaveGroups_ListErrorWithoutExistingLayoutSavesLoose(t *testing.T) {
	tree := groupTestTree(groupTestWorkspace("workspace:1", "api", 0))
	backend := &groupSaveClient{mockClient: &mockClient{treeResp: tree}, listErr: errors.New("boom")}
	saver, _ := newGroupSaver(t, backend)

	var layout *model.Layout
	printed := captureStderr(t, func() {
		saved, err := saver.Save("fresh", "")
		if err != nil {
			t.Fatalf("save: %v", err)
		}
		layout = saved
	})

	if !strings.Contains(printed, "boom") {
		t.Errorf("no warning for the failed group list, stderr = %q", printed)
	}
	if len(layout.Groups) != 0 || layout.Workspaces[0].Group != "" {
		t.Errorf("a fresh layout invented groups: %+v", layout)
	}
}

func TestSaveGroups_BackendWithoutGroupsSavesAsToday(t *testing.T) {
	tree := groupTestTree(
		groupTestWorkspace("workspace:1", "api", 0),
		groupTestWorkspace("workspace:2", "web", 1),
	)
	saver, _ := newGroupSaver(t, &mockClient{treeResp: tree})

	var layout *model.Layout
	printed := captureStderr(t, func() {
		saved, err := saver.Save("ghostty", "")
		if err != nil {
			t.Fatalf("save: %v", err)
		}
		layout = saved
	})

	if strings.Contains(printed, "group") {
		t.Errorf("a backend without groups printed a group warning: %q", printed)
	}
	if layout.Groups != nil {
		t.Errorf("Groups = %+v, want nil", layout.Groups)
	}
	for _, workspace := range layout.Workspaces {
		if workspace.Group != "" || workspace.GroupAnchor {
			t.Errorf("workspace %q got group data: %+v", workspace.Title, workspace)
		}
	}
}

// savedTOMLWithoutTimestamp reads the stored file and drops the saved_at key
// and the "# Saved at" header comment, the only lines two saves of the same
// state may disagree on when they land in different seconds.
func savedTOMLWithoutTimestamp(t *testing.T, store *persist.FileStore, name string) string {
	t.Helper()

	raw, err := os.ReadFile(store.Path(name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}

	var kept []string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "saved_at") || strings.HasPrefix(line, "# Saved at") {
			continue
		}
		kept = append(kept, line)
	}

	return strings.Join(kept, "\n")
}

func TestSaveGroups_ZeroGroupsWritesTheSameTOMLAsBefore(t *testing.T) {
	tree := groupTestTree(
		groupTestWorkspace("workspace:1", "api", 0),
		groupTestWorkspace("workspace:2", "web", 1),
	)
	plainSaver, plainStore := newGroupSaver(t, &mockClient{treeResp: tree})
	groupSaver, groupStore := newGroupSaver(t, &groupSaveClient{mockClient: &mockClient{treeResp: tree}})

	if _, err := plainSaver.Save("same", ""); err != nil {
		t.Fatalf("plain save: %v", err)
	}
	if _, err := groupSaver.Save("same", ""); err != nil {
		t.Fatalf("group save: %v", err)
	}

	plain := savedTOMLWithoutTimestamp(t, plainStore, "same")
	grouped := savedTOMLWithoutTimestamp(t, groupStore, "same")
	if plain != grouped {
		t.Errorf("zero groups changed the TOML:\n--- without groups ---\n%s\n--- with zero groups ---\n%s", plain, grouped)
	}
	if strings.Contains(grouped, "group") {
		t.Errorf("a layout without groups wrote a group key:\n%s", grouped)
	}
}

func TestDeduplicateWorkspaces_KeysByGroupAndTitle(t *testing.T) {
	realOne := groupTestWorkspace("workspace:1", "api", 0)
	realOne.Panes[0].Surfaces[0].TTY = "/dev/ttys001"
	otherGroup := groupTestWorkspace("workspace:2", "api", 1)
	otherGroup.Panes[0].Surfaces[0].TTY = "/dev/ttys002"
	ghost := groupTestWorkspace("workspace:3", "api", 2)
	looseReal := groupTestWorkspace("workspace:4", "notes", 3)
	looseReal.Panes[0].Surfaces[0].TTY = "/dev/ttys004"
	looseGhost := groupTestWorkspace("workspace:5", "notes", 4)

	membership := map[string]groupMembership{
		"workspace:1": {name: "backend"},
		"workspace:2": {name: "frontend"},
		"workspace:3": {name: "backend"},
	}

	kept := deduplicateWorkspaces([]client.TreeWorkspace{realOne, otherGroup, ghost, looseReal, looseGhost}, membership)

	var refs []string
	for _, workspace := range kept {
		refs = append(refs, workspace.Ref)
	}
	got := strings.Join(refs, ",")
	if got != "workspace:1,workspace:2,workspace:4" {
		t.Errorf("kept = %s, want both api workspaces of different groups and no ghosts", got)
	}
}

func TestDeduplicateWorkspaces_NilMembershipBehavesAsBefore(t *testing.T) {
	realOne := groupTestWorkspace("workspace:1", "api", 0)
	realOne.Panes[0].Surfaces[0].TTY = "/dev/ttys001"
	ghost := groupTestWorkspace("workspace:2", "api", 1)

	kept := deduplicateWorkspaces([]client.TreeWorkspace{ghost, realOne}, nil)

	if len(kept) != 1 || kept[0].Ref != "workspace:1" {
		t.Errorf("kept = %+v, want only the workspace with a tty", kept)
	}
}

// keptRefs joins the refs of the workspaces dedupe kept, in order.
func keptRefs(kept []client.TreeWorkspace) string {
	var refs []string
	for _, workspace := range kept {
		refs = append(refs, workspace.Ref)
	}
	return strings.Join(refs, ",")
}

// TestDeduplicateWorkspaces_KeepsLiveWorkspacesThatShareATitle: two shells in
// the same folder give two members of one group the same title. Both have a
// tty, so both are real and both must be saved.
func TestDeduplicateWorkspaces_KeepsLiveWorkspacesThatShareATitle(t *testing.T) {
	anchor := groupTestWorkspace("workspace:1", "Group 1", 0)
	anchor.Panes[0].Surfaces[0].TTY = "/dev/ttys001"
	firstShell := groupTestWorkspace("workspace:2", "txeo@Mac: ~/src/repo", 1)
	firstShell.Panes[0].Surfaces[0].TTY = "/dev/ttys002"
	secondShell := groupTestWorkspace("workspace:3", "txeo@Mac: ~/src/repo", 2)
	secondShell.Panes[0].Surfaces[0].TTY = "/dev/ttys003"

	membership := map[string]groupMembership{
		"workspace:1": {name: "Cmux", anchor: true},
		"workspace:2": {name: "Cmux"},
		"workspace:3": {name: "Cmux"},
	}

	kept := deduplicateWorkspaces([]client.TreeWorkspace{anchor, firstShell, secondShell}, membership)

	got := keptRefs(kept)
	if got != "workspace:1,workspace:2,workspace:3" {
		t.Errorf("kept = %s, want every workspace: all of them have a tty", got)
	}
}

// TestDeduplicateWorkspaces_DropsGhostWithLiveSibling: a workspace without a
// tty is only a ghost when a sibling with the same key is alive.
func TestDeduplicateWorkspaces_DropsGhostWithLiveSibling(t *testing.T) {
	firstShell := groupTestWorkspace("workspace:1", "repo", 0)
	firstShell.Panes[0].Surfaces[0].TTY = "/dev/ttys001"
	ghost := groupTestWorkspace("workspace:2", "repo", 1)
	secondShell := groupTestWorkspace("workspace:3", "repo", 2)
	secondShell.Panes[0].Surfaces[0].TTY = "/dev/ttys003"

	kept := deduplicateWorkspaces([]client.TreeWorkspace{firstShell, ghost, secondShell}, nil)

	got := keptRefs(kept)
	if got != "workspace:1,workspace:3" {
		t.Errorf("kept = %s, want both live workspaces and no ghost", got)
	}
}

// TestDeduplicateWorkspaces_KeepsOneOfSeveralGhosts: with no live sibling
// there is nothing better to save, so the first ghost stays and the rest go.
func TestDeduplicateWorkspaces_KeepsOneOfSeveralGhosts(t *testing.T) {
	firstGhost := groupTestWorkspace("workspace:1", "repo", 0)
	secondGhost := groupTestWorkspace("workspace:2", "repo", 1)
	other := groupTestWorkspace("workspace:3", "notes", 2)

	// A tty elsewhere in the window marks the tree as cmux's, the only
	// backend that reports ttys and so the only one with ghosts.
	other.Panes[0].Surfaces[0].TTY = "/dev/ttys003"

	kept := deduplicateWorkspaces([]client.TreeWorkspace{firstGhost, secondGhost, other}, nil)

	got := keptRefs(kept)
	if got != "workspace:1,workspace:3" {
		t.Errorf("kept = %s, want the first ghost and the unrelated workspace", got)
	}
}

// groupedLayout is a small layout with one group, for the change detector.
func groupedLayout() *model.Layout {
	layout := baseLayout()
	layout.Groups = []model.Group{{Name: "backend", Color: "#00FF00"}}
	layout.Workspaces[0].Group = "backend"
	layout.Workspaces[0].GroupAnchor = true

	return layout
}

func TestLayoutContentChanged_Groups(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(layout *model.Layout)
	}{
		{"group added", func(layout *model.Layout) { layout.Groups = append(layout.Groups, model.Group{Name: "web"}) }},
		{"group renamed", func(layout *model.Layout) { layout.Groups[0].Name = "api" }},
		{"group collapsed", func(layout *model.Layout) { layout.Groups[0].Collapsed = true }},
		{"group pinned", func(layout *model.Layout) { layout.Groups[0].Pinned = true }},
		{"group color", func(layout *model.Layout) { layout.Groups[0].Color = "#0000FF" }},
		{"group icon", func(layout *model.Layout) { layout.Groups[0].Icon = "flame" }},
		{"workspace left its group", func(layout *model.Layout) { layout.Workspaces[0].Group = "" }},
		{"anchor moved", func(layout *model.Layout) { layout.Workspaces[0].GroupAnchor = false }},
	}

	if layoutContentChanged(groupedLayout(), groupedLayout()) {
		t.Fatal("identical grouped layouts reported as changed")
	}
	for _, testCase := range cases {
		changed := groupedLayout()
		testCase.mutate(changed)

		if !layoutContentChanged(groupedLayout(), changed) {
			t.Errorf("%s: not detected as a change", testCase.name)
		}
	}
}

func TestSaveGroups_RevisionFollowsGroupState(t *testing.T) {
	tree := groupTestTree(
		groupTestWorkspace("workspace:1", "Group 7", 0),
		groupTestWorkspace("workspace:2", "api", 1),
	)
	backend := &groupSaveClient{
		mockClient: &mockClient{treeResp: tree},
		groups: []client.WorkspaceGroup{{
			Ref: "workspace_group:1", Name: "backend",
			AnchorRef: "workspace:1", MemberRefs: []string{"workspace:1", "workspace:2"},
		}},
	}
	saver, _ := newGroupSaver(t, backend)

	if _, err := saver.Save("rev", ""); err != nil {
		t.Fatalf("first save: %v", err)
	}
	same, err := saver.Save("rev", "")
	if err != nil {
		t.Fatalf("second save: %v", err)
	}
	if same.Revision != 1 {
		t.Errorf("identical re-save: revision = %d, want 1", same.Revision)
	}

	backend.groups[0].Collapsed = true
	collapsed, err := saver.Save("rev", "")
	if err != nil {
		t.Fatalf("third save: %v", err)
	}
	if collapsed.Revision != 2 {
		t.Errorf("collapse-only change: revision = %d, want 2", collapsed.Revision)
	}
}

func TestSaveGroups_ReSaveNeitherLosesNorInventsGroups(t *testing.T) {
	tree := groupTestTree(
		groupTestWorkspace("workspace:1", "Group 7", 0),
		groupTestWorkspace("workspace:2", "api", 1),
		groupTestWorkspace("workspace:3", "notes", 2),
	)
	backend := &groupSaveClient{
		mockClient: &mockClient{treeResp: tree},
		groups: []client.WorkspaceGroup{{
			Ref: "workspace_group:1", Name: "backend",
			AnchorRef: "workspace:1", MemberRefs: []string{"workspace:1", "workspace:2"},
		}},
	}
	saver, store := newGroupSaver(t, backend)
	if _, err := saver.Save("resave", ""); err != nil {
		t.Fatalf("first save: %v", err)
	}

	// The user leaves the group in cmux: the live state has no groups now,
	// and the old file must not bring them back.
	backend.groups = nil
	dissolved, err := saver.Save("resave", "")
	if err != nil {
		t.Fatalf("second save: %v", err)
	}
	if len(dissolved.Groups) != 0 {
		t.Errorf("re-save invented groups from the old file: %+v", dissolved.Groups)
	}
	for _, workspace := range dissolved.Workspaces {
		if workspace.Group != "" || workspace.GroupAnchor {
			t.Errorf("workspace %q kept stale group data: %+v", workspace.Title, workspace)
		}
	}

	// And a hand-edited file with a group the live state lacks is not
	// copied either; only live state names groups.
	edited, _ := store.Load("resave")
	edited.Groups = []model.Group{{Name: "invented"}}
	edited.Workspaces[2].Group = "invented"
	if err := store.Save("resave", edited); err != nil {
		t.Fatalf("store: %v", err)
	}
	backend.groups = []client.WorkspaceGroup{{
		Ref: "workspace_group:1", Name: "backend",
		AnchorRef: "workspace:1", MemberRefs: []string{"workspace:1", "workspace:2"},
	}}
	regrouped, err := saver.Save("resave", "")
	if err != nil {
		t.Fatalf("third save: %v", err)
	}
	if got := strings.Join(groupNames(regrouped), ","); got != "backend" {
		t.Errorf("groups = %q, want only the live backend group", got)
	}
	workspaceTitled(t, regrouped, "", "notes")
	workspaceTitled(t, regrouped, "backend", "api")
}

func TestMergeUserEdits_DuplicateTitlesAcrossGroupsKeepTheirOwnEdits(t *testing.T) {
	livePanes := func() []model.Pane {
		return []model.Pane{{Type: "terminal", Index: 0}}
	}
	live := &model.Layout{Workspaces: []model.Workspace{
		{Title: "api", Group: "backend", Panes: livePanes()},
		{Title: "api", Group: "frontend", Panes: livePanes()},
	}}
	existing := &model.Layout{
		Groups: []model.Group{{Name: "backend"}, {Name: "frontend"}},
		Workspaces: []model.Workspace{
			{Title: "api", Group: "frontend", Description: "the web api", Panes: []model.Pane{{Type: "terminal", Index: 0, Command: "npm run dev"}}},
			{Title: "api", Group: "backend", Description: "the go api", Panes: []model.Pane{{Type: "terminal", Index: 0, Command: "make run"}}},
		},
	}

	mergeUserEdits(live, existing, nil)

	backend := live.Workspaces[0]
	if backend.Description != "the go api" || backend.Panes[0].Command != "make run" {
		t.Errorf("backend api got the wrong edits: %+v", backend)
	}
	frontend := live.Workspaces[1]
	if frontend.Description != "the web api" || frontend.Panes[0].Command != "npm run dev" {
		t.Errorf("frontend api got the wrong edits: %+v", frontend)
	}
	if live.Groups != nil {
		t.Errorf("merge copied groups from the existing layout: %+v", live.Groups)
	}
}

// TestMergeUserEdits_SameTitleInOneGroupPairsInFileOrder: two shells in the
// same folder give two members of one group the same title. Each must take
// the edits of its own stored workspace, in file order, never both the last
// one's: a copied "claude --resume <id>" would open one session twice.
func TestMergeUserEdits_SameTitleInOneGroupPairsInFileOrder(t *testing.T) {
	livePanes := func() []model.Pane {
		return []model.Pane{{Type: "terminal", Index: 0}}
	}
	live := &model.Layout{Workspaces: []model.Workspace{
		{Title: "repo", Group: "Cmux", Panes: livePanes()},
		{Title: "repo", Group: "Cmux", Panes: livePanes()},
	}}
	existing := &model.Layout{Workspaces: []model.Workspace{
		{Title: "repo", Group: "Cmux", Description: "first", Panes: []model.Pane{{Type: "terminal", Index: 0, Command: "claude --resume aaa"}}},
		{Title: "repo", Group: "Cmux", Description: "second", Panes: []model.Pane{{Type: "terminal", Index: 0, Command: "claude --resume bbb"}}},
	}}

	mergeUserEdits(live, existing, nil)

	first := live.Workspaces[0]
	if first.Description != "first" || first.Panes[0].Command != "claude --resume aaa" {
		t.Errorf("first repo got the wrong edits: %+v", first)
	}
	second := live.Workspaces[1]
	if second.Description != "second" || second.Panes[0].Command != "claude --resume bbb" {
		t.Errorf("second repo got the wrong edits: %+v", second)
	}
}

// TestMergeUserEdits_StoredWorkspaceIsUsedOnlyOnce: with one stored workspace
// and two live ones sharing its title, only the first live one inherits the
// edits; the other is new and starts clean, also through the title fallback.
func TestMergeUserEdits_StoredWorkspaceIsUsedOnlyOnce(t *testing.T) {
	livePanes := func() []model.Pane {
		return []model.Pane{{Type: "terminal", Index: 0}}
	}
	cases := []struct {
		name        string
		storedGroup string
	}{
		{name: "same group", storedGroup: "Cmux"},
		{name: "title fallback", storedGroup: ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			live := &model.Layout{Workspaces: []model.Workspace{
				{Title: "repo", Group: "Cmux", Panes: livePanes()},
				{Title: "repo", Group: "Cmux", Panes: livePanes()},
			}}
			existing := &model.Layout{Workspaces: []model.Workspace{
				{Title: "repo", Group: testCase.storedGroup, Description: "saved", Panes: []model.Pane{{Type: "terminal", Index: 0, Command: "claude --resume aaa"}}},
			}}

			mergeUserEdits(live, existing, nil)

			if live.Workspaces[0].Panes[0].Command != "claude --resume aaa" {
				t.Errorf("first repo did not inherit the stored edits: %+v", live.Workspaces[0])
			}
			second := live.Workspaces[1]
			if second.Description != "" || second.Panes[0].Command != "" {
				t.Errorf("second repo inherited edits that belong to the first: %+v", second)
			}
		})
	}
}

func TestMergeUserEdits_FallsBackToTitleForAWorkspaceThatJoinedAGroup(t *testing.T) {
	live := &model.Layout{Workspaces: []model.Workspace{
		{Title: "api", Group: "backend", Panes: []model.Pane{{Type: "terminal", Index: 0}}},
	}}
	existing := &model.Layout{Workspaces: []model.Workspace{
		{Title: "api", Description: "saved before groups", Panes: []model.Pane{{Type: "terminal", Index: 0, Command: "make run"}}},
	}}

	mergeUserEdits(live, existing, nil)

	merged := live.Workspaces[0]
	if merged.Description != "saved before groups" || merged.Panes[0].Command != "make run" {
		t.Errorf("title fallback did not merge: %+v", merged)
	}
	if merged.Group != "backend" || merged.GroupAnchor {
		t.Errorf("merge touched the live group data: %+v", merged)
	}
}

// TestDeduplicateWorkspaces_KeepsEveryTabWhenNoTTYIsReported: Ghostty never
// reports a tty, so a tab without one is not a ghost there. Two tabs in the
// same folder share a title and both must be saved (GitHub: 8 live tabs, 7
// saved).
func TestDeduplicateWorkspaces_KeepsEveryTabWhenNoTTYIsReported(t *testing.T) {
	firstTab := groupTestWorkspace("tab:1", "txeo@Mac: ~/ghtest-a", 0)
	secondTab := groupTestWorkspace("tab:2", "txeo@Mac: ~/ghtest-a", 1)
	otherTab := groupTestWorkspace("tab:3", "txeo@Mac: ~/ghtest-b", 2)

	kept := deduplicateWorkspaces([]client.TreeWorkspace{firstTab, secondTab, otherTab}, nil)

	got := keptRefs(kept)
	if got != "tab:1,tab:2,tab:3" {
		t.Errorf("kept = %s, want all three tabs", got)
	}
}
