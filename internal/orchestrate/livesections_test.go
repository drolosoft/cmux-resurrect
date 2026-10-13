package orchestrate

import (
	"errors"
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/client"
)

// groupListingBackend is a backend that only answers ListGroups. The embedded
// interfaces are nil: any other call panics, which proves LiveSections never
// touches the live app beyond reading the groups.
type groupListingBackend struct {
	client.Backend
	client.GroupManager

	groups    []client.WorkspaceGroup
	err       error
	windowRef string
}

// ListGroups records the window it was asked about and returns the fixture.
func (b *groupListingBackend) ListGroups(windowRef string) ([]client.WorkspaceGroup, error) {
	b.windowRef = windowRef
	return b.groups, b.err
}

// plainBackend has no GroupManager at all, like Ghostty.
type plainBackend struct {
	client.Backend
}

// liveWindowFixture is a cmux window with a group in the middle of the
// sidebar: a loose workspace, the group's anchor and member, another loose one.
func liveWindowFixture() client.TreeWindow {
	return client.TreeWindow{
		Ref: "window:2",
		Workspaces: []client.TreeWorkspace{
			{Ref: "workspace:1", Title: "notes", Index: 0},
			{Ref: "workspace:7", Title: "Group 7", Index: 1},
			{Ref: "workspace:8", Title: "crex", Index: 2},
			{Ref: "workspace:9", Title: "mail", Index: 3},
		},
	}
}

func TestLiveSections_GroupsTheCallersWindow(t *testing.T) {
	backend := &groupListingBackend{groups: []client.WorkspaceGroup{{
		Ref:        "workspace_group:1",
		Name:       "🎮 Cmux",
		AnchorRef:  "workspace:7",
		MemberRefs: []string{"workspace:7", "workspace:8"},
		Pinned:     true,
		Collapsed:  true,
	}}}

	sections := LiveSections(backend, liveWindowFixture())

	if backend.windowRef != "window:2" {
		t.Errorf("ListGroups asked for window %q, want window:2", backend.windowRef)
	}
	if len(sections) != 3 {
		t.Fatalf("got %d sections, want 3: %+v", len(sections), sections)
	}

	group := sections[1].Group
	if group == nil || group.Name != "🎮 Cmux" || !group.Pinned || !group.Collapsed {
		t.Errorf("middle section group = %+v, want pinned collapsed 🎮 Cmux", group)
	}
	if !sections[1].IsAnchor(1) || len(sections[1].Members) != 2 || sections[1].Members[1] != 2 {
		t.Errorf("group section = %+v, want anchor 1 then member 2", sections[1])
	}
	if sections[0].Group != nil || sections[2].Group != nil {
		t.Error("the workspaces outside the group should be loose")
	}
}

func TestLiveSections_DuplicateNamesStayApart(t *testing.T) {
	window := liveWindowFixture()
	backend := &groupListingBackend{groups: []client.WorkspaceGroup{
		{Name: "Work", AnchorRef: "workspace:1", MemberRefs: []string{"workspace:1"}},
		{Name: "Work", AnchorRef: "workspace:8", MemberRefs: []string{"workspace:8"}},
	}}

	sections := LiveSections(backend, window)

	var names []string
	for _, section := range sections {
		if section.Group != nil {
			names = append(names, section.Group.Name)
		}
	}
	if len(names) != 2 || names[0] != "Work" || names[1] != "Work (2)" {
		t.Errorf("group headers = %q, want [Work, Work (2)]", names)
	}
}

func TestLiveSections_FallsBackToFlatListing(t *testing.T) {
	window := liveWindowFixture()

	cases := map[string]client.Backend{
		"no group support": &plainBackend{},
		"list error":       &groupListingBackend{err: errors.New("socket closed")},
		"no groups":        &groupListingBackend{},
	}

	for name, backend := range cases {
		t.Run(name, func(t *testing.T) {
			sections := LiveSections(backend, window)

			if len(sections) != 1 || sections[0].Group != nil || len(sections[0].Members) != 4 {
				t.Errorf("sections = %+v, want one loose section with all four", sections)
			}
		})
	}
}
