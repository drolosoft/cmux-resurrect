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
	AutoAccept []string // tool names or ["all"] for auto-accept injection
	OnProgress func(event ImportEvent) // called per workspace and per warning
}

// applyAutoAccept injects the auto-accept flag into a command if configured.
func (im *Importer) applyAutoAccept(command string) string {
	cmd, tool := InjectAutoAccept(command, im.AutoAccept)
	if tool != "" && im.OnProgress != nil {
		flag := autoAcceptCache[tool]
		im.OnProgress(ImportEvent{
			Status: ImportWarn,
			Warn:   fmt.Sprintf("⚡ auto-accept: %s %s", tool, flag),
		})
	}
	return cmd
}

// paneRefForSurface finds the pane ref that contains a given surface ref.
func paneRefForSurface(cl client.Backend, surfaceRef, workspaceRef string) string {
	tree, err := cl.Tree()
	if err != nil {
		return ""
	}
	for _, w := range tree.Windows {
		for _, ws := range w.Workspaces {
			if ws.Ref != workspaceRef {
				continue
			}
			for _, p := range ws.Panes {
				for _, s := range p.Surfaces {
					if s.Ref == surfaceRef {
						return p.Ref
					}
				}
			}
		}
	}
	return ""
}

// firstPaneRef returns the ref of the first pane in a workspace.
func firstPaneRef(cl client.Backend, workspaceRef string) string {
	tree, err := cl.Tree()
	if err != nil {
		return ""
	}
	for _, w := range tree.Windows {
		for _, ws := range w.Workspaces {
			if ws.Ref != workspaceRef {
				continue
			}
			for _, p := range ws.Panes {
				if p.Index == 0 {
					return p.Ref
				}
			}
		}
	}
	return ""
}

// importSurfaces creates additional surfaces in a pane during import.
func (im *Importer) importSurfaces(pane model.Pane, paneRef, workspaceRef, title string) {
	for j, surf := range pane.Surfaces {
		surfRef, err := im.Client.NewSurface(paneRef, workspaceRef)
		if err != nil {
			if err == client.ErrNotSupported {
				im.emit(ImportEvent{
					Status: ImportWarn,
					Warn:   "pane tabs not supported on this backend",
				})
				return
			}
			im.emit(ImportEvent{
				Status: ImportWarn,
				Title:  title,
				Warn:   fmt.Sprintf("%s pane surface %d: %v", title, j+1, err),
			})
			continue
		}
		if surf.Command != "" {
			if err := waitForShellReady(im.Client, workspaceRef, surfRef); err == nil {
				_ = im.Client.Send(workspaceRef, surfRef, noHistoryCmd(im.applyAutoAccept(surf.Command)))
			}
		}
	}
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

		// 3. Create splits and send commands (first pass: splits only).
		// Track surface refs per pane for the second pass (extra surfaces).
		paneSurfaceRefs := make(map[int]string) // pane index → surface ref
		for j, pane := range panes {
			if j == 0 {
				if pane.Type == "browser" && pane.Command != "" {
					if err := waitForShellReady(im.Client, ref, ""); err == nil {
						_ = im.Client.Send(ref, "", noHistoryCmd(fmt.Sprintf("open %q", pane.Command)))
					}
				} else if pane.Command != "" {
					if err := waitForShellReady(im.Client, ref, ""); err == nil {
						_ = im.Client.Send(ref, "", noHistoryCmd(im.applyAutoAccept(pane.Command)))
					}
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

			if pane.Type == "browser" {
				_, err := im.Client.NewPane(client.NewPaneOpts{
					Type:         "browser",
					Direction:    split,
					WorkspaceRef: ref,
					URL:          pane.Command,
				})
				if err != nil {
					im.emit(ImportEvent{
						Status: ImportWarn,
						Title:  title,
						Panes:  panes,
						Warn:   fmt.Sprintf("%s pane %d: browser pane failed: %v", title, j, err),
					})
				}
			} else {
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
				paneSurfaceRefs[j] = surfaceRef
				if pane.Command != "" {
					if err := waitForShellReady(im.Client, ref, surfaceRef); err == nil {
						_ = im.Client.Send(ref, surfaceRef, noHistoryCmd(im.applyAutoAccept(pane.Command)))
					}
				} else {
					time.Sleep(DelayAfterSplit)
				}
			}
		}

		// 3b. Second pass: add extra surfaces (tabs) to panes.
		// Done after all splits to avoid focus interference.
		for j, pane := range panes {
			if len(pane.Surfaces) == 0 {
				continue
			}
			var paneRef string
			if j == 0 {
				// First pane: find via tree (pane:0 index isn't reliable)
				paneRef = firstPaneRef(im.Client, ref)
			} else if surfRef, ok := paneSurfaceRefs[j]; ok {
				paneRef = paneRefForSurface(im.Client, surfRef, ref)
			}
			if paneRef != "" {
				im.importSurfaces(pane, paneRef, ref, title)
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
