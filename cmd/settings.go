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

var settingsBackendCmd = &cobra.Command{
	Use:   "backend",
	Short: "Get, set, or list the default backend",
}

var settingsBackendGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show the configured default backend",
	Args:  cobra.NoArgs,
	RunE:  runBackendGet,
}

var settingsBackendSetCmd = &cobra.Command{
	Use:   "set <auto|cmux|ghostty>",
	Short: "Set the default backend (auto clears it)",
	Args:  cobra.ExactArgs(1),
	RunE:  runBackendSet,
}

var settingsBackendListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available backend choices",
	Args:  cobra.NoArgs,
	RunE:  runBackendList,
}

func init() {
	settingsRestoreModeCmd.AddCommand(settingsRestoreModeGetCmd, settingsRestoreModeSetCmd, settingsRestoreModeListCmd)
	settingsBackendCmd.AddCommand(settingsBackendGetCmd, settingsBackendSetCmd, settingsBackendListCmd)
	settingsCmd.AddCommand(settingsRestoreModeCmd, settingsBackendCmd)
	rootCmd.AddCommand(settingsCmd)
}

func runBackendGet(cmd *cobra.Command, args []string) error {
	b := cfg.Backend
	if b == "" {
		b = "auto (detect at runtime)"
	}
	o := newWF(cmd.OutOrStdout())
	o.f("  Default backend: %s\n", greenStyle.Render(b))
	return nil
}

func runBackendSet(cmd *cobra.Command, args []string) error {
	choice := strings.ToLower(args[0])
	switch choice {
	case "auto", "":
		cfg.Backend = "" // clear → auto-detect
		choice = "auto"
	case "cmux", "ghostty", "cmux-applescript":
		cfg.Backend = choice
	default:
		return fmt.Errorf("invalid backend %q: use auto, cmux, or ghostty", choice)
	}

	path := cfgFile
	if path == "" {
		path = config.DefaultConfigPath()
	}
	if err := config.Save(path, cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	o := newWF(cmd.OutOrStdout())
	o.f("  %s Default backend set to %s\n", greenStyle.Render("✓"), greenStyle.Render(choice))
	return nil
}

func runBackendList(cmd *cobra.Command, args []string) error {
	o := newWF(cmd.OutOrStdout())
	o.ln("  Available backends:")
	o.f("    %s     detect at runtime — cmux if its socket answers, else Ghostty (default)\n", greenStyle.Render("auto"))
	o.f("    %s     always use cmux\n", greenStyle.Render("cmux"))
	o.f("    %s  always use Ghostty\n", greenStyle.Render("ghostty"))
	o.ln("")
	o.ln(dimStyle.Render("  The CREX_BACKEND env var overrides this per-invocation."))
	return nil
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
