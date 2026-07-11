package cmd

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/drolosoft/cmux-resurrect/internal/config"
	"github.com/drolosoft/cmux-resurrect/internal/tui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:     "tui",
	Aliases: []string{"shell"},
	Short:   "Interactive shell",
	Long:    "Launch the crex interactive shell for browsing layouts, templates, and live state.",
	Args:    cobra.NoArgs,
	RunE:    runTUI,
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}

func runTUI(cmd *cobra.Command, args []string) error {
	store, err := newStore()
	if err != nil {
		return fmt.Errorf("init store: %w", err)
	}
	cl := newClient()
	backend := cachedBackend

	m := tui.NewShellModel(store, cl, backend, cfg.WorkspaceFile, Version)
	style := cfg.BannerStyle
	if style == "" {
		style = "flame"
	}
	m.SetBannerStyle(style)
	m.SetRestoreMode(cfg.RestoreMode)
	m.SetAutoAccept(cfg.AutoAccept)
	m.OnSettingChanged = func(key, value string) {
		if key == "restore_mode" {
			cfg.RestoreMode = value
		}
		_ = config.Save(config.DefaultConfigPath(), cfg)
	}
	m.BannerCycle = func(explicit string) (string, string, error) {
		next := explicit
		if next == "" {
			next = cycleBannerStyle(cfg.BannerStyle)
		}
		cfg.BannerStyle = next
		path := cfgFile
		if path == "" {
			path = config.DefaultConfigPath()
		}
		if err := config.Save(path, cfg); err != nil {
			return next, "", err
		}
		return next, banner(), nil
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}
	if sm, ok := finalModel.(*tui.ShellModel); ok && sm.ByeMsg() != "" {
		fmt.Println("\n  " + sm.ByeMsg())
		fmt.Println()
	}
	return nil
}
