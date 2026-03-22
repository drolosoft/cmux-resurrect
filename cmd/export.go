package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/txeo/cmux-persist/internal/mdfile"
	"github.com/txeo/cmux-persist/internal/model"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export live cmux state to the workspace file",
	Long:  "Captures current cmux workspaces and writes them to the workspace MD file with default templates.",
	RunE:  runExport,
}

func init() {
	rootCmd.AddCommand(exportCmd)
}

func runExport(cmd *cobra.Command, args []string) error {
	cl := newClient()
	wsFile := cfg.WorkspaceFile

	tree, err := cl.Tree()
	if err != nil {
		return fmt.Errorf("get tree: %w", err)
	}

	if len(tree.Windows) == 0 {
		return fmt.Errorf("no windows found")
	}

	win := tree.Windows[0]

	// Try to load existing file to preserve templates.
	var wf *model.WorkspaceFile
	existing, parseErr := mdfile.Parse(wsFile)
	if parseErr == nil {
		wf = existing
		wf.Projects = nil // Rebuild projects from live state.
	} else {
		wf = &model.WorkspaceFile{
			Templates: defaultExportTemplates(),
		}
	}

	for _, tw := range win.Workspaces {
		sidebar, err := cl.SidebarState(tw.Ref)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  warning: %q: %v\n", tw.Title, err)
			continue
		}

		// Guess template from pane count.
		template := "single"
		if len(tw.Panes) >= 3 {
			template = "dev"
		} else if len(tw.Panes) == 2 {
			template = "go"
		}

		// Extract icon from title if present (emoji at start after index).
		icon, name := extractIconAndName(tw.Title)

		// Convert absolute path to ~ form.
		path := abbreviateHome(sidebar.CWD)

		p := model.Project{
			Enabled:  true,
			Icon:     icon,
			Name:     name,
			Template: template,
			Pin:      tw.Pinned,
			Path:     path,
		}
		wf.Projects = append(wf.Projects, p)
	}

	if err := mdfile.Write(wsFile, wf); err != nil {
		return fmt.Errorf("write workspace file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Exported %d workspaces to %s\n", len(wf.Projects), wsFile)
	for _, p := range wf.Projects {
		check := "[x]"
		if !p.Enabled {
			check = "[ ]"
		}
		fmt.Fprintf(os.Stderr, "  %s %s %s  (%s)  %s\n", check, p.Icon, p.Name, p.Template, p.Path)
	}
	return nil
}

// extractIconAndName parses a title like "0 🥌 ioc-events" into icon and name.
func extractIconAndName(title string) (string, string) {
	// Strip leading index + space: "0 🥌 ioc-events" -> "🥌 ioc-events"
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
	rest = trimSpace(rest)

	// Try to separate emoji from name.
	// Emoji characters are typically > 0xFF or specific symbols.
	if len(rest) == 0 {
		return "📁", title
	}

	// Find first ASCII letter or digit after potential emoji.
	for i := 0; i < len(rest); {
		r := rune(rest[i])
		if r < 128 && ((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			icon := trimSpace(rest[:i])
			name := trimSpace(rest[i:])
			if icon == "" {
				icon = "📁"
			}
			if name == "" {
				name = title
			}
			return icon, name
		}
		// UTF-8: advance by rune width.
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
	// Entire string is emoji-like, use as icon, title as name.
	return rest, title
}

func trimSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

func abbreviateHome(path string) string {
	home, _ := os.UserHomeDir()
	if home != "" && len(path) >= len(home) && path[:len(home)] == home {
		return "~" + path[len(home):]
	}
	return path
}

func defaultExportTemplates() map[string]*model.Template {
	return map[string]*model.Template{
		"dev": {
			Name: "dev",
			Panes: []model.TemplatePan{
				{Enabled: true, IsMain: true, Type: "terminal", Focus: true},
				{Enabled: true, Split: "right", Type: "terminal", Command: "claude"},
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
