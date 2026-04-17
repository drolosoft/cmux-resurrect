package orchestrate

import (
	"fmt"
	"path/filepath"
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
			if pane.Command != "" {
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
		result.Commands = append(result.Commands, f.FmtNewSplit(direction, ref))
		if pane.Command != "" {
			result.Commands = append(result.Commands, f.FmtSend(ref, pane.Command))
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

	// 3. Create splits and send commands.
	for i, pane := range panes {
		if i == 0 {
			if pane.Command != "" {
				if err := tu.Client.Send(ref, "", pane.Command+"\\n"); err != nil {
					tu.progress(fmt.Sprintf("pane %d send: %v", i, err))
				}
			}
			continue
		}

		// Focus a specific pane before splitting (for quad, etc.)
		if pane.FocusTarget >= 0 {
			targetRef := fmt.Sprintf("pane:%d", pane.FocusTarget)
			if err := tu.Client.FocusPane(targetRef, ref); err != nil {
				tu.progress(fmt.Sprintf("pane %d focus target: %v", i, err))
			}
			time.Sleep(DelayAfterSelect)
		}

		direction := pane.Split
		if direction == "" {
			direction = "right"
		}
		surfaceRef, err := tu.Client.NewSplit(direction, ref)
		if err != nil {
			tu.progress(fmt.Sprintf("pane %d split: %v", i, err))
			continue
		}
		time.Sleep(DelayAfterSplit)

		if pane.Command != "" {
			if err := tu.Client.Send(ref, surfaceRef, pane.Command+"\\n"); err != nil {
				tu.progress(fmt.Sprintf("pane %d send: %v", i, err))
			}
		}
	}

	// 4. Wait for shell to settle, then rename.
	time.Sleep(DelayBeforeRename)
	if err := tu.Client.RenameWorkspace(ref, title); err != nil {
		tu.progress(fmt.Sprintf("rename: %v", err))
	}

	// 5. Pin if requested.
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
