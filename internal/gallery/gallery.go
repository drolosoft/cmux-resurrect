package gallery

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/drolosoft/cmux-resurrect/internal/model"
)

var (
	registry     []*model.Template
	registryMap  map[string]*model.Template
	registryOnce sync.Once
)

// ensureLoaded lazily parses all embedded template files exactly once.
func ensureLoaded() {
	registryOnce.Do(func() {
		registryMap = make(map[string]*model.Template)

		entries, err := templatesFS.ReadDir("templates")
		if err != nil {
			return
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			data, err := templatesFS.ReadFile("templates/" + entry.Name())
			if err != nil {
				continue
			}
			tmpl, err := parseTemplateFile(string(data))
			if err != nil {
				continue
			}
			registry = append(registry, tmpl)
			registryMap[tmpl.Name] = tmpl
		}
	})
}

// List returns all gallery templates.
func List() []*model.Template {
	ensureLoaded()
	out := make([]*model.Template, len(registry))
	copy(out, registry)
	return out
}

// Get looks up a gallery template by name.
func Get(name string) (*model.Template, bool) {
	ensureLoaded()
	tmpl, ok := registryMap[name]
	return tmpl, ok
}

// ListByCategory returns gallery templates filtered by category ("layout" or "workflow").
func ListByCategory(category string) []*model.Template {
	ensureLoaded()
	var out []*model.Template
	for _, tmpl := range registry {
		if tmpl.Category == category {
			out = append(out, tmpl)
		}
	}
	return out
}

// ResolveTemplate resolves a template name to panes using three-tier precedence:
//  1. User-defined template in the workspace file
//  2. Gallery built-in template
//  3. Fallback: single focused terminal pane
func ResolveTemplate(wf *model.WorkspaceFile, name string) []model.Pane {
	// Tier 1: user-defined template.
	if wf != nil && wf.Templates != nil {
		if tmpl, ok := wf.Templates[name]; ok {
			return buildPanesFromTemplate(tmpl)
		}
	}

	// Tier 2: gallery template.
	if tmpl, ok := Get(name); ok {
		return BuildPanes(tmpl)
	}

	// Tier 3: fallback.
	return []model.Pane{{Type: "terminal", Focus: true}}
}

// BuildPanes converts a gallery template's TemplatePan slice into []model.Pane.
// Note: FocusTarget is not copied to Pane because the Pane struct does not have
// that field yet (added in Task 4).
func BuildPanes(tmpl *model.Template) []model.Pane {
	var panes []model.Pane
	for i, tp := range tmpl.Panes {
		if !tp.Enabled {
			continue
		}
		pane := model.Pane{
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
		return []model.Pane{{Type: "terminal", Focus: true}}
	}
	return panes
}

// buildPanesFromTemplate converts a user-defined template (same logic as model.ResolveTemplate).
func buildPanesFromTemplate(tmpl *model.Template) []model.Pane {
	var panes []model.Pane
	for i, tp := range tmpl.Panes {
		if !tp.Enabled {
			continue
		}
		pane := model.Pane{
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
		return []model.Pane{{Type: "terminal", Focus: true}}
	}
	return panes
}

// parseTemplateFile parses a .md template file with YAML frontmatter + pane definitions.
func parseTemplateFile(content string) (*model.Template, error) {
	// Split on --- delimiters for frontmatter.
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("missing YAML frontmatter delimiters")
	}

	frontmatter := parts[1]
	body := parts[2]

	tmpl := &model.Template{}

	// Parse YAML frontmatter manually (key: value pairs).
	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:colonIdx])
		val := strings.TrimSpace(line[colonIdx+1:])

		// Strip surrounding quotes.
		val = stripQuotes(val)

		switch key {
		case "name":
			tmpl.Name = val
		case "category":
			tmpl.Category = val
		case "icon":
			tmpl.Icon = val
		case "description":
			tmpl.Description = val
		case "tags":
			tmpl.Tags = parseTags(val)
		}
	}

	if tmpl.Name == "" {
		return nil, fmt.Errorf("template missing name in frontmatter")
	}

	// Parse body for pane definitions.
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [") {
			tp := parseGalleryPaneLine(trimmed)
			tmpl.Panes = append(tmpl.Panes, tp)
		}
	}

	return tmpl, nil
}

// parseGalleryPaneLine parses a pane line from a gallery template.
// It extends the standard Blueprint pane syntax with @focus=N for focus targeting.
func parseGalleryPaneLine(line string) model.TemplatePan {
	var tp model.TemplatePan
	tp.FocusTarget = -1

	if !strings.HasPrefix(line, "- [") {
		return tp
	}

	if strings.HasPrefix(line, "- [x]") || strings.HasPrefix(line, "- [X]") {
		tp.Enabled = true
	}

	// Strip checkbox.
	rest := line
	if idx := strings.Index(rest, "]"); idx >= 0 {
		rest = strings.TrimSpace(rest[idx+1:])
	}

	// Extract @focus=N before parsing the rest.
	rest = extractFocusTarget(rest, &tp)

	// Extract command in backticks if present, preserving text after the closing backtick.
	if backtickStart := strings.Index(rest, "`"); backtickStart >= 0 {
		backtickEnd := strings.Index(rest[backtickStart+1:], "`")
		if backtickEnd >= 0 {
			tp.Command = rest[backtickStart+1 : backtickStart+1+backtickEnd]
			rest = rest[:backtickStart] + rest[backtickStart+1+backtickEnd+1:]
		}
	}

	// Check for (focused).
	if strings.Contains(rest, "(focused)") {
		tp.Focus = true
		rest = strings.Replace(rest, "(focused)", "", 1)
	}

	rest = strings.TrimSpace(rest)

	// Parse "main terminal", "split right:", etc.
	switch {
	case strings.HasPrefix(rest, "main"):
		tp.IsMain = true
		tp.Type = "terminal"
		remaining := strings.TrimSpace(strings.TrimPrefix(rest, "main"))
		if remaining != "" && remaining != ":" {
			tp.Type = strings.TrimSuffix(remaining, ":")
		}
	case strings.HasPrefix(rest, "split "):
		parts := strings.Fields(rest)
		if len(parts) >= 2 {
			tp.Split = strings.TrimSuffix(parts[1], ":")
		}
		tp.Type = "terminal"
	default:
		tp.Type = "terminal"
	}

	return tp
}

// extractFocusTarget finds and removes @focus=N from a line, setting FocusTarget on the pane.
func extractFocusTarget(line string, tp *model.TemplatePan) string {
	idx := strings.Index(line, "@focus=")
	if idx < 0 {
		return line
	}

	// Extract the number after @focus=
	numStart := idx + len("@focus=")
	numEnd := numStart
	for numEnd < len(line) && line[numEnd] >= '0' && line[numEnd] <= '9' {
		numEnd++
	}

	if numEnd > numStart {
		if n, err := strconv.Atoi(line[numStart:numEnd]); err == nil {
			tp.FocusTarget = n
		}
	}

	// Remove the @focus=N token from the line.
	result := line[:idx] + line[numEnd:]
	return strings.TrimSpace(result)
}

// parseTags parses a YAML-style inline list: [tag1, tag2, tag3]
func parseTags(val string) []string {
	val = strings.TrimPrefix(val, "[")
	val = strings.TrimSuffix(val, "]")
	var tags []string
	for _, t := range strings.Split(val, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

// stripQuotes removes surrounding single or double quotes from a string.
func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
