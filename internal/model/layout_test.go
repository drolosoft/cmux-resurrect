package model

import (
	"strings"
	"testing"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

func TestLayoutRoundTrip(t *testing.T) {
	original := Layout{
		Name:        "test-session",
		Description: "A test layout",
		Version:     1,
		SavedAt:     time.Date(2026, 3, 22, 11, 0, 0, 0, time.UTC),
		Workspaces: []Workspace{
			{
				Title:  "0 dev",
				CWD:    "/Users/txeo/Git/project",
				Pinned: true,
				Index:  0,
				Panes: []Pane{
					{Type: "terminal", Focus: true},
					{Type: "terminal", Split: "right", Command: "go test ./..."},
				},
			},
			{
				Title:  "1 docs",
				CWD:    "/Users/txeo/Documents",
				Pinned: false,
				Index:  1,
				Panes: []Pane{
					{Type: "terminal"},
				},
			},
		},
	}

	data, err := toml.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Layout
	if err := toml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Name != original.Name {
		t.Errorf("Name = %q, want %q", decoded.Name, original.Name)
	}
	if len(decoded.Workspaces) != 2 {
		t.Fatalf("Workspaces = %d, want 2", len(decoded.Workspaces))
	}
	if decoded.Workspaces[0].CWD != "/Users/txeo/Git/project" {
		t.Errorf("CWD = %q", decoded.Workspaces[0].CWD)
	}
	if len(decoded.Workspaces[0].Panes) != 2 {
		t.Fatalf("Panes = %d, want 2", len(decoded.Workspaces[0].Panes))
	}
	if decoded.Workspaces[0].Panes[1].Split != "right" {
		t.Errorf("Split = %q, want right", decoded.Workspaces[0].Panes[1].Split)
	}
	if decoded.Workspaces[0].Panes[1].Command != "go test ./..." {
		t.Errorf("Command = %q", decoded.Workspaces[0].Panes[1].Command)
	}
}

func TestLayoutTOMLFormat(t *testing.T) {
	layout := Layout{
		Name:    "minimal",
		Version: 1,
		SavedAt: time.Date(2026, 3, 22, 11, 0, 0, 0, time.UTC),
		Workspaces: []Workspace{
			{
				Title:  "0 main",
				CWD:    "/tmp",
				Pinned: true,
				Index:  0,
				Panes: []Pane{
					{Type: "terminal", Focus: true},
				},
			},
		},
	}

	data, err := toml.Marshal(layout)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	s := string(data)
	// go-toml/v2 uses single quotes for strings.
	if !strings.Contains(s, `name = 'minimal'`) {
		t.Errorf("missing name field in:\n%s", s)
	}
	if !strings.Contains(s, `title = '0 main'`) {
		t.Errorf("missing workspace title in:\n%s", s)
	}
	if !strings.Contains(s, `type = 'terminal'`) {
		t.Errorf("missing pane type in:\n%s", s)
	}
}
