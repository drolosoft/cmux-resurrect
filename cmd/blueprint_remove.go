package cmd

import (
	"fmt"
	"os"

	"github.com/drolosoft/cmux-resurrect/internal/mdfile"
	"github.com/spf13/cobra"
)

var blueprintRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Short:   "Remove an entry from the Blueprint",
	Aliases: []string{"rm"},
	Args:    cobra.ExactArgs(1),
	RunE:    runBlueprintRemove,
}

func init() {
	blueprintRemoveCmd.ValidArgsFunction = completeBlueprintNames
	blueprintCmd.AddCommand(blueprintRemoveCmd)

	// Legacy subcommand under workspaceLegacyCmd for backward compatibility.
	legacyRemove := &cobra.Command{
		Use:  "remove <name>",
		Args: cobra.ExactArgs(1),
		RunE: runBlueprintRemove,
	}
	legacyRemove.ValidArgsFunction = completeBlueprintNames
	workspaceLegacyCmd.AddCommand(legacyRemove)
}

func runBlueprintRemove(cmd *cobra.Command, args []string) error {
	name := args[0]
	wsFile := cfg.WorkspaceFile

	if err := mdfile.RemoveProject(wsFile, name); err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "%s\n\n",
		greenStyle.Render(fmt.Sprintf("✅ Removed %q from Blueprint", name)))
	return nil
}
