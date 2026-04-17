package orchestrate

import (
	"fmt"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/config"
	"github.com/drolosoft/cmux-resurrect/internal/gallery"
	"github.com/drolosoft/cmux-resurrect/internal/model"
)

// ImportStatus describes the outcome for a single workspace during import.
type ImportStatus int

const (
	// ImportCreated means the workspace was created (or would be in dry-run).
	ImportCreated ImportStatus = iota
	// ImportSkipped means the workspace already existed and was skipped.
	ImportSkipped
	// ImportFailed means workspace creation failed.
	ImportFailed
	// ImportWarn means a non-fatal issue occurred during workspace setup.
	ImportWarn
)

// ImportEvent describes what happened to a single workspace during import.
type ImportEvent struct {
	Status       ImportStatus
	Title        string
	Panes        []model.Pane // resolved template panes
	ExpandedPath string       // expanded CWD path
	Template     string       // template name
	Pin          bool
	Warn         string // non-empty for ImportWarn events
	Err          error  // non-nil for ImportFailed
}

// ImportResult reports the outcome of an import operation.
type ImportResult struct {
	Created int
	Skipped int
}

// Importer creates cmux workspaces from a parsed Workspace Blueprint.
type Importer struct {
	Client     client.Backend
	OnProgress func(event ImportEvent) // called per workspace and per warning
}

// ImportFromMD resolves templates and creates workspaces that don't already
// exist in cmux. When dryRun is true, no client calls are made; the OnProgress
// callback is invoked with ImportCreated for each enabled project so the
// caller can render a preview.
func (im *Importer) ImportFromMD(wf *model.WorkspaceFile, dryRun bool) (*ImportResult, error) {
	enabled := wf.EnabledProjects()
	if len(enabled) == 0 {
		return &ImportResult{}, nil
	}

	// Get current workspaces to avoid duplicates.
	var existingTitles map[string]bool
	if !dryRun {
		existing, err := im.Client.ListWorkspaces()
		if err != nil {
			return nil, fmt.Errorf("list workspaces: %w", err)
		}
		existingTitles = make(map[string]bool)
		for _, ws := range existing {
			existingTitles[ws.Title] = true
		}
	}

	result := &ImportResult{}

	for i, p := range enabled {
		title := p.BuildTitle(i)
		expandedPath := config.ExpandHome(p.Path)
		panes := gallery.ResolveTemplate(wf, p.Template)

		if dryRun {
			im.emit(ImportEvent{
				Status:       ImportCreated,
				Title:        title,
				Panes:        panes,
				ExpandedPath: expandedPath,
				Template:     p.Template,
				Pin:          p.Pin,
			})
			result.Created++
			continue
		}

		// Skip if workspace with this title already exists.
		if existingTitles[title] {
			im.emit(ImportEvent{
				Status: ImportSkipped,
				Title:  title,
				Panes:  panes,
			})
			result.Skipped++
			continue
		}

		// 1. Create workspace.
		ref, err := im.Client.NewWorkspace(client.NewWorkspaceOpts{CWD: expandedPath})
		if err != nil {
			im.emit(ImportEvent{
				Status: ImportFailed,
				Title:  title,
				Panes:  panes,
				Err:    err,
			})
			continue
		}

		time.Sleep(DelayAfterCreate)

		// 2. Select workspace to ensure splits target the correct one.
		if err := im.Client.SelectWorkspace(ref); err != nil {
			im.emit(ImportEvent{
				Status: ImportWarn,
				Title:  title,
				Panes:  panes,
				Warn:   fmt.Sprintf("%s: select failed: %v", title, err),
			})
		}
		time.Sleep(DelayAfterSelect)

		// 3. Create splits and send commands.
		for j, pane := range panes {
			if j == 0 {
				if pane.Command != "" {
					_ = im.Client.Send(ref, "", pane.Command+"\\n")
				}
				continue
			}
			// Focus a specific pane before splitting (for quad, etc.)
			if pane.FocusTarget >= 0 {
				targetRef := fmt.Sprintf("pane:%d", pane.FocusTarget)
				if err := im.Client.FocusPane(targetRef, ref); err != nil {
					im.emit(ImportEvent{
						Status: ImportWarn,
						Title:  title,
						Panes:  panes,
						Warn:   fmt.Sprintf("%s pane %d: focus target failed: %v", title, j, err),
					})
				}
				time.Sleep(DelayAfterSelect)
			}

			split := pane.Split
			if split == "" {
				split = "right"
			}
			surfaceRef, err := im.Client.NewSplit(split, ref)
			if err != nil {
				im.emit(ImportEvent{
					Status: ImportWarn,
					Title:  title,
					Panes:  panes,
					Warn:   fmt.Sprintf("%s pane %d: split failed: %v", title, j, err),
				})
				continue
			}
			// Wait for shell to fully initialize in the new pane.
			time.Sleep(DelayAfterSplit)
			if pane.Command != "" {
				_ = im.Client.Send(ref, surfaceRef, pane.Command+"\\n")
			}
		}

		// 4. Wait for shell to settle, then rename.
		time.Sleep(DelayBeforeRename)
		if err := im.Client.RenameWorkspace(ref, title); err != nil {
			im.emit(ImportEvent{
				Status: ImportWarn,
				Title:  title,
				Panes:  panes,
				Warn:   fmt.Sprintf("%s: rename failed: %v", title, err),
			})
		}

		// 5. Pin if requested.
		if p.Pin {
			if err := im.Client.PinWorkspace(ref); err != nil {
				im.emit(ImportEvent{
					Status: ImportWarn,
					Title:  title,
					Panes:  panes,
					Warn:   fmt.Sprintf("%s: pin failed: %v", title, err),
				})
			}
		}

		im.emit(ImportEvent{
			Status: ImportCreated,
			Title:  title,
			Panes:  panes,
		})
		result.Created++
	}

	return result, nil
}

// emit sends an event to the OnProgress callback if one is set.
func (im *Importer) emit(event ImportEvent) {
	if im.OnProgress != nil {
		im.OnProgress(event)
	}
}
