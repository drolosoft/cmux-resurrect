package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/config"
	"github.com/drolosoft/cmux-resurrect/internal/mdfile"
	"github.com/spf13/cobra"
)

var importDryRun bool

var importFromMDCmd = &cobra.Command{
	Use:   "import-from-md",
	Short: "Create cmux workspaces from a Markdown workspace file",
	Long:  "Reads the workspace Markdown file, resolves templates, and creates any workspaces that don't already exist in cmux.",
	RunE:  runImportFromMD,
}

func init() {
	importFromMDCmd.Flags().BoolVar(&importDryRun, "dry-run", false, "show what would happen without executing")
	rootCmd.AddCommand(importFromMDCmd)
}

func runImportFromMD(cmd *cobra.Command, args []string) error {
	cl := newClient()
	wsFile := cfg.WorkspaceFile

	wf, err := mdfile.Parse(wsFile)
	if err != nil {
		return fmt.Errorf("parse workspace file: %w", err)
	}

	if err := cl.Ping(); err != nil && !importDryRun {
		return fmt.Errorf("cmux not reachable: %w", err)
	}

	enabled := wf.EnabledProjects()
	if len(enabled) == 0 {
		fmt.Println("No enabled workspaces in workspace file.")
		return nil
	}

	// Get current workspaces to avoid duplicates.
	var existingTitles map[string]bool
	if !importDryRun {
		existing, err := cl.ListWorkspaces()
		if err != nil {
			return fmt.Errorf("list workspaces: %w", err)
		}
		existingTitles = make(map[string]bool)
		for _, ws := range existing {
			existingTitles[ws.Title] = true
		}
	}

	created := 0
	skipped := 0

	for i, p := range enabled {
		title := p.BuildTitle(i)
		expandedPath := config.ExpandHome(p.Path)
		panes := wf.ResolveTemplate(p.Template)

		if importDryRun {
			pin := ""
			if p.Pin {
				pin = " [pin]"
			}
			fmt.Printf("CREATE  %s  cwd=%s  template=%s  panes=%d%s\n",
				title, expandedPath, p.Template, len(panes), pin)
			for j, pane := range panes {
				if j == 0 {
					desc := "main"
					if pane.Command != "" {
						desc += fmt.Sprintf(" cmd=%q", pane.Command)
					}
					fmt.Printf("        pane %d: %s\n", j, desc)
				} else {
					split := pane.Split
					if split == "" {
						split = "right"
					}
					desc := fmt.Sprintf("split %s", split)
					if pane.Command != "" {
						desc += fmt.Sprintf(" cmd=%q", pane.Command)
					}
					fmt.Printf("        pane %d: %s\n", j, desc)
				}
			}
			created++
			continue
		}

		// Skip if workspace with this title already exists.
		if existingTitles[title] {
			fmt.Fprintf(os.Stderr, "  SKIP  %s (already exists)\n", title)
			skipped++
			continue
		}

		// 1. Create workspace.
		ref, err := cl.NewWorkspace(client.NewWorkspaceOpts{CWD: expandedPath})
		if err != nil {
			fmt.Fprintf(os.Stderr, "  FAIL  %s: %v\n", title, err)
			continue
		}

		time.Sleep(300 * time.Millisecond)

		// 2. Select workspace to ensure splits target the correct one.
		if err := cl.SelectWorkspace(ref); err != nil {
			fmt.Fprintf(os.Stderr, "  WARN  %s: select failed: %v\n", title, err)
		}
		time.Sleep(100 * time.Millisecond)

		// 3. Create splits and send commands.
		for j, pane := range panes {
			if j == 0 {
				if pane.Command != "" {
					cl.Send(ref, "", pane.Command+"\n")
				}
				continue
			}
			split := pane.Split
			if split == "" {
				split = "right"
			}
			if err := cl.NewSplit(split, ref); err != nil {
				fmt.Fprintf(os.Stderr, "  WARN  %s pane %d: split failed: %v\n", title, j, err)
				continue
			}
			time.Sleep(200 * time.Millisecond)
			if pane.Command != "" {
				cl.Send(ref, "", pane.Command+"\n")
			}
		}

		// 4. Wait for shell to settle, then rename.
		// Shell prompt sets terminal title on startup; renaming too early gets overwritten.
		time.Sleep(500 * time.Millisecond)
		if err := cl.RenameWorkspace(ref, title); err != nil {
			fmt.Fprintf(os.Stderr, "  WARN  %s: rename failed: %v\n", title, err)
		}

		// 5. Pin if requested.
		if p.Pin {
			if err := cl.PinWorkspace(ref); err != nil {
				fmt.Fprintf(os.Stderr, "  WARN  %s: pin failed: %v\n", title, err)
			}
		}

		fmt.Fprintf(os.Stderr, "  OK    %s (%d panes)\n", title, len(panes))
		created++
	}

	if importDryRun {
		fmt.Fprintf(os.Stderr, "\nDry-run: would create %d workspaces\n", created)
	} else {
		fmt.Fprintf(os.Stderr, "\nImport complete: %d created, %d skipped\n", created, skipped)
	}
	return nil
}
