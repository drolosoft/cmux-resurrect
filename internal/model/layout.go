package model

import "time"

// Layout represents a complete terminal session layout.
type Layout struct {
	Name        string      `toml:"name"`
	Description string      `toml:"description,omitempty"`
	Version     int         `toml:"version"`
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
	Type        string `toml:"type"`
	Split       string `toml:"split,omitempty"`
	CWD         string `toml:"cwd,omitempty"`
	Command     string `toml:"command,omitempty"`
	Focus       bool   `toml:"focus,omitempty"`
	URL         string `toml:"url,omitempty"`
	Index       int    `toml:"index,omitempty"`
	FocusTarget int    `toml:"focus_target,omitempty"`
}

// LayoutMeta holds summary info about a saved layout (for list command).
type LayoutMeta struct {
	Name           string
	Description    string
	SavedAt        time.Time
	WorkspaceCount int
	FilePath       string
}
