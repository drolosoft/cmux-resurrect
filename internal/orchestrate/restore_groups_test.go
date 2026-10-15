package orchestrate

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

// restoreFakeWorkspace is one live workspace of restoreFake, in sidebar order.
type restoreFakeWorkspace struct {
	ref   string
	title string
}

// restoreFake is a backend without sidebar groups (Ghostty-like) that records
// every call in order, so a test can compare whole call sequences. It also
// holds the group state for restoreGroupFake, because closing a workspace has to
// update the groups it belonged to.
type restoreFake struct {
	calls      []string
	workspaces []restoreFakeWorkspace
	groups     []client.WorkspaceGroup
	callerRef  string

	// formatter is the dry-run formatter; nil means client.CmuxDryRun.
	formatter client.DryRunFormatter

	// listErr makes ListGroups fail, to test the fallback to loose workspaces.
	listErr error

	nextWorkspace int
	nextGroup     int
}

// record appends one call to the log, formatted like a short command line.
func (f *restoreFake) record(format string, args ...any) {
	f.calls = append(f.calls, fmt.Sprintf(format, args...))
}

// addWorkspace creates a live workspace and returns its ref. A non-empty
// groupRef puts it at the end of that group, as `--group-placement end` does,
// and fails like cmux when the group does not exist.
func (f *restoreFake) addWorkspace(title, groupRef string) (string, error) {
	group := f.group(groupRef)
	if groupRef != "" && group == nil {
		return "", fmt.Errorf("group %s not found", groupRef)
	}

	f.nextWorkspace++
	ref := fmt.Sprintf("workspace:%d", f.nextWorkspace)
	f.workspaces = append(f.workspaces, restoreFakeWorkspace{ref: ref, title: title})

	if group != nil {
		group.MemberRefs = append(group.MemberRefs, ref)
	}
	return ref, nil
}

// group returns the live group with that ref, or nil.
func (f *restoreFake) group(groupRef string) *client.WorkspaceGroup {
	for index := range f.groups {
		if f.groups[index].Ref == groupRef {
			return &f.groups[index]
		}
	}
	return nil
}

// groupNamed returns the live group with that name, or nil.
func (f *restoreFake) groupNamed(name string) *client.WorkspaceGroup {
	for index := range f.groups {
		if f.groups[index].Name == name {
			return &f.groups[index]
		}
	}
	return nil
}

// titleOf returns the title of a live workspace, "" when it does not exist.
func (f *restoreFake) titleOf(ref string) string {
	for _, workspace := range f.workspaces {
		if workspace.ref == ref {
			return workspace.title
		}
	}
	return ""
}

// callsWith returns the recorded calls that start with one of the prefixes,
// in order, so a test can look at the group-related part of a restore.
func (f *restoreFake) callsWith(prefixes ...string) []string {
	var matching []string
	for _, call := range f.calls {
		for _, prefix := range prefixes {
			if strings.HasPrefix(call, prefix) {
				matching = append(matching, call)
				break
			}
		}
	}
	return matching
}

func (f *restoreFake) Ping() error { return nil }

// Tree reports the live workspaces and the caller, which Restore uses to keep
// the caller's own workspace open in replace mode.
func (f *restoreFake) Tree() (*client.TreeResponse, error) {
	var workspaces []client.TreeWorkspace
	for _, workspace := range f.workspaces {
		workspaces = append(workspaces, client.TreeWorkspace{Ref: workspace.ref, Title: workspace.title})
	}

	tree := &client.TreeResponse{Windows: []client.TreeWindow{{Ref: "window:1", Workspaces: workspaces}}}
	if f.callerRef != "" {
		tree.Caller = &client.CallerInfo{WorkspaceRef: f.callerRef}
	}
	return tree, nil
}

func (f *restoreFake) SidebarState(ref string) (*client.SidebarState, error) {
	return &client.SidebarState{CWD: "/tmp", FocusedCWD: "/tmp"}, nil
}

func (f *restoreFake) ListWorkspaces() ([]client.WorkspaceInfo, error) {
	var infos []client.WorkspaceInfo
	for _, workspace := range f.workspaces {
		infos = append(infos, client.WorkspaceInfo{Ref: workspace.ref, Title: workspace.title})
	}
	return infos, nil
}

func (f *restoreFake) NewWorkspace(opts client.NewWorkspaceOpts) (string, error) {
	f.record("new-workspace %s group=%s", opts.CWD, opts.Group)
	return f.addWorkspace("", opts.Group)
}

func (f *restoreFake) RenameWorkspace(ref, title string) error {
	f.record("rename %s %s", ref, title)
	for index := range f.workspaces {
		if f.workspaces[index].ref == ref {
			f.workspaces[index].title = title
		}
	}
	return nil
}

func (f *restoreFake) SelectWorkspace(ref string) error {
	f.record("select %s", ref)
	return nil
}

func (f *restoreFake) NewSplit(direction, workspaceRef, surfaceRef string) (string, error) {
	f.record("split %s %s %s", direction, workspaceRef, surfaceRef)
	return "surface:split", nil
}

func (f *restoreFake) NewPane(opts client.NewPaneOpts) (string, error) {
	f.record("new-pane %s %s", opts.Type, opts.WorkspaceRef)
	return "surface:pane", nil
}

func (f *restoreFake) NewSurface(paneRef, workspaceRef string) (string, error) {
	f.record("new-surface %s %s", paneRef, workspaceRef)
	return "surface:tab", nil
}

func (f *restoreFake) FocusPane(paneRef, workspaceRef string) error {
	f.record("focus-pane %s %s", paneRef, workspaceRef)
	return nil
}

func (f *restoreFake) Send(workspaceRef, surfaceRef, text string) error {
	f.record("send %s %s %s", workspaceRef, surfaceRef, strings.TrimSpace(text))
	return nil
}

func (f *restoreFake) PinWorkspace(ref string) error {
	f.record("pin-workspace %s", ref)
	return nil
}

func (f *restoreFake) UnpinWorkspace(ref string) error {
	f.record("unpin-workspace %s", ref)
	return nil
}

// CloseWorkspace removes the workspace and models what cmux does to its
// group (verified live): closing the anchor promotes the next member, and
// closing the last member removes the group.
func (f *restoreFake) CloseWorkspace(ref string) error {
	f.record("close %s", ref)

	f.workspaces = slices.DeleteFunc(f.workspaces, func(workspace restoreFakeWorkspace) bool {
		return workspace.ref == ref
	})

	for index := range f.groups {
		group := &f.groups[index]
		group.MemberRefs = slices.DeleteFunc(group.MemberRefs, func(member string) bool {
			return member == ref
		})
		if group.AnchorRef == ref && len(group.MemberRefs) > 0 {
			group.AnchorRef = group.MemberRefs[0]
		}
	}

	f.groups = slices.DeleteFunc(f.groups, func(group client.WorkspaceGroup) bool {
		return len(group.MemberRefs) == 0
	})
	return nil
}

func (f *restoreFake) DryRunFormatter() client.DryRunFormatter {
	if f.formatter != nil {
		return f.formatter
	}
	return client.CmuxDryRun{}
}

// restoreGroupFake is restoreFake plus client.GroupManager, modelling the cmux group
// rules from the live probe: create always adds a brand-new anchor titled
// with the group name, set-anchor moves a member to the front.
type restoreGroupFake struct {
	*restoreFake
}

func (f restoreGroupFake) ListGroups(windowRef string) ([]client.WorkspaceGroup, error) {
	f.record("list-groups %s", windowRef)
	if f.listErr != nil {
		return nil, f.listErr
	}

	groups := make([]client.WorkspaceGroup, len(f.groups))
	for index, group := range f.groups {
		group.MemberRefs = slices.Clone(group.MemberRefs)
		groups[index] = group
	}
	return groups, nil
}

func (f restoreGroupFake) CreateGroup(name string, fromRefs []string) (client.WorkspaceGroup, error) {
	f.record("create-group %s from=%s", name, strings.Join(fromRefs, ","))

	f.nextWorkspace++
	extraAnchor := fmt.Sprintf("workspace:%d", f.nextWorkspace)
	f.workspaces = append(f.workspaces, restoreFakeWorkspace{ref: extraAnchor, title: name})

	f.nextGroup++
	group := client.WorkspaceGroup{
		Ref:        fmt.Sprintf("workspace_group:%d", f.nextGroup),
		Name:       name,
		AnchorRef:  extraAnchor,
		MemberRefs: append([]string{extraAnchor}, fromRefs...),
	}
	f.groups = append(f.groups, group)

	group.MemberRefs = slices.Clone(group.MemberRefs)
	return group, nil
}

func (f restoreGroupFake) SetGroupAnchor(groupRef, workspaceRef string) error {
	f.record("set-anchor %s %s", groupRef, workspaceRef)

	group := f.group(groupRef)
	if group == nil {
		return fmt.Errorf("group %s not found", groupRef)
	}

	others := slices.DeleteFunc(slices.Clone(group.MemberRefs), func(member string) bool {
		return member == workspaceRef
	})
	group.MemberRefs = append([]string{workspaceRef}, others...)
	group.AnchorRef = workspaceRef
	return nil
}

func (f restoreGroupFake) AddToGroup(groupRef, workspaceRef string) error {
	f.record("add-to-group %s %s", groupRef, workspaceRef)
	return nil
}

func (f restoreGroupFake) RenameGroup(groupRef, name string) error {
	f.record("rename-group %s %s", groupRef, name)
	return nil
}

func (f restoreGroupFake) CollapseGroup(groupRef string) error {
	f.record("collapse %s", groupRef)
	return f.updateGroup(groupRef, func(group *client.WorkspaceGroup) { group.Collapsed = true })
}

func (f restoreGroupFake) PinGroup(groupRef string) error {
	f.record("pin %s", groupRef)
	return f.updateGroup(groupRef, func(group *client.WorkspaceGroup) { group.Pinned = true })
}

func (f restoreGroupFake) SetGroupColor(groupRef, hex string) error {
	f.record("color %s %s", groupRef, hex)
	return f.updateGroup(groupRef, func(group *client.WorkspaceGroup) { group.Color = hex })
}

func (f restoreGroupFake) SetGroupIcon(groupRef, symbol string) error {
	f.record("icon %s %s", groupRef, symbol)
	return f.updateGroup(groupRef, func(group *client.WorkspaceGroup) { group.Icon = symbol })
}

// updateGroup applies a state change to a live group and fails like cmux
// when the group is gone.
func (f restoreGroupFake) updateGroup(groupRef string, change func(*client.WorkspaceGroup)) error {
	group := f.group(groupRef)
	if group == nil {
		return fmt.Errorf("group %s not found", groupRef)
	}
	change(group)
	return nil
}

// restoreLayoutGroupFake adds atomic layout creation (cmux `--layout`) on top of
// restoreGroupFake, so the test covers the second creation path of restoreWorkspace.
type restoreLayoutGroupFake struct {
	restoreGroupFake
}

func (f restoreLayoutGroupFake) NewWorkspaceLayout(opts client.NewWorkspaceOpts, layoutJSON string) (string, error) {
	f.record("new-workspace-layout %s group=%s", opts.CWD, opts.Group)
	return f.addWorkspace("", opts.Group)
}

// Compile-time checks: the fakes must expose exactly the optional
// interfaces Restore looks for, and the plain one must not have groups.
var (
	_ client.GroupManager           = restoreGroupFake{}
	_ client.LayoutWorkspaceCreator = restoreLayoutGroupFake{}
)

// newRestoreGroupFake returns a group-aware fake with an empty window.
func newRestoreGroupFake() restoreGroupFake {
	return restoreGroupFake{restoreFake: &restoreFake{}}
}

// livePreset seeds a fake with live workspaces and groups before a restore.
// Each group lists its member titles, anchor first.
func (f *restoreFake) livePreset(loose []string, groups map[string][]string, groupOrder []string) {
	for _, title := range loose {
		_, _ = f.addWorkspace(title, "")
	}
	for _, name := range groupOrder {
		var members []string
		for _, title := range groups[name] {
			ref, _ := f.addWorkspace(title, "")
			members = append(members, ref)
		}
		f.nextGroup++
		f.groups = append(f.groups, client.WorkspaceGroup{
			Ref:        fmt.Sprintf("workspace_group:%d", f.nextGroup),
			Name:       name,
			AnchorRef:  members[0],
			MemberRefs: members,
		})
	}
}

// restoreSinglePane is the smallest workspace body: one terminal pane, no command,
// so restoreWorkspace never waits on a shell.
func restoreSinglePane() []model.Pane {
	return []model.Pane{{Type: "terminal", Focus: true, FocusTarget: -1}}
}

// storeRestoreLayout stores a layout in a fresh temp store and returns the store.
func storeRestoreLayout(t *testing.T, layout *model.Layout) persist.Store {
	t.Helper()

	store, err := persist.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	layout.Version = 1
	layout.SavedAt = time.Now().UTC()
	if err := store.Save(layout.Name, layout); err != nil {
		t.Fatalf("save layout: %v", err)
	}
	return store
}

// restoreOneGroupLayout is a group "Backend" with its anchor ("Group 1") and one
// member ("api"), followed by a loose workspace.
func restoreOneGroupLayout() *model.Layout {
	return &model.Layout{
		Name:   "one-group",
		Groups: []model.Group{{Name: "Backend"}},
		Workspaces: []model.Workspace{
			{Title: "Group 1", CWD: "/w/anchor", Index: 0, Group: "Backend", GroupAnchor: true, Panes: restoreSinglePane()},
			{Title: "api", CWD: "/w/api", Index: 1, Group: "Backend", Panes: restoreSinglePane()},
			{Title: "notes", CWD: "/w/notes", Index: 2, Panes: restoreSinglePane()},
		},
	}
}

// assertRestoreCalls fails with both sequences side by side when they differ.
func assertRestoreCalls(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("calls differ\n got: %s\nwant: %s", strings.Join(got, "\n      "), strings.Join(want, "\n      "))
	}
}

// assertRestoreNoErrors fails when the restore reported any error.
func assertRestoreNoErrors(t *testing.T, result *RestoreResult) {
	t.Helper()
	if len(result.Errors) > 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}
}

// TestRestoreGroups_AnchorFirstThenMembersInside: the first member restored
// becomes the anchor through create + set-anchor + close of the extra anchor
// cmux always adds; later members are created straight inside the group.
func TestRestoreGroups_AnchorFirstThenMembersInside(t *testing.T) {
	t.Parallel()

	fake := newRestoreGroupFake()
	store := storeRestoreLayout(t, restoreOneGroupLayout())
	restorer := &Restorer{Client: fake, Store: store, SkipPing: true}

	result, err := restorer.Restore("one-group", false, RestoreModeAdd, "", true)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	assertRestoreNoErrors(t, result)

	got := fake.callsWith("new-workspace", "rename ", "create-group", "set-anchor", "close")
	want := []string{
		"new-workspace /w/anchor group=",
		"rename workspace:1 Group 1",
		"create-group Backend from=workspace:1",
		"set-anchor workspace_group:1 workspace:1",
		"close workspace:2",
		"new-workspace /w/api group=workspace_group:1",
		"rename workspace:3 api",
		"new-workspace /w/notes group=",
		"rename workspace:4 notes",
	}
	assertRestoreCalls(t, got, want)

	group := fake.groupNamed("Backend")
	if group == nil {
		t.Fatal("group Backend does not exist after restore")
	}
	if group.AnchorRef != "workspace:1" {
		t.Errorf("anchor = %s, want the restored Group 1 (workspace:1)", group.AnchorRef)
	}
	if !reflect.DeepEqual(group.MemberRefs, []string{"workspace:1", "workspace:3"}) {
		t.Errorf("members = %v, want [workspace:1 workspace:3]", group.MemberRefs)
	}
	if len(result.Warnings) > 0 {
		t.Errorf("unexpected warnings: %v", result.Warnings)
	}
}

// TestRestoreGroups_LayoutPathPassesGroup: the atomic `--layout` creation
// path has to put members in their group too, not only the sequential one.
func TestRestoreGroups_LayoutPathPassesGroup(t *testing.T) {
	t.Parallel()

	twoPanes := []model.Pane{
		{Type: "terminal", Focus: true, FocusTarget: -1},
		{Type: "terminal", Split: "right", FocusTarget: -1},
	}
	layout := &model.Layout{
		Name:   "layout-path",
		Groups: []model.Group{{Name: "Backend"}},
		Workspaces: []model.Workspace{
			{Title: "Group 1", CWD: "/w/anchor", Index: 0, Group: "Backend", GroupAnchor: true, Panes: twoPanes},
			{Title: "api", CWD: "/w/api", Index: 1, Group: "Backend", Panes: twoPanes},
		},
	}

	fake := restoreLayoutGroupFake{restoreGroupFake: newRestoreGroupFake()}
	restorer := &Restorer{Client: fake, Store: storeRestoreLayout(t, layout), SkipPing: true}

	result, err := restorer.Restore("layout-path", false, RestoreModeAdd, "", true)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	assertRestoreNoErrors(t, result)

	got := fake.callsWith("new-workspace", "create-group", "set-anchor", "close")
	want := []string{
		"new-workspace-layout /w/anchor group=",
		"create-group Backend from=workspace:1",
		"set-anchor workspace_group:1 workspace:1",
		"close workspace:2",
		"new-workspace-layout /w/api group=workspace_group:1",
	}
	assertRestoreCalls(t, got, want)
}

// TestRestoreGroups_JoinsLiveGroupWithSameName: a group that already lives in
// the caller's window is joined instead of duplicated, and its member that is
// already open is skipped as usual.
func TestRestoreGroups_JoinsLiveGroupWithSameName(t *testing.T) {
	t.Parallel()

	fake := newRestoreGroupFake()
	fake.livePreset(nil, map[string][]string{"Backend": {"Group 1"}}, []string{"Backend"})
	restorer := &Restorer{Client: fake, Store: storeRestoreLayout(t, restoreOneGroupLayout()), SkipPing: true}

	result, err := restorer.Restore("one-group", false, RestoreModeAdd, "", true)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	assertRestoreNoErrors(t, result)

	got := fake.callsWith("new-workspace", "create-group", "set-anchor", "close")
	want := []string{
		"new-workspace /w/api group=workspace_group:1",
		"new-workspace /w/notes group=",
	}
	assertRestoreCalls(t, got, want)

	if len(fake.groups) != 1 {
		t.Errorf("groups = %d, want the one live group joined, not a second one", len(fake.groups))
	}
}

// TestRestoreGroups_FilteredMemberCreatesItsGroup: `--workspace api` restores
// one member whose group is not open; it becomes the anchor of a new group
// holding just itself.
func TestRestoreGroups_FilteredMemberCreatesItsGroup(t *testing.T) {
	t.Parallel()

	fake := newRestoreGroupFake()
	restorer := &Restorer{Client: fake, Store: storeRestoreLayout(t, restoreOneGroupLayout()), SkipPing: true}

	result, err := restorer.Restore("one-group", false, RestoreModeAdd, "api", true)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	assertRestoreNoErrors(t, result)

	got := fake.callsWith("new-workspace", "create-group", "set-anchor", "close")
	want := []string{
		"new-workspace /w/api group=",
		"create-group Backend from=workspace:1",
		"set-anchor workspace_group:1 workspace:1",
		"close workspace:2",
	}
	assertRestoreCalls(t, got, want)

	group := fake.groupNamed("Backend")
	if group == nil || !reflect.DeepEqual(group.MemberRefs, []string{"workspace:1"}) {
		t.Errorf("group = %+v, want Backend holding only the restored api", group)
	}
}

// TestRestoreGroups_FilteredMemberJoinsLiveGroup: the same filter joins the
// group when it is already open.
func TestRestoreGroups_FilteredMemberJoinsLiveGroup(t *testing.T) {
	t.Parallel()

	fake := newRestoreGroupFake()
	fake.livePreset(nil, map[string][]string{"Backend": {"Group 1"}}, []string{"Backend"})
	restorer := &Restorer{Client: fake, Store: storeRestoreLayout(t, restoreOneGroupLayout()), SkipPing: true}

	result, err := restorer.Restore("one-group", false, RestoreModeAdd, "api", true)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	assertRestoreNoErrors(t, result)

	got := fake.callsWith("new-workspace", "create-group")
	assertRestoreCalls(t, got, []string{"new-workspace /w/api group=workspace_group:1"})
}

// TestRestoreGroups_StateAfterWorkspacesPinsReversed: collapse, color and icon
// are applied once every workspace exists, and pins run in reverse layout
// order because pinning moves a group to the top of the sidebar.
func TestRestoreGroups_StateAfterWorkspacesPinsReversed(t *testing.T) {
	t.Parallel()

	layout := &model.Layout{
		Name: "state",
		Groups: []model.Group{
			{Name: "Alpha", Collapsed: true, Pinned: true, Color: "#FF0000", Icon: "star"},
			{Name: "Beta", Pinned: true},
			{Name: "Gamma"},
		},
		Workspaces: []model.Workspace{
			{Title: "Group 1", CWD: "/w/a", Index: 0, Group: "Alpha", GroupAnchor: true, Panes: restoreSinglePane()},
			{Title: "Group 2", CWD: "/w/b", Index: 1, Group: "Beta", GroupAnchor: true, Panes: restoreSinglePane()},
			{Title: "Group 3", CWD: "/w/c", Index: 2, Group: "Gamma", GroupAnchor: true, Panes: restoreSinglePane()},
		},
	}

	fake := newRestoreGroupFake()
	restorer := &Restorer{Client: fake, Store: storeRestoreLayout(t, layout), SkipPing: true}

	result, err := restorer.Restore("state", false, RestoreModeAdd, "", true)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	assertRestoreNoErrors(t, result)

	got := fake.callsWith("new-workspace", "collapse", "color", "icon", "pin ")
	want := []string{
		"new-workspace /w/a group=",
		"new-workspace /w/b group=",
		"new-workspace /w/c group=",
		"collapse workspace_group:1",
		"color workspace_group:1 #FF0000",
		"icon workspace_group:1 star",
		"pin workspace_group:2",
		"pin workspace_group:1",
	}
	assertRestoreCalls(t, got, want)
}

// TestRestoreGroups_JoinedGroupOnlyGetsMissingState: a live group that is
// already collapsed and pinned is not toggled or re-pinned (a second pin
// would move it again); only the color it lacks is set.
func TestRestoreGroups_JoinedGroupOnlyGetsMissingState(t *testing.T) {
	t.Parallel()

	layout := restoreOneGroupLayout()
	layout.Groups[0] = model.Group{Name: "Backend", Collapsed: true, Pinned: true, Color: "#00FF00"}

	fake := newRestoreGroupFake()
	fake.livePreset(nil, map[string][]string{"Backend": {"Group 1"}}, []string{"Backend"})
	fake.groups[0].Collapsed = true
	fake.groups[0].Pinned = true

	restorer := &Restorer{Client: fake, Store: storeRestoreLayout(t, layout), SkipPing: true}
	result, err := restorer.Restore("one-group", false, RestoreModeAdd, "", true)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	assertRestoreNoErrors(t, result)

	got := fake.callsWith("collapse", "color", "icon", "pin ")
	assertRestoreCalls(t, got, []string{"color workspace_group:1 #00FF00"})
}

// TestRestoreGroups_SkipMatchingKeysOnGroupAndTitle: every anchor is titled
// "Group N", so a live "Group 2" of another group must not make the layout's
// "Group 2" look already open.
func TestRestoreGroups_SkipMatchingKeysOnGroupAndTitle(t *testing.T) {
	t.Parallel()

	layout := &model.Layout{
		Name:   "same-titles",
		Groups: []model.Group{{Name: "Yankee"}},
		Workspaces: []model.Workspace{
			{Title: "Group 2", CWD: "/w/y", Index: 0, Group: "Yankee", GroupAnchor: true, Panes: restoreSinglePane()},
		},
	}

	fake := newRestoreGroupFake()
	fake.livePreset(nil, map[string][]string{"Xray": {"Group 2"}}, []string{"Xray"})
	restorer := &Restorer{Client: fake, Store: storeRestoreLayout(t, layout), SkipPing: true}

	result, err := restorer.Restore("same-titles", false, RestoreModeAdd, "", true)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	assertRestoreNoErrors(t, result)

	if result.WorkspacesOK != 1 {
		t.Errorf("WorkspacesOK = %d, want 1: Yankee's Group 2 was skipped because Xray has one", result.WorkspacesOK)
	}
	if fake.groupNamed("Yankee") == nil {
		t.Error("group Yankee was not created")
	}
}

// restoreSameTitleLayout is a group "Cmux" with its anchor and two members
// that share a title, as two shells in the same folder do.
func restoreSameTitleLayout() *model.Layout {
	return &model.Layout{
		Name:   "same-title-members",
		Groups: []model.Group{{Name: "Cmux"}},
		Workspaces: []model.Workspace{
			{Title: "Group 1", CWD: "/w/anchor", Index: 0, Group: "Cmux", GroupAnchor: true, Panes: restoreSinglePane()},
			{Title: "txeo@Mac: ~/repo", CWD: "/w/one", Index: 1, Group: "Cmux", Panes: restoreSinglePane()},
			{Title: "txeo@Mac: ~/repo", CWD: "/w/two", Index: 2, Group: "Cmux", Panes: restoreSinglePane()},
		},
	}
}

// TestRestoreGroups_SameTitleMembersBothCreated: the second member with the
// same title is not "already open" just because the first one was created a
// moment ago in this same restore.
func TestRestoreGroups_SameTitleMembersBothCreated(t *testing.T) {
	t.Parallel()

	fake := newRestoreGroupFake()
	restorer := &Restorer{Client: fake, Store: storeRestoreLayout(t, restoreSameTitleLayout()), SkipPing: true}

	result, err := restorer.Restore("same-title-members", false, RestoreModeAdd, "", true)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	assertRestoreNoErrors(t, result)

	got := fake.callsWith("new-workspace", "create-group")
	want := []string{
		"new-workspace /w/anchor group=",
		"create-group Cmux from=workspace:1",
		"new-workspace /w/one group=workspace_group:1",
		"new-workspace /w/two group=workspace_group:1",
	}
	assertRestoreCalls(t, got, want)

	group := fake.groupNamed("Cmux")
	if group == nil {
		t.Fatal("group Cmux does not exist after restore")
	}
	if !reflect.DeepEqual(group.MemberRefs, []string{"workspace:1", "workspace:3", "workspace:4"}) {
		t.Errorf("members = %v, want the anchor and both same-title members", group.MemberRefs)
	}
}

// TestRestoreGroups_SkipMatchingCountsSameTitleMembers: with one of the two
// same-title members already open in its group, add mode skips that one and
// still creates the other, instead of letting one live workspace stand for
// both.
func TestRestoreGroups_SkipMatchingCountsSameTitleMembers(t *testing.T) {
	t.Parallel()

	fake := newRestoreGroupFake()
	fake.livePreset(nil, map[string][]string{"Cmux": {"Group 1", "txeo@Mac: ~/repo"}}, []string{"Cmux"})
	restorer := &Restorer{Client: fake, Store: storeRestoreLayout(t, restoreSameTitleLayout()), SkipPing: true}

	result, err := restorer.Restore("same-title-members", false, RestoreModeAdd, "", true)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	assertRestoreNoErrors(t, result)

	got := fake.callsWith("new-workspace", "create-group")
	want := []string{
		"new-workspace /w/two group=workspace_group:1",
	}
	assertRestoreCalls(t, got, want)

	group := fake.groupNamed("Cmux")
	if group == nil {
		t.Fatal("group Cmux is gone")
	}
	if len(group.MemberRefs) != 3 {
		t.Errorf("members = %v, want the live anchor, the live member and one new member", group.MemberRefs)
	}
}

// restoreReplaceLive seeds the window used by the replace-mode tests: the caller
// (loose), a loose "notes", group Backend with a kept anchor and a stale
// member, and group Old that the layout no longer has.
func restoreReplaceLive() restoreGroupFake {
	fake := newRestoreGroupFake()
	fake.livePreset(
		[]string{"shell", "notes"},
		map[string][]string{
			"Backend": {"Group 1", "stale"},
			"Old":     {"Group 9"},
		},
		[]string{"Backend", "Old"},
	)
	fake.callerRef = "workspace:1"
	return fake
}

// TestRestoreGroups_ReplaceKeepsMatchingClosesStale: replace + skipMatching
// closes what the layout no longer has, keyed on (group, title), keeps the
// matching anchor, joins its group and leaves no orphan group behind.
func TestRestoreGroups_ReplaceKeepsMatchingClosesStale(t *testing.T) {
	t.Parallel()

	fake := restoreReplaceLive()
	restorer := &Restorer{Client: fake, Store: storeRestoreLayout(t, restoreOneGroupLayout()), SkipPing: true}

	result, err := restorer.Restore("one-group", false, RestoreModeReplace, "", true)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	assertRestoreNoErrors(t, result)

	// workspace:4 is Backend's "stale", workspace:5 is Old's anchor.
	got := fake.callsWith("close", "new-workspace", "create-group")
	want := []string{
		"close workspace:4",
		"close workspace:5",
		"new-workspace /w/api group=workspace_group:1",
	}
	assertRestoreCalls(t, got, want)

	if fake.groupNamed("Old") != nil {
		t.Error("group Old is still there after all its members were closed")
	}

	backend := fake.groupNamed("Backend")
	if backend == nil {
		t.Fatal("group Backend is gone")
	}
	if !reflect.DeepEqual(backend.MemberRefs, []string{"workspace:3", "workspace:6"}) {
		t.Errorf("Backend members = %v, want the kept anchor and the new api", backend.MemberRefs)
	}
}

// TestRestoreGroups_ReplaceRecreatesClosedGroup: replace without skipMatching
// closes every member of Backend, which removes the group in cmux; restore
// must notice and create it again instead of joining a dead ref.
func TestRestoreGroups_ReplaceRecreatesClosedGroup(t *testing.T) {
	t.Parallel()

	fake := restoreReplaceLive()
	restorer := &Restorer{Client: fake, Store: storeRestoreLayout(t, restoreOneGroupLayout()), SkipPing: true}

	result, err := restorer.Restore("one-group", false, RestoreModeReplace, "", false)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	assertRestoreNoErrors(t, result)

	if result.WorkspacesOK != 3 {
		t.Errorf("WorkspacesOK = %d, want 3", result.WorkspacesOK)
	}
	if len(fake.groups) != 1 {
		t.Fatalf("groups = %+v, want only Backend", fake.groups)
	}

	backend := fake.groups[0]
	if backend.Name != "Backend" || len(backend.MemberRefs) != 2 {
		t.Fatalf("group = %+v, want Backend with anchor and api", backend)
	}
	if fake.titleOf(backend.AnchorRef) != "Group 1" {
		t.Errorf("anchor %s is titled %q, want the restored Group 1", backend.AnchorRef, fake.titleOf(backend.AnchorRef))
	}
	if fake.titleOf(backend.MemberRefs[1]) != "api" {
		t.Errorf("second member is %q, want api", fake.titleOf(backend.MemberRefs[1]))
	}
}

// TestRestoreGroups_UnknownGroupRestoredLoose: a workspace naming a group the
// layout does not define is restored loose, with a warning.
func TestRestoreGroups_UnknownGroupRestoredLoose(t *testing.T) {
	t.Parallel()

	layout := &model.Layout{
		Name: "unknown",
		Workspaces: []model.Workspace{
			{Title: "lost", CWD: "/w/lost", Index: 0, Group: "Ghost", Panes: restoreSinglePane()},
		},
	}

	fake := newRestoreGroupFake()
	restorer := &Restorer{Client: fake, Store: storeRestoreLayout(t, layout), SkipPing: true}

	result, err := restorer.Restore("unknown", false, RestoreModeAdd, "", true)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	assertRestoreNoErrors(t, result)

	assertRestoreCalls(t, fake.callsWith("new-workspace", "create-group"), []string{"new-workspace /w/lost group="})
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "Ghost") {
		t.Errorf("warnings = %v, want one naming the unknown group", result.Warnings)
	}
}

// TestRestoreGroups_NoGroupManagerRestoresLooseWithOneWarning: Ghostty has no
// groups; every workspace comes back loose, with one warning and no error.
func TestRestoreGroups_NoGroupManagerRestoresLooseWithOneWarning(t *testing.T) {
	t.Parallel()

	layout := restoreOneGroupLayout()
	layout.Groups = append(layout.Groups, model.Group{Name: "Frontend"})
	layout.Workspaces = append(layout.Workspaces, model.Workspace{
		Title: "Group 2", CWD: "/w/web", Index: 3, Group: "Frontend", GroupAnchor: true, Panes: restoreSinglePane(),
	})

	fake := &restoreFake{}
	restorer := &Restorer{Client: fake, Store: storeRestoreLayout(t, layout), SkipPing: true}

	result, err := restorer.Restore("one-group", false, RestoreModeAdd, "", true)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	assertRestoreNoErrors(t, result)

	if result.WorkspacesOK != 4 {
		t.Errorf("WorkspacesOK = %d, want 4", result.WorkspacesOK)
	}
	for _, call := range fake.callsWith("new-workspace") {
		if !strings.HasSuffix(call, "group=") {
			t.Errorf("%q asks for a group on a backend without groups", call)
		}
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "2 groups") {
		t.Errorf("warnings = %v, want exactly one mentioning 2 groups", result.Warnings)
	}
}

// TestRestoreGroups_ListGroupsFailureFallsBackLoose: when the live groups
// cannot be listed, restore goes on with loose workspaces and says why.
func TestRestoreGroups_ListGroupsFailureFallsBackLoose(t *testing.T) {
	t.Parallel()

	fake := newRestoreGroupFake()
	fake.listErr = fmt.Errorf("socket closed")
	restorer := &Restorer{Client: fake, Store: storeRestoreLayout(t, restoreOneGroupLayout()), SkipPing: true}

	result, err := restorer.Restore("one-group", false, RestoreModeAdd, "", true)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	assertRestoreNoErrors(t, result)

	if len(fake.callsWith("create-group")) > 0 {
		t.Error("groups were created although listing them failed")
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "socket closed") {
		t.Errorf("warnings = %v, want one carrying the list error", result.Warnings)
	}
}

// expectedWithSplitsCalls is the call sequence the restore produced for
// testdata/layouts/with-splits.toml before groups existed, captured from the
// code at that point. A layout without groups must keep producing it.
var expectedWithSplitsCalls = []string{
	"new-workspace /home/user/projects/myapp group=",
	"select workspace:1",
	"focus-pane pane:0 workspace:1",
	"split right workspace:1 ",
	`send workspace:1 surface:split cd '/home/user/projects/myapp' && go test ./...\n`,
	"rename workspace:1 0 dev",
	"pin-workspace workspace:1",
	"new-workspace /home/user/Documents group=",
	"select workspace:2",
	"rename workspace:2 1 docs",
}

// TestRestoreGroups_LayoutWithoutGroupsUnchanged: an existing fixture with no
// groups gives the same calls as before, with or without a GroupManager, and
// no warning.
func TestRestoreGroups_LayoutWithoutGroupsUnchanged(t *testing.T) {
	t.Parallel()

	fixtures := &persist.FileStore{Dir: "../../testdata/layouts"}

	plain := &restoreFake{}
	plainResult, err := (&Restorer{Client: plain, Store: fixtures, SkipPing: true}).Restore("with-splits", false, RestoreModeAdd, "", true)
	if err != nil {
		t.Fatalf("restore without groups: %v", err)
	}

	grouped := newRestoreGroupFake()
	groupedResult, err := (&Restorer{Client: grouped, Store: fixtures, SkipPing: true}).Restore("with-splits", false, RestoreModeAdd, "", true)
	if err != nil {
		t.Fatalf("restore with groups: %v", err)
	}

	assertRestoreCalls(t, plain.calls, expectedWithSplitsCalls)

	// The group-aware backend only adds the initial listing of live groups.
	withoutListing := slices.DeleteFunc(slices.Clone(grouped.calls), func(call string) bool {
		return strings.HasPrefix(call, "list-groups")
	})
	assertRestoreCalls(t, withoutListing, expectedWithSplitsCalls)

	for _, result := range []*RestoreResult{plainResult, groupedResult} {
		if len(result.Warnings) > 0 || len(result.Errors) > 0 {
			t.Errorf("warnings %v / errors %v, want none", result.Warnings, result.Errors)
		}
	}
}

// TestRestoreGroups_DryRunPrintsGroupCommands: the dry run shows the group
// commands in the order a real restore would issue them.
func TestRestoreGroups_DryRunPrintsGroupCommands(t *testing.T) {
	t.Parallel()

	layout := restoreOneGroupLayout()
	layout.Groups[0].Collapsed = true
	layout.Groups[0].Pinned = true

	fake := &restoreFake{}
	restorer := &Restorer{Client: fake, Store: storeRestoreLayout(t, layout)}

	result, err := restorer.Restore("one-group", true, RestoreModeAdd, "", true)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}

	format := client.CmuxDryRun{}
	groupRef := "workspace_group:new_0"
	want := []string{
		format.FmtNewWorkspace("/w/anchor"),
		format.FmtCreateGroup("Backend", "workspace:new_0"),
		format.FmtSetGroupAnchor(groupRef, "workspace:new_0"),
		"# close the extra anchor workspace cmux adds to a new group",
		format.FmtNewWorkspaceInGroup("/w/api", groupRef),
		format.FmtNewWorkspace("/w/notes"),
		format.FmtCollapseGroup(groupRef),
		format.FmtPinGroup(groupRef),
	}

	var got []string
	for _, command := range result.Commands {
		if slices.Contains(want, command) {
			got = append(got, command)
		}
	}
	assertRestoreCalls(t, got, want)

	if len(fake.calls) > 0 {
		t.Errorf("dry run touched the backend: %v", fake.calls)
	}
}

// TestRestoreGroups_DryRunWithoutGroupFormatter: a formatter without group
// commands (Ghostty) gets one comment saying groups are skipped.
func TestRestoreGroups_DryRunWithoutGroupFormatter(t *testing.T) {
	t.Parallel()

	fake := &restoreFake{formatter: client.GhosttyDryRun{}}
	restorer := &Restorer{Client: fake, Store: storeRestoreLayout(t, restoreOneGroupLayout())}

	result, err := restorer.Restore("one-group", true, RestoreModeAdd, "", true)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}

	skipped := 0
	for _, command := range result.Commands {
		if strings.HasPrefix(command, "#") && strings.Contains(command, "groups") {
			skipped++
		}
		if strings.Contains(command, "workspace-group") {
			t.Errorf("group command %q printed for a formatter without groups", command)
		}
	}
	if skipped != 1 {
		t.Errorf("found %d group comments, want exactly one\n%s", skipped, strings.Join(result.Commands, "\n"))
	}
}
