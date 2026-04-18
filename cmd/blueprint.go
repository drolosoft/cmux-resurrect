package cmd

import "github.com/spf13/cobra"

var blueprintCmd = &cobra.Command{
	Use:     "blueprint",
	Short:   "Manage entries in the Blueprint",
	Long:    "Add, remove, list, and toggle entries in the Blueprint (.md).",
	Aliases: []string{"bp"},
}

var workspaceLegacyCmd = &cobra.Command{
	Use:    "workspace",
	Short:  "Manage entries in the Blueprint",
	Long:   "Add, remove, list, and toggle entries in the Blueprint (.md).",
	Hidden: true,
	Aliases: []string{"ws"},
}

func init() {
	rootCmd.AddCommand(blueprintCmd)
	rootCmd.AddCommand(workspaceLegacyCmd)
}
