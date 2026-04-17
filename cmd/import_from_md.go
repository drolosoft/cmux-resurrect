package cmd

import (
	"fmt"
	"os"

	"github.com/drolosoft/cmux-resurrect/internal/mdfile"
	"github.com/drolosoft/cmux-resurrect/internal/orchestrate"
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
		return fmt.Errorf("backend not reachable: %w", err)
	}

	enabled := wf.EnabledProjects()
	if len(enabled) == 0 {
		fmt.Fprintln(os.Stderr, dimStyle.Render("  No enabled workspaces in Workspace Blueprint."))
		return nil
	}

	fmt.Fprintln(os.Stderr)
	if importDryRun {
		fmt.Fprintf(os.Stderr, "%s %s\n", yellowStyle.Render("👁️  Dry-run import from"), dimStyle.Render(wsFile))
	} else {
		fmt.Fprintf(os.Stderr, "%s %s\n", greenStyle.Render("📥 Importing from"), dimStyle.Render(wsFile))
	}
	fmt.Fprintln(os.Stderr)

	importer := &orchestrate.Importer{
		Client: cl,
		OnProgress: func(event orchestrate.ImportEvent) {
			switch event.Status {
			case orchestrate.ImportCreated:
				if importDryRun {
					renderDryRunWorkspace(event)
				} else {
					fmt.Fprintf(os.Stderr, "  %s  %s (%d panes)\n",
						greenStyle.Render("OK"),
						padTitle(event.Title),
						len(event.Panes))
				}
			case orchestrate.ImportSkipped:
				fmt.Fprintf(os.Stderr, "  %s  %s %s\n",
					dimStyle.Render("SKIP"),
					padTitle(event.Title),
					dimStyle.Render("(already exists)"))
			case orchestrate.ImportFailed:
				fmt.Fprintf(os.Stderr, "  %s  %s: %v\n",
					yellowStyle.Render("FAIL"),
					event.Title,
					event.Err)
			case orchestrate.ImportWarn:
				fmt.Fprintf(os.Stderr, "  %s  %s\n",
					yellowStyle.Render("WARN"),
					event.Warn)
			}
		},
	}

	result, err := importer.ImportFromMD(wf, importDryRun)
	if err != nil {
		return err
	}

	if importDryRun {
		fmt.Fprintf(os.Stderr, "%s\n\n",
			greenStyle.Render(fmt.Sprintf("✅ Would create %d workspaces", result.Created)))
	} else {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "%s\n\n",
			greenStyle.Render(fmt.Sprintf("✅ Import complete: %d created, %d skipped", result.Created, result.Skipped)))
	}
	return nil
}

// renderDryRunWorkspace prints the detailed dry-run preview for a single workspace.
func renderDryRunWorkspace(event orchestrate.ImportEvent) {
	pin := ""
	if event.Pin {
		pin = " 📌"
	}

	// Workspace header
	fmt.Fprintf(os.Stderr, "  %s %s%s\n",
		greenStyle.Render("CREATE"),
		greenStyle.Render(padTitle(event.Title)),
		pin)

	// Details
	fmt.Fprintf(os.Stderr, "         %s  %s  %s\n",
		dimStyle.Render("cwd="+event.ExpandedPath),
		cyanStyle.Render("template="+event.Template),
		dimStyle.Render(fmt.Sprintf("panes=%d", len(event.Panes))))

	// Pane tree
	for j, pane := range event.Panes {
		isLast := j == len(event.Panes)-1
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
}
