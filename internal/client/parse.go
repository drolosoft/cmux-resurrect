package client

import (
	"encoding/json"
	"fmt"
	"strings"
)

// workspaceGroupJSON mirrors one group object in cmux's workspace-group JSON.
// Color and icon are pointers because cmux sends null when they are unset.
type workspaceGroupJSON struct {
	Ref        string   `json:"ref"`
	Name       string   `json:"name"`
	AnchorRef  string   `json:"anchor_workspace_ref"`
	MemberRefs []string `json:"member_workspace_refs"`
	Collapsed  bool     `json:"is_collapsed"`
	Pinned     bool     `json:"is_pinned"`
	Color      *string  `json:"custom_color"`
	Icon       *string  `json:"icon_symbol"`
}

// toWorkspaceGroup converts the wire shape into the client type, turning a
// null color or icon into "" so callers never deal with pointers.
func (raw workspaceGroupJSON) toWorkspaceGroup() WorkspaceGroup {
	group := WorkspaceGroup{
		Ref:        raw.Ref,
		Name:       raw.Name,
		AnchorRef:  raw.AnchorRef,
		MemberRefs: raw.MemberRefs,
		Collapsed:  raw.Collapsed,
		Pinned:     raw.Pinned,
	}

	if raw.Color != nil {
		group.Color = *raw.Color
	}
	if raw.Icon != nil {
		group.Icon = *raw.Icon
	}
	return group
}

// ParseWorkspaceGroups parses the output of `cmux --json workspace-group
// list`. Groups and their members keep the order cmux reports them in, which
// is the sidebar order.
func ParseWorkspaceGroups(raw string) ([]WorkspaceGroup, error) {
	var response struct {
		Groups []workspaceGroupJSON `json:"groups"`
	}
	if err := json.Unmarshal([]byte(raw), &response); err != nil {
		return nil, fmt.Errorf("parse workspace-group list JSON: %w", err)
	}

	groups := make([]WorkspaceGroup, 0, len(response.Groups))
	for _, group := range response.Groups {
		groups = append(groups, group.toWorkspaceGroup())
	}
	return groups, nil
}

// ParseWorkspaceGroup parses the output of `cmux --json workspace-group
// create`, which wraps the new group in a "group" key. A response without it
// is an error: the caller needs the group ref and the new anchor to go on.
func ParseWorkspaceGroup(raw string) (WorkspaceGroup, error) {
	var response struct {
		Group *workspaceGroupJSON `json:"group"`
	}
	if err := json.Unmarshal([]byte(raw), &response); err != nil {
		return WorkspaceGroup{}, fmt.Errorf("parse workspace-group JSON: %w", err)
	}

	if response.Group == nil {
		return WorkspaceGroup{}, fmt.Errorf("workspace-group: no group in response")
	}
	return response.Group.toWorkspaceGroup(), nil
}

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
