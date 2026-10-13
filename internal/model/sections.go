package model

import "sort"

// Membership is the part of a workspace that decides where it sits in a
// grouped listing. Saved layouts, layout metadata and the live cmux window
// all describe workspaces differently; each of them reduces to a slice of
// Membership so the CLI and the TUI group workspaces with the same code.
type Membership struct {
	// Group is the Name of the workspace's group, "" for a loose workspace.
	Group string `json:"group,omitempty"`

	// Anchor marks the workspace that is its group's header in cmux.
	Anchor bool `json:"anchor,omitempty"`

	// Index is the sidebar position, which orders a grouped listing.
	Index int `json:"index"`
}

// Section is one block of a grouped workspace listing: either a group with
// its members, or a run of loose workspaces that sit between groups.
type Section struct {
	// Group points into the groups slice handed to GroupSections, or is nil
	// for a run of loose workspaces.
	Group *Group

	// Members are positions in the slice handed to GroupSections, in the
	// order they should be displayed: the anchor first, then by Index.
	Members []int

	// Anchor is the position of the group's anchor, or -1 when the section
	// is loose or no member is flagged as anchor.
	Anchor int
}

// IsAnchor reports whether position is the anchor of this section. Renderers
// use it to tag the anchor without reaching back into the source data.
func (s Section) IsAnchor(position int) bool {
	return s.Group != nil && s.Anchor == position
}

// GroupSections splits workspaces into display sections. It is the single
// place where crex decides how groups look in a listing, shared by `crex
// show`, `crex now` and every TUI view, so the CLI and the TUI cannot drift.
//
// When no workspace belongs to a known group the result is one loose section
// in slice order, which keeps the output of layouts without groups exactly as
// it was before groups existed. Otherwise workspaces are ordered by Index; a
// group appears where its first member appears, with its anchor moved to the
// front, and the loose workspaces between groups form their own sections. A
// workspace naming a group that is not in groups is treated as loose.
//
// The returned sections point into groups, so callers must keep that slice
// alive and unmodified while they render.
func GroupSections(groups []Group, members []Membership) []Section {
	if len(members) == 0 {
		return nil
	}

	// Look groups up by name once; the pointer goes into the caller's slice
	// so renderers see the real group state.
	byName := make(map[string]*Group, len(groups))
	for index := range groups {
		byName[groups[index].Name] = &groups[index]
	}

	if !anyKnownGroup(byName, members) {
		return []Section{looseSection(sliceOrder(len(members)))}
	}

	ordered := sliceOrder(len(members))
	sort.SliceStable(ordered, func(left, right int) bool {
		return members[ordered[left]].Index < members[ordered[right]].Index
	})

	var sections []Section

	// sectionOf remembers which section each group already opened, so a
	// member that comes later joins it instead of opening a second header.
	sectionOf := make(map[string]int, len(groups))

	for _, position := range ordered {
		group := byName[members[position].Group]

		if group == nil {
			last := len(sections) - 1
			if last >= 0 && sections[last].Group == nil {
				sections[last].Members = append(sections[last].Members, position)
				continue
			}

			sections = append(sections, looseSection([]int{position}))
			continue
		}

		target, opened := sectionOf[group.Name]
		if !opened {
			sections = append(sections, Section{Group: group, Anchor: -1})
			target = len(sections) - 1
			sectionOf[group.Name] = target
		}

		sections[target].Members = append(sections[target].Members, position)
		if members[position].Anchor && sections[target].Anchor < 0 {
			sections[target].Anchor = position
		}
	}

	for index := range sections {
		moveAnchorFirst(&sections[index])
	}

	return sections
}

// Memberships returns the group membership of every workspace, in the same
// order as l.Workspaces.
func (l *Layout) Memberships() []Membership {
	members := make([]Membership, len(l.Workspaces))
	for index, workspace := range l.Workspaces {
		members[index] = Membership{Group: workspace.Group, Anchor: workspace.GroupAnchor, Index: workspace.Index}
	}

	return members
}

// Sections groups the layout's workspaces for display; positions in the
// result index l.Workspaces.
func (l *Layout) Sections() []Section {
	return GroupSections(l.Groups, l.Memberships())
}

// Sections groups the summarised workspaces for display; positions in the
// result index WorkspaceTitles, which is why the titles, not WorkspaceCount,
// set the length. Metadata written for a layout without groups carries no
// memberships, so every workspace counts as loose.
func (m LayoutMeta) Sections() []Section {
	members := m.Memberships
	if len(members) != len(m.WorkspaceTitles) {
		members = make([]Membership, len(m.WorkspaceTitles))
	}

	return GroupSections(m.Groups, members)
}

// anyKnownGroup reports whether at least one workspace belongs to a group
// that exists, which is what switches a listing into grouped mode.
func anyKnownGroup(byName map[string]*Group, members []Membership) bool {
	for _, member := range members {
		if byName[member.Group] != nil {
			return true
		}
	}

	return false
}

// sliceOrder returns the positions 0..count-1 in order.
func sliceOrder(count int) []int {
	positions := make([]int, count)
	for index := range positions {
		positions[index] = index
	}

	return positions
}

// looseSection builds a section for workspaces outside any group.
func looseSection(positions []int) Section {
	return Section{Members: positions, Anchor: -1}
}

// moveAnchorFirst puts the anchor at the front of its section, because in
// the cmux sidebar the anchor is the group's header row.
func moveAnchorFirst(section *Section) {
	if section.Anchor < 0 {
		return
	}

	for index, position := range section.Members {
		if position != section.Anchor {
			continue
		}

		copy(section.Members[1:index+1], section.Members[:index])
		section.Members[0] = position
		return
	}
}
