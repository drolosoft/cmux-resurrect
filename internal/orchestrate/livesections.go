package orchestrate

import (
	"fmt"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/model"
)

// LiveSections groups the workspaces of a live window for display, the same
// way model.Layout.Sections groups a saved layout, so `crex now` and the TUI
// show a live window and a saved layout with one shape. Positions in the
// result index win.Workspaces.
//
// It only reads: backends without group support (Ghostty) and a failing
// ListGroups both fall back to the flat listing crex showed before groups,
// because a display command should never fail over decoration.
func LiveSections(c client.Backend, win client.TreeWindow) []model.Section {
	groups, members := liveMemberships(c, win)
	return model.GroupSections(groups, members)
}

// liveMemberships asks the backend for the window's groups and translates
// them into the model's terms: one model.Group per live group and one
// Membership per workspace. Without groups every workspace is loose.
func liveMemberships(c client.Backend, win client.TreeWindow) ([]model.Group, []model.Membership) {
	members := make([]model.Membership, len(win.Workspaces))
	for position, workspace := range win.Workspaces {
		members[position] = model.Membership{Index: workspace.Index}
	}

	manager, ok := c.(client.GroupManager)
	if !ok {
		return nil, members
	}

	live, err := manager.ListGroups(win.Ref)
	if err != nil || len(live) == 0 {
		return nil, members
	}

	// groupOfRef maps every member workspace to the display name of its
	// group, and anchors records which refs are group headers.
	groupOfRef := make(map[string]string)
	anchors := make(map[string]bool, len(live))
	groups := make([]model.Group, 0, len(live))

	// seen counts how often each name appeared: sections are keyed by name,
	// so two live groups called the same must get distinct labels or their
	// members would merge under one header. Save uses the same "(2)" form.
	seen := make(map[string]int, len(live))

	for _, group := range live {
		seen[group.Name]++
		name := group.Name
		if seen[group.Name] > 1 {
			name = fmt.Sprintf("%s (%d)", group.Name, seen[group.Name])
		}

		groups = append(groups, model.Group{
			Name:      name,
			Collapsed: group.Collapsed,
			Pinned:    group.Pinned,
			Color:     group.Color,
			Icon:      group.Icon,
		})
		for _, ref := range group.MemberRefs {
			groupOfRef[ref] = name
		}
		anchors[group.AnchorRef] = true
	}

	for position, workspace := range win.Workspaces {
		members[position].Group = groupOfRef[workspace.Ref]
		members[position].Anchor = anchors[workspace.Ref]
	}

	return groups, members
}
