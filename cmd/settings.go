package cmd

import (
	"fmt"
	"strings"

	"github.com/drolosoft/cmux-resurrect/internal/config"
	"github.com/spf13/cobra"
)

var settingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "View and change crex settings",
	Long:  "Manage persistent crex settings stored in the config file.\n\nMirrors the TUI 'settings' command so both surfaces behave identically.",
}

var settingsRestoreModeCmd = &cobra.Command{
	Use:   "restore-mode",
	Short: "Get, set, or list the default restore mode",
}

var settingsRestoreModeGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show the current restore mode",
	Args:  cobra.NoArgs,
	RunE:  runRestoreModeGet,
}

var settingsRestoreModeSetCmd = &cobra.Command{
	Use:   "set <ask|replace|add>",
	Short: "Set the default restore mode",
	Args:  cobra.ExactArgs(1),
	RunE:  runRestoreModeSet,
}

var settingsRestoreModeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available restore modes",
	Args:  cobra.NoArgs,
	RunE:  runRestoreModeList,
}

func init() {
	settingsRestoreModeCmd.AddCommand(settingsRestoreModeGetCmd, settingsRestoreModeSetCmd, settingsRestoreModeListCmd)
	settingsCmd.AddCommand(settingsRestoreModeCmd)
	rootCmd.AddCommand(settingsCmd)
}

func runRestoreModeGet(cmd *cobra.Command, args []string) error {
	mode := cfg.RestoreMode
	if mode == "" {
		mode = "ask"
	}
	o := newWF(cmd.OutOrStdout())
	o.f("  Current restore mode: %s\n", greenStyle.Render(mode))
	return nil
}

func runRestoreModeSet(cmd *cobra.Command, args []string) error {
	mode := strings.ToLower(args[0])
	switch mode {
	case "ask", "replace", "add":
		// valid
	default:
		return fmt.Errorf("invalid mode %q: use ask, replace, or add", mode)
	}

	cfg.RestoreMode = mode
	path := cfgFile
	if path == "" {
		path = config.DefaultConfigPath()
	}
	if err := config.Save(path, cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	o := newWF(cmd.OutOrStdout())
	o.f("  %s Restore mode set to %s\n", greenStyle.Render("✓"), greenStyle.Render(mode))
	return nil
}

func runRestoreModeList(cmd *cobra.Command, args []string) error {
	o := newWF(cmd.OutOrStdout())
	o.ln("  Available restore modes:")
	o.f("    %s  prompt for replace/add each time (default)\n", greenStyle.Render("ask"))
	o.f("    %s  always replace existing workspaces\n", greenStyle.Render("replace"))
	o.f("    %s  always add alongside existing workspaces\n", greenStyle.Render("add"))
	return nil
}
