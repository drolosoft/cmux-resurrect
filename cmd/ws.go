package cmd

import (
	"github.com/spf13/cobra"
)

var workspaceCmd = &cobra.Command{
	Use:     "workspace",
	Short:   "Manage workspace entries in the Workspace Blueprint",
	Long:    "Add, remove, list, and toggle workspace entries in the Workspace Blueprint (.md).",
	Aliases: []string{"ws"},
}

func init() {
	rootCmd.AddCommand(workspaceCmd)
}
