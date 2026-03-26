package cmd

import (
	"fmt"
	"os"

	"github.com/drolosoft/cmux-resurrect/internal/mdfile"
	"github.com/spf13/cobra"
)

var wsRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Short:   "Remove a workspace entry from the workspace file",
	Aliases: []string{"rm"},
	Args:    cobra.ExactArgs(1),
	RunE:    runProjectRemove,
}

func init() {
	workspaceCmd.AddCommand(wsRemoveCmd)
}

func runProjectRemove(cmd *cobra.Command, args []string) error {
	name := args[0]
	wsFile := cfg.WorkspaceFile

	if err := mdfile.RemoveProject(wsFile, name); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Removed %q from %s\n", name, wsFile)
	return nil
}
