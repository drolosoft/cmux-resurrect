package cmd

import (
	"fmt"
	"os"

	"github.com/drolosoft/cmux-resurrect/internal/mdfile"
	"github.com/drolosoft/cmux-resurrect/internal/orchestrate"
	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export live cmux state to the workspace file",
	Long:  "Captures current cmux workspaces and writes them to the workspace MD file with default templates.",
	RunE:  runExport,
}

func init() {
	rootCmd.AddCommand(exportCmd)
}

func runExport(cmd *cobra.Command, args []string) error {
	cl := newClient()
	wsFile := cfg.WorkspaceFile

	exporter := &orchestrate.Exporter{Client: cl}
	if err := exporter.ExportToMD(wsFile); err != nil {
		return err
	}

	// Read back for the report.
	wf, err := mdfile.Parse(wsFile)
	if err != nil {
		return nil // export succeeded, just can't report
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
