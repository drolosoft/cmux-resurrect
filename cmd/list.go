package cmd

import (
	"fmt"
	"os"
	"strings"

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
		fmt.Println(dimStyle.Render("  No saved layouts. Use 'crex save <name>' to create one."))
		return nil
	}

	fmt.Fprintln(os.Stderr, headingStyle.Render("💾 Saved Layouts"))
	fmt.Fprintln(os.Stderr)

	for _, m := range metas {
		name := greenStyle.Render(fmt.Sprintf("%-16s", m.Name))
		ws := cyanStyle.Render(fmt.Sprintf("%d %s", m.WorkspaceCount, unitName(m.WorkspaceCount)))
		date := dimStyle.Render(m.SavedAt.Local().Format("Jan 02 15:04"))

		var parts []string
		parts = append(parts, ws, date)
		if m.Description != "" {
			desc := m.Description
			if len(desc) > 40 {
				desc = desc[:37] + "..."
			}
			parts = append(parts, desc)
		}

		fmt.Fprintf(os.Stderr, "  %s %s\n", name, strings.Join(parts, dimStyle.Render(" · ")))
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, dimStyle.Render(fmt.Sprintf("  %d layout(s)", len(metas))))
	return nil
}
