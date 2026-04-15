package cmd

import "github.com/spf13/cobra"

var templateCmd = &cobra.Command{
	Use:     "template",
	Short:   "Browse and use the built-in template gallery",
	Long:    "Discover, preview, and use pre-built workspace templates for common developer workflows.",
	Aliases: []string{"tpl"},
}

func init() {
	rootCmd.AddCommand(templateCmd)
}
