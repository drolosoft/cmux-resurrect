package cmd

import (
	"fmt"
	"os"

	"github.com/drolosoft/cmux-resurrect/internal/mdfile"
	"github.com/spf13/cobra"
)

var wsToggleCmd = &cobra.Command{
	Use:   "toggle <name>",
	Short: "Toggle a workspace entry between enabled and disabled",
	Args:  cobra.ExactArgs(1),
	RunE:  runWorkspaceToggle,
}

func init() {
	workspaceCmd.AddCommand(wsToggleCmd)
}

func runWorkspaceToggle(cmd *cobra.Command, args []string) error {
	name := args[0]
	wsFile := cfg.WorkspaceFile

	newState, err := mdfile.ToggleProject(wsFile, name)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr)
	if newState {
		fmt.Fprintf(os.Stderr, "  %s %s\n", greenStyle.Render("✅"), greenStyle.Render(name+" enabled"))
	} else {
		fmt.Fprintf(os.Stderr, "  %s %s\n", dimStyle.Render("⬜"), dimStyle.Render(name+" disabled"))
	}
	fmt.Fprintln(os.Stderr)
	return nil
}
