package cmd

import (
	"fmt"
	"os"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/config"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
	"github.com/spf13/cobra"
)

var (
	cfgFile    string
	layoutsDir string
	cfg        *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "cmres",
	Short: "Resurrect your cmux sessions",
	Long:  "cmres saves/restores cmux layouts and manages workspaces from an Obsidian-friendly markdown file.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(styledHelp())
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default ~/.config/cmres/config.toml)")
	rootCmd.PersistentFlags().StringVar(&layoutsDir, "layouts-dir", "", "layouts directory (default ~/.config/cmres/layouts)")

	// Override the default help command to use our styled output for the root.
	defaultHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if cmd.Name() == rootCmd.Name() && cmd.Parent() == nil {
			fmt.Print(styledHelp())
		} else {
			// For subcommands, use cobra's default help.
			defaultHelp(cmd, args)
		}
	})
}

func initConfig() {
	path := cfgFile
	if path == "" {
		path = config.DefaultConfigPath()
	}
	var err error
	cfg, err = config.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: config load: %v\n", err)
		cfg = config.DefaultConfig()
	}
	if layoutsDir != "" {
		cfg.LayoutsDir = layoutsDir
	}
}

func newClient() client.CmuxClient {
	return client.NewCLIClient()
}

func newStore() (persist.Store, error) {
	return persist.NewFileStore(cfg.LayoutsDir)
}
