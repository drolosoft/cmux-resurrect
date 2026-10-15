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
