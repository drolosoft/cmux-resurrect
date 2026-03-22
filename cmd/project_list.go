package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/juanatsap/cmux-resurrect/internal/mdfile"
)

var projectListAll bool

var projectListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List projects from the workspace file",
	Aliases: []string{"ls"},
	Args:    cobra.NoArgs,
	RunE:    runProjectList,
}

func init() {
	projectListCmd.Flags().BoolVarP(&projectListAll, "all", "a", false, "show disabled projects too")
	projectCmd.AddCommand(projectListCmd)
}

func runProjectList(cmd *cobra.Command, args []string) error {
	wsFile := cfg.WorkspaceFile
	wf, err := mdfile.Parse(wsFile)
	if err != nil {
		return fmt.Errorf("read workspace file: %w", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "  \tICON\tNAME\tTEMPLATE\tPIN\tPATH")

	for _, p := range wf.Projects {
		if !projectListAll && !p.Enabled {
			continue
		}
		check := "[x]"
		if !p.Enabled {
			check = "[ ]"
		}
		pin := "yes"
		if !p.Pin {
			pin = "no"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			check, p.Icon, p.Name, p.Template, pin, p.Path)
	}
	w.Flush()
	return nil
}
