package model

import "time"

// Layout represents a complete terminal session layout.
type Layout struct {
	Name        string      `toml:"name"`
	Description string      `toml:"description,omitempty"`
	Version     int         `toml:"version"`
	Revision    uint64      `toml:"revision"`
	SavedAt     time.Time   `toml:"saved_at"`
	Workspaces  []Workspace `toml:"workspace"`
}

// Workspace represents a single cmux workspace (tab).
type Workspace struct {
	Title       string `toml:"title"`
	Description string `toml:"description,omitempty"`
	CWD         string `toml:"cwd"`
	Pinned      bool   `toml:"pinned"`
	Index       int    `toml:"index"`
	Active      bool   `toml:"active,omitempty"`
	Panes       []Pane `toml:"pane"`
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
}
