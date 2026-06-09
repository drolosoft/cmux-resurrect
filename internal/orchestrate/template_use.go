package orchestrate

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/model"
)

// TemplateUseOpts configures a one-shot workspace creation from a template.
type TemplateUseOpts struct {
	Title string
	Icon  string
	CWD   string
	Pin   bool
}

// TemplateUseResult reports what happened.
type TemplateUseResult struct {
	Title    string
	Panes    int
	DryRun   bool
	Commands []string
}

// TemplateUser creates a single workspace from resolved template panes.
type TemplateUser struct {
	Client     client.Backend
	OnProgress func(msg string)
}

// Use creates a workspace from template panes.
func (tu *TemplateUser) Use(panes []model.Pane, opts TemplateUseOpts, dryRun bool) (*TemplateUseResult, error) {
	// Build title from opts or CWD basename.
	title := opts.Title
	if title == "" {
		title = filepath.Base(opts.CWD)
	}
	if opts.Icon != "" {
		title = opts.Icon + " " + title
	}

	result := &TemplateUseResult{
		Title:  title,
		Panes:  len(panes),
		DryRun: dryRun,
	}

	if dryRun {
		return tu.dryRun(panes, opts, title, result)
	}

	return tu.execute(panes, opts, title, result)
}

func (tu *TemplateUser) dryRun(panes []model.Pane, opts TemplateUseOpts, title string, result *TemplateUseResult) (*TemplateUseResult, error) {
	ref := "workspace:new"
	f := tu.Client.DryRunFormatter()

	result.Commands = append(result.Commands, f.FmtNewWorkspace(opts.CWD))

	for i, pane := range panes {
		if i == 0 {
			if pane.Type == "browser" && pane.Command != "" {
				result.Commands = append(result.Commands, f.FmtSend(ref, fmt.Sprintf("open %q", pane.Command)))
			} else if pane.Command != "" {
				result.Commands = append(result.Commands, f.FmtSend(ref, pane.Command))
			}
			continue
		}
		if pane.FocusTarget >= 0 {
			result.Commands = append(result.Commands,
				f.FmtFocusPane(fmt.Sprintf("pane:%d", pane.FocusTarget), ref))
		}
		direction := pane.Split
		if direction == "" {
			direction = "right"
		}
		if pane.Type == "browser" {
			result.Commands = append(result.Commands, f.FmtNewPane(pane.Type, direction, ref, pane.Command))
		} else {
			result.Commands = append(result.Commands, f.FmtNewSplit(direction, ref))
			if pane.Command != "" {
				result.Commands = append(result.Commands, f.FmtSend(ref, pane.Command))
			}
		}
	}

	result.Commands = append(result.Commands, f.FmtRenameWorkspace(ref, title))

	if opts.Pin {
		result.Commands = append(result.Commands, f.FmtPinWorkspace(ref))
	}

	return result, nil
}

func (tu *TemplateUser) execute(panes []model.Pane, opts TemplateUseOpts, title string, result *TemplateUseResult) (*TemplateUseResult, error) {
	tu.progress("Creating workspace...")

	// 1. Create workspace.
	ref, err := tu.Client.NewWorkspace(client.NewWorkspaceOpts{CWD: opts.CWD})
	if err != nil {
		return nil, fmt.Errorf("new-workspace: %w", err)
	}
	time.Sleep(DelayAfterCreate)

	// 2. Select workspace to ensure splits target the correct one.
	if err := tu.Client.SelectWorkspace(ref); err != nil {
		return nil, fmt.Errorf("select-workspace: %w", err)
	}
	time.Sleep(DelayAfterSelect)

	// 3. Create all splits first (structure), then apply resizes and commands.
	// This avoids focus interference: resize-pane and interactive commands
	// (nvim, lazygit) can steal focus from the pane we need for the next split.
	//
	// cmux reindexes panes by position after each split, so we track actual
	// pane refs (from the tree) to use for FocusTarget instead of indices.
	type deferredAction struct {
		paneIdx    int
		surfaceRef string
		paneRef    string // actual cmux pane ref for focus
		command    string
		ratio      float64
		direction  string
	}
	var deferred []deferredAction

	// paneRefs maps creation index → actual cmux pane ref.
	paneRefs := make(map[int]string)

	// Resolve the first pane's ref from the tree.
	if tree, err := tu.Client.Tree(); err == nil {
		for _, w := range tree.Windows {
			for _, ws := range w.Workspaces {
				if ws.Ref == ref && len(ws.Panes) > 0 {
					paneRefs[0] = ws.Panes[0].Ref
				}
			}
		}
	}

	for i, pane := range panes {
		if i == 0 {
			// Main pane command deferred to after all splits.
			if pane.Type == "browser" && pane.Command != "" {
				deferred = append(deferred, deferredAction{paneIdx: 0, paneRef: paneRefs[0], command: fmt.Sprintf("open %q", pane.Command)})
			} else if pane.Command != "" {
				deferred = append(deferred, deferredAction{paneIdx: 0, paneRef: paneRefs[0], command: pane.Command})
			}
			continue
		}

		// Focus a specific pane before splitting (for quad, etc.)
		// Use the actual pane ref we tracked, not a position-based index.
		if pane.FocusTarget >= 0 {
			if actualRef, ok := paneRefs[pane.FocusTarget]; ok {
				if err := tu.Client.FocusPane(actualRef, ref); err != nil {
					tu.progress(fmt.Sprintf("pane %d focus target: %v", i, err))
				}
			}
			time.Sleep(DelayAfterSelect)
		}

		direction := pane.Split
		if direction == "" {
			direction = "right"
		}

		var surfaceRef string
		if pane.Type == "browser" {
			_, err := tu.Client.NewPane(client.NewPaneOpts{
				Type:         "browser",
				Direction:    direction,
				WorkspaceRef: ref,
				URL:          pane.Command,
			})
			if err != nil {
				tu.progress(fmt.Sprintf("pane %d new-pane browser: %v", i, err))
				continue
			}
		} else {
			var err error
			surfaceRef, err = tu.Client.NewSplit(direction, ref, "")
			if err != nil {
				tu.progress(fmt.Sprintf("pane %d split: %v", i, err))
				continue
			}
		}

		// Resolve the new pane's actual ref from the tree (indices shift after splits).
		if tree, err := tu.Client.Tree(); err == nil {
			for _, w := range tree.Windows {
				for _, ws := range w.Workspaces {
					if ws.Ref != ref {
						continue
					}
					// The new pane is the one whose ref we haven't seen yet.
					seen := make(map[string]bool)
					for _, r := range paneRefs {
						seen[r] = true
					}
					for _, p := range ws.Panes {
						if !seen[p.Ref] {
							paneRefs[i] = p.Ref
							break
						}
					}
				}
			}
		}

		// Focus the new pane using its actual ref.
		if actualRef, ok := paneRefs[i]; ok {
			_ = tu.Client.FocusPane(actualRef, ref)
		}
		time.Sleep(DelayAfterSelect)

		// Defer commands and resizes to after all splits.
		if pane.Command != "" || needsResize(pane.SplitRatio) {
			deferred = append(deferred, deferredAction{
				paneIdx:    i,
				surfaceRef: surfaceRef,
				paneRef:    paneRefs[i],
				command:    pane.Command,
				ratio:      pane.SplitRatio,
				direction:  direction,
			})
		}
	}

	// 4. Apply deferred resizes and commands now that all splits are done.
	if resizer, ok := tu.Client.(client.PaneResizer); ok {
		for _, d := range deferred {
			if !needsResize(d.ratio) {
				continue
			}
			delta := d.ratio - 0.5
			amount := int(absf(delta) * 950) // pixels, ~950px workspace height
			if d.direction == "right" || d.direction == "left" {
				amount = int(absf(delta) * 1400)
			}
			if amount > 0 {
				resizeDir := strings.ToUpper(d.direction[:1])
				target := "pane:0"
				if delta > 0 {
					target = fmt.Sprintf("pane:%d", d.paneIdx)
				}
				_ = resizer.ResizePane(client.ResizePaneOpts{
					PaneRef:      target,
					WorkspaceRef: ref,
					Direction:    resizeDir,
					Amount:       amount,
				})
			}
		}
	}

	// 5. Send deferred commands using actual pane refs.
	for _, d := range deferred {
		if d.command == "" {
			continue
		}
		if d.paneRef != "" {
			_ = tu.Client.FocusPane(d.paneRef, ref)
		}
		time.Sleep(DelayAfterSelect)
		if err := waitForShellReady(tu.Client, ref, d.surfaceRef); err != nil {
			tu.progress(fmt.Sprintf("pane %d shell not ready: %v", d.paneIdx, err))
		} else if err := tu.Client.Send(ref, d.surfaceRef, noHistoryCmd(d.command)); err != nil {
			tu.progress(fmt.Sprintf("pane %d send: %v", d.paneIdx, err))
		}
	}

	// 6. Wait for shell to settle, then rename.
	time.Sleep(DelayBeforeRename)
	if err := tu.Client.RenameWorkspace(ref, title); err != nil {
		tu.progress(fmt.Sprintf("rename: %v", err))
	}

	// 7. Pin if requested.
	if opts.Pin {
		if err := tu.Client.PinWorkspace(ref); err != nil {
			tu.progress(fmt.Sprintf("pin: %v", err))
		}
	}

	return result, nil
}

// progress sends a message to the OnProgress callback if one is set.
func (tu *TemplateUser) progress(msg string) {
	if tu.OnProgress != nil {
		tu.OnProgress(msg)
	}
}
