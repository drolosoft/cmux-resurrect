package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/drolosoft/cmux-resurrect/internal/orchestrate"
)

var restoreDryRun bool

var restoreCmd = &cobra.Command{
	Use:   "restore <name>",
	Short: "Restore a saved cmux layout",
	Long:  "Recreates workspaces, splits, and sends commands from a saved layout.",
	Args:  cobra.ExactArgs(1),
	RunE:  runRestore,
}

func init() {
	restoreCmd.Flags().BoolVar(&restoreDryRun, "dry-run", false, "show commands without executing")
	rootCmd.AddCommand(restoreCmd)
}

func runRestore(cmd *cobra.Command, args []string) error {
	name := args[0]

	cl := newClient()
	store, err := newStore()
	if err != nil {
		return err
	}

	restorer := &orchestrate.Restorer{Client: cl, Store: store}

	if restoreDryRun {
		fmt.Fprintf(os.Stderr, "Dry-run restore of %q:\n\n", name)
	} else {
		fmt.Fprintf(os.Stderr, "Restoring layout %q...\n", name)
	}

	result, err := restorer.Restore(name, restoreDryRun)
	if err != nil {
		return err
	}

	if restoreDryRun {
		for _, c := range result.Commands {
			fmt.Println(c)
		}
		fmt.Fprintf(os.Stderr, "\n%d commands for %d workspaces\n", len(result.Commands), result.WorkspacesTotal)
		return nil
	}

	fmt.Fprintf(os.Stderr, "Restored %d/%d workspaces\n", result.WorkspacesOK, result.WorkspacesTotal)
	if len(result.Errors) > 0 {
		fmt.Fprintf(os.Stderr, "Errors:\n")
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "  - %s\n", e)
		}
	}
	return nil
}
