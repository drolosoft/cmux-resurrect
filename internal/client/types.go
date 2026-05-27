package client

// TreeResponse is the top-level response from `cmux tree --json`.
type TreeResponse struct {
	Caller  *CallerInfo  `json:"caller"`
	Active  *CallerInfo  `json:"active"`
	Windows []TreeWindow `json:"windows"`
}

// CallerInfo identifies the calling terminal context.
type CallerInfo struct {
	WorkspaceRef string `json:"workspace_ref"`
	PaneRef      string `json:"pane_ref"`
	WindowRef    string `json:"window_ref"`
	SurfaceRef   string `json:"surface_ref"`
	SurfaceType  string `json:"surface_type"`
}

// TreeWindow represents a cmux window.
type TreeWindow struct {
	Ref                  string          `json:"ref"`
	Index                int             `json:"index"`
	Active               bool            `json:"active"`
	Visible              bool            `json:"visible"`
	Current              bool            `json:"current"`
	WorkspaceCount       int             `json:"workspace_count"`
	SelectedWorkspaceRef string          `json:"selected_workspace_ref"`
	Workspaces           []TreeWorkspace `json:"workspaces"`
}

// TreeWorkspace represents a cmux workspace in the tree.
type TreeWorkspace struct {
	Ref      string     `json:"ref"`
	Title    string     `json:"title"`
	Index    int        `json:"index"`
	Pinned   bool       `json:"pinned"`
	Active   bool       `json:"active"`
	Selected bool       `json:"selected"`
	Panes    []TreePane `json:"panes"`
}

// TreePane represents a pane in the tree.
type TreePane struct {
	Ref                string        `json:"ref"`
	Index              int           `json:"index"`
	Active             bool          `json:"active"`
	Focused            bool          `json:"focused"`
	SurfaceCount       int           `json:"surface_count"`
	SelectedSurfaceRef string        `json:"selected_surface_ref"`
	SurfaceRefs        []string      `json:"surface_refs"`
	Surfaces           []TreeSurface `json:"surfaces"`
}

// TreeSurface represents a surface (terminal or browser) in a pane.
type TreeSurface struct {
	Ref            string  `json:"ref"`
	PaneRef        string  `json:"pane_ref"`
	Type           string  `json:"type"`
	Title          string  `json:"title"`
	URL            *string `json:"url"`
	Index          int     `json:"index"`
	IndexInPane    int     `json:"index_in_pane"`
	Active         bool    `json:"active"`
	Focused        bool    `json:"focused"`
	Selected       bool    `json:"selected"`
	SelectedInPane bool    `json:"selected_in_pane"`
	Here           bool    `json:"here"`
	TTY            string  `json:"tty"`
}

// SidebarState holds parsed sidebar-state for a workspace.
type SidebarState struct {
	CWD        string
	FocusedCWD string
	GitBranch  string
	GitDirty   bool
}

// WorkspaceInfo from list-workspaces output.
type WorkspaceInfo struct {
	Ref      string
	Title    string
	Selected bool
}

// NewWorkspaceOpts for creating a new workspace.
type NewWorkspaceOpts struct {
	CWD     string
	Command string
}

// NewPaneOpts for creating a new pane (terminal or browser).
type NewPaneOpts struct {
	Type         string // "terminal" or "browser"
	Direction    string // "left", "right", "up", "down"
	WorkspaceRef string
	URL          string // for browser panes
}

// PaneListResponse is the parsed output of `cmux rpc pane.list`.
type PaneListResponse struct {
	WorkspaceRef   string         `json:"workspace_ref"`
	ContainerFrame ContainerFrame `json:"container_frame"`
	Panes          []PaneListPane `json:"panes"`
}

// ContainerFrame is the workspace container dimensions.
type ContainerFrame struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// PaneListPane is a pane with pixel geometry from pane.list.
type PaneListPane struct {
	Ref        string     `json:"ref"`
	Index      int        `json:"index"`
	Focused    bool       `json:"focused"`
	PixelFrame PixelFrame `json:"pixel_frame"`
}

// PixelFrame is the pixel position and size of a pane.
type PixelFrame struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// ResizePaneOpts for resizing a pane.
type ResizePaneOpts struct {
	PaneRef      string
	WorkspaceRef string
	Direction    string // "L", "R", "U", "D"
	Amount       int    // cells
}
