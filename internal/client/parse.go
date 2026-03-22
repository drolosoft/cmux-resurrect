package client

import (
	"fmt"
	"strings"
)

// ParseSidebarState parses the key=value text output of `cmux sidebar-state`.
func ParseSidebarState(raw string) (*SidebarState, error) {
	state := &SidebarState{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "cwd":
			state.CWD = val
		case "focused_cwd":
			state.FocusedCWD = val
		case "git_branch":
			// format: "main dirty" or "main" or "none"
			if val != "none" {
				fields := strings.Fields(val)
				if len(fields) > 0 {
					state.GitBranch = fields[0]
				}
				if len(fields) > 1 && fields[1] == "dirty" {
					state.GitDirty = true
				}
			}
		}
	}
	if state.CWD == "" {
		return nil, fmt.Errorf("sidebar-state: no cwd found in output")
	}
	return state, nil
}

// ParseListWorkspaces parses the text output of `cmux list-workspaces`.
// Each line: "  workspace:N  <index> <title>" or "* workspace:N  <title>  [selected]"
func ParseListWorkspaces(raw string) ([]WorkspaceInfo, error) {
	var workspaces []WorkspaceInfo
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		selected := false
		if strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "*\t") {
			selected = true
			line = strings.TrimSpace(line[1:])
		}

		// Check for [selected] suffix
		if strings.HasSuffix(line, "[selected]") {
			selected = true
			line = strings.TrimSuffix(line, "[selected]")
			line = strings.TrimSpace(line)
		}

		// Extract ref (workspace:N)
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		ref := fields[0]
		if !strings.HasPrefix(ref, "workspace:") {
			continue
		}

		// Title is everything after the ref
		title := strings.TrimSpace(strings.TrimPrefix(line, ref))

		workspaces = append(workspaces, WorkspaceInfo{
			Ref:      ref,
			Title:    title,
			Selected: selected,
		})
	}
	return workspaces, nil
}
