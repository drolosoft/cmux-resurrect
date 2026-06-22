package model

import (
	"fmt"
	"strings"
)

// ExtractPaneName pulls a double-quoted display name out of a pane descriptor,
// returning the name and the descriptor with the quoted segment removed. The
// colon that follows the descriptor stays attached exactly as in the no-name
// form, so existing prefix parsing is unaffected.
//
//	`split right "Diff"` → ("Diff", "split right")
//	`main terminal "Plan":` → ("Plan", "main terminal:")
//	`split right` → ("", "split right")   // no quotes, unchanged
func ExtractPaneName(s string) (name, rest string) {
	start := strings.IndexByte(s, '"')
	if start < 0 {
		return "", s
	}
	rel := strings.IndexByte(s[start+1:], '"')
	if rel < 0 {
		return "", s
	}
	end := start + 1 + rel
	name = s[start+1 : end]
	left := strings.TrimRight(s[:start], " ")
	rest = strings.TrimSpace(left + s[end+1:])
	return name, rest
}

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
	Name        string
	Category    string   // "layout" or "workflow"
	Icon        string   // display icon (▥, 🤖, etc.)
	Description string   // one-line summary
	Tags        []string // for filtering: ["ai", "git"], ["monitoring"], etc.
	Panes       []TemplatePan
}

// TemplatePan is a pane definition within a template.
type TemplatePan struct {
	Enabled     bool      // [x] or [ ]
	IsMain      bool      // "main" keyword = first pane
	IsTab       bool      // "tab N:" line — extra surface in preceding pane
	Split       string    // "right", "down", "left", "up"
	Type        string    // "terminal" (default), "browser"
	Command     string    // command in backticks
	Focus       bool      // "(focused)" suffix — gets final focus
	FocusTarget int       // pane index to focus BEFORE this split (-1 = no refocus)
	SplitRatio  float64   // 0.0 = equal; e.g. 0.30 = new pane takes 30%
	Name        string    // display label: "main", "console", "git"
	Surfaces    []Surface // extra tabs in this pane (parsed from "tab N:" lines)
}

// WorkspaceFile is the full parsed content of the MD file.
type WorkspaceFile struct {
	Projects  []Project
	Templates map[string]*Template
	Tail      string // Everything after Templates section (preserved on write)
}

// BuildTitle constructs the cmux workspace title: "{icon} {name}"
// The icon field already contains any numeric prefix from the MD file.
func (p *Project) BuildTitle(_ int) string {
	return fmt.Sprintf("%s %s", p.Icon, p.Name)
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
			Name:    tp.Name,
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
