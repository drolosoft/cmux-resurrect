package model

import (
	"testing"
)

func TestProject_BuildTitle(t *testing.T) {
	p := Project{Icon: "🏟️", Name: "LaPorrA"}

	got := p.BuildTitle(2)
	want := "2 🏟️ LaPorrA"
	if got != want {
		t.Errorf("BuildTitle = %q, want %q", got, want)
	}
}

func TestProject_BuildTitle_Zero(t *testing.T) {
	p := Project{Icon: "🥌", Name: "ioc-events"}
	got := p.BuildTitle(0)
	if got != "0 🥌 ioc-events" {
		t.Errorf("BuildTitle = %q", got)
	}
}

func TestWorkspaceFile_EnabledProjects(t *testing.T) {
	wf := WorkspaceFile{
		Projects: []Project{
			{Name: "a", Enabled: true},
			{Name: "b", Enabled: false},
			{Name: "c", Enabled: true},
		},
	}
	enabled := wf.EnabledProjects()
	if len(enabled) != 2 {
		t.Fatalf("enabled = %d, want 2", len(enabled))
	}
	if enabled[0].Name != "a" || enabled[1].Name != "c" {
		t.Errorf("names: %q, %q", enabled[0].Name, enabled[1].Name)
	}
}

func TestWorkspaceFile_EnabledProjects_AllDisabled(t *testing.T) {
	wf := WorkspaceFile{
		Projects: []Project{
			{Name: "a", Enabled: false},
		},
	}
	if len(wf.EnabledProjects()) != 0 {
		t.Error("expected 0 enabled")
	}
}

func TestWorkspaceFile_ResolveTemplate_Known(t *testing.T) {
	wf := WorkspaceFile{
		Templates: map[string]*Template{
			"dev": {
				Name: "dev",
				Panes: []TemplatePan{
					{Enabled: true, IsMain: true, Type: "terminal", Focus: true},
					{Enabled: true, Split: "right", Type: "terminal", Command: "claude"},
					{Enabled: false, Split: "right", Type: "terminal", Command: "lazygit"},
				},
			},
		},
	}

	panes := wf.ResolveTemplate("dev")
	// Only 2 enabled panes (lazygit is disabled).
	if len(panes) != 2 {
		t.Fatalf("panes = %d, want 2", len(panes))
	}
	if panes[0].Type != "terminal" || !panes[0].Focus {
		t.Errorf("pane 0: type=%q focus=%v", panes[0].Type, panes[0].Focus)
	}
	if panes[1].Split != "right" || panes[1].Command != "claude" {
		t.Errorf("pane 1: split=%q cmd=%q", panes[1].Split, panes[1].Command)
	}
}

func TestWorkspaceFile_ResolveTemplate_Unknown(t *testing.T) {
	wf := WorkspaceFile{Templates: map[string]*Template{}}
	panes := wf.ResolveTemplate("nonexistent")
	if len(panes) != 1 {
		t.Fatalf("fallback panes = %d, want 1", len(panes))
	}
	if panes[0].Type != "terminal" || !panes[0].Focus {
		t.Error("fallback should be single focused terminal")
	}
}

func TestWorkspaceFile_ResolveTemplate_AllDisabled(t *testing.T) {
	wf := WorkspaceFile{
		Templates: map[string]*Template{
			"empty": {
				Name:  "empty",
				Panes: []TemplatePan{{Enabled: false, IsMain: true}},
			},
		},
	}
	panes := wf.ResolveTemplate("empty")
	if len(panes) != 1 {
		t.Fatalf("panes = %d, want 1 (fallback)", len(panes))
	}
}
