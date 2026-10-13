package tui

import "github.com/drolosoft/cmux-resurrect/internal/model"

// groupIndent is the extra margin of a workspace listed inside a group, the
// same step the CLI uses in `crex show` and `crex now`.
const groupIndent = "  "

// groupHeader renders a workspace group's header line with the badges the
// CLI prints for it, pinned and collapsed, so a group reads the same in the
// shell and on the command line.
func groupHeader(group model.Group) string {
	header := shellGroupStyle.Render(group.Name)
	if group.Pinned {
		header += " 📌"
	}
	if group.Collapsed {
		header += " " + shellDimStyle.Render("▸ collapsed")
	}

	return header
}

// anchorTag is the dim marker after the title of a group's anchor workspace.
// The anchor is the group's header row in cmux and its own title ("Group 7")
// rarely matches the group name, so without the tag it looks like a stray.
func anchorTag() string {
	return " " + shellDimStyle.Render("anchor")
}
