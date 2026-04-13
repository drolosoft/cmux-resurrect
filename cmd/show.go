package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var showRaw bool

var showCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show details of a saved layout",
	Args:  cobra.ExactArgs(1),
	RunE:  runShow,
}

func init() {
	showCmd.Flags().BoolVar(&showRaw, "raw", false, "show raw TOML content")
	showCmd.ValidArgsFunction = completeLayoutNames
	rootCmd.AddCommand(showCmd)
}

func runShow(cmd *cobra.Command, args []string) error {
	name := args[0]
	store, err := newStore()
	if err != nil {
		return err
	}

	if showRaw {
		data, err := os.ReadFile(store.Path(name))
		if err != nil {
			return fmt.Errorf("layout %q not found", name)
		}
		fmt.Print(string(data))
		return nil
	}

	layout, err := store.Load(name)
	if err != nil {
		return err
	}

	// Header
	fmt.Fprintf(os.Stderr, "\n%s\n", headingStyle.Render("📦 "+layout.Name))
	if layout.Description != "" {
		fmt.Fprintf(os.Stderr, "   %s\n", dimStyle.Render(layout.Description))
	}
	saved := layout.SavedAt.Local().Format("Jan 02, 2006 15:04")
	fmt.Fprintf(os.Stderr, "   %s\n", dimStyle.Render(fmt.Sprintf("Saved %s · %d workspaces", saved, len(layout.Workspaces))))
	fmt.Fprintln(os.Stderr)

	for _, ws := range layout.Workspaces {
		// Workspace title with badges
		title := greenStyle.Render(ws.Title)
		badges := ""
		if ws.Pinned {
			badges += " " + cyanStyle.Render("📌")
		}
		if ws.Active {
			badges += " " + yellowStyle.Render("◀ active")
		}
		fmt.Fprintf(os.Stderr, "   %s%s\n", title, badges)

		// CWD
		fmt.Fprintf(os.Stderr, "   %s\n", dimStyle.Render("cwd "+ws.CWD))

		// Panes as a tree
		for i, p := range ws.Panes {
			isLast := i == len(ws.Panes)-1
			prefix := "├──"
			if isLast {
				prefix = "└──"
			}
			prefix = dimStyle.Render(prefix)

			// Build pane description
			var desc string
			if p.Split != "" {
				desc = magentaStyle.Render("→"+p.Split) + " "
			}
			if p.Command != "" {
				cmd := p.Command
				if len(cmd) > 50 {
					cmd = cmd[:47] + "..."
				}
				desc += cyanStyle.Render(cmd)
			} else {
				desc += dimStyle.Render("shell")
			}
			if p.Focus {
				desc += " " + yellowStyle.Render("★")
			}

			fmt.Fprintf(os.Stderr, "   %s %s\n", prefix, desc)
		}
		fmt.Fprintln(os.Stderr)
	}

	return nil
}
