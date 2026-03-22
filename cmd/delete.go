package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var deleteForce bool

var deleteCmd = &cobra.Command{
	Use:     "delete <name>",
	Short:   "Delete a saved layout",
	Aliases: []string{"rm"},
	Args:    cobra.ExactArgs(1),
	RunE:    runDelete,
}

func init() {
	deleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "skip confirmation")
	rootCmd.AddCommand(deleteCmd)
}

func runDelete(cmd *cobra.Command, args []string) error {
	name := args[0]
	store, err := newStore()
	if err != nil {
		return err
	}

	if !store.Exists(name) {
		return fmt.Errorf("layout %q not found", name)
	}

	if err := store.Delete(name); err != nil {
		return err
	}
	fmt.Printf("Deleted layout %q\n", name)
	return nil
}
