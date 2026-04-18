package cmd

import (
	"fmt"
	"os"

	"github.com/drolosoft/cmux-resurrect/internal/orchestrate"
	"github.com/spf13/cobra"
)

var saveDescription string

var saveCmd = &cobra.Command{
	Use:   "save [name]",
	Short: "Save current layout",
	Long:  "Captures all tabs, pane arrangements, CWDs, and pinned state from the running terminal.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runSave,
}

func init() {
	saveCmd.Flags().StringVarP(&saveDescription, "description", "d", "", "layout description")
	saveCmd.ValidArgsFunction = completeLayoutNames
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

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "%s %s\n", yellowStyle.Render("💾 Saving layout"), greenStyle.Render(name))

	layout, err := saver.Save(name, saveDescription)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr)
	for _, ws := range layout.Workspaces {
		pin := ""
		if ws.Pinned {
			pin = " 📌"
		}
		panes := ""
		if len(ws.Panes) > 1 {
			panes = fmt.Sprintf(" (%d panes)", len(ws.Panes))
		}
		fmt.Fprintf(os.Stderr, "  %s  %s%s%s\n",
			greenStyle.Render("OK"),
			padTitle(ws.Title),
			dimStyle.Render(panes),
			pin)
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "%s\n",
		greenStyle.Render(fmt.Sprintf("✅ Saved %d %s to %s", len(layout.Workspaces), unitName(len(layout.Workspaces)), store.Path(name))))
	fmt.Fprintln(os.Stderr)
	return nil
}
