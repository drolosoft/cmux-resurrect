package cmd

import (
	"fmt"
	"os"

	"github.com/drolosoft/cmux-resurrect/internal/mdfile"
	"github.com/drolosoft/cmux-resurrect/internal/orchestrate"
	"github.com/spf13/cobra"
)

var exportToMDCmd = &cobra.Command{
	Use:   "export-to-md",
	Short: "Export live cmux state to a Workspace Blueprint",
	Long:  "Captures current cmux workspaces and writes them to a Workspace Blueprint (.md) with default templates.",
	RunE:  runExportToMD,
}

func init() {
	rootCmd.AddCommand(exportToMDCmd)
}

func runExportToMD(cmd *cobra.Command, args []string) error {
	cl := newClient()
	wsFile := cfg.WorkspaceFile

	exporter := &orchestrate.Exporter{Client: cl}
	if err := exporter.ExportToMD(wsFile); err != nil {
		return err
	}

	// Read back for the report.
	wf, err := mdfile.Parse(wsFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not re-read exported file: %v\n", err)
		return nil
	}

	fmt.Fprintf(os.Stderr, "Exported %d workspaces to %s\n", len(wf.Projects), wsFile)
	for _, p := range wf.Projects {
		check := "[x]"
		if !p.Enabled {
			check = "[ ]"
		}
		fmt.Fprintf(os.Stderr, "  %s %s %s  (%s)  %s\n", check, p.Icon, p.Name, p.Template, p.Path)
	}
	return nil
}
