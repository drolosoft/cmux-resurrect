package orchestrate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/txeo/cmux-persist/internal/client"
	"github.com/txeo/cmux-persist/internal/mdfile"
	"github.com/txeo/cmux-persist/internal/model"
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
			Templates: DefaultTemplates(),
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

// ExtractIconAndName parses a title like "0 🥌 ioc-events" into icon and name.
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
		if r < 0x80 {
			i++
		} else if r < 0xE0 {
			i += 2
		} else if r < 0xF0 {
			i += 3
		} else {
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

// DefaultTemplates returns the built-in starter templates.
func DefaultTemplates() map[string]*model.Template {
	return map[string]*model.Template{
		"dev": {Name: "dev", Panes: []model.TemplatePan{
			{Enabled: true, IsMain: true, Type: "terminal", Focus: true},
			{Enabled: true, Split: "right", Type: "terminal", Command: "claude"},
			{Enabled: true, Split: "right", Type: "terminal", Command: "lazygit"},
		}},
		"go": {Name: "go", Panes: []model.TemplatePan{
			{Enabled: true, IsMain: true, Type: "terminal", Focus: true},
			{Enabled: true, Split: "right", Type: "terminal", Command: "go test ./..."},
		}},
		"single": {Name: "single", Panes: []model.TemplatePan{
			{Enabled: true, IsMain: true, Type: "terminal", Focus: true},
		}},
		"monitor": {Name: "monitor", Panes: []model.TemplatePan{
			{Enabled: true, IsMain: true, Type: "terminal", Command: "htop"},
			{Enabled: true, Split: "right", Type: "terminal", Command: "tail -f /var/log/system.log"},
		}},
	}
}

// ExpandHome expands ~ to absolute path.
func ExpandHome(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}
