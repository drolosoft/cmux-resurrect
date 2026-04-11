package orchestrate

import (
	"fmt"
	"os"
	"strings"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/mdfile"
	"github.com/drolosoft/cmux-resurrect/internal/model"
)

// Exporter captures live cmux state and writes it to the MD file.
type Exporter struct {
	Client client.CmuxClient
}

// ExportToMD reads live cmux state and updates the MD file,
// preserving templates and tail sections (docs, etc.).
func (e *Exporter) ExportToMD(mdPath string) error {
	tree, err := e.Client.Tree()
	if err != nil {
		return fmt.Errorf("get tree: %w", err)
	}
	if len(tree.Windows) == 0 {
		return fmt.Errorf("no windows found")
	}

	win := tree.Windows[0]

	// Load existing file to preserve templates and tail.
	var wf *model.WorkspaceFile
	existing, parseErr := mdfile.Parse(mdPath)
	if parseErr == nil {
		wf = existing
		wf.Projects = nil // Rebuild projects from live state.
	} else {
		wf = &model.WorkspaceFile{
			Templates: mdfile.DefaultTemplates(),
		}
	}

	for _, tw := range win.Workspaces {
		sidebar, err := e.Client.SidebarState(tw.Ref)
		if err != nil {
			continue
		}

		template := "single"
		if len(tw.Panes) >= 3 {
			template = "dev"
		} else if len(tw.Panes) == 2 {
			template = "go"
		}

		icon, name := ExtractIconAndName(tw.Title)
		path := AbbreviateHome(sidebar.CWD)

		wf.Projects = append(wf.Projects, model.Project{
			Enabled:  true,
			Icon:     icon,
			Name:     name,
			Template: template,
			Pin:      tw.Pinned,
			Path:     path,
		})
	}

	return mdfile.Write(mdPath, wf)
}

// ExtractIconAndName parses a title like "0 🌐 webapp" into icon and name.
func ExtractIconAndName(title string) (string, string) {
	rest := title
	for i, r := range rest {
		if r == ' ' {
			rest = rest[i+1:]
			break
		}
		if r < '0' || r > '9' {
			break
		}
	}
	rest = strings.TrimSpace(rest)

	if len(rest) == 0 {
		return "📁", title
	}

	for i := 0; i < len(rest); {
		r := rune(rest[i])
		if r < 128 && ((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			icon := strings.TrimSpace(rest[:i])
			name := strings.TrimSpace(rest[i:])
			if icon == "" {
				icon = "📁"
			}
			if name == "" {
				name = title
			}
			return icon, name
		}
		switch {
		case r < 0x80:
			i++
		case r < 0xE0:
			i += 2
		case r < 0xF0:
			i += 3
		default:
			i += 4
		}
	}
	return rest, title
}

// AbbreviateHome replaces the home directory prefix with ~.
func AbbreviateHome(path string) string {
	home, _ := os.UserHomeDir()
	if home != "" && len(path) >= len(home) && path[:len(home)] == home {
		return "~" + path[len(home):]
	}
	return path
}
