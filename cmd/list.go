package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Short:   "List saved layouts",
	Args:    cobra.NoArgs,
	RunE:    runList,
	Aliases: []string{"ls"},
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	store, err := newStore()
	if err != nil {
		return err
	}

	metas, err := store.List()
	if err != nil {
		return err
	}

	if len(metas) == 0 {
		fmt.Println("No saved layouts. Use 'crex save <name>' to save one.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tWORKSPACES\tSAVED\tDESCRIPTION")
	for _, m := range metas {
		desc := m.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\n",
			m.Name,
			m.WorkspaceCount,
			m.SavedAt.Local().Format("2006-01-02 15:04"),
			desc,
		)
	}
	w.Flush()
	return nil
}
