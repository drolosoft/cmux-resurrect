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
	FmtNewPane(paneType, direction, ref, url string) string
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
func (CmuxDryRun) FmtNewPane(paneType, direction, ref, url string) string {
	cmd := fmt.Sprintf("cmux new-pane --type %s --direction %s --workspace %s", paneType, direction, ref)
	if url != "" {
		cmd += fmt.Sprintf(" --url %q", url)
	}
	return cmd
}
func (CmuxDryRun) FmtNewSurface(paneRef, workspaceRef string) string {
	return fmt.Sprintf("cmux new-surface --pane %s --workspace %s", paneRef, workspaceRef)
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
func (GhosttyDryRun) FmtNewPane(paneType, direction, ref, url string) string {
	if paneType == "browser" && url != "" {
		return fmt.Sprintf("osascript: split %s in %s + open %q in system browser", direction, ref, url)
	}
	return fmt.Sprintf("osascript: split %s in %s", direction, ref)
}
func (GhosttyDryRun) FmtNewSurface(paneRef, workspaceRef string) string {
	return fmt.Sprintf("osascript: new surface in %s (not supported)", paneRef)
}
