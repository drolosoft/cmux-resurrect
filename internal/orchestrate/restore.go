package orchestrate

import (
	"fmt"
	"sort"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

// RestoreMode determines how restore interacts with existing workspaces.
type RestoreMode int

const (
	// RestoreModeReplace closes all existing workspaces before restoring.
	RestoreModeReplace RestoreMode = iota
	// RestoreModeAdd adds restored workspaces on top of existing ones.
	RestoreModeAdd
)

// Restorer recreates a saved layout in cmux.
type Restorer struct {
	Client client.CmuxClient
	Store  persist.Store
}

// RestoreResult reports what happened during restore.
type RestoreResult struct {
	LayoutName       string
	WorkspacesTotal  int
	WorkspacesOK     int
	WorkspacesClosed int
	Errors           []string
	DryRun           bool
	Commands         []string // populated in dry-run mode
}

// Restore loads a layout and recreates it in cmux.
func (r *Restorer) Restore(name string, dryRun bool, mode RestoreMode) (*RestoreResult, error) {
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

	// In replace mode, close all existing workspaces first.
	if mode == RestoreModeReplace && !dryRun {
		existing, err := r.Client.ListWorkspaces()
		if err != nil {
			return nil, fmt.Errorf("list existing workspaces: %w", err)
		}
		for _, ws := range existing {
			if err := r.Client.CloseWorkspace(ws.Ref); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("close %q: %v", ws.Title, err))
			} else {
				result.WorkspacesClosed++
			}
			time.Sleep(100 * time.Millisecond)
		}
		if result.WorkspacesClosed > 0 {
			time.Sleep(300 * time.Millisecond)
		}
	} else if mode == RestoreModeReplace && dryRun {
		result.Commands = append(result.Commands, "# Close all existing workspaces")
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
	time.Sleep(300 * time.Millisecond)

	// 2. Select workspace to ensure splits target the correct one.
	// Rename is deferred to after all workspaces are created (shell prompt overwrites title).
	if err := r.Client.SelectWorkspace(ref); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("select workspace: %v", err))
	}
	time.Sleep(100 * time.Millisecond)

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

	// 5. Wait for shell to settle, then rename.
	// Shell prompt sets terminal title on startup; renaming too early gets overwritten.
	time.Sleep(500 * time.Millisecond)
	if err := r.Client.RenameWorkspace(ref, ws.Title); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("rename %q: %v", ws.Title, err))
	}

	// 6. Pin if requested.
	if ws.Pinned {
		if err := r.Client.PinWorkspace(ref); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("pin %q: %v", ws.Title, err))
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
