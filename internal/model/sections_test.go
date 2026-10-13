package model

import (
	"reflect"
	"testing"
)

// sectionShape flattens sections into something a table test can compare:
// the group name ("" for a loose run), the member positions and the anchor.
type sectionShape struct {
	Group   string
	Members []int
	Anchor  int
}

// shapeOf turns the helper's output into comparable values, so a failure
// prints the whole listing instead of a pointer.
func shapeOf(sections []Section) []sectionShape {
	shapes := make([]sectionShape, len(sections))
	for index, section := range sections {
		name := ""
		if section.Group != nil {
			name = section.Group.Name
		}
		shapes[index] = sectionShape{Group: name, Members: section.Members, Anchor: section.Anchor}
	}

	return shapes
}

func TestGroupSections_NoGroupsKeepsSliceOrder(t *testing.T) {
	// Indexes deliberately out of order: without groups the listing must be
	// exactly what show printed before groups existed, which is slice order.
	members := []Membership{{Index: 2}, {Index: 0}, {Index: 1}}

	got := shapeOf(GroupSections(nil, members))
	want := []sectionShape{{Members: []int{0, 1, 2}, Anchor: -1}}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("sections = %+v, want %+v", got, want)
	}
}

func TestGroupSections_Empty(t *testing.T) {
	if got := GroupSections(nil, nil); len(got) != 0 {
		t.Errorf("sections = %+v, want none", got)
	}
}

func TestGroupSections_OrdersByIndexAnchorFirstLooseBetween(t *testing.T) {
	groups := []Group{{Name: "🎮 Cmux", Pinned: true}, {Name: "Docs", Collapsed: true}}
	members := []Membership{
		{Index: 4, Group: "Docs", Anchor: true},
		{Index: 0},
		{Index: 2, Group: "🎮 Cmux", Anchor: true},
		{Index: 1, Group: "🎮 Cmux"},
		{Index: 3},
		{Index: 5, Group: "Docs"},
		{Index: 6},
	}

	got := shapeOf(GroupSections(groups, members))
	want := []sectionShape{
		{Members: []int{1}, Anchor: -1},
		{Group: "🎮 Cmux", Members: []int{2, 3}, Anchor: 2},
		{Members: []int{4}, Anchor: -1},
		{Group: "Docs", Members: []int{0, 5}, Anchor: 0},
		{Members: []int{6}, Anchor: -1},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("sections =\n%+v\nwant\n%+v", got, want)
	}
}

func TestGroupSections_UnknownGroupIsLoose(t *testing.T) {
	groups := []Group{{Name: "Work"}}
	members := []Membership{
		{Index: 0, Group: "Work", Anchor: true},
		{Index: 1, Group: "Ghost"},
		{Index: 2},
	}

	got := shapeOf(GroupSections(groups, members))
	want := []sectionShape{
		{Group: "Work", Members: []int{0}, Anchor: 0},
		{Members: []int{1, 2}, Anchor: -1},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("sections = %+v, want %+v", got, want)
	}
}

func TestGroupSections_AllUnknownFallsBackToSliceOrder(t *testing.T) {
	// A layout whose [[group]] table was deleted by hand still names groups
	// on its workspaces; it must render like a layout without groups.
	members := []Membership{{Index: 1, Group: "Gone"}, {Index: 0, Group: "Gone"}}

	got := shapeOf(GroupSections(nil, members))
	want := []sectionShape{{Members: []int{0, 1}, Anchor: -1}}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("sections = %+v, want %+v", got, want)
	}
}

func TestGroupSections_NoAnchorAndDoubleAnchor(t *testing.T) {
	groups := []Group{{Name: "A"}, {Name: "B"}}
	members := []Membership{
		{Index: 0, Group: "A"},
		{Index: 1, Group: "A"},
		{Index: 2, Group: "B"},
		{Index: 3, Group: "B", Anchor: true},
		{Index: 4, Group: "B", Anchor: true},
	}

	got := shapeOf(GroupSections(groups, members))
	want := []sectionShape{
		{Group: "A", Members: []int{0, 1}, Anchor: -1},
		{Group: "B", Members: []int{3, 2, 4}, Anchor: 3},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("sections = %+v, want %+v", got, want)
	}
}

func TestGroupSections_GroupPointsIntoSlice(t *testing.T) {
	groups := []Group{{Name: "A", Pinned: true}}
	sections := GroupSections(groups, []Membership{{Group: "A"}})

	if sections[0].Group != &groups[0] {
		t.Error("section group should point at the caller's group, not a copy")
	}
}

func TestLayoutSections(t *testing.T) {
	layout := Layout{
		Groups: []Group{{Name: "G"}},
		Workspaces: []Workspace{
			{Title: "loose", Index: 0},
			{Title: "member", Index: 2, Group: "G"},
			{Title: "anchor", Index: 1, Group: "G", GroupAnchor: true},
		},
	}

	got := shapeOf(layout.Sections())
	want := []sectionShape{
		{Members: []int{0}, Anchor: -1},
		{Group: "G", Members: []int{2, 1}, Anchor: 2},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("sections = %+v, want %+v", got, want)
	}
}

func TestLayoutMetaSections(t *testing.T) {
	plain := LayoutMeta{WorkspaceTitles: []string{"a", "b"}}
	if got := shapeOf(plain.Sections()); !reflect.DeepEqual(got, []sectionShape{{Members: []int{0, 1}, Anchor: -1}}) {
		t.Errorf("plain meta sections = %+v", got)
	}

	grouped := LayoutMeta{
		WorkspaceTitles: []string{"loose", "anchor"},
		Groups:          []Group{{Name: "G"}},
		Memberships:     []Membership{{Index: 0}, {Index: 1, Group: "G", Anchor: true}},
	}
	want := []sectionShape{{Members: []int{0}, Anchor: -1}, {Group: "G", Members: []int{1}, Anchor: 1}}
	if got := shapeOf(grouped.Sections()); !reflect.DeepEqual(got, want) {
		t.Errorf("grouped meta sections = %+v, want %+v", got, want)
	}
}

func TestSectionIsAnchor(t *testing.T) {
	section := Section{Group: &Group{Name: "G"}, Members: []int{3, 1}, Anchor: 3}

	if !section.IsAnchor(3) || section.IsAnchor(1) {
		t.Error("IsAnchor should be true only for the anchor position")
	}

	loose := Section{Members: []int{0}, Anchor: -1}
	if loose.IsAnchor(0) {
		t.Error("a loose section has no anchor")
	}
}
