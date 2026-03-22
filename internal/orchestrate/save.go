package orchestrate

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

// Saver captures the current cmux state and persists it.
type Saver struct {
	Client client.CmuxClient
	Store  persist.Store
}

// Save captures the live cmux state and writes it to the store.
func (s *Saver) Save(name, description string) (*model.Layout, error) {
	tree, err := s.Client.Tree()
	if err != nil {
		return nil, fmt.Errorf("get tree: %w", err)
	}

	if len(tree.Windows) == 0 {
		return nil, fmt.Errorf("no windows found in cmux")
	}

	// Use the first (typically only) window.
	win := tree.Windows[0]

	layout := &model.Layout{
		Name:        name,
		Description: description,
		Version:     1,
		SavedAt:     time.Now().UTC(),
	}

	for _, tw := range win.Workspaces {
		ws, err := s.buildWorkspace(tw)
		if err != nil {
			// Log but don't fail — isolate errors per workspace.
			fmt.Fprintf(os.Stderr, "  warning: workspace %q: %v\n", tw.Title, err)
			continue
		}
		layout.Workspaces = append(layout.Workspaces, *ws)
	}

	if len(layout.Workspaces) == 0 {
		return nil, fmt.Errorf("no workspaces could be captured")
	}

	// If a TOML already exists, merge user-edited fields (split direction, commands).
	if existing, err := s.Store.Load(name); err == nil {
		mergeUserEdits(layout, existing)
	}

	if err := s.Store.Save(name, layout); err != nil {
		return nil, fmt.Errorf("save layout: %w", err)
	}
	return layout, nil
}

func (s *Saver) buildWorkspace(tw client.TreeWorkspace) (*model.Workspace, error) {
	// Get CWD from sidebar-state.
	sidebar, err := s.Client.SidebarState(tw.Ref)
	if err != nil {
		return nil, fmt.Errorf("sidebar-state: %w", err)
	}

	ws := &model.Workspace{
		Title:  tw.Title,
		CWD:    sidebar.CWD,
		Pinned: tw.Pinned,
		Index:  tw.Index,
		Active: tw.Active || tw.Selected,
	}

	// Sort panes by index.
	panes := make([]client.TreePane, len(tw.Panes))
	copy(panes, tw.Panes)
	sort.Slice(panes, func(i, j int) bool {
		return panes[i].Index < panes[j].Index
	})

	for i, tp := range panes {
		pane := model.Pane{
			Type:  "terminal",
			Focus: tp.Focused,
			Index: tp.Index,
		}

		// First pane has no split direction; subsequent default to "right".
		if i > 0 {
			pane.Split = "right"
		}

		// Use surface info for type and URL.
		if len(tp.Surfaces) > 0 {
			surf := tp.Surfaces[0]
			pane.Type = surf.Type
			if surf.URL != nil {
				pane.URL = *surf.URL
			}
		}

		ws.Panes = append(ws.Panes, pane)
	}

	// Ensure at least one pane.
	if len(ws.Panes) == 0 {
		ws.Panes = []model.Pane{{Type: "terminal", Focus: true}}
	}

	return ws, nil
}

// mergeUserEdits preserves user-edited fields from an existing TOML.
// Fields like split direction, command, and description are kept from existing
// if the user has edited them (since the live tree doesn't expose these).
func mergeUserEdits(live, existing *model.Layout) {
	if live.Description == "" && existing.Description != "" {
		live.Description = existing.Description
	}

	// Build index of existing workspaces by title for matching.
	existByTitle := make(map[string]*model.Workspace)
	for i := range existing.Workspaces {
		existByTitle[existing.Workspaces[i].Title] = &existing.Workspaces[i]
	}

	for i := range live.Workspaces {
		lw := &live.Workspaces[i]
		ew, ok := existByTitle[lw.Title]
		if !ok {
			continue
		}
		// Merge pane-level user edits.
		for j := range lw.Panes {
			if j >= len(ew.Panes) {
				break
			}
			ep := &ew.Panes[j]
			lp := &lw.Panes[j]
			// Preserve user-set split direction.
			if ep.Split != "" && ep.Split != "right" {
				lp.Split = ep.Split
			}
			// Preserve user-set command.
			if ep.Command != "" {
				lp.Command = ep.Command
			}
		}
	}
}
