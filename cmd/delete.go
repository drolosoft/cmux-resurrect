package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

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
	deleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "skip confirmation prompt")
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

	if !deleteForce {
		fmt.Fprintf(os.Stderr, "Delete layout %q? [y/N] ", name)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Fprintln(os.Stderr, "Cancelled.")
			return nil
		}
	}

	if err := store.Delete(name); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Deleted layout %q\n", name)
	return nil
}
