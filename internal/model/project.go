package model

import "fmt"

// Project represents a workspace entry in the MD file.
type Project struct {
	Enabled  bool   // [x] or [ ]
	Icon     string // emoji
	Name     string // short project name
	Template string // template name (dev, go, single, monitor, etc.)
	Pin      bool   // pinned in cmux sidebar
	Path     string // filesystem path (may contain ~)
}

// Template defines a reusable pane layout.
type Template struct {
	Name  string
	Panes []TemplatePan
}

// TemplatePan is a pane definition within a template.
type TemplatePan struct {
	Enabled   bool   // [x] or [ ]
	IsMain    bool   // "main" keyword = first pane
	Split     string // "right", "down", "left", "up"
	Type      string // "terminal" (default), "browser"
	Command   string // command in backticks
	Focus     bool   // "(focused)" suffix
}

// WorkspaceFile is the full parsed content of the MD file.
type WorkspaceFile struct {
	Projects  []Project
	Templates map[string]*Template
}

// BuildTitle constructs the cmux workspace title: "{index} {icon} {name}"
func (p *Project) BuildTitle(index int) string {
	return fmt.Sprintf("%d %s %s", index, p.Icon, p.Name)
}

// EnabledProjects returns only projects with [x].
func (wf *WorkspaceFile) EnabledProjects() []Project {
	var out []Project
	for _, p := range wf.Projects {
		if p.Enabled {
			out = append(out, p)
		}
	}
	return out
}

// ResolveTemplate returns the panes for a project based on its template.
func (wf *WorkspaceFile) ResolveTemplate(templateName string) []Pane {
	tmpl, ok := wf.Templates[templateName]
	if !ok {
		// Fallback: single terminal pane.
		return []Pane{{Type: "terminal", Focus: true}}
	}

	var panes []Pane
	for i, tp := range tmpl.Panes {
		if !tp.Enabled {
			continue
		}
		pane := Pane{
			Type:    tp.Type,
			Command: tp.Command,
			Focus:   tp.Focus,
		}
		if pane.Type == "" {
			pane.Type = "terminal"
		}
		if i > 0 && tp.Split != "" {
			pane.Split = tp.Split
		}
		panes = append(panes, pane)
	}
	if len(panes) == 0 {
		return []Pane{{Type: "terminal", Focus: true}}
	}
	return panes
}
