package cmd

import (
	"fmt"
	"io"

	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/orchestrate"
)

// progressIndent is the left margin of every save and restore result line.
const progressIndent = "  "

// renderSaveSummary prints what `crex save` captured: one line per
// workspace, under its group header like `crex show`, then the total. A
// layout without groups comes back as one loose section in slice order, so
// it prints exactly the flat list it printed before groups existed.
func renderSaveSummary(w io.Writer, layout *model.Layout, path string) {
	o := newWF(w)
	o.ln()

	browserCount := 0
	for _, section := range layout.Sections() {
		indent := progressIndent
		if section.Group != nil {
			o.f("%s%s\n", progressIndent, showGroupHeader(*section.Group))
			indent = progressIndent + showGroupIndent
		}

		for _, position := range section.Members {
			ws := layout.Workspaces[position]
			o.f("%s%s  %s%s%s\n",
				indent,
				greenStyle.Render("OK"),
				workspaceLabel(ws, section.Group != nil),
				dimStyle.Render(savedPaneCount(ws)),
				pinBadge(ws))

			browserCount += countBrowserPanes(ws)
		}
	}

	successMsg := fmt.Sprintf("✅ Saved %d %s to %s", len(layout.Workspaces), unitName(len(layout.Workspaces)), path)
	if browserCount > 0 {
		successMsg += fmt.Sprintf(" (%d browser)", browserCount)
	}
	o.ln()
	o.f("%s\n", greenStyle.Render(successMsg))
	o.ln()
}

// savedPaneCount is the pane note of a save line, left out for a single
// pane because that is the common case and would only add noise.
func savedPaneCount(ws model.Workspace) string {
	if len(ws.Panes) <= 1 {
		return ""
	}

	return fmt.Sprintf(" (%d panes)", len(ws.Panes))
}

// pinBadge marks a pinned workspace on a save line.
func pinBadge(ws model.Workspace) string {
	if !ws.Pinned {
		return ""
	}

	return " 📌"
}

// countBrowserPanes returns how many of the workspace's panes are browsers,
// which the save total reports on its own.
func countBrowserPanes(ws model.Workspace) int {
	count := 0
	for _, pane := range ws.Panes {
		if pane.Type == "browser" {
			count++
		}
	}

	return count
}

// workspaceLabel is how a save or restore line names a workspace. An anchor
// still carrying cmux's "Group N" placeholder is printed as "(anchor)" under
// its group header, because the placeholder's number has nothing to do with
// the group and only confuses; restored loose, it reads "(group anchor)". An
// anchor the user renamed keeps its title with the dim "anchor" tag `crex
// show` uses.
func workspaceLabel(ws model.Workspace, inGroup bool) string {
	if !ws.GroupAnchor {
		return padTitle(ws.Title)
	}

	if model.IsDefaultGroupTitle(ws.Title) {
		if inGroup {
			return dimStyle.Render("(anchor)")
		}

		return dimStyle.Render("(group anchor)")
	}

	return padTitle(ws.Title) + " " + dimStyle.Render("anchor")
}

// restoreProgress prints the per-workspace lines of `crex restore` and `crex
// pop` as Restore reports them, with a group header before the first member
// of each group, so the output reads like `crex show`.
type restoreProgress struct {
	out io.Writer

	// group is the name of the group whose header was printed last, "" while
	// loose workspaces are being printed.
	group string
}

// newRestoreProgress returns a printer that writes to out.
func newRestoreProgress(out io.Writer) *restoreProgress {
	return &restoreProgress{out: out}
}

// attach wires the printer into a Restorer.
func (p *restoreProgress) attach(restorer *orchestrate.Restorer) {
	restorer.OnWorkspace = p.workspace
	restorer.OnProgress = p.notice
}

// workspace prints one SKIP, FAIL or OK line, opening a group header first
// when the workspace starts a new group. Restore walks the sidebar in order
// and cmux keeps a group's members together, so a header normally appears
// once per group.
func (p *restoreProgress) workspace(progress orchestrate.WorkspaceProgress) {
	o := newWF(p.out)

	groupName := ""
	if progress.Group != nil {
		groupName = progress.Group.Name
	}
	if groupName != "" && groupName != p.group {
		o.f("%s%s\n", progressIndent, showGroupHeader(*progress.Group))
	}
	p.group = groupName

	ws := progress.Workspace
	label := workspaceLabel(ws, progress.Group != nil)

	switch {
	case progress.Skipped:
		o.f("%s%s  %s %s\n", p.indent(), dimStyle.Render("SKIP"), label, dimStyle.Render("("+progress.Err.Error()+")"))
	case progress.Err != nil:
		o.f("%s%s  %s: %v\n", p.indent(), yellowStyle.Render("FAIL"), label, progress.Err)
	default:
		o.f("%s%s  %s (%d panes)\n", p.indent(), greenStyle.Render("OK"), label, len(ws.Panes))
	}
}

// notice prints a message Restore sends that is not about a whole
// workspace, such as an auto-accept flag, at the indent of the workspace it
// came from. The pane count and error are unused: notices never carry them.
func (p *restoreProgress) notice(message string, _ int, _ error) {
	newWF(p.out).f("%s%s  %s\n", p.indent(), dimStyle.Render("INFO"), message)
}

// indent is the margin of the next line: one level deeper inside a group.
func (p *restoreProgress) indent() string {
	if p.group != "" {
		return progressIndent + showGroupIndent
	}

	return progressIndent
}
