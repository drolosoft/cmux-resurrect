package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/drolosoft/cmux-resurrect/internal/orchestrate"
)

var saveDescription string

var saveCmd = &cobra.Command{
	Use:   "save [name]",
	Short: "Save current cmux layout",
	Long:  "Captures all workspaces, splits, CWDs, and pinned state from the running cmux instance.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runSave,
}

func init() {
	saveCmd.Flags().StringVarP(&saveDescription, "description", "d", "", "layout description")
	rootCmd.AddCommand(saveCmd)
}

func runSave(cmd *cobra.Command, args []string) error {
	name := "default"
	if len(args) > 0 {
		name = args[0]
	}

	cl := newClient()
	store, err := newStore()
	if err != nil {
		return err
	}

	saver := &orchestrate.Saver{Client: cl, Store: store}

	fmt.Fprintf(os.Stderr, "Saving layout %q...\n", name)
	layout, err := saver.Save(name, saveDescription)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Saved %d workspaces to %s\n", len(layout.Workspaces), store.Path(name))
	for _, ws := range layout.Workspaces {
		pin := " "
		if ws.Pinned {
			pin = "📌"
		}
		panes := ""
		if len(ws.Panes) > 1 {
			panes = fmt.Sprintf(" (%d panes)", len(ws.Panes))
		}
		fmt.Fprintf(os.Stderr, "  %s %s  %s%s\n", pin, ws.Title, ws.CWD, panes)
	}
	return nil
}
