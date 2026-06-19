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
	// If surfaceRef is non-empty, the split targets that specific surface instead of
	// the currently focused one.
	NewSplit(direction, workspaceRef, surfaceRef string) (string, error)

	// NewPane creates a new pane in a workspace, supporting type (terminal/browser) and URL.
	// Returns the new surface ref.
	NewPane(opts NewPaneOpts) (string, error)

	// NewSurface creates an additional surface (tab) in an existing pane.
	// Returns the new surface ref. cmux-only; Ghostty returns ErrNotSupported.
	NewSurface(paneRef, workspaceRef string) (string, error)

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

// validateSplitDirection enforces a strict allowlist on split directions before
// they reach a shell/AppleScript boundary. An empty direction defaults to "right".
// This prevents injection via attacker-controlled `split` values in saved layouts
// (the Ghostty backend interpolates the direction directly into an AppleScript
// statement). Returns the normalized direction or an error for anything else.
func validateSplitDirection(direction string) (string, error) {
	if direction == "" {
		return "right", nil
	}
	switch direction {
	case "right", "left", "up", "down":
		return direction, nil
	default:
		return "", fmt.Errorf("invalid split direction %q (allowed: right, left, up, down)", direction)
	}
}

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
