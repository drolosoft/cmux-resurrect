package model

import "time"

// Layout represents a complete terminal session layout.
type Layout struct {
	Name        string    `toml:"name"`
	Description string    `toml:"description,omitempty"`
	Version     int       `toml:"version"`
	Revision    uint64    `toml:"revision"`
	SavedAt     time.Time `toml:"saved_at"`
	// Groups lists the cmux sidebar groups in sidebar order. It is omitted
	// when empty so a layout without groups serialises exactly as it did
	// before groups existed, and older crex builds simply skip the key.
	Groups     []Group     `toml:"group,omitempty"`
	Workspaces []Workspace `toml:"workspace"`
}

// Group is a cmux sidebar group. The group itself owns no panes: in cmux
// the group header is one of its member workspaces (the anchor), so the
// layout keeps only the group's own state here and workspaces point back to
// it by Name. Names are unique within a layout; save disambiguates
// duplicates before they get here.
type Group struct {
	Name      string `toml:"name"`
	Collapsed bool   `toml:"collapsed"`
	Pinned    bool   `toml:"pinned"`

	// Color is the custom colour as "#RRGGBB"; empty means cmux's default.
	Color string `toml:"color,omitempty"`

	// Icon is an SF Symbol name such as "flame"; empty means no icon.
	Icon string `toml:"icon,omitempty"`
}

// GroupNamed returns the group called name, or nil when the layout has no
// such group. The pointer goes into Layout.Groups, so callers can update a
// group's state in place while they walk the workspaces that reference it.
func (l *Layout) GroupNamed(name string) *Group {
	for index := range l.Groups {
		if l.Groups[index].Name == name {
			return &l.Groups[index]
		}
	}

	return nil
}

// Workspace represents a single cmux workspace (tab).
type Workspace struct {
	Title       string `toml:"title"`
	Description string `toml:"description,omitempty"`
	CWD         string `toml:"cwd"`
	Pinned      bool   `toml:"pinned"`
	Index       int    `toml:"index"`
	Active      bool   `toml:"active,omitempty"`

	// Group is the Name of the sidebar group this workspace belongs to, or
	// empty for a loose workspace outside any group.
	Group string `toml:"group,omitempty"`

	// GroupAnchor marks the workspace that acts as its group's header in
	// the cmux sidebar. Restore makes it the group's anchor again; at most
	// one workspace per group should carry it.
	GroupAnchor bool `toml:"group_anchor,omitempty"`

	Panes []Pane `toml:"pane"`
}

// Pane represents a terminal or browser pane within a workspace.
type Pane struct {
	Type        string    `toml:"type"`
	Split       string    `toml:"split,omitempty"`
	CWD         string    `toml:"cwd,omitempty"`
	Name        string    `toml:"name,omitempty"` // optional tab/pane label, shown as the surface title
	Command     string    `toml:"command,omitempty"`
	Focus       bool      `toml:"focus,omitempty"`
	URL         string    `toml:"url,omitempty"`
	Profile     string    `toml:"profile,omitempty"` // browser profile slug (cmux browser panes only)
	Index       int       `toml:"index,omitempty"`
	FocusTarget int       `toml:"focus_target,omitempty"`
	SplitRatio  float64   `toml:"split_ratio,omitempty"`
	Surfaces    []Surface `toml:"surface,omitempty"`
}

// Surface represents an additional tab within a pane (surfaces 2..N).
// The first surface's data lives in the parent Pane's flat fields.
type Surface struct {
	Type    string `toml:"type,omitempty"`
	Name    string `toml:"name,omitempty"` // optional tab label, shown as the surface title
	Command string `toml:"command,omitempty"`
	CWD     string `toml:"cwd,omitempty"`
	URL     string `toml:"url,omitempty"`
	Profile string `toml:"profile,omitempty"` // browser profile slug (cmux browser surfaces only)
	Focus   bool   `toml:"focus,omitempty"`
}

// LayoutMeta holds summary info about a saved layout (for list command).
type LayoutMeta struct {
	Name               string    `json:"name"`
	Description        string    `json:"description"`
	SavedAt            time.Time `json:"saved_at"`
	WorkspaceCount     int       `json:"workspace_count"`
	WorkspaceTitles    []string  `json:"workspace_titles"`
	WorkspacePanes     []int     `json:"workspace_panes"`
	WorkspaceSummaries []string  `json:"workspace_summaries"`
	FilePath           string    `json:"file_path"`

	// Groups and Memberships carry the layout's sidebar groups so listings
	// built from metadata can group workspaces like `crex show` does. Both
	// stay empty for layouts without groups; Memberships lines up with
	// WorkspaceTitles.
	Groups      []Group      `json:"groups,omitempty"`
	Memberships []Membership `json:"memberships,omitempty"`
}
