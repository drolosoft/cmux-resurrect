package cmd

import (
	"github.com/spf13/cobra"
)

var workspaceCmd = &cobra.Command{
	Use:     "workspace",
	Short:   "Manage workspace entries in the workspace file",
	Long:    "Add, remove, list, and toggle workspace entries in the Obsidian-friendly workspace markdown file.",
	Aliases: []string{"ws"},
}

func init() {
	rootCmd.AddCommand(workspaceCmd)
}
