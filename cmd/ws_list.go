package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/drolosoft/cmux-resurrect/internal/mdfile"
	"github.com/spf13/cobra"
)

var wsListAll bool

var wsListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List workspace entries from the Workspace Blueprint",
	Aliases: []string{"ls"},
	Args:    cobra.NoArgs,
	RunE:    runWorkspaceList,
}

func init() {
	wsListCmd.Flags().BoolVarP(&wsListAll, "all", "a", false, "show disabled workspaces too")
	workspaceCmd.AddCommand(wsListCmd)
}

func runWorkspaceList(cmd *cobra.Command, args []string) error {
	wsFile := cfg.WorkspaceFile
	wf, err := mdfile.Parse(wsFile)
	if err != nil {
		return fmt.Errorf("read Workspace Blueprint: %w", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "  \tICON\tNAME\tTEMPLATE\tPIN\tPATH")

	for _, p := range wf.Projects {
		if !wsListAll && !p.Enabled {
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
