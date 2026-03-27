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
	Short: "Create cmux workspaces from a Workspace Blueprint",
	Long:  "Reads a Workspace Blueprint (.md), resolves templates, and creates any workspaces that don't already exist in cmux.",
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
		return fmt.Errorf("parse blueprint: %w", err)
	}

	if err := cl.Ping(); err != nil && !importDryRun {
		return fmt.Errorf("cmux not reachable: %w", err)
	}

	enabled := wf.EnabledProjects()
	if len(enabled) == 0 {
		fmt.Fprintln(os.Stderr, dimStyle.Render("  No enabled workspaces in Workspace Blueprint."))
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

	fmt.Fprintln(os.Stderr)
	if importDryRun {
		fmt.Fprintf(os.Stderr, "%s %s\n", yellowStyle.Render("👁️  Dry-run import from"), dimStyle.Render(wsFile))
	} else {
		fmt.Fprintf(os.Stderr, "%s %s\n", greenStyle.Render("📥 Importing from"), dimStyle.Render(wsFile))
	}
	fmt.Fprintln(os.Stderr)

	created := 0
	skipped := 0

	for i, p := range enabled {
		title := p.BuildTitle(i)
		expandedPath := config.ExpandHome(p.Path)
		panes := wf.ResolveTemplate(p.Template)

		if importDryRun {
			pin := ""
			if p.Pin {
				pin = " 📌"
			}

			// Workspace header
			fmt.Fprintf(os.Stderr, "  %s %s%s\n",
				greenStyle.Render("CREATE"),
				greenStyle.Render(padTitle(title)),
				pin)

			// Details
			fmt.Fprintf(os.Stderr, "         %s  %s  %s\n",
				dimStyle.Render("cwd="+expandedPath),
				cyanStyle.Render("template="+p.Template),
				dimStyle.Render(fmt.Sprintf("panes=%d", len(panes))))

			// Pane tree
			for j, pane := range panes {
				isLast := j == len(panes)-1
				prefix := dimStyle.Render("         ├──")
				if isLast {
					prefix = dimStyle.Render("         └──")
				}

				if j == 0 {
					desc := "main"
					if pane.Command != "" {
						desc += " " + cyanStyle.Render(pane.Command)
					}
					fmt.Fprintf(os.Stderr, "%s %s\n", prefix, desc)
				} else {
					split := pane.Split
					if split == "" {
						split = "right"
					}
					desc := magentaStyle.Render("→" + split)
					if pane.Command != "" {
						desc += " " + cyanStyle.Render(pane.Command)
					}
					fmt.Fprintf(os.Stderr, "%s %s\n", prefix, desc)
				}
			}

			fmt.Fprintln(os.Stderr)
			created++
			continue
		}

		// Skip if workspace with this title already exists.
		if existingTitles[title] {
			fmt.Fprintf(os.Stderr, "  %s  %s %s\n",
				dimStyle.Render("SKIP"),
				padTitle(title),
				dimStyle.Render("(already exists)"))
			skipped++
			continue
		}

		// 1. Create workspace.
		ref, err := cl.NewWorkspace(client.NewWorkspaceOpts{CWD: expandedPath})
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %s  %s: %v\n", yellowStyle.Render("FAIL"), title, err)
			continue
		}

		time.Sleep(300 * time.Millisecond)

		// 2. Select workspace to ensure splits target the correct one.
		if err := cl.SelectWorkspace(ref); err != nil {
			fmt.Fprintf(os.Stderr, "  %s  %s: select failed: %v\n", yellowStyle.Render("WARN"), title, err)
		}
		time.Sleep(100 * time.Millisecond)

		// 3. Create splits and send commands.
		for j, pane := range panes {
			if j == 0 {
				if pane.Command != "" {
					cl.Send(ref, "", pane.Command+"\\n")
				}
				continue
			}
			split := pane.Split
			if split == "" {
				split = "right"
			}
			surfaceRef, err := cl.NewSplit(split, ref)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  %s  %s pane %d: split failed: %v\n", yellowStyle.Render("WARN"), title, j, err)
				continue
			}
			// Wait for shell to fully initialize in the new pane.
			time.Sleep(500 * time.Millisecond)
			if pane.Command != "" {
				cl.Send(ref, surfaceRef, pane.Command+"\\n")
			}
		}

		// 4. Wait for shell to settle, then rename.
		time.Sleep(500 * time.Millisecond)
		if err := cl.RenameWorkspace(ref, title); err != nil {
			fmt.Fprintf(os.Stderr, "  %s  %s: rename failed: %v\n", yellowStyle.Render("WARN"), title, err)
		}

		// 5. Pin if requested.
		if p.Pin {
			if err := cl.PinWorkspace(ref); err != nil {
				fmt.Fprintf(os.Stderr, "  %s  %s: pin failed: %v\n", yellowStyle.Render("WARN"), title, err)
			}
		}

		fmt.Fprintf(os.Stderr, "  %s  %s (%d panes)\n", greenStyle.Render("OK"), padTitle(title), len(panes))
		created++
	}

	if importDryRun {
		fmt.Fprintf(os.Stderr, "%s\n\n",
			greenStyle.Render(fmt.Sprintf("✅ Would create %d workspaces", created)))
	} else {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "%s\n\n",
			greenStyle.Render(fmt.Sprintf("✅ Import complete: %d created, %d skipped", created, skipped)))
	}
	return nil
}
