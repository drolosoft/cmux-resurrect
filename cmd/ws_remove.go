package cmd

import (
	"fmt"
	"os"

	"github.com/drolosoft/cmux-resurrect/internal/mdfile"
	"github.com/spf13/cobra"
)

var wsRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Short:   "Remove a workspace entry from the Workspace Blueprint",
	Aliases: []string{"rm"},
	Args:    cobra.ExactArgs(1),
	RunE:    runWorkspaceRemove,
}

func init() {
	wsRemoveCmd.ValidArgsFunction = completeWorkspaceNames
	workspaceCmd.AddCommand(wsRemoveCmd)
}

func runWorkspaceRemove(cmd *cobra.Command, args []string) error {
	name := args[0]
	wsFile := cfg.WorkspaceFile

	if err := mdfile.RemoveProject(wsFile, name); err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "%s\n\n",
		greenStyle.Render(fmt.Sprintf("✅ Removed %q from Workspace Blueprint", name)))
	return nil
}
