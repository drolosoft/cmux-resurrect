package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/txeo/cmux-persist/internal/client"
	"github.com/txeo/cmux-persist/internal/config"
	"github.com/txeo/cmux-persist/internal/persist"
)

var (
	cfgFile    string
	layoutsDir string
	cfg        *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "cmux-persist",
	Short: "Save and restore cmux layouts",
	Long:  "cmux-persist captures your cmux workspace layout (splits, CWDs, commands) and restores it after a restart.",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default ~/.config/cmux-persist/config.toml)")
	rootCmd.PersistentFlags().StringVar(&layoutsDir, "layouts-dir", "", "layouts directory (default ~/.config/cmux-persist/layouts)")
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
