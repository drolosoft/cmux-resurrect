package cmd

import (
	"fmt"
	"os"

	"github.com/drolosoft/cmux-resurrect/internal/mdfile"
	"github.com/spf13/cobra"
)

var projectToggleCmd = &cobra.Command{
	Use:   "toggle <name>",
	Short: "Toggle a project between enabled and disabled",
	Args:  cobra.ExactArgs(1),
	RunE:  runProjectToggle,
}

func init() {
	projectCmd.AddCommand(projectToggleCmd)
}

func runProjectToggle(cmd *cobra.Command, args []string) error {
	name := args[0]
	wsFile := cfg.WorkspaceFile

	newState, err := mdfile.ToggleProject(wsFile, name)
	if err != nil {
		return err
	}

	state := "disabled"
	if newState {
		state = "enabled"
	}
	fmt.Fprintf(os.Stderr, "%s is now %s\n", name, state)
	return nil
}
