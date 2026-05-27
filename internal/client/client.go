package client

import "fmt"

// Backend abstracts interaction with a terminal multiplexer/emulator.
// Implementations exist for cmux (CLIClient) and Ghostty (planned).
type Backend interface {
	// Ping checks if the backend is running and reachable.
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

	// NewSplit creates a new split pane in a workspace, returning the new surface ref.
	NewSplit(direction, workspaceRef string) (string, error)

	// NewPane creates a new pane in a workspace, supporting type (terminal/browser) and URL.
	// Returns the new surface ref.
	NewPane(opts NewPaneOpts) (string, error)

	// FocusPane focuses a specific pane in a workspace.
	FocusPane(paneRef, workspaceRef string) error

	// Send sends text to a surface in a workspace.
	Send(workspaceRef, surfaceRef, text string) error

	// PinWorkspace pins a workspace in the sidebar.
	PinWorkspace(ref string) error

	// UnpinWorkspace unpins a workspace in the sidebar.
	UnpinWorkspace(ref string) error

	// CloseWorkspace closes a workspace.
	CloseWorkspace(ref string) error

	// DryRunFormatter returns a formatter for generating dry-run command output.
	DryRunFormatter() DryRunFormatter
}

// ErrNotSupported is returned by backends that don't support an optional operation.
var ErrNotSupported = fmt.Errorf("operation not supported by this backend")

// PaneGeometryProvider is optionally implemented by backends that expose
// pane pixel geometry. Used during save to infer split directions and ratios.
// Backends that don't support this are detected via type assertion; save
// falls back to default split directions.
type PaneGeometryProvider interface {
	PaneList(workspaceRef string) (*PaneListResponse, error)
}

// PaneResizer is optionally implemented by backends that support
// programmatic pane resizing. Used during restore to apply saved split ratios.
type PaneResizer interface {
	ResizePane(opts ResizePaneOpts) error
}
