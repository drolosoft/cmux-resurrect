package mdfile

import (
	"fmt"
	"os"
	"strings"

	"github.com/drolosoft/cmux-resurrect/internal/model"
)

// Write serializes a WorkspaceFile back to the MD format.
func Write(path string, wf *model.WorkspaceFile) error {
	var b strings.Builder

	b.WriteString("## Projects\n")
	b.WriteString("**Icon | Name | Template | Pin | Path**\n\n")

	for _, p := range wf.Projects {
		check := " "
		if p.Enabled {
			check = "x"
		}
		pin := "no"
		if p.Pin {
			pin = "yes"
		}
		fmt.Fprintf(&b, "- [%s] | %s | %s | %s | %s | %s |\n",
			check, p.Icon, p.Name, p.Template, pin, p.Path)
	}

	// Write templates.
	if len(wf.Templates) > 0 {
		b.WriteString("\n## Templates\n")

		// Sort templates for stable output.
		names := sortedTemplateNames(wf.Templates)
		for _, name := range names {
			tmpl := wf.Templates[name]
			fmt.Fprintf(&b, "\n### %s\n", tmpl.Name)
			for _, tp := range tmpl.Panes {
				check := " "
				if tp.Enabled {
					check = "x"
				}
				line := fmt.Sprintf("- [%s] ", check)
				if tp.IsMain {
					line += "main"
					if tp.Type != "" && tp.Type != "terminal" {
						line += " " + tp.Type
					}
				} else if tp.Split != "" {
					line += "split " + tp.Split + ":"
				}
				if tp.Command != "" {
					line += " `" + tp.Command + "`"
				}
				if tp.Focus {
					line += " (focused)"
				}
				b.WriteString(line + "\n")
			}
		}
	}

	// Append preserved tail sections (docs, etc.).
	if wf.Tail != "" {
		b.WriteString("\n")
		b.WriteString(wf.Tail)
	}

	// Atomic write.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

func sortedTemplateNames(m map[string]*model.Template) []string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	// Simple insertion sort for small maps.
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j] < names[j-1]; j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}
	return names
}

// AddProject appends a project to the MD file, preserving existing content.
func AddProject(path string, p model.Project) error {
	wf, err := Parse(path)
	if err != nil {
		if os.IsNotExist(err) {
			wf = &model.WorkspaceFile{
				Templates: DefaultTemplates(),
			}
		} else {
			return err
		}
	}

	// Check for duplicate.
	for _, existing := range wf.Projects {
		if strings.EqualFold(existing.Name, p.Name) {
			return fmt.Errorf("project %q already exists", p.Name)
		}
	}

	wf.Projects = append(wf.Projects, p)
	return Write(path, wf)
}

// RemoveProject removes a project by name from the MD file.
func RemoveProject(path string, name string) error {
	wf, err := Parse(path)
	if err != nil {
		return err
	}

	found := false
	var kept []model.Project
	for _, p := range wf.Projects {
		if strings.EqualFold(p.Name, name) {
			found = true
			continue
		}
		kept = append(kept, p)
	}
	if !found {
		return fmt.Errorf("project %q not found", name)
	}

	wf.Projects = kept
	return Write(path, wf)
}

// ToggleProject toggles the enabled state of a project by name.
func ToggleProject(path string, name string) (bool, error) {
	wf, err := Parse(path)
	if err != nil {
		return false, err
	}

	found := false
	var newState bool
	for i := range wf.Projects {
		if strings.EqualFold(wf.Projects[i].Name, name) {
			wf.Projects[i].Enabled = !wf.Projects[i].Enabled
			newState = wf.Projects[i].Enabled
			found = true
			break
		}
	}
	if !found {
		return false, fmt.Errorf("project %q not found", name)
	}

	return newState, Write(path, wf)
}

// DefaultTemplates returns the built-in starter templates.
// This is the single source of truth — used by both mdfile and orchestrate.
func DefaultTemplates() map[string]*model.Template {
	return map[string]*model.Template{
		"dev": {
			Name: "dev",
			Panes: []model.TemplatePan{
				{Enabled: true, IsMain: true, Type: "terminal", Focus: true},
				{Enabled: true, Split: "right", Type: "terminal", Command: "npm run dev"},
				{Enabled: true, Split: "right", Type: "terminal", Command: "lazygit"},
			},
		},
		"go": {
			Name: "go",
			Panes: []model.TemplatePan{
				{Enabled: true, IsMain: true, Type: "terminal", Focus: true},
				{Enabled: true, Split: "right", Type: "terminal", Command: "go test ./..."},
			},
		},
		"single": {
			Name: "single",
			Panes: []model.TemplatePan{
				{Enabled: true, IsMain: true, Type: "terminal", Focus: true},
			},
		},
		"monitor": {
			Name: "monitor",
			Panes: []model.TemplatePan{
				{Enabled: true, IsMain: true, Type: "terminal", Command: "htop"},
				{Enabled: true, Split: "right", Type: "terminal", Command: "tail -f /var/log/system.log"},
			},
		},
	}
}
