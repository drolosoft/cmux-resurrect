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

	fmt.Printf("Layout: %s\n", layout.Name)
	if layout.Description != "" {
		fmt.Printf("Description: %s\n", layout.Description)
	}
	fmt.Printf("Saved: %s\n", layout.SavedAt.Local().Format("2006-01-02 15:04:05"))
	fmt.Printf("Workspaces: %d\n\n", len(layout.Workspaces))

	for _, ws := range layout.Workspaces {
		pin := " "
		if ws.Pinned {
			pin = "pin"
		}
		active := ""
		if ws.Active {
			active = " [active]"
		}
		fmt.Printf("  [%d] %s  (%s)%s\n", ws.Index, ws.Title, pin, active)
		fmt.Printf("      cwd: %s\n", ws.CWD)
		for i, p := range ws.Panes {
			split := ""
			if p.Split != "" {
				split = fmt.Sprintf(" split=%s", p.Split)
			}
			command := ""
			if p.Command != "" {
				command = fmt.Sprintf(" cmd=%q", p.Command)
			}
			focus := ""
			if p.Focus {
				focus = " *"
			}
			fmt.Printf("      pane %d: %s%s%s%s\n", i, p.Type, split, command, focus)
		}
	}
	return nil
}
