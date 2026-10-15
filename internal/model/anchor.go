package model

import "regexp"

// defaultGroupTitle is the title cmux gives the anchor workspace of a group
// it creates. cmux never renames it, and its number has nothing to do with
// the group, so it tells the user nothing about which group it heads.
var defaultGroupTitle = regexp.MustCompile(`^Group \d+$`)

// IsDefaultGroupTitle reports whether title is the "Group N" placeholder
// cmux gives a group's anchor. Output that lists workspaces uses it to name
// such an anchor by its group instead of printing the placeholder.
func IsDefaultGroupTitle(title string) bool {
	return defaultGroupTitle.MatchString(title)
}

// AnchorTitle is the title a group's anchor goes by: the group name while the
// anchor still has cmux's "Group N" placeholder or no title at all, its own
// title once the user has chosen one. Without a group name there is nothing
// better to offer, so the title is returned as it is.
func AnchorTitle(title, groupName string) string {
	if groupName == "" {
		return title
	}

	if title == "" || IsDefaultGroupTitle(title) {
		return groupName
	}

	return title
}

// AnchorShownAsGroup reports whether an anchor listed under its group's
// header reads better as "(anchor)": its title is the group name, or still
// cmux's "Group N" placeholder, so the header already says what it is.
// save, restore, show, now and the TUI all print anchors by this rule.
func AnchorShownAsGroup(title, groupName string) bool {
	return groupName != "" && AnchorTitle(title, groupName) == groupName
}

// EffectiveTitle is the title crex saves, restores and matches the workspace
// by. Only a group's anchor can differ from Title, as AnchorTitle explains;
// every other workspace keeps its own title, placeholder-looking or not.
func (ws Workspace) EffectiveTitle() string {
	if !ws.GroupAnchor {
		return ws.Title
	}

	return AnchorTitle(ws.Title, ws.Group)
}

// NameAnchorsAfterGroups replaces the title of every anchor by its effective
// title. Layouts saved by v1.28.0 stored the placeholder ("Group 2"), and
// running them through this makes them read like newer saves.
func (l *Layout) NameAnchorsAfterGroups() {
	for index := range l.Workspaces {
		ws := &l.Workspaces[index]
		ws.Title = ws.EffectiveTitle()
	}
}
