package orchestrate

import (
	"fmt"
	"sort"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

// Restorer recreates a saved layout in cmux.
type Restorer struct {
	Client client.CmuxClient
	Store  persist.Store
}

// RestoreResult reports what happened during restore.
type RestoreResult struct {
	LayoutName      string
	WorkspacesTotal int
	WorkspacesOK    int
	Errors          []string
	DryRun          bool
	Commands        []string // populated in dry-run mode
}

// Restore loads a layout and recreates it in cmux.
func (r *Restorer) Restore(name string, dryRun bool) (*RestoreResult, error) {
	layout, err := r.Store.Load(name)
	if err != nil {
		return nil, fmt.Errorf("load layout: %w", err)
	}

	if !dryRun {
		if err := r.Client.Ping(); err != nil {
			return nil, fmt.Errorf("cmux not reachable: %w", err)
		}
	}

	result := &RestoreResult{
		LayoutName:      layout.Name,
		WorkspacesTotal: len(layout.Workspaces),
		DryRun:          dryRun,
	}

	// Sort workspaces by index.
	workspaces := make([]model.Workspace, len(layout.Workspaces))
	copy(workspaces, layout.Workspaces)
	sort.Slice(workspaces, func(i, j int) bool {
		return workspaces[i].Index < workspaces[j].Index
	})

	var activeRef string

	for _, ws := range workspaces {
		ref, err := r.restoreWorkspace(ws, dryRun, result)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("workspace %q: %v", ws.Title, err))
			continue
		}
		result.WorkspacesOK++
		if ws.Active {
			activeRef = ref
		}
	}

	// Restore active workspace.
	if activeRef != "" && !dryRun {
		if err := r.Client.SelectWorkspace(activeRef); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("select active workspace: %v", err))
		}
	} else if activeRef != "" {
		result.Commands = append(result.Commands, fmt.Sprintf("cmux select-workspace --workspace %s", activeRef))
	}

	return result, nil
}

func (r *Restorer) restoreWorkspace(ws model.Workspace, dryRun bool, result *RestoreResult) (string, error) {
	if dryRun {
		return r.dryRunWorkspace(ws, result)
	}

	// 1. Create workspace.
	ref, err := r.Client.NewWorkspace(client.NewWorkspaceOpts{CWD: ws.CWD})
	if err != nil {
		return "", fmt.Errorf("new-workspace: %w", err)
	}

	// Small delay after creation.
	time.Sleep(200 * time.Millisecond)

	// 2. Rename workspace.
	if err := r.Client.RenameWorkspace(ref, ws.Title); err != nil {
		return ref, fmt.Errorf("rename-workspace: %w", err)
	}

	// 3. Create additional panes (splits).
	for i, pane := range ws.Panes {
		if i == 0 {
			// First pane is the default one created with the workspace.
			if pane.Command != "" {
				if err := r.Client.Send(ref, "", pane.Command+"\n"); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("  pane %d send command: %v", i, err))
				}
			}
			continue
		}

		direction := pane.Split
		if direction == "" {
			direction = "right"
		}
		if err := r.Client.NewSplit(direction, ref); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("  pane %d split: %v", i, err))
			continue
		}

		time.Sleep(200 * time.Millisecond)

		if pane.Command != "" {
			if err := r.Client.Send(ref, "", pane.Command+"\n"); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("  pane %d send command: %v", i, err))
			}
		}
	}

	// 4. Focus the right pane.
	for _, pane := range ws.Panes {
		if pane.Focus && pane.Index > 0 {
			paneRef := fmt.Sprintf("pane:%d", pane.Index)
			_ = r.Client.FocusPane(paneRef, ref)
			break
		}
	}

	return ref, nil
}

func (r *Restorer) dryRunWorkspace(ws model.Workspace, result *RestoreResult) (string, error) {
	ref := fmt.Sprintf("workspace:new_%d", ws.Index)

	result.Commands = append(result.Commands,
		fmt.Sprintf("cmux new-workspace --cwd %q", ws.CWD))
	result.Commands = append(result.Commands,
		fmt.Sprintf("cmux rename-workspace --workspace %s %q", ref, ws.Title))

	for i, pane := range ws.Panes {
		if i == 0 {
			if pane.Command != "" {
				result.Commands = append(result.Commands,
					fmt.Sprintf("cmux send --workspace %s %q", ref, pane.Command))
			}
			continue
		}
		direction := pane.Split
		if direction == "" {
			direction = "right"
		}
		result.Commands = append(result.Commands,
			fmt.Sprintf("cmux new-split %s --workspace %s", direction, ref))
		if pane.Command != "" {
			result.Commands = append(result.Commands,
				fmt.Sprintf("cmux send --workspace %s %q", ref, pane.Command))
		}
	}

	return ref, nil
}
