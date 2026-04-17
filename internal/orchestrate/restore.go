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
	Client     client.Backend
	Store      persist.Store
	OnProgress func(title string, panes int, err error) // called after each workspace
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

	// Remember the caller's workspace and snapshot existing workspace refs/titles.
	var callerRef string
	var callerTitle string
	var oldRefs []string
	existingTitles := make(map[string]bool)
	if !dryRun {
		if tree, err := r.Client.Tree(); err == nil && tree.Caller != nil {
			callerRef = tree.Caller.WorkspaceRef
			// Find the caller's title from the tree.
			for _, w := range tree.Windows {
				for _, ws := range w.Workspaces {
					if ws.Ref == callerRef {
						callerTitle = ws.Title
					}
				}
			}
		}
		if existing, err := r.Client.ListWorkspaces(); err == nil {
			for _, ws := range existing {
				if mode == RestoreModeReplace {
					oldRefs = append(oldRefs, ws.Ref)
				}
				existingTitles[ws.Title] = true
			}
		}
	} else if mode == RestoreModeReplace {
		result.Commands = append(result.Commands, "# Close all existing workspaces (except caller)")
	}

	// In replace mode, close old workspaces BEFORE creating new ones.
	// Skip the caller's workspace so the running terminal survives.
	if mode == RestoreModeReplace && !dryRun {
		for _, ref := range oldRefs {
			if ref == callerRef {
				continue
			}
			if err := r.Client.CloseWorkspace(ref); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("close old %s: %v", ref, err))
			} else {
				result.WorkspacesClosed++
			}
			time.Sleep(DelayAfterClose)
		}
		if result.WorkspacesClosed > 0 {
			time.Sleep(DelayAfterCloseAll)
		}
	}

	// Sort workspaces by index.
	workspaces := make([]model.Workspace, len(layout.Workspaces))
	copy(workspaces, layout.Workspaces)
	sort.Slice(workspaces, func(i, j int) bool {
		return workspaces[i].Index < workspaces[j].Index
	})

	// Create new workspaces (skip duplicates in add mode, skip caller title in replace mode).
	for _, ws := range workspaces {
		if !dryRun {
			// In add mode, skip any workspace whose title already exists.
			// In replace mode, skip only the caller's title (all others were closed).
			if mode == RestoreModeAdd && existingTitles[ws.Title] {
				if r.OnProgress != nil {
					r.OnProgress(ws.Title, len(ws.Panes), fmt.Errorf("already exists, skipped"))
				}
				continue
			}
			if mode == RestoreModeReplace && callerTitle != "" && ws.Title == callerTitle {
				if r.OnProgress != nil {
					r.OnProgress(ws.Title, len(ws.Panes), fmt.Errorf("caller workspace, skipped"))
				}
				continue
			}
		}

		_, err := r.restoreWorkspace(ws, dryRun, result)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("workspace %q: %v", ws.Title, err))
			if r.OnProgress != nil && !dryRun {
				r.OnProgress(ws.Title, len(ws.Panes), err)
			}
			continue
		}
		result.WorkspacesOK++
		if r.OnProgress != nil && !dryRun {
			r.OnProgress(ws.Title, len(ws.Panes), nil)
		}
	}

	// Return focus to the caller's workspace (the terminal that ran crex).
	if callerRef != "" && !dryRun {
		if err := r.Client.SelectWorkspace(callerRef); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("select caller workspace: %v", err))
		}
	} else if dryRun {
		result.Commands = append(result.Commands, "cmux select-workspace --workspace <caller>")
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
	time.Sleep(DelayAfterCreate)

	// 2. Select workspace to ensure splits target the correct one.
	// Rename is deferred to after all workspaces are created (shell prompt overwrites title).
	if err := r.Client.SelectWorkspace(ref); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("select workspace: %v", err))
	}
	time.Sleep(DelayAfterSelect)

	// 3. Create additional panes (splits) and send commands.
	for i, pane := range ws.Panes {
		if i == 0 {
			// First pane is the default one created with the workspace.
			if pane.Command != "" {
				if err := r.Client.Send(ref, "", pane.Command+"\\n"); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("  pane %d send command: %v", i, err))
				}
			}
			continue
		}

		// Focus a specific pane before splitting (for quad, etc.)
		if pane.FocusTarget >= 0 {
			targetRef := fmt.Sprintf("pane:%d", pane.FocusTarget)
			if err := r.Client.FocusPane(targetRef, ref); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("  pane %d focus target: %v", i, err))
			}
			time.Sleep(DelayAfterSelect)
		}

		direction := pane.Split
		if direction == "" {
			direction = "right"
		}
		surfaceRef, err := r.Client.NewSplit(direction, ref)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("  pane %d split: %v", i, err))
			continue
		}

		// Wait for the shell in the new pane to fully initialize.
		time.Sleep(DelayAfterSplit)

		if pane.Command != "" {
			// Send to the specific surface — without --surface, cmux defaults to pane 0.
			if err := r.Client.Send(ref, surfaceRef, pane.Command+"\\n"); err != nil {
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
	time.Sleep(DelayBeforeRename)
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

	// Blank line separator between workspace groups.
	result.Commands = append(result.Commands, "")
	// Workspace header comment.
	result.Commands = append(result.Commands,
		fmt.Sprintf("# %s", ws.Title))

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
		if pane.FocusTarget >= 0 {
			result.Commands = append(result.Commands,
				fmt.Sprintf("cmux focus-pane --pane pane:%d --workspace %s", pane.FocusTarget, ref))
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
