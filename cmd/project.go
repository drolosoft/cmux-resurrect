package cmd

import (
	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage projects in the workspace file",
	Long:  "Add, remove, list, and toggle projects in the Obsidian-friendly workspace markdown file.",
	Aliases: []string{"p"},
}

func init() {
	rootCmd.AddCommand(projectCmd)
}
