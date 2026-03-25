package client

// CmuxClient abstracts interaction with cmux.
// v1 uses CLI exec; future versions can use the Unix socket directly.
type CmuxClient interface {
	// Ping checks if cmux is running and reachable.
	Ping() error

	// Tree returns the full workspace/pane hierarchy.
	Tree() (*TreeResponse, error)

	// SidebarState returns metadata (CWD, git info) for a workspace.
	SidebarState(workspaceRef string) (*SidebarState, error)

	// ListWorkspaces returns all workspaces with their refs and titles.
	ListWorkspaces() ([]WorkspaceInfo, error)

	// NewWorkspace creates a new workspace, returning its ref.
	NewWorkspace(opts NewWorkspaceOpts) (string, error)

	// RenameWorkspace renames a workspace.
	RenameWorkspace(ref, title string) error

	// SelectWorkspace makes a workspace the active/visible one.
	SelectWorkspace(ref string) error

	// NewSplit creates a new split pane in a workspace.
	NewSplit(direction, workspaceRef string) error

	// FocusPane focuses a specific pane in a workspace.
	FocusPane(paneRef, workspaceRef string) error

	// Send sends text to a surface in a workspace.
	Send(workspaceRef, surfaceRef, text string) error

	// PinWorkspace pins a workspace in the sidebar.
	PinWorkspace(ref string) error
}
