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
	cfgFile       string
	layoutsDir    string
	workspaceFile string
	cfg           *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "crex",
	Short: "Save, restore, and template your terminal workspaces",
	Long:  "crex saves, restores, and templates your terminal workspaces.", // updated by updateRootLong()
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(banner())
		fmt.Print(styledHelp())
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default ~/.config/crex/config.toml)")
	rootCmd.PersistentFlags().StringVar(&layoutsDir, "layouts-dir", "", "layouts directory (default ~/.config/crex/layouts)")
	rootCmd.PersistentFlags().StringVar(&workspaceFile, "workspace-file", "", "Workspace Blueprint path (default ~/.config/crex/workspaces.md)")

	// Shell completion hints for persistent flags.
	_ = rootCmd.MarkPersistentFlagFilename("config", "toml")
	_ = rootCmd.MarkPersistentFlagDirname("layouts-dir")
	_ = rootCmd.MarkPersistentFlagFilename("workspace-file", "md")

	updateRootLong()

	// Override the default help command to use our styled output for the root.
	defaultHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if cmd.Name() == rootCmd.Name() && cmd.Parent() == nil {
			fmt.Print(banner())
			fmt.Print(styledHelp())
		} else {
			// For subcommands, use cobra's default help.
			defaultHelp(cmd, args)
		}
	})
}

func updateRootLong() {
	if isCmuxBranding() {
		rootCmd.Long = "crex (cmux-resurrect) saves, restores, and templates your terminal workspaces.\nWorks with cmux and Ghostty. Inspired by tmux-resurrect."
	} else {
		rootCmd.Long = "crex saves, restores, and templates your terminal workspaces.\nWorks with Ghostty. Inspired by tmux-resurrect."
	}
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
	if workspaceFile != "" {
		cfg.WorkspaceFile = config.ExpandHome(workspaceFile)
	}
}

func newClient() client.Backend {
	detected := client.Detect()
	switch detected {
	case client.BackendGhostty:
		return client.NewGhosttyClient()
	default:
		return client.NewCLIClient()
	}
}

func newStore() (persist.Store, error) {
	return persist.NewFileStore(cfg.LayoutsDir)
}
