package client

import "fmt"

// DryRunFormatter generates human-readable command strings for dry-run mode.
type DryRunFormatter interface {
	FmtNewWorkspace(cwd string) string
	FmtRenameWorkspace(ref, title string) string
	FmtSelectWorkspace(ref string) string
	FmtNewSplit(direction, ref string) string
	FmtFocusPane(paneRef, workspaceRef string) string
	FmtSend(workspaceRef, text string) string
	FmtPinWorkspace(ref string) string
	FmtUnpinWorkspace(ref string) string
	FmtNewPane(paneType, direction, ref, url, profile string) string
	FmtNewSurface(paneRef, workspaceRef string) string
}

// CmuxDryRun formats dry-run commands as cmux CLI commands.
type CmuxDryRun struct{}

func (CmuxDryRun) FmtNewWorkspace(cwd string) string {
	return fmt.Sprintf("cmux new-workspace --cwd %q", cwd)
}
func (CmuxDryRun) FmtRenameWorkspace(ref, title string) string {
	return fmt.Sprintf("cmux rename-workspace --workspace %s %q", ref, title)
}
func (CmuxDryRun) FmtSelectWorkspace(ref string) string {
	return fmt.Sprintf("cmux select-workspace --workspace %s", ref)
}
func (CmuxDryRun) FmtNewSplit(direction, ref string) string {
	return fmt.Sprintf("cmux new-split %s --workspace %s", direction, ref)
}
func (CmuxDryRun) FmtFocusPane(paneRef, workspaceRef string) string {
	return fmt.Sprintf("cmux focus-pane --pane %s --workspace %s", paneRef, workspaceRef)
}
func (CmuxDryRun) FmtSend(workspaceRef, text string) string {
	return fmt.Sprintf("cmux send --workspace %s %q", workspaceRef, text)
}
func (CmuxDryRun) FmtPinWorkspace(ref string) string {
	return fmt.Sprintf("cmux workspace-action --action pin --workspace %s", ref)
}
func (CmuxDryRun) FmtUnpinWorkspace(ref string) string {
	return fmt.Sprintf("cmux workspace-action --action unpin --workspace %s", ref)
}
func (CmuxDryRun) FmtNewPane(paneType, direction, ref, url, profile string) string {
	cmd := fmt.Sprintf("cmux new-pane --type %s --direction %s --workspace %s", paneType, direction, ref)
	if url != "" {
		cmd += fmt.Sprintf(" --url %q", url)
	}
	if profile != "" {
		cmd += fmt.Sprintf(" --profile %s", profile)
	}
	return cmd
}
func (CmuxDryRun) FmtNewSurface(paneRef, workspaceRef string) string {
	return fmt.Sprintf("cmux new-surface --pane %s --workspace %s", paneRef, workspaceRef)
}

// GroupDryRunFormatter is optionally implemented by dry-run formatters whose
// backend has sidebar groups. Restore detects it via type assertion, like
// GroupManager on the backend; only CmuxDryRun implements it.
type GroupDryRunFormatter interface {
	FmtCreateGroup(name, anchorRef string) string
	FmtSetGroupAnchor(groupRef, workspaceRef string) string
	FmtNewWorkspaceInGroup(cwd, groupRef string) string
	FmtCollapseGroup(groupRef string) string
	FmtPinGroup(groupRef string) string
	FmtExpandGroup(groupRef string) string
	FmtUnpinGroup(groupRef string) string
	FmtSetGroupColor(groupRef, hex string) string
	FmtSetGroupIcon(groupRef, symbol string) string
}

// The group commands below follow `cmux workspace-group --help`: some take
// the group positionally, others through --group.
var _ GroupDryRunFormatter = CmuxDryRun{}

// FmtCreateGroup prints the create call that seeds a group from its anchor.
// The name is quoted because group names often carry spaces and emoji.
func (CmuxDryRun) FmtCreateGroup(name, anchorRef string) string {
	return fmt.Sprintf("cmux workspace-group create --name %q --from %s", name, anchorRef)
}

// FmtSetGroupAnchor prints the call that hands the group to our own anchor.
func (CmuxDryRun) FmtSetGroupAnchor(groupRef, workspaceRef string) string {
	return fmt.Sprintf("cmux workspace-group set-anchor --group %s --workspace %s", groupRef, workspaceRef)
}

// FmtNewWorkspaceInGroup prints the creation of a member at the end of its
// group, which is how restore keeps the saved member order.
func (CmuxDryRun) FmtNewWorkspaceInGroup(cwd, groupRef string) string {
	return fmt.Sprintf("cmux new-workspace --cwd %q --group %s --group-placement end", cwd, groupRef)
}

// FmtCollapseGroup prints the call that folds a group in the sidebar.
func (CmuxDryRun) FmtCollapseGroup(groupRef string) string {
	return fmt.Sprintf("cmux workspace-group collapse %s", groupRef)
}

// FmtPinGroup prints the call that pins a group to the top of the sidebar.
func (CmuxDryRun) FmtPinGroup(groupRef string) string {
	return fmt.Sprintf("cmux workspace-group pin %s", groupRef)
}

// FmtExpandGroup prints the call that unfolds a group in the sidebar.
func (CmuxDryRun) FmtExpandGroup(groupRef string) string {
	return fmt.Sprintf("cmux workspace-group expand %s", groupRef)
}

// FmtUnpinGroup prints the call that unpins a group.
func (CmuxDryRun) FmtUnpinGroup(groupRef string) string {
	return fmt.Sprintf("cmux workspace-group unpin %s", groupRef)
}

// FmtSetGroupColor prints the color call. The hex is quoted so the line can
// be pasted into a shell, where an unquoted # would start a comment.
func (CmuxDryRun) FmtSetGroupColor(groupRef, hex string) string {
	return fmt.Sprintf("cmux workspace-group set-color %s --hex %q", groupRef, hex)
}

// FmtSetGroupIcon prints the icon call with its SF Symbol name.
func (CmuxDryRun) FmtSetGroupIcon(groupRef, symbol string) string {
	return fmt.Sprintf("cmux workspace-group set-icon %s --symbol %q", groupRef, symbol)
}

// GhosttyDryRun formats dry-run commands as Ghostty AppleScript snippets.
type GhosttyDryRun struct{}

func (GhosttyDryRun) FmtNewWorkspace(cwd string) string {
	return fmt.Sprintf(`osascript: new tab in front window (cwd: %s)`, cwd)
}
func (GhosttyDryRun) FmtRenameWorkspace(ref, title string) string {
	return fmt.Sprintf(`osascript: set_tab_title:%q on %s`, title, ref)
}
func (GhosttyDryRun) FmtSelectWorkspace(ref string) string {
	return fmt.Sprintf(`osascript: select %s`, ref)
}
func (GhosttyDryRun) FmtNewSplit(direction, ref string) string {
	return fmt.Sprintf(`osascript: split %s in %s`, direction, ref)
}
func (GhosttyDryRun) FmtFocusPane(paneRef, workspaceRef string) string {
	return fmt.Sprintf(`osascript: focus %s in %s`, paneRef, workspaceRef)
}
func (GhosttyDryRun) FmtSend(workspaceRef, text string) string {
	return fmt.Sprintf(`osascript: input text %q + enter in %s`, text, workspaceRef)
}
func (GhosttyDryRun) FmtPinWorkspace(ref string) string {
	return "# pin: not supported by Ghostty"
}
func (GhosttyDryRun) FmtUnpinWorkspace(ref string) string {
	return "# unpin: not supported by Ghostty"
}
func (GhosttyDryRun) FmtNewPane(paneType, direction, ref, url, _ string) string {
	if paneType == "browser" && url != "" {
		return fmt.Sprintf("osascript: split %s in %s + open %q in system browser", direction, ref, url)
	}
	return fmt.Sprintf("osascript: split %s in %s", direction, ref)
}
func (GhosttyDryRun) FmtNewSurface(paneRef, workspaceRef string) string {
	// Ghostty has no sub-tabs; restore recreates each one as a split beside
	// the pane so its shell, folder and session survive.
	return fmt.Sprintf("osascript: split right of %s (pane tab → split)", paneRef)
}
