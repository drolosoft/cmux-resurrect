package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/drolosoft/cmux-resurrect/internal/orchestrate"
	"github.com/spf13/cobra"
)

var restoreDryRun bool

var restoreCmd = &cobra.Command{
	Use:   "restore <name>",
	Short: "Restore a saved cmux layout",
	Long:  "Recreates workspaces, splits, and sends commands from a saved layout.\n\nYou will be asked whether to replace your current workspaces or add to them.",
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

	// Determine restore mode.
	mode := orchestrate.RestoreModeReplace
	if !restoreDryRun {
		mode, err = askRestoreMode()
		if err != nil {
			return err
		}
	}

	if restoreDryRun {
		fmt.Fprintf(os.Stderr, "Dry-run restore of %q:\n\n", name)
	} else {
		action := "Replacing"
		if mode == orchestrate.RestoreModeAdd {
			action = "Adding to"
		}
		fmt.Fprintf(os.Stderr, "%s current workspaces with layout %q...\n", action, name)
	}

	result, err := restorer.Restore(name, restoreDryRun, mode)
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

	if result.WorkspacesClosed > 0 {
		fmt.Fprintf(os.Stderr, "Closed %d existing workspaces\n", result.WorkspacesClosed)
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

// askRestoreMode prompts the user to choose between replacing or adding workspaces.
func askRestoreMode() (orchestrate.RestoreMode, error) {
	fmt.Fprintf(os.Stderr, "\nHow do you want to restore?\n")
	fmt.Fprintf(os.Stderr, "  [r] Replace — close all current workspaces, then restore\n")
	fmt.Fprintf(os.Stderr, "  [a] Add     — keep current workspaces, add restored ones\n")
	fmt.Fprintf(os.Stderr, "\nChoice [r/a]: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return orchestrate.RestoreModeReplace, fmt.Errorf("read input: %w", err)
	}

	switch strings.TrimSpace(strings.ToLower(input)) {
	case "a", "add":
		return orchestrate.RestoreModeAdd, nil
	case "r", "replace", "":
		return orchestrate.RestoreModeReplace, nil
	default:
		return orchestrate.RestoreModeReplace, fmt.Errorf("invalid choice %q — use 'r' or 'a'", strings.TrimSpace(input))
	}
}
