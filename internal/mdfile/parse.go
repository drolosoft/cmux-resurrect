package mdfile

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/juanatsap/cmux-resurrect/internal/model"
)

// Parse reads and parses the workspace MD file.
func Parse(path string) (*model.WorkspaceFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open workspace file: %w", err)
	}
	defer f.Close()

	wf := &model.WorkspaceFile{
		Templates: make(map[string]*model.Template),
	}

	scanner := bufio.NewScanner(f)
	var section string       // "projects", "templates", or "tail"
	var currentTmpl *model.Template
	var tailLines []string

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Detect section headers.
		if strings.HasPrefix(trimmed, "## Projects") {
			section = "projects"
			currentTmpl = nil
			continue
		}
		if strings.HasPrefix(trimmed, "## Templates") {
			section = "templates"
			currentTmpl = nil
			continue
		}
		// Any other ## header after templates = tail section (preserved as-is).
		if section == "templates" && strings.HasPrefix(trimmed, "## ") && !strings.HasPrefix(trimmed, "### ") {
			section = "tail"
			tailLines = append(tailLines, line)
			continue
		}
		if section == "tail" {
			tailLines = append(tailLines, line)
			continue
		}

		// Skip bold header line and empty lines.
		if strings.HasPrefix(trimmed, "**") || trimmed == "" {
			continue
		}

		switch section {
		case "projects":
			if p, ok := parseProjectLine(trimmed); ok {
				wf.Projects = append(wf.Projects, p)
			}
		case "templates":
			// ### template-name
			if strings.HasPrefix(trimmed, "### ") {
				name := strings.TrimSpace(strings.TrimPrefix(trimmed, "###"))
				currentTmpl = &model.Template{Name: name}
				wf.Templates[name] = currentTmpl
				continue
			}
			if currentTmpl != nil {
				if tp, ok := parseTemplatePaneLine(trimmed); ok {
					currentTmpl.Panes = append(currentTmpl.Panes, tp)
				}
			}
		}
	}

	if len(tailLines) > 0 {
		wf.Tail = strings.Join(tailLines, "\n") + "\n"
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read workspace file: %w", err)
	}
	return wf, nil
}

// parseProjectLine parses: - [x] | 🏟️ | LaPorrA | dev | yes | ~/Git/htmx/laporra |
func parseProjectLine(line string) (model.Project, bool) {
	var p model.Project

	// Must start with checkbox.
	if !strings.HasPrefix(line, "- [") {
		return p, false
	}

	// Parse enabled state.
	if strings.HasPrefix(line, "- [x]") || strings.HasPrefix(line, "- [X]") {
		p.Enabled = true
	}

	// Strip the checkbox prefix: "- [x] " or "- [ ] "
	rest := line
	if idx := strings.Index(rest, "]"); idx >= 0 {
		rest = strings.TrimSpace(rest[idx+1:])
	}

	// Split by | and trim.
	parts := strings.Split(rest, "|")
	// We expect: "" icon name template pin path ""
	// Filter out empty parts from leading/trailing pipes.
	var fields []string
	for _, part := range parts {
		s := strings.TrimSpace(part)
		if s != "" {
			fields = append(fields, s)
		}
	}

	if len(fields) < 5 {
		return p, false
	}

	p.Icon = fields[0]
	p.Name = fields[1]
	p.Template = fields[2]
	p.Pin = strings.EqualFold(fields[3], "yes")
	p.Path = fields[4]

	return p, true
}

// parseTemplatePaneLine parses:
//   - [x] main terminal (focused)
//   - [x] split right: `claude`
//   - [ ] split down: `lazygit`
func parseTemplatePaneLine(line string) (model.TemplatePan, bool) {
	var tp model.TemplatePan

	if !strings.HasPrefix(line, "- [") {
		return tp, false
	}

	if strings.HasPrefix(line, "- [x]") || strings.HasPrefix(line, "- [X]") {
		tp.Enabled = true
	}

	// Strip checkbox.
	rest := line
	if idx := strings.Index(rest, "]"); idx >= 0 {
		rest = strings.TrimSpace(rest[idx+1:])
	}

	// Extract command in backticks if present.
	if backtickStart := strings.Index(rest, "`"); backtickStart >= 0 {
		backtickEnd := strings.Index(rest[backtickStart+1:], "`")
		if backtickEnd >= 0 {
			tp.Command = rest[backtickStart+1 : backtickStart+1+backtickEnd]
			rest = rest[:backtickStart]
		}
	}

	// Check for (focused).
	if strings.Contains(rest, "(focused)") {
		tp.Focus = true
		rest = strings.Replace(rest, "(focused)", "", 1)
	}

	rest = strings.TrimSpace(rest)

	// Parse "main terminal", "split right:", etc.
	if strings.HasPrefix(rest, "main") {
		tp.IsMain = true
		tp.Type = "terminal"
		remaining := strings.TrimSpace(strings.TrimPrefix(rest, "main"))
		if remaining != "" && remaining != ":" {
			tp.Type = strings.TrimSuffix(remaining, ":")
		}
	} else if strings.HasPrefix(rest, "split ") {
		parts := strings.Fields(rest)
		if len(parts) >= 2 {
			tp.Split = strings.TrimSuffix(parts[1], ":")
		}
		tp.Type = "terminal"
	} else {
		tp.Type = "terminal"
	}

	return tp, true
}
